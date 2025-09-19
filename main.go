package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// Version information - set at build time
var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildDate = "unknown"
)

// QBittorrentArgs holds the arguments passed from qBittorrent
type QBittorrentArgs struct {
	TorrentName string // %N
	ContentPath string // %F  
	Category    string // %L
	InfoHash    string // %I
}

// Global variable to track update availability
var (
	updateAvailable = false
	latestVersion   = ""
	updateCheckDone = false
)

// getCleanVersion returns a clean version string for User-Agent
// Removes git suffixes like -dirty, -dev, commit hashes, etc.
func getCleanVersion() string {
	version := Version

	// Remove -dev suffix
	if strings.HasSuffix(version, "-dev") {
		version = strings.TrimSuffix(version, "-dev")
	}

	// Remove -dirty suffix (from git describe)
	if strings.HasSuffix(version, "-dirty") {
		version = strings.TrimSuffix(version, "-dirty")
	}

	// Remove git commit hash (format: v1.0.0-1-g1234567)
	// Split on dash and take only the version part
	parts := strings.Split(version, "-")
	if len(parts) > 0 {
		version = parts[0]
	}

	// Remove 'v' prefix if present
	if strings.HasPrefix(version, "v") {
		version = strings.TrimPrefix(version, "v")
	}

	// Fallback if version is empty or invalid
	if version == "" || version == "dev" || version == "unknown" {
		version = "1.0.0"
	}

	return version
}

// getUserAgent returns the User-Agent string for API requests
func getUserAgent() string {
	return fmt.Sprintf("crowdclient-qBittorrent/%s", getCleanVersion())
}

