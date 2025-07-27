package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var (
	mediaInfoExtensions = []string{".mkv", ".mp4", ".avi", ".mov", ".wmv", ".flv", ".mpeg", ".mpg", ".webm", ".m4v", ".divx", ".xvid", ".mp3", ".aac", ".flac", ".wav", ".ogg", ".opus", ".m4a", ".mka", ".wma", ".alac", ".dts", ".dtshd", ".ac3", ".eac3", ".ec3", ".m4b"}
	hashOnlyExtensions  = []string{".iso", ".img"}
)

// findBiggestFile finds the biggest file in the given directory and subdirectories
func findBiggestFile(dir string) (string, error) {
	var biggestFile string
	var biggestSize int64

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))

		// Check for media files (video only, no audio)
		for _, videoExt := range mediaInfoExtensions {
			if ext == videoExt {
				// Skip audio files for biggest file search (only for video)
				if isAudioExtension(ext) {
					break
				}

				info, err := d.Info()
				if err != nil {
					return err
				}

				if info.Size() > biggestSize {
					biggestSize = info.Size()
					biggestFile = path
				}
				break
			}
		}

		// Check for hash-only files (ISO/IMG)
		for _, hashExt := range hashOnlyExtensions {
			if ext == hashExt {
				info, err := d.Info()
				if err != nil {
					return err
				}

				if info.Size() > biggestSize {
					biggestSize = info.Size()
					biggestFile = path
				}
				break
			}
		}

		return nil
	})

	return biggestFile, err
}

// findFirstAudioFile finds the first audio file with "01", "001" etc. in the name
func findFirstAudioFile(dir string) (string, error) {
	var firstAudioFile string
	var fallbackFile string

	// Pattern to match track numbers like "01", "001", "1", etc.
	trackPattern := regexp.MustCompile(`\b0*1\b`)

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if isAudioExtension(ext) {
			// Store first audio file as fallback
			if fallbackFile == "" {
				fallbackFile = path
			}

			// Check if filename contains track number "01", "001" etc.
			fileName := strings.TrimSuffix(d.Name(), filepath.Ext(d.Name()))
			if trackPattern.MatchString(fileName) {
				firstAudioFile = path
				return filepath.SkipAll // Found what we're looking for
			}
		}

		return nil
	})

	// Return first track if found, otherwise fallback to first audio file
	if firstAudioFile != "" {
		return firstAudioFile, err
	}
	return fallbackFile, err
}

func findNFOFile(dir string) (string, error) {
	var nfoFile string

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		if strings.ToLower(filepath.Ext(path)) == ".nfo" {
			nfoFile = path
			return filepath.SkipAll // Stop walking after finding the first NFO file
		}

		return nil
	})

	if nfoFile == "" {
		return "", fmt.Errorf("no NFO file found")
	}

	return nfoFile, err
}

func calculateSHA256(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// shouldCalculateHash checks if hash should be calculated based on file size and config
func shouldCalculateHash(config *Config, filePath string) (bool, error) {
	// If max_hash_file_size is not set or empty, always calculate hash
	if config.MaxHashFileSize == "" {
		return true, nil
	}

	// If max_hash_file_size is "0", never calculate hash
	if config.MaxHashFileSize == "0" {
		return false, nil
	}

	// Parse the size limit from config with unit support (MB/GB)
	maxSizeBytes, err := parseSizeWithUnit(config.MaxHashFileSize)
	if err != nil {
		log.Printf("⚠️ Invalid max_hash_file_size format: %s, ignoring limit", config.MaxHashFileSize)
		return true, nil
	}

	// Get file size
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return false, err
	}

	if fileInfo.Size() > maxSizeBytes {
		fileSizeGB := float64(fileInfo.Size()) / (1024 * 1024 * 1024)
		maxSizeGB := float64(maxSizeBytes) / (1024 * 1024 * 1024)
		log.Printf("⏭️ Skipping hash calculation (%.2f GB > %.2f GB limit)", fileSizeGB, maxSizeGB)
		return false, nil
	}

	return true, nil
}

