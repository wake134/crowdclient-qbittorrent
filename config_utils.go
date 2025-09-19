package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type Config struct {
	APIKey             string               `json:"api_key"`
	BaseURL            string               `json:"base_url"`
	MediaInfoPath      string               `json:"mediainfo_path"`
	MaxHashFileSize    string               `json:"max_hash_file_size"`
	VerifySSL          bool                 `json:"verify_ssl"`
	CategoryMappings   map[string][]string  `json:"category_mappings,omitempty"`
	ExcludedCategories []string             `json:"excluded_categories,omitempty"`
	PostProcessing     PostProcessingConfig `json:"post_processing"`
	Umlautadaptarr     UmlautadaptarrConfig `json:"umlautadaptarr"`
}

type PostProcessingConfig struct {
	Global     PostProcessCommand            `json:"global,omitempty"`
	Categories map[string]PostProcessCommand `json:"categories"`
}

type PostProcessCommand struct {
	Command   string   `json:"command"`
	Arguments []string `json:"arguments"`
	Enabled   bool     `json:"enabled"`
}

type UmlautadaptarrConfig struct {
	Enabled bool   `json:"enabled"`
	BaseURL string `json:"base_url"`
}

// Valid CrowdNFO categories
var validCategories = []string{"Movies", "TV", "Games", "Software", "Music", "Audiobooks", "Books", "Other"}

// Built-in regex patterns for category detection
var categoryRegexPatterns = []struct {
	Pattern  string
	Category string
}{
	{`(?i)\b(audiobook|abook|abookde|hörbuch|hoerbuch|horbuch|m4b)\b`, "Audiobooks"},
	{`(?i)\b(ebook|epaper|pdf|epub|mobi)\b`, "Books"},
	{`(?i)\b((s\d{1,4}e\d{1,4})|(s\d{1,4})|(e\d{1,4})|season|staffel|episode|folge|(\d{4}-\d{2}-\d{2}))\b`, "TV"},
	{`(?i)\b(elamigos|gog|xbox|xbox360|x360|ps\d|nintendo|nsw|amiga|atari|wii[u]?)\b`, "Games"},
	{`(?i)\b(patch|crack|cracked|keygen|keymaker|keyfilemaker|x64|dvt|btcr|macos)\b`, "Software"},
	{`(?i)\b((\d{3,4}[pi])|bluray|dvdrip|webrip|hdtv|bdrip|dvd|remux|mpeg[-]?2|vc[-]?1|avc|hevc|([xh][. ]?26[456]))\b`, "Movies"},
	{`(?i)\b(mp3|flac|webflac|aac|wav|album|artist|discography|single|vinyl|cd|\d+bit|\d+khz)\b`, "Music"},
}

func loadConfig() (*Config, error) {
	configPath := filepath.Join(getCurrentDir(), "crowdclient-config.json")

	// Create default config if it doesn't exist
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		defaultConfig := &Config{
			APIKey:          "YOUR_API_KEY_HERE",
			BaseURL:         "https://crowdnfo.net/api/releases",
			MediaInfoPath:   "", // Optional - will be auto-detected if empty
			MaxHashFileSize: "", // Optional - no limit by default, use "0" to disable, or "5GB"/"800MB" to set limit
			VerifySSL:       true, // Verify SSL certificates by default
			CategoryMappings: map[string][]string{
				"Movies":     []string{"movies", "movie", "radarr", "film"},
				"TV":         []string{"tv", "television", "sonarr", "series", "shows", "serien", "anime"},
				"Games":      []string{"games", "gaming", "pc-games"},
				"Software":   []string{"software", "apps", "programs"},
				"Music":      []string{"music", "audio", "mp3", "flac"},
				"Audiobooks": []string{"audiobooks", "hoerbuch", "abook"},
				"Books":      []string{"books", "ebooks", "epub"},
				"Other":      []string{"other", "misc"},
			},
			ExcludedCategories: []string{}, // Categories to exclude from processing
			PostProcessing: PostProcessingConfig{
				Global: PostProcessCommand{
					Command:   "",
					Arguments: nil,
					Enabled:   false,
				},
				Categories: make(map[string]PostProcessCommand),
			},
			Umlautadaptarr: UmlautadaptarrConfig{
				Enabled: false,
				BaseURL: "http://localhost:5050",
			},
		}

		configData, err := json.MarshalIndent(defaultConfig, "", "  ")
		if err != nil {
			return nil, err
		}

		if err := os.WriteFile(configPath, configData, 0644); err != nil {
			return nil, err
		}

		log.Printf("Created default config file at %s. Please update your API key.", configPath)
		return nil, fmt.Errorf("please update the API key in %s", configPath)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	if config.APIKey == "YOUR_API_KEY_HERE" {
		return nil, fmt.Errorf("please update the API key in %s", configPath)
	}

	return &config, nil
}