func main() {
	log.SetFlags(0)

	// Check for version flag
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Printf("CrowdNFO qBittorrent Post-Processor %s\n", Version)
		fmt.Printf("Git Commit: %s\n", GitCommit)
		fmt.Printf("Build Date: %s\n", BuildDate)
		return
	}

	if len(os.Args) < 5 {
		log.Fatal("Insufficient arguments. Expected 4 arguments from qBittorrent: torrent_name content_path category info_hash")
	}

	// Parse qBittorrent arguments
	cleanJobName := os.Args[1]  // %N - Torrent name
	finalDir := os.Args[2]      // %F - Content path
	qbtCategory := os.Args[3]   // %L - Category
	infoHash := os.Args[4]      // %I - Info hash

	// Store qBittorrent arguments for post-processing
	qbtArgs := QBittorrentArgs{
		TorrentName: cleanJobName,
		ContentPath: finalDir,
		Category:    qbtCategory,
		InfoHash:    infoHash,
	}

	// Load configuration first
	config, err := loadConfig()
	if err != nil {
		log.Fatal("❌ Failed to load configuration: ", err)
	}

	// Check if category should be excluded from processing
	if isCategoryExcluded(config, qbtCategory) {
		log.Printf("ℹ️ Category '%s' is excluded from processing, skipping CrowdNFO upload", qbtCategory)
		
		// Execute post-processing commands even if category is excluded
		executePostProcessing(config, qbtArgs)
		return
	}

	// Check UmlautAdaptarr for title changes
	originalTitle, err := checkUmlautadaptarr(config, cleanJobName)
	if err != nil {
		log.Printf("❌ UmlautAdaptarr check failed: %v", err)
		log.Printf("⚠️ Skipping CrowdNFO processing, but continuing with post-processing scripts...")

		// Execute post-processing commands even if UmlautAdaptarr fails
		executePostProcessing(config, qbtArgs)
		return
	}

	// Use original title if Umlautadaptarr made changes
	if originalTitle != "" {
		log.Printf("ℹ️ Using original title from UmlautAdaptarr: %s", originalTitle)
		cleanJobName = originalTitle
	}

	// Create archive directory
	archiveDir := filepath.Join(getCurrentDir(), "archive", cleanJobName)
	if err := os.MkdirAll(archiveDir, 0755); err != nil {
		log.Fatal("Failed to create archive directory: ", err)
	}

	// Check if this is a season pack
	if isSeasonPack(cleanJobName) || isSeasonPackFallback(finalDir) {
		if isSeasonPack(cleanJobName) {
			log.Printf("📦 Detected season pack via name pattern: %s", cleanJobName)
		} else {
			log.Printf("📦 Detected season pack via file count (≥3 episodes): %s", cleanJobName)
		}
		if err := processSeasonPack(config, finalDir, cleanJobName, qbtCategory, archiveDir, qbtArgs); err != nil {
			log.Printf("❌ Season pack processing failed: %v", err)
			return
		}
		log.Printf("✅ Season pack processing completed")
		return
	}

	// Try to initialize MediaInfo (optional)
	mediaInfoPath, hasMediaInfo := initializeMediaInfo(config.MediaInfoPath)
	if !hasMediaInfo {
		log.Println("ℹ️ MediaInfo not available - some features may be limited")
	}

	// Try to find media file for MediaInfo generation
	var mediaFile string
	var hash string
	var mediaInfoJSON []byte

	// First try to find video file
	videoFile, err := findBiggestFile(finalDir)
	if err == nil && videoFile != "" {
		mediaFile = videoFile
	} else {
		// If no video file, try to find audio file (for music/audiobooks)
		audioFile, err := findFirstAudioFile(finalDir)
		if err == nil && audioFile != "" {
			mediaFile = audioFile
		}
	}

	// Generate MediaInfo and hash if media file found
	if mediaFile != "" && hasMediaInfo {
		log.Printf("⏳ Processing media file: %s", filepath.Base(mediaFile))

		// Generate MediaInfo JSON only for non-hash-only files
		if !isHashOnlyFile(mediaFile) {
			mediaInfoJSON, err = generateMediaInfoJSON(mediaFile, mediaInfoPath)
			if err != nil {
				log.Printf("⚠️ Failed to generate MediaInfo: %v", err)
			}
		}
	} else if mediaFile != "" && !hasMediaInfo {
		log.Println("ℹ️ Skipping MediaInfo generation - MediaInfo not available")
	}

	// Calculate hash for any file found (media or ISO/IMG)
	if mediaFile != "" {
		shouldHash, err := shouldCalculateHash(config, mediaFile)
		if err != nil {
			log.Printf("⚠️ Failed to check file size for hash calculation: %v", err)
		} else if shouldHash {
			hash, err = calculateSHA256(mediaFile)
			if err != nil {
				log.Printf("⚠️ Failed to calculate SHA256: %v", err)
			}
		}
	}

	// Find NFO file (independent of media files)
	nfoFile, err := findNFOFile(finalDir)
	if err != nil {
		log.Printf("ℹ️ No NFO file found")
		nfoFile = "" // Set empty string for upload function
	}

	// Upload to CrowdNFO API (works with or without media files/NFO)
	if err := uploadToCrowdNFO(config, cleanJobName, qbtCategory, hash, finalDir, mediaInfoJSON, nfoFile, archiveDir); err != nil {
		errStr := err.Error()
		if strings.HasPrefix(errStr, "partial_failure:") {
			log.Printf("⚠️ Upload completed with partial success: %s", strings.TrimPrefix(errStr, "partial_failure:"))
		} else if strings.HasPrefix(errStr, "total_failure:") {
			log.Printf("❌ Upload process failed: %s", strings.TrimPrefix(errStr, "total_failure:"))
		} else {
			log.Printf("❌ Upload process failed: %v", err)
		}
	}

	log.Printf("✅ All processing completed successfully")

	// Check and display update notification if available
	displayUpdateNotification()

	// Execute post-processing commands (always run, regardless of upload success)
	executePostProcessing(config, qbtArgs)
}