// parseSizeWithUnit parses size strings like "24GB", "800MB", or "5.5" (defaults to GB)
func parseSizeWithUnit(sizeStr string) (int64, error) {
	sizeStr = strings.TrimSpace(strings.ToUpper(sizeStr))

	// Check for MB suffix
	if strings.HasSuffix(sizeStr, "MB") {
		numStr := strings.TrimSuffix(sizeStr, "MB")
		sizeMB, err := strconv.ParseFloat(numStr, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid number format: %s", numStr)
		}
		return int64(sizeMB * 1024 * 1024), nil
	}

	// Check for GB suffix
	if strings.HasSuffix(sizeStr, "GB") {
		numStr := strings.TrimSuffix(sizeStr, "GB")
		sizeGB, err := strconv.ParseFloat(numStr, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid number format: %s", numStr)
		}
		return int64(sizeGB * 1024 * 1024 * 1024), nil
	}

	// No unit specified - assume GB for backward compatibility
	sizeGB, err := strconv.ParseFloat(sizeStr, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid number format: %s", sizeStr)
	}
	return int64(sizeGB * 1024 * 1024 * 1024), nil
}

// Structures for season pack processing
type VideoFile struct {
	Path string
	Dir  string
	Name string
}

type EpisodeInfo struct {
	VideoFile   VideoFile
	EpisodeNum  string // "E01", "E19" etc.
	ReleaseName string
	NFOFile     string
}

// FileListEntry represents a single file in a file list
type FileListEntry struct {
	FilePath      string `json:"filePath"`
	FileSizeBytes int64  `json:"fileSizeBytes"`
}

// FileListRequest represents the JSON structure for file list API requests
type FileListRequest struct {
	ReleaseName string          `json:"releaseName"`
	Category    string          `json:"category"`
	Entries     []FileListEntry `json:"entries"`
}

// findAllVideoFiles finds all video files in the given directory and subdirectories
func findAllVideoFiles(dir string) ([]VideoFile, error) {
	var videoFiles []VideoFile

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		for _, videoExt := range mediaInfoExtensions {
			if ext == videoExt {
				// Skip audio files for season pack processing
				if isAudioExtension(ext) {
					break
				}

				videoFile := VideoFile{
					Path: path,
					Dir:  filepath.Dir(path),
					Name: filepath.Base(path),
				}
				videoFiles = append(videoFiles, videoFile)
				break
			}
		}

		return nil
	})

	return videoFiles, err
}

// createFileList creates a file list for the given directory
func createFileList(dir, releaseName string) ([]FileListEntry, error) {
	var entries []FileListEntry
	baseDir := filepath.Clean(dir)

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		// Get relative path from base directory
		relPath, err := filepath.Rel(baseDir, path)
		if err != nil {
			return err
		}

		// Get file size
		info, err := d.Info()
		if err != nil {
			return err
		}

		// Use forward slashes for API compatibility
		relPath = filepath.ToSlash(relPath)

		entries = append(entries, FileListEntry{
			FilePath:      relPath,
			FileSizeBytes: info.Size(),
		})

		return nil
	})

	return entries, err
}

// createEpisodeFileList creates a file list for a specific episode directory or related files
func createEpisodeFileList(episodeInfo EpisodeInfo) ([]FileListEntry, error) {
	// Get the parent directory (season pack directory)
	seasonPackDir := filepath.Dir(episodeInfo.VideoFile.Dir)

	// Find all video files in the season pack to determine structure
	allVideoFiles, err := findAllVideoFiles(seasonPackDir)
	if err != nil {
		return nil, err
	}

	// Check if all videos are in the same directory (main directory structure)
	// vs. spread across subdirectories (episode folder structure)
	mainDirVideoCount := 0
	for _, vf := range allVideoFiles {
		if vf.Dir == episodeInfo.VideoFile.Dir {
			mainDirVideoCount++
		}
	}

	// If mainDirVideoCount is 0, something is wrong - treat as single video file
	if mainDirVideoCount == 0 {
		log.Printf("⚠️ No videos found in the expected directory structure, treating as single video file: %s", episodeInfo.VideoFile.Name)
		return createFileList(episodeInfo.VideoFile.Dir, episodeInfo.ReleaseName)
	}

	// If most/all videos are in the same directory as this episode,
	// then we have a main directory structure
	if mainDirVideoCount > 1 {
		// Videos are in main directory - find only related files for this specific episode
		videoBaseName := strings.TrimSuffix(episodeInfo.VideoFile.Name, filepath.Ext(episodeInfo.VideoFile.Name))

		entries, err := findRelatedFiles(episodeInfo.VideoFile.Dir, videoBaseName, episodeInfo.VideoFile.Path)
		if err != nil {
			return nil, err
		}
		return entries, nil
	} else {
		// Video is in episode-specific subdirectory - create file list for that directory only
		return createFileList(episodeInfo.VideoFile.Dir, episodeInfo.ReleaseName)
	}
}