// mapCategory maps SABnzbd category to CrowdNFO category
func mapCategory(config *Config, sabnzbdCategory, releaseName string) string {
	// Clean up the category
	category := strings.TrimSpace(sabnzbdCategory)

	// Check if category is empty or wildcard
	if category == "" || category == "*" {
		return matchCategoryByRegex(releaseName)
	}

	// First try custom mappings from config - search through all CrowdNFO categories
	if config.CategoryMappings != nil {
		for crowdNFOCategory, sabnzbdCategories := range config.CategoryMappings {
			if !isValidCategory(crowdNFOCategory) {
				log.Printf("⚠️ Invalid CrowdNFO category in config: '%s', skipping", crowdNFOCategory)
				continue
			}

			// Check if our SABnzbd category is in the list for this CrowdNFO category
			for _, sabnzbdCat := range sabnzbdCategories {
				if strings.EqualFold(category, sabnzbdCat) {
					log.Printf("🏷️ Category mapped via config -> '%s'", crowdNFOCategory)
					return crowdNFOCategory
				}
			}
		}
	}

	// Try standard mapping (case-insensitive)
	for _, validCat := range validCategories {
		if strings.EqualFold(sabnzbdCategory, validCat) {
			log.Printf("🏷️ Category mapped via built-in mapping -> '%s'", validCat)
			return validCat
		}
	}

	// If no direct mapping found, try regex on release name
	return matchCategoryByRegex(releaseName)
}

// matchCategoryByRegex tries to determine category from release name using built-in regex patterns
func matchCategoryByRegex(releaseName string) string {
	for _, regexRule := range categoryRegexPatterns {
		regex, err := regexp.Compile(regexRule.Pattern)
		if err != nil {
			log.Printf("⚠️ Invalid built-in regex pattern '%s': %v", regexRule.Pattern, err)
			continue
		}

		if regex.MatchString(releaseName) {
			log.Printf("🏷️ Category matched via regex -> '%s'", regexRule.Category)
			return regexRule.Category
		}
	}

	log.Printf("⚠️ Could not detect category")
	return ""
}

// isValidCategory checks if category is valid for CrowdNFO
func isValidCategory(category string) bool {
	for _, validCat := range validCategories {
		if category == validCat {
			return true
		}
	}
	return false
}

// isCategoryExcluded checks if a category should be excluded from processing
// Uses case-insensitive matching against the excluded_categories list
func isCategoryExcluded(config *Config, category string) bool {
	if len(config.ExcludedCategories) == 0 {
		return false
	}

	// Check if the category matches any excluded category (case-insensitive)
	for _, excludedCategory := range config.ExcludedCategories {
		if strings.EqualFold(category, excludedCategory) {
			return true
		}
	}

	return false
}

func getCurrentDir() string {
	dir, err := os.Executable()
	if err != nil {
		log.Fatal("Failed to get executable directory:", err)
	}
	return filepath.Dir(dir)
}
