package main

import (
	"archive/zip"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type MediaInfoData struct {
	Media struct {
		Track []MediaTrack `json:"track"`
	} `json:"media"`
}

type MediaTrack struct {
	Type string `json:"@type"`
	// Add other fields as needed
}

// findMediaInfoBinary searches for MediaInfo binary in various locations
func findMediaInfoBinary(configPath string) (string, error) {
	// 1. Check if path is specified in config
	if configPath != "" {
		if isExecutable(configPath) {
			log.Printf("ℹ️ Using MediaInfo from config: %s", configPath)
			return configPath, nil
		}
		log.Printf("Warning: MediaInfo path from config is not executable: %s", configPath)
	}

	// 2. Check in the same directory as the executable
	executableDir := getCurrentDir()
	localPath := filepath.Join(executableDir, "mediainfo")
	if runtime.GOOS == "windows" {
		localPath += ".exe"
	}
	if isExecutable(localPath) {
		log.Printf("ℹ️ Using local MediaInfo: %s", localPath)
		return localPath, nil
	}

	// 3. Check in PATH
	pathBinary, err := exec.LookPath("mediainfo")
	if err == nil && isExecutable(pathBinary) {
		log.Printf("ℹ️ Using MediaInfo from PATH: %s", pathBinary)
		return pathBinary, nil
	}

	// 4. Check standard installation paths based on OS
	standardPaths := getStandardMediaInfoPaths()
	for _, path := range standardPaths {
		if isExecutable(path) {
			log.Printf("ℹ️ Using MediaInfo from default path: %s", path)
			return path, nil
		}
	}

	// 5. For Windows, try to download MediaInfo CLI automatically
	if runtime.GOOS == "windows" {
		if downloadedPath, err := downloadMediaInfoForWindows(executableDir); err == nil {
			log.Printf("Successfully downloaded MediaInfo CLI: %s", downloadedPath)
			return downloadedPath, nil
		} else {
			log.Printf("Warning: Failed to download MediaInfo CLI: %v", err)
		}
	}

	return "", fmt.Errorf("MediaInfo binary not found. Please install MediaInfo or specify the path in config.json")
}

// downloadMediaInfoForWindows downloads MediaInfo CLI for Windows
func downloadMediaInfoForWindows(targetDir string) (string, error) {
	const mediaInfoURL = "https://mediaarea.net/download/binary/mediainfo/25.04/MediaInfo_CLI_25.04_Windows_x64.zip"

	log.Println("MediaInfo not found. Attempting to download MediaInfo CLI for Windows...")

	// Download the zip file
	zipPath := filepath.Join(targetDir, "mediainfo_cli.zip")
	if err := downloadFile(mediaInfoURL, zipPath); err != nil {
		return "", fmt.Errorf("failed to download MediaInfo CLI: %v", err)
	}
	defer os.Remove(zipPath) // Clean up zip file after extraction

	// Extract MediaInfo.exe from the zip
	targetPath := filepath.Join(targetDir, "mediainfo.exe")
	if err := extractMediaInfoFromZip(zipPath, targetPath); err != nil {
		return "", fmt.Errorf("failed to extract MediaInfo CLI: %v", err)
	}

	// Verify the extracted binary works
	if !isExecutable(targetPath) {
		return "", fmt.Errorf("downloaded MediaInfo binary is not functional")
	}

	return targetPath, nil
}

// downloadFile downloads a file from URL to local path
func downloadFile(url, filepath string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status: %s", resp.Status)
	}

	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

// extractMediaInfoFromZip extracts MediaInfo.exe from the downloaded zip
func extractMediaInfoFromZip(zipPath, targetPath string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	// Look for MediaInfo.exe in the root of the zip
	for _, f := range r.File {
		if strings.ToLower(filepath.Base(f.Name)) == "mediainfo.exe" {
			return extractFileFromZip(f, targetPath)
		}
	}

	return fmt.Errorf("MediaInfo.exe not found in zip archive")
}

// extractFileFromZip extracts a single file from zip archive
func extractFileFromZip(f *zip.File, targetPath string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	outFile, err := os.Create(targetPath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, rc)
	return err
}

// getStandardMediaInfoPaths returns standard installation paths for MediaInfo based on OS
func getStandardMediaInfoPaths() []string {
	switch runtime.GOOS {
	case "linux":
		return []string{
			"/usr/bin/mediainfo",
			"/usr/local/bin/mediainfo",
			"/opt/mediainfo/bin/mediainfo",
		}
	case "darwin": // macOS
		return []string{
			"/usr/local/bin/mediainfo",
			"/opt/homebrew/bin/mediainfo",
			"/usr/bin/mediainfo",
		}
	case "windows":
		// No standard paths for Windows - will try to download CLI version if not found
		return []string{}
	default:
		return []string{}
	}
}

// isExecutable checks if a file exists and is executable
func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	// Check if it's a regular file
	if !info.Mode().IsRegular() {
		return false
	}

	// Test if it's executable by trying to run --version
	if err := exec.Command(path, "--Version").Run(); err != nil {
		return false
	}

	return true
}

// initializeMediaInfo finds and verifies MediaInfo binary
func initializeMediaInfo(configPath string) (string, bool) {
	mediaInfoPath, err := findMediaInfoBinary(configPath)
	if err != nil {
		log.Printf("Warning: %v", err)
		return "", false
	}

	// Test if the binary works
	if err := exec.Command(mediaInfoPath, "--Version").Run(); err != nil {
		log.Printf("Warning: MediaInfo binary is not functional: %v", err)
		return "", false
	}

	// log.Println("MediaInfo binary found and verified successfully")
	return mediaInfoPath, true
}

func generateMediaInfoJSON(filePath, mediaInfoPath string) ([]byte, error) {
	if mediaInfoPath == "" {
		return nil, fmt.Errorf("MediaInfo is not available")
	}

	cmd := exec.Command(mediaInfoPath, "--Output=JSON", filePath)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to run MediaInfo: %v", err)
	}

	return output, nil
}