// findRelatedFiles finds all files that are related to a specific video file
func findRelatedFiles(dir, videoBaseName, videoPath string) ([]FileListEntry, error) {
	var entries []FileListEntry
	baseDir := filepath.Clean(dir)

	// Always include the video file first
	videoInfo, err := os.Stat(videoPath)
	if err != nil {
		return nil, err
	}

	videoRelPath, err := filepath.Rel(baseDir, videoPath)
	if err != nil {
		return nil, err
	}

	entries = append(entries, FileListEntry{
		FilePath:      filepath.ToSlash(videoRelPath),
		FileSizeBytes: videoInfo.Size(),
	})

	// Extract episode number from video file name for matching
	episodeNum := extractEpisodeNumber(videoBaseName)
	if episodeNum == "" {
		log.Printf("⚠️ Could not extract episode number from: %s", videoBaseName)
		return entries, nil
	}

	// Look for related files based on episode number
	dirEntries, err := os.ReadDir(dir)
	if err != nil {
		return entries, err
	}

	for _, entry := range dirEntries {
		if entry.IsDir() {
			continue
		}

		fileName := entry.Name()
		fileBaseName := strings.TrimSuffix(fileName, filepath.Ext(fileName))

		// Skip the video file itself
		if fileName == filepath.Base(videoPath) {
			continue
		}

		// Check if file is related based on episode number
		if isRelatedFileByEpisode(fileBaseName, episodeNum) {
			filePath := filepath.Join(dir, fileName)
			info, err := entry.Info()
			if err != nil {
				continue
			}

			relPath, err := filepath.Rel(baseDir, filePath)
			if err != nil {
				continue
			}

			entries = append(entries, FileListEntry{
				FilePath:      filepath.ToSlash(relPath),
				FileSizeBytes: info.Size(),
			})
		}
	}

	return entries, nil
}

// extractEpisodeNumber extracts episode number from filename (S01E02 or E02)
func extractEpisodeNumber(fileName string) string {
	// Pattern to match SxxExx or just Exx (case insensitive)
	episodePattern := regexp.MustCompile(`(?i)S\d{2,4}(E\d{2,4})|(?i)(E\d{2,4})`)

	matches := episodePattern.FindStringSubmatch(fileName)
	if len(matches) > 0 {
		// Return the episode part (E01, E02, etc.)
		for i := 1; i < len(matches); i++ {
			if matches[i] != "" && strings.HasPrefix(strings.ToUpper(matches[i]), "E") {
				return strings.ToUpper(matches[i])
			}
		}
	}

	return ""
}

// isRelatedFileByEpisode checks if a file is related based on episode number
func isRelatedFileByEpisode(fileName, episodeNum string) bool {
	// Extract episode number from the file name
	fileEpisodeNum := extractEpisodeNumber(fileName)

	// Compare episode numbers (case insensitive)
	return strings.EqualFold(fileEpisodeNum, episodeNum)
}

// countVideoFilesInDirectory counts video files in the main directory (not subdirectories)
func countVideoFilesInDirectory(dir string) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		ext := strings.ToLower(filepath.Ext(entry.Name()))
		for _, videoExt := range mediaInfoExtensions {
			if ext == videoExt && !isAudioExtension(ext) {
				count++
				break
			}
		}
	}

	return count, nil
}

