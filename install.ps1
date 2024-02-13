# Check if Clang is installed
if (!(Get-Command clang -ErrorAction SilentlyContinue)) {
  # Check if Chocolatey is installed
  if (!(Get-Command choco -ErrorAction SilentlyContinue)) {
    # Install Chocolatey
    Set-ExecutionPolicy Bypass -Scope Process -Force; [System.Net.ServicePointManager]::SecurityProtocol = [System.Net.ServicePointManager]::SecurityProtocol -bor 3072; iex ((New-Object System.Net.WebClient).DownloadString('https://chocolatey.org/install.ps1'))
  }

  # Install Clang
  choco install llvm -y
}

# Download and install CaffeineC.exe
$latest_version = Invoke-RestMethod -Uri "https://api.github.com/repos/vyPal/CaffeineC/releases/latest" | Select-Object -ExpandProperty tag_name
$architecture = $env:PROCESSOR_ARCHITECTURE
if ($architecture -eq "AMD64") {
  $arch_suffix = "-amd64"
} elseif ($architecture -eq "ARM64") {
  $arch_suffix = "-arm64"
} else {
  Write-Host "Unsupported architecture."
  exit 1
}

$download_url = "https://github.com/vyPal/CaffeineC/releases/latest/download/CaffeineC-Windows${arch_suffix}.exe"
$install_dir = "$env:USERPROFILE\AppData\Local\Programs\CaffeineC"

# Create the directory if it doesn't exist
if (!(Test-Path -Path $install_dir)) {
  New-Item -ItemType Directory -Path $install_dir
}

# Download the binary
Invoke-WebRequest -Uri $download_url -OutFile "$install_dir\CaffeineC.exe"

$env:Path += ";$install_dir"
[Environment]::SetEnvironmentVariable("Path", "$($env:Path);$install_dir", [System.EnvironmentVariableTarget]::User)

# Download the autocomplete script
$autocomplete_url = "https://raw.githubusercontent.com/vyPal/CaffeineC/master/autocomplete/powershell_autocomplete.ps1"
$autocomplete_path = "$install_dir\powershell_autocomplete.ps1"
Invoke-WebRequest -Uri $autocomplete_url -OutFile $autocomplete_path

# Add the autocomplete script to the PowerShell profile
Add-Content -Path $PROFILE -Value ". $autocomplete_path"

Write-Output "Installation complete."