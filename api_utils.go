package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SABnzbd API response structure for config check
type SABConfigResponse struct {
	Config struct {
		Misc struct {
			DeobfuscateFinalFilenames interface{} `json:"deobfuscate_final_filenames"`
		} `json:"misc"`
	} `json:"config"`
}

func uploadToCrowdNFO(config *Config, releaseName, sabnzbdCategory, hash, finalDir string, mediaInfoJSON []byte, nfoFile, archiveDir string) error {
	var uploadErrors []string
	var successCount int

	// Map SABnzbd category to CrowdNFO category
	crowdNFOCategory := mapCategory(config, sabnzbdCategory, releaseName)

	// Upload MediaInfo only if available
	if mediaInfoJSON != nil && len(mediaInfoJSON) > 0 {
		if err := uploadFile(config, releaseName, "MediaInfo", "", mediaInfoJSON, hash, crowdNFOCategory, archiveDir); err != nil {
			uploadErrors = append(uploadErrors, fmt.Sprintf("MediaInfo: %v", err))
			log.Printf("❌ MediaInfo upload failed: %v", err)
		} else {
			log.Printf("✅ MediaInfo uploaded successfully")
			successCount++
		}
	} else {
		log.Printf("⏭️ Skipping MediaInfo upload - no MediaInfo data available")
	}

	// Upload NFO if found (independent of MediaInfo upload result)
	if nfoFile != "" {
		nfoData, err := os.ReadFile(nfoFile)
		if err != nil {
			uploadErrors = append(uploadErrors, fmt.Sprintf("NFO: failed to read file - %v", err))
			log.Printf("❌ NFO upload failed: failed to read file - %v", err)
		} else {
			nfoFileName := filepath.Base(nfoFile)
			if err := uploadFile(config, releaseName, "NFO", nfoFileName, nfoData, hash, crowdNFOCategory, archiveDir); err != nil {
				uploadErrors = append(uploadErrors, fmt.Sprintf("NFO: %v", err))
				log.Printf("❌ NFO upload failed: %v", err)
			} else {
				log.Printf("✅ NFO uploaded successfully")
				successCount++
			}
		}
	} else {
		log.Printf("⏭️ No NFO file found to upload")
	}

	// Create and upload file list
	fileListEntries, err := createFileList(finalDir, releaseName)
	if err != nil {
		uploadErrors = append(uploadErrors, fmt.Sprintf("FileList: failed to create file list - %v", err))
		log.Printf("❌ File list creation failed: %v", err)
	} else if len(fileListEntries) > 0 {
		fileListRequest := FileListRequest{
			ReleaseName: releaseName,
			Category:    crowdNFOCategory,
			Entries:     fileListEntries,
		}

		if err := uploadFileList(config, fileListRequest); err != nil {
			uploadErrors = append(uploadErrors, fmt.Sprintf("FileList: %v", err))
			log.Printf("❌ File list upload failed: %v", err)
		} else {
			log.Printf("✅ File list uploaded successfully (%d files)", len(fileListEntries))
			successCount++
		}
	} else {
		log.Printf("⏭️ No files found for file list")
	}

	// Return combined errors if any occurred
	if len(uploadErrors) > 0 {
		if successCount > 0 {
			return fmt.Errorf("partial_failure:%d successful, %d failed", successCount, len(uploadErrors))
		} else {
			return fmt.Errorf("total_failure:%d upload(s) failed", len(uploadErrors))
		}
	}

	return nil
}

