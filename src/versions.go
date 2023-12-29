package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/fatih/color"
	"github.com/urfave/cli/v2"
)

func init() {
	commands = append(commands, &cli.Command{
		Name:     "update",
		Usage:    "Update CaffeineC to the latest version",
		Category: "version",
		Action:   update,
	})
	commands = append(commands, &cli.Command{
		Name:     "autocomplete",
		Usage:    "Install autocomplete for CaffeineC",
		Category: "version",
		Action:   autocomplete,
	})
}

func downloadFile(url string, filepath string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

type Release struct {
	TagName string `json:"tag_name"`
}

func checkUpdate(c *cli.Context) {
	resp, err := http.Get("https://api.github.com/repos/vyPal/CaffeineC/releases/latest")
	if err != nil {
		fmt.Println("Failed to fetch the latest release:", err)
		return
	}
	defer resp.Body.Close()

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		fmt.Println("Failed to decode the release data:", err)
		return
	}

	// Remove the 'v' prefix from the tag name
	latestVersion := strings.TrimPrefix(release.TagName, "v")

	if latestVersion != c.App.Version {
		fmt.Printf("A new version is available: %s to update, run 'CaffeineC update'\n", latestVersion)
	}
}

func autocomplete(c *cli.Context) error {
	shell := filepath.Base(os.Getenv("SHELL"))
	homeDir, _ := os.UserHomeDir()
	var autocompleteScriptURL, shellConfigFile string

	switch shell {
	case "bash":
		autocompleteScriptURL = "https://raw.githubusercontent.com/vyPal/CaffeineC/master/autocomplete/bash_autocomplete"
		shellConfigFile = filepath.Join(homeDir, ".bashrc")
	case "zsh":
		autocompleteScriptURL = "https://raw.githubusercontent.com/vyPal/CaffeineC/master/autocomplete/zsh_autocomplete"
		shellConfigFile = filepath.Join(homeDir, ".zshrc")
	default:
		fmt.Println("Unsupported shell for autocomplete. Skipping...")
		return nil
	}

	installDir := path.Join(homeDir, ".local", "share", "CaffeineC")
	if err := os.MkdirAll(installDir, 0755); err != nil {
		return err
	}
	autocompleteScriptPath := filepath.Join(installDir, "CaffeineC_autocomplete")

	fmt.Printf("Downloading autocomplete script for %s...\n", shell)
	err := downloadFile(autocompleteScriptURL, autocompleteScriptPath)
	if err != nil {
		return err
	}

	// Add the source command to the shell's configuration file to make it persistent
	file, err := os.OpenFile(shellConfigFile, os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	// Check if the source line already exists in the file
	sourceLine := fmt.Sprintf("source %s", autocompleteScriptPath)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), sourceLine) {
			fmt.Println("Autocomplete script already installed.")
			return nil
		}
	}

	// If the source line doesn't exist, append it to the file
	_, err = file.WriteString("\n" + sourceLine + "\n")
	if err != nil {
		return err
	}

	fmt.Println("Autocomplete script installed. It will be sourced automatically in new shell sessions.")
	fmt.Println("To source it in the current session, run:")
	relativePath := strings.Replace(autocompleteScriptPath, homeDir, "~", 1)
	fmt.Printf("\tsource %s\n", relativePath)
	return nil
}

func update(c *cli.Context) error {
	if runtime.GOOS == "windows" {
		return cli.Exit(color.RedString("Windows automatic updates are not supported at this time."), 1)
	}
	resp, err := http.Get("https://api.github.com/repos/vyPal/CaffeineC/releases/latest")
	if err != nil {
		fmt.Println("Failed to fetch the latest release:", err)
		return nil
	}
	defer resp.Body.Close()

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		fmt.Println("Failed to decode the release data:", err)
		return nil
	}

	// Remove the 'v' prefix from the tag name
	latestVersion := strings.TrimPrefix(release.TagName, "v")

	if latestVersion != c.App.Version {
		fmt.Printf("A new version is available: %s. Updating...\n", latestVersion)

		osSuffix := "Linux"
		if runtime.GOOS == "darwin" {
			osSuffix = "macOS"
		}

		// Download the new binary
		resp, err = http.Get("https://github.com/vyPal/CaffeineC/releases/download/" + release.TagName + "/CaffeineC-" + osSuffix)
		if err != nil {
			fmt.Println("Failed to download the new version:", err)
			return nil
		}
		defer resp.Body.Close()

		// Write the new binary to a temporary file
		tmpFile, err := os.CreateTemp("", "CaffeineC")
		if err != nil {
			fmt.Println("Failed to create a temporary file:", err)
			return nil
		}
		defer os.Remove(tmpFile.Name())

		_, err = io.Copy(tmpFile, resp.Body)
		if err != nil {
			fmt.Println("Failed to write to the temporary file:", err)
			return nil
		}

		// Expand the ~ to the user's home directory
		homeDir, err := os.UserHomeDir()
		if err != nil {
			fmt.Println("Failed to get the user's home directory:", err)
			return nil
		}

		baseDir := filepath.Join(homeDir, ".local", "bin")
		if runtime.GOOS == "darwin" {
			baseDir = filepath.Join("/usr", "local", "bin")
		}

		// Set the destination file path
		dstFilePath := filepath.Join(baseDir, "CaffeineC")

		// Rename the old file
		oldFilePath := dstFilePath + ".old"
		if err := os.Rename(dstFilePath, oldFilePath); err != nil {
			fmt.Println("Failed to rename the old file:", err)
			return nil
		}

		// Create the destination file
		dstFile, err := os.Create(dstFilePath)
		if err != nil {
			fmt.Println("Failed to create the destination file:", err)
			return nil
		}
		defer dstFile.Close()

		// Copy the temporary file to the destination file
		_, err = io.Copy(dstFile, tmpFile)
		if err != nil {
			fmt.Println("Failed to copy the file:", err)
			return nil
		}

		// Make the new file executable
		if err := os.Chmod(dstFilePath, 0755); err != nil {
			fmt.Println("Failed to make the new file executable:", err)
			return nil
		}

		// Remove the old file
		if err := os.Remove(oldFilePath); err != nil {
			fmt.Println("Failed to remove the old file:", err)
			return nil
		}

		fmt.Println("Update successful!")
	} else {
		fmt.Println("You're up to date!")
	}

	return nil
}
