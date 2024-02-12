#!/bin/bash
set -e

# Function to check and install packages using the package manager
install_package() {
    local package_manager=$1
    local package_name=$3
    local test_cmd=$2

    # Check if the package is already installed
    if ! command -v $test_cmd &> /dev/null; then
        echo "Installing $package_name..."
        command $package_manager install -y $package_name
    else
        echo "$package_name is already installed."
    fi
}

# Auto-detect the OS and package manager
OS=$(uname -s | tr A-Z a-z)

INSTALL_DIR="/usr/local/bin"
SHARE_DIR="/usr/local/lib"

USE_SUDO=1

case $OS in
  linux)
    DOWNLOAD_URL="https://github.com/vyPal/CaffeineC/releases/latest/download/CaffeineC-Linux"
    if [ $(ps -ef|grep -c com.termux ) -gt 1 ]; then
      DOWNLOAD_URL="https://github.com/vyPal/CaffeineC/releases/latest/download/CaffeineC-Android"
      PACKAGE_MANAGER="apt"
      INSTALL_DIR="/data/data/com.termux/files/usr/bin"
      SHARE_DIR="/data/data/com.termux/files/usr/lib"
      USE_SUDO=0
    else
      source /etc/os-release
      case $ID in
        debian|ubuntu|linuxmint)
          PACKAGE_MANAGER="sudo apt-get"
          ;;

        fedora|rhel|centos)
          PACKAGE_MANAGER="sudo yum"
          ;;

        opensuse*)
          PACKAGE_MANAGER="sudo zypper"
          ;;

        arch)
          PACKAGE_MANAGER="sudo pacman"
          ;;

        *)
          echo "Unsupported Linux distribution."
          exit 1
          ;;
      esac
    fi
    ;;

  darwin)
    DOWNLOAD_URL="https://github.com/vyPal/CaffeineC/releases/latest/download/CaffeineC-macOS"
    PACKAGE_MANAGER="brew"
    ;;

  *)
    echo "Unsupported OS."
    exit 1
    ;;
esac

# Determine architecture and append the appropriate suffix
ARCH=$(uname -m)
if [ "$ARCH" = "x86_64" ]; then
  DOWNLOAD_URL="${DOWNLOAD_URL}-amd64"
elif [ "$ARCH" = "aarch64" ]; then
  DOWNLOAD_URL="${DOWNLOAD_URL}-arm64"
else
  echo "Unsupported architecture."
  exit 1
fi

# Check and install clang
install_package "$PACKAGE_MANAGER" "clang" "llvm"

# Create the directory if it doesn't exist
mkdir -p $INSTALL_DIR
mkdir -p $SHARE_DIR

# Download and install your compiler binary
latest_version=$(curl -sL https://github.com/vyPal/CaffeineC/releases/latest | grep -Eo 'tag/v[0-9\.]+' | head -n 1)

echo "Downloading CaffeineC version $latest_version..."
if [ $USE_SUDO -eq 1 ]; then
  sudo curl -sL $DOWNLOAD_URL -o $INSTALL_DIR/CaffeineC
  sudo chmod +x $INSTALL_DIR/CaffeineC
else
  curl -sL $DOWNLOAD_URL -o $INSTALL_DIR/CaffeineC
  chmod +x $INSTALL_DIR/CaffeineC
fi

# Determine the current shell
current_shell=$(basename "$SHELL")

# Set the autocomplete script URL based on the current shell
case $current_shell in
  bash)
    autocomplete_script_url="https://raw.githubusercontent.com/vyPal/CaffeineC/master/autocomplete/bash_autocomplete"
    shell_config_file="$HOME/.bashrc"
    ;;
  zsh)
    autocomplete_script_url="https://raw.githubusercontent.com/vyPal/CaffeineC/master/autocomplete/zsh_autocomplete"
    shell_config_file="$HOME/.zshrc"
    ;;
  *)
    echo "Unsupported shell for autocomplete. Skipping..."
    return
    ;;
esac

# If the shell is supported, continue with the rest of the script
if [ -n "$autocomplete_script_url" ]; then
  # Download the autocomplete script
  autocomplete_script_path="$SHARE_DIR/CaffeineC_autocomplete"
  echo "Downloading autocomplete script for $current_shell..."
  if [ $USE_SUDO -eq 1 ]; then
    sudo curl -sL $autocomplete_script_url -o $autocomplete_script_path
  else
    curl -sL $autocomplete_script_url -o $autocomplete_script_path
  fi

  touch $shell_config_file

  # Source the downloaded script
  if [ "$current_shell" = "zsh" ]; then
    zsh -c "source $shell_config_file && source $autocomplete_script_path"
  else
    source $autocomplete_script_path
  fi

  # Add the source command to the shell's configuration file to make it persistent
  if ! grep -q "source $autocomplete_script_path" $shell_config_file; then
    echo "source $autocomplete_script_path" >> $shell_config_file
  fi

  echo "Autocomplete script installed and sourced. It will be sourced automatically in new shell sessions."
fi

# Check if the install directory is in PATH
if [[ ":$PATH:" == *":$INSTALL_DIR:"* ]]; then
    echo "The CaffeineC compiler is now installed and in your PATH."
else
    echo "Add the following line to your shell configuration file (e.g., .bashrc, .zshrc, .config/fish/config.fish):"
    echo "export PATH=\$PATH:$INSTALL_DIR"
    echo "Then restart your terminal or run 'source <config-file>' to update the PATH."
fi

echo "Installation complete."