// isAudioExtension checks if the extension is for audio files
func isAudioExtension(ext string) bool {
	audioExts := []string{".mp3", ".aac", ".flac", ".wav", ".ogg", ".opus", ".m4a", ".mka", ".wma", ".alac", ".dts", ".dtshd", ".ac3", ".eac3", ".ec3", ".m4b"}
	for _, audioExt := range audioExts {
		if ext == audioExt {
			return true
		}
	}
	return false
}

// findGeneralNFO finds a general NFO file in the main directory
func findGeneralNFO(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := strings.ToLower(entry.Name())
		if strings.HasSuffix(name, ".nfo") {
			// Check if it's not an episode-specific NFO
			if !regexp.MustCompile(`s\d{2,4}e\d{2,4}`).MatchString(name) {
				return filepath.Join(dir, entry.Name())
			}
		}
	}

	return ""
}

// extractEpisodeInfo extracts episode information from video file path
func extractEpisodeInfo(videoFile VideoFile, seasonPackName, generalNFO string) EpisodeInfo {
	episodeInfo := EpisodeInfo{
		VideoFile: videoFile,
	}

	// Check if video is in subdirectory (episode folder structure)
	parentDir := filepath.Base(videoFile.Dir)

	// Pattern to match episode numbers SxxExx
	episodePattern := regexp.MustCompile(`(?i)S(\d{2,4})E(\d{2,4})`)

	// Pattern to match ISO date format (yyyy-mm-dd)
	isoDatePattern := regexp.MustCompile(`(\d{4}-\d{2}-\d{2})`)

	// Check if video is in a subdirectory by comparing parent directory with season pack name
	// If parentDir matches seasonPackName, then videos are in main directory
	if strings.EqualFold(parentDir, seasonPackName) {
		// Videos are in main directory - analyze filename
		fileName := strings.TrimSuffix(videoFile.Name, filepath.Ext(videoFile.Name))

		// Try SxxExx pattern first
		if matches := episodePattern.FindStringSubmatch(fileName); len(matches) > 0 {
			episodeInfo.EpisodeNum = "E" + matches[2]

			// Check if filename matches season pack prefix AND is not completely lowercase
			if isValidEpisodeFileName(fileName, seasonPackName) && !isCompletelyLowercase(fileName) {
				// Normal release name with correct case - use as is
				episodeInfo.ReleaseName = fileName

				// Look for NFO with same name
				nfoPath := filepath.Join(videoFile.Dir, fileName+".nfo")
				if _, err := os.Stat(nfoPath); err == nil {
					episodeInfo.NFOFile = nfoPath
				}
			} else {
				// Either shortened/different release name OR lowercase normal name
				// In both cases: generate from season pack name
				episodeInfo.ReleaseName = generateEpisodeReleaseName(seasonPackName, episodeInfo.EpisodeNum)
			}

			// If no episode-specific NFO found and this is E01, use general NFO
			if episodeInfo.NFOFile == "" && episodeInfo.EpisodeNum == "E01" && generalNFO != "" {
				episodeInfo.NFOFile = generalNFO
			}
		} else if matches := isoDatePattern.FindStringSubmatch(fileName); len(matches) > 0 {
			// Handle ISO date format
			episodeInfo.EpisodeNum = matches[1] // Use the full date as episode identifier
			episodeInfo.ReleaseName = fileName

			// Look for NFO with same name
			nfoPath := filepath.Join(videoFile.Dir, fileName+".nfo")
			if _, err := os.Stat(nfoPath); err == nil {
				episodeInfo.NFOFile = nfoPath
			}

			// If no episode-specific NFO found, use general NFO
			if episodeInfo.NFOFile == "" && generalNFO != "" {
				episodeInfo.NFOFile = generalNFO
			}
		}
	} else {
		// Video is in subdirectory - use directory name as release name
		// Try SxxExx pattern first
		if matches := episodePattern.FindStringSubmatch(parentDir); len(matches) > 0 {
			episodeInfo.EpisodeNum = "E" + matches[2]

			// For subdirectory names, check if they match season pack prefix
			if isValidEpisodeFileName(parentDir, seasonPackName) {
				// Normal release name - reject if completely lowercase
				if isCompletelyLowercase(parentDir) {
					log.Printf("⚠️ Rejecting lowercase normal release name: %s", parentDir)
					return episodeInfo // Return empty episodeInfo
				}
			}

			episodeInfo.ReleaseName = parentDir

			// Look for NFO in the same directory
			episodeInfo.NFOFile = findNFOInDirectory(videoFile.Dir)

			// If no episode-specific NFO found and this is E01, use general NFO
			if episodeInfo.NFOFile == "" && episodeInfo.EpisodeNum == "E01" && generalNFO != "" {
				episodeInfo.NFOFile = generalNFO
			}
		} else if matches := isoDatePattern.FindStringSubmatch(parentDir); len(matches) > 0 {
			// Handle ISO date format in subdirectory
			episodeInfo.EpisodeNum = matches[1] // Use the full date as episode identifier
			episodeInfo.ReleaseName = parentDir

			// Look for NFO in the same directory
			episodeInfo.NFOFile = findNFOInDirectory(videoFile.Dir)

			// If no episode-specific NFO found, use general NFO
			if episodeInfo.NFOFile == "" && generalNFO != "" {
				episodeInfo.NFOFile = generalNFO
			}
		}
	}

	return episodeInfo
}