func uploadEpisodeToCrowdNFO(config *Config, episodeInfo EpisodeInfo, sabnzbdCategory, hash string, mediaInfoJSON []byte, archiveDir string) error {
	var uploadErrors []string
	var successCount int

	// Map SABnzbd category to CrowdNFO category
	crowdNFOCategory := mapCategory(config, sabnzbdCategory, episodeInfo.ReleaseName)

	// Upload MediaInfo only if available
	if mediaInfoJSON != nil && len(mediaInfoJSON) > 0 {
		if err := uploadFile(config, episodeInfo.ReleaseName, "MediaInfo", "", mediaInfoJSON, hash, crowdNFOCategory, archiveDir); err != nil {
			uploadErrors = append(uploadErrors, fmt.Sprintf("MediaInfo: %v", err))
			log.Printf("❌ MediaInfo upload failed: %v", err)
		} else {
			log.Printf("✅ MediaInfo uploaded successfully")
			successCount++
		}
	} else {
		log.Printf("⏭️ Skipping MediaInfo upload - no MediaInfo data available")
	}

	// Upload NFO if found (independent of MediaInfo upload result)
	if episodeInfo.NFOFile != "" {
		nfoData, err := os.ReadFile(episodeInfo.NFOFile)
		if err != nil {
			uploadErrors = append(uploadErrors, fmt.Sprintf("NFO: failed to read file - %v", err))
			log.Printf("❌ NFO upload failed: failed to read file - %v", err)
		} else {
			nfoFileName := filepath.Base(episodeInfo.NFOFile)
			if err := uploadFile(config, episodeInfo.ReleaseName, "NFO", nfoFileName, nfoData, hash, crowdNFOCategory, archiveDir); err != nil {
				uploadErrors = append(uploadErrors, fmt.Sprintf("NFO: %v", err))
				log.Printf("❌ NFO upload failed: %v", err)
			} else {
				log.Printf("✅ NFO uploaded successfully")
				successCount++
			}
		}
	} else {
		log.Printf("⏭️ No NFO file found to upload")
	}

	// Create and upload episode file list
	fileListEntries, err := createEpisodeFileList(episodeInfo)
	if err != nil {
		uploadErrors = append(uploadErrors, fmt.Sprintf("FileList: failed to create file list - %v", err))
		log.Printf("❌ File list creation failed: %v", err)
	} else if len(fileListEntries) > 0 {
		fileListRequest := FileListRequest{
			ReleaseName: episodeInfo.ReleaseName,
			Category:    crowdNFOCategory,
			Entries:     fileListEntries,
		}

		if err := uploadFileList(config, fileListRequest); err != nil {
			uploadErrors = append(uploadErrors, fmt.Sprintf("FileList: %v", err))
			log.Printf("❌ File list upload failed: %v", err)
		} else {
			log.Printf("✅ File list uploaded successfully (%d files)", len(fileListEntries))
			successCount++
		}
	} else {
		log.Printf("⏭️ No files found for file list")
	}

	// Return combined errors if any occurred
	if len(uploadErrors) > 0 {
		if successCount > 0 {
			return fmt.Errorf("partial_failure:%d successful, %d failed", successCount, len(uploadErrors))
		} else {
			return fmt.Errorf("total_failure:%d upload(s) failed", len(uploadErrors))
		}
	}

	return nil
}

