package main

import (
	"encoding/json"
	"fmt"
	"image"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/disintegration/imaging"
	"github.com/kolesa-team/go-webp/encoder"
	"github.com/kolesa-team/go-webp/webp"
)

// ImageSize defines dimensions for image conversion
type ImageSize struct {
	Width  int
	Height int
}

// ConvertedFileInfo represents information about a converted file
type ConvertedFileInfo struct {
	MediaID   string `json:"media_id"`
	FileName  string `json:"file_name"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	Format    string `json:"-"` // Excluded from JSON
	Path      string `json:"-"` // Excluded from JSON
	Size      int64  `json:"-"` // Excluded from JSON
	Original  bool   `json:"-"` // Excluded from JSON
	MediaType string `json:"-"` // Excluded from JSON
}

// MediaFilesMap represents a mapping from original files to their converted versions
type MediaFilesMap map[string]struct {
	Original ConvertedFileInfo   `json:"original"`
	Versions []ConvertedFileInfo `json:"versions"`
}

// Standard image sizes to generate
var imageSizes = []ImageSize{
	{Width: 1200, Height: 0},
	{Width: 800, Height: 0},
	{Width: 400, Height: 0},
	{Width: 150, Height: 0},
}

// Maximum number of concurrent operations
const (
	maxConcurrentSizes  = 4 // Max concurrent image size processing
	maxConcurrentMedia  = 3 // Max concurrent media items processing
)

// DownloadFile downloads a file from a URL to a local file
func DownloadFile(url, filepath string) error {
	out, err := os.Create(filepath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer out.Close()
	
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	} 
	
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}
	
	return nil
}

// ResizeAndConvertToWebP resizes an image and converts it to WebP format
func ResizeAndConvertToWebP(srcPath, destPath string, width, height int) (int, error) {
	// Open the source image
	src, err := imaging.Open(srcPath)
	if err != nil {
		return 0, fmt.Errorf("failed to open image: %w", err)
	}

	// Resize the image preserving aspect ratio
	var resized image.Image
	if height == 0 {
		resized = imaging.Resize(src, width, 0, imaging.Lanczos)
	} else if width == 0 {
		resized = imaging.Resize(src, 0, height, imaging.Lanczos)
	} else {
		resized = imaging.Resize(src, width, height, imaging.Lanczos)
	}
	
	actualHeight := resized.Bounds().Dy()

	// Create output file
	output, err := os.Create(destPath)
	if err != nil {
		return 0, fmt.Errorf("failed to create output file: %w", err)
	}
	defer output.Close()

	// Configure WebP encoder and save the image
	options, err := encoder.NewLossyEncoderOptions(encoder.PresetDefault, 80)
	if err != nil {
		return 0, fmt.Errorf("failed to create encoder options: %w", err)
	}

	if err := webp.Encode(output, resized, options); err != nil {
		return 0, fmt.Errorf("failed to encode to WebP: %w", err)
	}

	return actualHeight, nil
}

// GetMediaType determines the media type from a URL
func GetMediaType(url string) (string, string) {
	fileExt := ".jpg" // Default extension
	mediaType := "image" // Default type
	
	if strings.Contains(url, ".mp4") {
		fileExt = ".mp4"
		mediaType = "video"
	} else if strings.Contains(url, ".jpg") {
		fileExt = ".jpg"
	} else if strings.Contains(url, ".png") {
		fileExt = ".png"
	}
	
	return fileExt, mediaType
}

// GetImageDimensions returns the width and height of an image
func GetImageDimensions(imagePath string) (int, int, error) {
	img, err := imaging.Open(imagePath)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to open image: %w", err)
	}
	
	bounds := img.Bounds()
	return bounds.Dx(), bounds.Dy(), nil
}

// CreateFileInfo creates a ConvertedFileInfo struct from file details
func CreateFileInfo(mediaID, filePath, format, mediaType string, width, height int, isOriginal bool) (ConvertedFileInfo, error) {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return ConvertedFileInfo{}, fmt.Errorf("failed to get file info: %w", err)
	}
	
	return ConvertedFileInfo{
		MediaID:   mediaID,
		FileName:  filepath.Base(filePath),
		Width:     width,
		Height:    height,
		Format:    format,
		Path:      filePath,
		Size:      fileInfo.Size(),
		Original:  isOriginal,
		MediaType: mediaType,
	}, nil
}

// EnsureDirectoryExists creates a directory if it doesn't exist
func EnsureDirectoryExists(path string) error {
	return os.MkdirAll(path, 0755)
}

// ProcessImage downloads an image and converts it to multiple WebP sizes
func ProcessImage(url, mediaID, mediaDir string) ([]ConvertedFileInfo, error) {
	var convertedFiles []ConvertedFileInfo
	
	// Determine file type and media type
	fileExt, mediaType := GetMediaType(url)
	
	// Skip video files
	if mediaType == "video" {
		return nil, fmt.Errorf("video files are not processed for WebP conversion")
	}
	
	// Setup directories
	originalDir := filepath.Join(mediaDir, "original")
	webpDir := filepath.Join(mediaDir, "webp")
	
	if err := EnsureDirectoryExists(originalDir); err != nil {
		return nil, err
	}
	
	if err := EnsureDirectoryExists(webpDir); err != nil {
		return nil, err
	}
	
	// Download original file
	originalPath := filepath.Join(originalDir, mediaID+fileExt)
	if err := DownloadFile(url, originalPath); err != nil {
		return nil, fmt.Errorf("download failed: %w", err)
	}
	
	// Get original dimensions
	width, height, err := GetImageDimensions(originalPath)
	if err != nil {
		return nil, err
	}
	
	// Create original file info
	originalInfo, err := CreateFileInfo(
		mediaID, 
		originalPath, 
		strings.TrimPrefix(fileExt, "."), 
		mediaType, 
		width, 
		height,  
		true,
	)
	if err != nil {
		return nil, err
	}
	
	convertedFiles = append(convertedFiles, originalInfo)
	
	// Process each image size in parallel
	var wg sync.WaitGroup
	var mu sync.Mutex
	semaphore := make(chan struct{}, maxConcurrentSizes)
	
	for _, size := range imageSizes {
		wg.Add(1)
		currentSize := size
		
		go func() {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			
			// Create temporary file to get dimensions after resize
			tempFile := filepath.Join(os.TempDir(), fmt.Sprintf("temp_%d.webp", currentSize.Width))
			actualHeight, err := ResizeAndConvertToWebP(originalPath, tempFile, currentSize.Width, currentSize.Height)
			if err != nil {
				fmt.Printf("Error calculating dimensions for %s at size %d: %v\n", mediaID, currentSize.Width, err)
				return
			}
			
			// Create final WebP file
			webpFilename := fmt.Sprintf("%s_%dx%d.webp", mediaID, currentSize.Width, actualHeight)
			webpPath := filepath.Join(webpDir, webpFilename)
			
			_, err = ResizeAndConvertToWebP(originalPath, webpPath, currentSize.Width, currentSize.Height)
			if err != nil {
				fmt.Printf("Error processing size %dx%d for %s: %v\n", currentSize.Width, actualHeight, mediaID, err)
				return
			}
			
			// Create file info for this size
			webpInfo, err := CreateFileInfo(
				mediaID,
				webpPath,
				"webp",
				mediaType,
				currentSize.Width,
				actualHeight,
				false,
			)
			if err != nil {
				fmt.Printf("Error creating file info for %s: %v\n", webpPath, err)
				return
			}
			
			mu.Lock()
			convertedFiles = append(convertedFiles, webpInfo)
			mu.Unlock()
			
			fmt.Printf("Created %s (%dx%d)\n", webpFilename, currentSize.Width, actualHeight)
		}()
	}
	
	wg.Wait()
	return convertedFiles, nil
}

// ProcessVideoFile downloads and tracks a video file
func ProcessVideoFile(url, mediaID, mediaDir string) ([]ConvertedFileInfo, error) {
	var convertedFiles []ConvertedFileInfo
	
	fileExt, mediaType := GetMediaType(url)
	originalDir := filepath.Join(mediaDir, "original")
	
	if err := EnsureDirectoryExists(originalDir); err != nil {
		return nil, err
	}
	
	// Download the file
	filename := filepath.Join(originalDir, mediaID+fileExt)
	if err := DownloadFile(url, filename); err != nil {
		return nil, fmt.Errorf("download failed: %w", err)
	}
	
	// Create file info (no dimensions for video)
	videoInfo, err := CreateFileInfo(
		mediaID,
		filename,
		strings.TrimPrefix(fileExt, "."),
		mediaType,
		0, // No width for video
		0, // No height for video
		true,
	)
	if err != nil {
		return nil, err
	}
	
	convertedFiles = append(convertedFiles, videoInfo)
	fmt.Printf("Successfully downloaded to %s\n", filename)
	
	return convertedFiles, nil
}

// ProcessMedia handles downloading, converting, and tracking a single media item
func ProcessMedia(media Media, mediaDir string) ([]ConvertedFileInfo, error) {
	// Determine which URL to use
	var url string
	if media.ThumbnailURL != "" {
		url = media.ThumbnailURL
		fmt.Printf("Processing thumbnail for %s\n", media.ID)
	} else if media.MediaURL != "" {
		url = media.MediaURL
		fmt.Printf("Processing media for %s\n", media.ID)
	} else {
		return nil, fmt.Errorf("no URL available for media %s", media.ID)
	}
	
	// Process based on media type
	fileExt, _ := GetMediaType(url)
	if fileExt == ".mp4" {
		return ProcessVideoFile(url, media.ID, mediaDir)
	} else {
		return ProcessImage(url, media.ID, mediaDir)
	}
}

// FetchAndTransformMedia downloads and processes multiple media items
func FetchAndTransformMedia(recentMedia []Media, mediaDir string) {
	if err := EnsureDirectoryExists(mediaDir); err != nil {
		fmt.Printf("Error creating media directory: %v\n", err)
		return
	}
	
	fmt.Printf("Downloading and processing %d media items...\n", len(recentMedia))
	
	var allConvertedFiles []ConvertedFileInfo
	var mu sync.Mutex
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, maxConcurrentMedia)
	
	for i, media := range recentMedia {
		wg.Add(1)
		index := i
		currentMedia := media
		
		go func() {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			
			fmt.Printf("[%d/%d] Processing media ID: %s\n", index+1, len(recentMedia), currentMedia.ID)
			
			convertedFiles, err := ProcessMedia(currentMedia, mediaDir)
			if err != nil {
				fmt.Printf("Error processing media %s: %v\n", currentMedia.ID, err)
				return
			}
			
			mu.Lock()
			allConvertedFiles = append(allConvertedFiles, convertedFiles...)
			mu.Unlock()
		}()
	}
	
	wg.Wait()
	
	// Create the media files map
	WriteMediaInfoJSON(allConvertedFiles, mediaDir)
	fmt.Println("Media download and processing complete")
}

// WriteMediaInfoJSON creates and writes the media info JSON file
func WriteMediaInfoJSON(allConvertedFiles []ConvertedFileInfo, mediaDir string) {
	mediaFilesMap := make(MediaFilesMap)
	
	// First pass: collect all original files
	for _, file := range allConvertedFiles {
		if file.Original {
			mediaFilesMap[file.MediaID] = struct {
				Original ConvertedFileInfo   `json:"original"`
				Versions []ConvertedFileInfo `json:"versions"`
			}{
				Original: file,
				Versions: []ConvertedFileInfo{},
			}
		}
	}
	
	// Second pass: add converted versions to their originals
	for _, file := range allConvertedFiles {
		if !file.Original {
			entry := mediaFilesMap[file.MediaID]
			entry.Versions = append(entry.Versions, file)
			mediaFilesMap[file.MediaID] = entry
		}
	}
	
	// Write the JSON file
	mediaInfoPath := filepath.Join(mediaDir, "media_info.json")
	mediaInfoJSON, err := json.MarshalIndent(mediaFilesMap, "", "  ")
	if err != nil {
		fmt.Printf("Error creating JSON: %v\n", err)
		return
	}
	
	if err := os.WriteFile(mediaInfoPath, mediaInfoJSON, 0644); err != nil {
		fmt.Printf("Error writing media info JSON to %s: %v\n", mediaInfoPath, err)
		return
	}
	
	fmt.Printf("Successfully wrote media info to %s\n", mediaInfoPath)
}