// isCompletelyLowercase checks if a string contains only lowercase letters (ignoring dots, numbers, etc.)
func isCompletelyLowercase(s string) bool {
	hasLetter := false
	for _, r := range s {
		if r >= 'A' && r <= 'Z' {
			return false // Found uppercase letter
		}
		if r >= 'a' && r <= 'z' {
			hasLetter = true
		}
	}
	return hasLetter // Only return true if there are actually letters
}

// findNFOInDirectory finds NFO file in the given directory
func findNFOInDirectory(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}

	// Simply find any .nfo file in the directory
	// Filename doesn't matter for episode directories
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		if strings.HasSuffix(strings.ToLower(entry.Name()), ".nfo") {
			return filepath.Join(dir, entry.Name())
		}
	}

	return ""
}

// isValidEpisodeFileName validates if episode filename matches season pack naming
func isValidEpisodeFileName(fileName, seasonPackName string) bool {
	// Extract prefix before Sxx from season pack name
	seasonPattern := regexp.MustCompile(`(?i)^(.+?)\.?S\d{2,4}`)
	seasonMatches := seasonPattern.FindStringSubmatch(seasonPackName)
	if len(seasonMatches) < 2 {
		return false
	}

	seasonPrefix := normalizeString(seasonMatches[1])

	// Extract prefix before SxxExx from filename
	episodePattern := regexp.MustCompile(`(?i)^(.+?)\.?S\d{2,4}E\d{2,4}`)
	episodeMatches := episodePattern.FindStringSubmatch(fileName)
	if len(episodeMatches) < 2 {
		return false
	}

	episodePrefix := normalizeString(episodeMatches[1])

	return seasonPrefix == episodePrefix
}

// normalizeString normalizes string for comparison (removes dots, spaces, case)
func normalizeString(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, ".", "")
	s = strings.ReplaceAll(s, " ", "")
	return s
}

// generateEpisodeReleaseName generates release name from season pack name and episode number
func generateEpisodeReleaseName(seasonPackName, episodeNum string) string {
	// Remove COMPLETE/iNCOMPLETE from season pack name
	cleanName := regexp.MustCompile(`(?i)\b(COMPLETE|iNCOMPLETE)\b`).ReplaceAllString(seasonPackName, "")
	cleanName = strings.TrimSpace(cleanName)

	// Replace Sxx with SxxExx
	seasonPattern := regexp.MustCompile(`(?i)\bS(\d{2,4})\b`)
	return seasonPattern.ReplaceAllStringFunc(cleanName, func(match string) string {
		seasonNum := regexp.MustCompile(`\d{2,4}`).FindString(match)
		return "S" + seasonNum + episodeNum
	})
}

// isHashOnlyFile checks if the file extension is for hash-only files (ISO/IMG)
func isHashOnlyFile(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	for _, hashExt := range hashOnlyExtensions {
		if ext == hashExt {
			return true
		}
	}
	return false
}