func uploadFile(config *Config, releaseName, fileType, originalFileName string, fileData []byte, hash, category, archiveDir string) error {
	url := fmt.Sprintf("%s/%s/files", config.BaseURL, releaseName)

	// Create multipart form
	var b bytes.Buffer
	writer := multipart.NewWriter(&b)

	// Add form fields
	writer.WriteField("FileType", fileType)
	if originalFileName != "" {
		writer.WriteField("OriginalFileName", originalFileName)
	}
	if category != "" {
		writer.WriteField("Category", category)
	}
	if hash != "" {
		writer.WriteField("FileHash", hash)
	}

	// Add file
	part, err := writer.CreateFormFile("File", getFileName(fileType, releaseName, originalFileName))
	if err != nil {
		return err
	}
	part.Write(fileData)

	writer.Close()

	// Create request
	req, err := http.NewRequest("POST", url, &b)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-Api-Key", config.APIKey)
	req.Header.Set("User-Agent", getUserAgent())

	// Debug: Print request details
	//log.Printf("🔍 DEBUG: HTTP Request Details")
	//log.Printf("   Method: %s", req.Method)
	//log.Printf("   URL: %s", req.URL.String())
	//log.Printf("   Headers:")
	//for name, values := range req.Header {
	//	for _, value := range values {
	//		// Mask API key for security
	//		if name == "X-Api-Key" && len(value) > 8 {
	//			maskedValue := value[:4] + "****" + value[len(value)-4:]
	//			log.Printf("     %s: %s", name, maskedValue)
	//		} else {
	//			log.Printf("     %s: %s", name, value)
	//		}
	//	}
	//}
	//log.Printf("   Content-Length: %d bytes", req.ContentLength)
	//if req.ContentLength == 0 && b.Len() > 0 {
	//	log.Printf("   Body Size: %d bytes", b.Len())
	//}

	// Send request
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Debug: Print response details
	//log.Printf("🔍 DEBUG: HTTP Response Details")
	//log.Printf("   Status: %s (%d)", resp.Status, resp.StatusCode)
	//log.Printf("   Headers:")
	//for name, values := range resp.Header {
	//	for _, value := range values {
	//		log.Printf("     %s: %s", name, value)
	//	}
	//}

	// Check for update headers
	checkUpdateHeaders(resp.Header)

	// Read response body for error details
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %v", err)
	}

	log.Printf("   Body Size: %d bytes", len(body))
	if len(body) > 0 && len(body) < 1000 { // Only log small response bodies
		log.Printf("   Body: %s", string(body))
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("upload failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Archive the uploaded file
	archiveFile := filepath.Join(archiveDir, getFileName(fileType, releaseName, originalFileName))
	if err := os.WriteFile(archiveFile, fileData, 0644); err != nil {
		log.Printf("⚠️ Failed to archive %s: %v", fileType, err)
	}

	return nil
}

func getFileName(fileType, releaseName, originalFileName string) string {
	if fileType == "NFO" && originalFileName != "" {
		return originalFileName
	}
	return fmt.Sprintf("%s.json", releaseName)
}

// checkSABnzbdConfig checks if deobfuscate_final_filenames is set to false
func checkSABnzbdConfig(sabApiUrl, sabApiKey string) error {
	if sabApiUrl == "" || sabApiKey == "" {
		log.Println("⚠️ SABnzbd API URL or API Key not provided, skipping deobfuscate check")
		return nil
	}

	// Construct API URL - handle cases where /api is already included
	baseUrl := strings.TrimSuffix(sabApiUrl, "/")
	var apiUrl string
	if strings.HasSuffix(baseUrl, "/api") {
		// URL already contains /api, just append the query parameters
		apiUrl = fmt.Sprintf("%s?mode=get_config&section=misc&keyword=deobfuscate_final_filenames&apikey=%s",
			baseUrl, sabApiKey)
	} else {
		// URL doesn't contain /api, add it
		apiUrl = fmt.Sprintf("%s/api?mode=get_config&section=misc&keyword=deobfuscate_final_filenames&apikey=%s",
			baseUrl, sabApiKey)
	}

	// Make API request
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(apiUrl)
	if err != nil {
		log.Printf("⚠️ Failed to connect to SABnzbd API, skipping deobfuscate check: %v", err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("⚠️ SABnzbd API returned status %d, skipping deobfuscate check", resp.StatusCode)
		return nil
	}

	// Parse response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("⚠️ Failed to read SABnzbd API response, skipping deobfuscate check: %v", err)
		return nil
	}

	var response SABConfigResponse
	if err := json.Unmarshal(body, &response); err != nil {
		log.Printf("⚠️ Failed to parse SABnzbd API response, skipping deobfuscate check: %v", err)
		return nil
	}

	// Check the setting
	deobfuscateValue := response.Config.Misc.DeobfuscateFinalFilenames
	if deobfuscateValue == "1" || strings.ToLower(fmt.Sprintf("%v", deobfuscateValue)) == "true" {
		return fmt.Errorf("Deobfuscate final filenames is enabled in SABnzbd. Please disable this setting for crowdNFO post-processing.")
	}
	return nil
}

// uploadFileList uploads a file list to CrowdNFO
func uploadFileList(config *Config, fileListRequest FileListRequest) error {
	url := fmt.Sprintf("%s/%s/filelists", config.BaseURL, fileListRequest.ReleaseName)

	// Convert to JSON
	jsonData, err := json.Marshal(fileListRequest)
	if err != nil {
		return fmt.Errorf("failed to marshal file list: %v", err)
	}

	// Create request
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", config.APIKey)
	req.Header.Set("User-Agent", getUserAgent())

	//// Debug: Print request details
	//log.Printf("🔍 DEBUG: File List HTTP Request Details")
	//log.Printf("   Method: %s", req.Method)
	//log.Printf("   URL: %s", req.URL.String())
	//log.Printf("   Headers:")
	//for name, values := range req.Header {
	//	for _, value := range values {
	//		// Mask API key for security
	//		if name == "X-Api-Key" && len(value) > 8 {
	//			maskedValue := value[:4] + "****" + value[len(value)-4:]
	//			log.Printf("     %s: %s", name, maskedValue)
	//		} else {
	//			log.Printf("     %s: %s", name, value)
	//		}
	//	}
	//}
	//log.Printf("   Content-Length: %d bytes", len(jsonData))

	// Send request
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Debug: Print response details
	//log.Printf("🔍 DEBUG: File List HTTP Response Details")
	//log.Printf("   Status: %s (%d)", resp.Status, resp.StatusCode)
	//log.Printf("   Headers:")
	//for name, values := range resp.Header {
	//	for _, value := range values {
	//		log.Printf("     %s: %s", name, value)
	//	}
	//}
	//log.Printf("   Content-Length: %d bytes", resp.ContentLength)

	// Check response
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode == http.StatusUnauthorized {
			return fmt.Errorf("unauthorized: please check your API key in config.json")
		}
		if resp.StatusCode == http.StatusBadRequest {
			return fmt.Errorf("%s", string(body))
		}
		return fmt.Errorf("file list upload failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// UmlautadaptarrResponse represents the response from Umlautadaptarr API
type UmlautadaptarrResponse struct {
	ChangedTitle  string `json:"changedTitle"`
	OriginalTitle string `json:"originalTitle"`
}

// checkUmlautadaptarr checks if the release name was modified by Umlautadaptarr
// Returns the original title if it was changed, or empty string if no change
func checkUmlautadaptarr(config *Config, releaseName string) (string, error) {
	// Skip if disabled
	if !config.Umlautadaptarr.Enabled {
		return "", nil
	}

	// Use default URL if not configured
	baseURL := config.Umlautadaptarr.BaseURL
	if baseURL == "" {
		baseURL = "http://localhost:5005"
	}

	// URL encode the release name
	encodedReleaseName := url.QueryEscape(releaseName)
	apiURL := fmt.Sprintf("%s/titlelookup?changedTitle=%s", baseURL, encodedReleaseName)

	// Create HTTP request
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(apiURL)
	if err != nil {
		// Check if this looks like a Docker networking issue
		if strings.Contains(err.Error(), "connection refused") && (strings.Contains(baseURL, "localhost") || strings.Contains(baseURL, "127.0.0.1")) {
			if isRunningInContainer() {
				return "", fmt.Errorf("failed to connect to UmlautAdaptarr API: %v\n\n💡 Possible container networking issue detected!\nYou're running in a container where localhost refers to the container, not the host.\nTry updating the UmlautAdaptarr settings in your config.json:\n  - \"base_url\": \"http://umlautadaptarr:5005\" (Docker container name, same network required!)\n  - \"base_url\": \"http://<host-ip>:5005\" (Docker host IP)\n  - \"base_url\": \"http://172.17.0.1:5005\" (Docker bridge IP)\n  - \"base_url\": \"http://host.docker.internal:5005\" (Docker Desktop)\n  - Or use --network host when running the container", err)
			} else {
				return "", fmt.Errorf("failed to connect to UmlautAdaptarr API: %v\n\n💡 Make sure UmlautAdaptarr is running on the configured URL", err)
			}
		}
		return "", fmt.Errorf("failed to connect to UmlautAdaptarr API: %v", err)
	}
	defer resp.Body.Close()

	// Handle 404 - title was not changed
	if resp.StatusCode == 404 {
		return "", nil
	}

	// Handle other errors
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("UmlautAdaptarr API error (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse the response
	var umlautResp UmlautadaptarrResponse
	if err := json.NewDecoder(resp.Body).Decode(&umlautResp); err != nil {
		return "", fmt.Errorf("failed to parse UmlautAdaptarr response: %v", err)
	}

	return umlautResp.OriginalTitle, nil
}

// isRunningInContainer detects if the application is running inside a container
func isRunningInContainer() bool {
	// Method 1: Check for /.dockerenv file (most reliable for Docker)
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}

	// Method 2: Check /proc/1/cgroup for container indicators
	if data, err := os.ReadFile("/proc/1/cgroup"); err == nil {
		content := string(data)
		// Look for container runtime indicators
		if strings.Contains(content, "/docker/") ||
			strings.Contains(content, "/containerd/") ||
			strings.Contains(content, "/lxc/") ||
			strings.Contains(content, "docker-") ||
			strings.Contains(content, ".scope") {
			return true
		}
	}

	// Method 3: Check if init process is not actually init (less reliable)
	if data, err := os.ReadFile("/proc/1/comm"); err == nil {
		comm := strings.TrimSpace(string(data))
		// In containers, PID 1 is often not the real init
		if comm != "init" && comm != "systemd" {
			return true
		}
	}

	return false
}

// checkUpdateHeaders checks response headers for update notifications
func checkUpdateHeaders(headers http.Header) {
	// Check for X-Client-Update-Available header
	if updateAvailableHeader := headers.Get("X-Client-Update-Available"); updateAvailableHeader != "" {
		if strings.ToLower(updateAvailableHeader) == "true" || updateAvailableHeader == "1" {
			updateAvailable = true
			updateCheckDone = true
		}
	}

	// Check for X-Latest-Version header
	if latestVersionHeader := headers.Get("X-Latest-Version"); latestVersionHeader != "" {
		latestVersion = latestVersionHeader
		updateCheckDone = true
	}
}