// displayUpdateNotification shows update information if available
func displayUpdateNotification() {
	if !updateCheckDone {
		return
	}

	if updateAvailable {
		log.Printf("🔔 Update available!")
		if latestVersion != "" {
			currentVersion := getCleanVersion()
			log.Printf("   Current version: %s", currentVersion)
			log.Printf("   Latest version:  %s", latestVersion)
		}
		log.Printf("   Visit https://github.com/your-repo/releases for the latest version")
	}
}

// isSeasonPack determines if the given job name corresponds to a season pack
func isSeasonPack(jobName string) bool {
	// First check for traditional season patterns (S\d{2,4}) but NOT episodes (SxxExx)
	seasonPattern := regexp.MustCompile(`(?i)\bS\d{2,4}\b`)
	episodePattern := regexp.MustCompile(`(?i)\bS\d{2,4}E\d{2,4}\b`)

	if seasonPattern.MatchString(jobName) && !episodePattern.MatchString(jobName) {
		return true
	}

	// Check for ISO date format years (S2024, S2023, etc.)
	isoYearPattern := regexp.MustCompile(`(?i)\bS(20\d{2})\b`)
	if isoYearPattern.MatchString(jobName) {
		return true
	}

	return false
}

// isSeasonPackFallback checks if a directory should be treated as season pack based on video file count
func isSeasonPackFallback(finalDir string) bool {
	videoFiles, err := findAllVideoFiles(finalDir)
	if err != nil {
		return false
	}

	// If we find 3 or more video files, treat as season pack
	return len(videoFiles) >= 3
}

// processSeasonPack handles the processing of season packs
func processSeasonPack(config *Config, finalDir, cleanJobName, qbtCategory, archiveDir string, qbtArgs QBittorrentArgs) error {
	// Check if this is actually a season pack by counting video files
	if !isSeasonPackFallback(finalDir) {
		log.Printf("ℹ️ Less than 3 video files found, processing as single release")
		return nil
	}

	// Try to initialize MediaInfo (optional)
	mediaInfoPath, hasMediaInfo := initializeMediaInfo(config.MediaInfoPath)
	if !hasMediaInfo {
		log.Println("ℹ️ MediaInfo not available - some features may be limited")
	}

	// Find all video files in the season pack
	videoFiles, err := findAllVideoFiles(finalDir)
	if err != nil {
		return err
	}

	if len(videoFiles) == 0 {
		log.Println("No video files found in season pack")
		return nil
	}

	log.Printf("🔍 Found %d video files in season pack", len(videoFiles))

	// Extract episode information for each video file
	episodes := make([]EpisodeInfo, 0)
	generalNFO := findGeneralNFO(finalDir)

	for _, videoFile := range videoFiles {
		episodeInfo := extractEpisodeInfo(videoFile, cleanJobName, generalNFO)
		if episodeInfo.ReleaseName != "" { // Only process valid episodes
			episodes = append(episodes, episodeInfo)
		}
	}

	if len(episodes) == 0 {
		log.Println("No valid episodes found in season pack")
		return nil
	}

	log.Printf("📺 Processing %d episodes", len(episodes))

	// Process each episode
	successCount := 0
	for i, episode := range episodes {
		log.Printf("📄 Processing episode %d/%d: %s", i+1, len(episodes), episode.ReleaseName)

		// Calculate SHA256 for this episode (check file size limit first)
		var hash string
		shouldHash, err := shouldCalculateHash(config, episode.VideoFile.Path)
		if err != nil {
			log.Printf("⚠️ Failed to check file size for hash calculation: %v", err)
		} else if shouldHash {
			hash, err = calculateSHA256(episode.VideoFile.Path)
			if err != nil {
				log.Printf("❌ Failed to calculate SHA256 for %s: %v", episode.ReleaseName, err)
				continue
			}
		}

		// Generate MediaInfo JSON for this episode
		var mediaInfoJSON []byte
		if hasMediaInfo {
			mediaInfoJSON, err = generateMediaInfoJSON(episode.VideoFile.Path, mediaInfoPath)
			if err != nil {
				log.Printf("⚠️ Failed to generate MediaInfo for %s: %v", episode.ReleaseName, err)
			}
		}

		// Upload this episode to CrowdNFO API with file list
		err = uploadEpisodeToCrowdNFO(config, episode, qbtCategory, hash, mediaInfoJSON, archiveDir)
		if err != nil {
			// Don't log additional error message - the upload function already logged the details
			continue
		}

		successCount++
	}

	log.Printf("✅ Season pack completed: %d/%d episodes successful", successCount, len(episodes))

	// Execute post-processing commands for season packs
	executePostProcessing(config, qbtArgs)

	return nil
}

// executePostProcessing runs post-processing commands based on configuration
func executePostProcessing(config *Config, qbtArgs QBittorrentArgs) {
	if config.PostProcessing.Global.Enabled {
		runPostProcessCommand("global", config.PostProcessing.Global, qbtArgs)
	}

	// Check for category-specific post-processing
	if config.PostProcessing.Categories != nil {
		// First try the exact qBittorrent category
		if cmd, exists := config.PostProcessing.Categories[qbtArgs.Category]; exists && cmd.Enabled {
			runPostProcessCommand(fmt.Sprintf("category '%s'", qbtArgs.Category), cmd, qbtArgs)
			return
		}

		// Try lowercase version
		if cmd, exists := config.PostProcessing.Categories[strings.ToLower(qbtArgs.Category)]; exists && cmd.Enabled {
			runPostProcessCommand(fmt.Sprintf("category '%s'", strings.ToLower(qbtArgs.Category)), cmd, qbtArgs)
			return
		}
	}
}

// runPostProcessCommand executes a post-processing command with qBittorrent arguments and placeholders
func runPostProcessCommand(configType string, cmd PostProcessCommand, qbtArgs QBittorrentArgs) {
	if cmd.Command == "" {
		return
	}

	log.Printf("🔧 Running %s post-processing: %s", configType, cmd.Command)

	// Build command arguments
	args := make([]string, 0)

	// Add additional arguments from config if specified, with placeholder substitution
	if len(cmd.Arguments) > 0 {
		for _, arg := range cmd.Arguments {
			// Replace qBittorrent placeholders
			arg = strings.ReplaceAll(arg, "%N", qbtArgs.TorrentName)
			arg = strings.ReplaceAll(arg, "%F", qbtArgs.ContentPath)
			arg = strings.ReplaceAll(arg, "%L", qbtArgs.Category)
			arg = strings.ReplaceAll(arg, "%I", qbtArgs.InfoHash)
			args = append(args, arg)
		}
	}

	// Execute the command
	execCmd := exec.Command(cmd.Command, args...)

	// Pass through all environment variables and add qBittorrent-specific ones
	env := os.Environ()
	env = append(env, fmt.Sprintf("QBT_TORRENT_NAME=%s", qbtArgs.TorrentName))
	env = append(env, fmt.Sprintf("QBT_CONTENT_PATH=%s", qbtArgs.ContentPath))
	env = append(env, fmt.Sprintf("QBT_CATEGORY=%s", qbtArgs.Category))
	env = append(env, fmt.Sprintf("QBT_INFO_HASH=%s", qbtArgs.InfoHash))
	execCmd.Env = env

	// Set working directory to current directory
	execCmd.Dir = getCurrentDir()

	// Capture output
	output, err := execCmd.CombinedOutput()
	if err != nil {
		log.Printf("❌ Post-processing command failed: %v", err)
		if len(output) > 0 {
			log.Printf("   Output: %s", string(output))
		}
	} else {
		log.Printf("✅ Post-processing command completed successfully")
		if len(output) > 0 {
			log.Printf("   Output: %s", string(output))
		}
	}
}
