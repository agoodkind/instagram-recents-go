package main

import (
	"encoding/json"
	"fmt"
	"image"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/disintegration/imaging"
	"github.com/kolesa-team/go-webp/encoder"
	"github.com/kolesa-team/go-webp/webp"
)

// ImageSize defines dimensions for image conversion
type ImageSize struct {
	Width  int
	Height int
	Name   string
}

// ConvertedFileInfo represents information about a converted file
type ConvertedFileInfo struct {
	FileName  string `json:"file_name"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	Timestamp string `json:"timestamp"` // Added timestamp field
	Format    string `json:"-"` // Excluded from JSON
	Path      string `json:"-"` // Excluded from JSON
	Size      int64  `json:"-"` // Excluded from JSON
	Original  bool   `json:"-"` // Excluded from JSON
	MediaType string `json:"-"` // Excluded from JSON (always "image" now)
	MediaID   string `json:"-"` // Excluded from JSON
}

// MediaFileEntry represents a single media entry with original and versions
type MediaFileEntry struct {
	MediaID   string             `json:"media_id"`
	Timestamp string             `json:"timestamp"`
	Versions  map[string]ConvertedFileInfo `json:"versions"`
}

// MediaFilesArray represents an array of media file entries
type MediaFilesArray []MediaFileEntry

// Standard image sizes to generate
var imageSizes = []ImageSize{
	{Width: 1200, Height: 0, Name: "large"},
	{Width: 800, Height: 0, Name: "medium"},
	{Width: 400, Height: 0, Name: "thumb"},
	{Width: 160, Height: 0, Name: "small"},
}

// SortMediaByTimestamp sorts media items by timestamp in descending order (newest first)
func SortMediaByTimestamp(mediaItems []Media) {
	sort.Slice(mediaItems, func(i, j int) bool {
		// Parse timestamps (format: 2023-04-21T12:34:56+0000)
		timeI, errI := time.Parse(time.RFC3339, mediaItems[i].Timestamp)
		timeJ, errJ := time.Parse(time.RFC3339, mediaItems[j].Timestamp)
		
		// If parsing fails, move items to the end
		if errI != nil {
			return false
		}
		if errJ != nil {
			return true
		}
		
		// Sort in descending order (newest first)
		return timeI.After(timeJ)
	})
}

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
	
	if strings.Contains(url, ".jpg") {
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
	
	// Ensure media directory exists
	if err := EnsureDirectoryExists(mediaDir); err != nil {
		return nil, err
	}
	
	// Download original file
	originalPath := filepath.Join(mediaDir, mediaID+fileExt)
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
	
	// Process each image size sequentially
	for _, size := range imageSizes {
		// Create temporary file to get dimensions after resize
		tempFile := filepath.Join(os.TempDir(), fmt.Sprintf("temp_%d.webp", size.Width))
		actualHeight, err := ResizeAndConvertToWebP(originalPath, tempFile, size.Width, size.Height)
		if err != nil {
			fmt.Printf("Error calculating dimensions for %s at size %d: %v\n", mediaID, size.Width, err)
			continue
		}
		
		// Create final WebP file
		webpFilename := fmt.Sprintf("%s_%dx%d.webp", mediaID, size.Width, actualHeight)
		webpPath := filepath.Join(mediaDir, webpFilename)
		
		_, err = ResizeAndConvertToWebP(originalPath, webpPath, size.Width, size.Height)
		if err != nil {
			fmt.Printf("Error processing size %dx%d for %s: %v\n", size.Width, actualHeight, mediaID, err)
			continue
		}
		
		// Create file info for this size
		webpInfo, err := CreateFileInfo(
			mediaID,
			webpPath,
			"webp",
			mediaType,
			size.Width,
			actualHeight,
			false,
		)
		if err != nil {
			fmt.Printf("Error creating file info for %s: %v\n", webpPath, err)
			continue
		}
		
		convertedFiles = append(convertedFiles, webpInfo)
		fmt.Printf("Created %s (%dx%d)\n", webpFilename, size.Width, actualHeight)
	}
	
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
	
	// Skip non-image media (like videos)
	if strings.Contains(url, ".mp4") {
		fmt.Printf("Skipping non-image file: %s\n", media.ID)
		return nil, nil
	}
	
	// Process the image
	files, err := ProcessImage(url, media.ID, mediaDir)
	if err != nil {
		return nil, err
	}
	
	// Add timestamp only to the original file (for sorting later)
	for i := range files {
		if files[i].Original {
			files[i].Timestamp = media.Timestamp
			break
		}
	}
	
	return files, nil
}

// FetchAndTransformMedia downloads and processes multiple image items
func FetchAndTransformMedia(recentMedia []Media, mediaDir string, outputDir string) {
	if err := EnsureDirectoryExists(mediaDir); err != nil {
		fmt.Printf("Error creating media directory: %v\n", err)
		return
	}
	
	fmt.Printf("Downloading and processing %d media items...\n", len(recentMedia))
	
	// Sort media by timestamp (newest first)
	SortMediaByTimestamp(recentMedia)
	
	var allConvertedFiles []ConvertedFileInfo
	var processedCount int
	var skippedCount int
	
	for i, media := range recentMedia {
		fmt.Printf("[%d/%d] Processing media ID: %s\n", i+1, len(recentMedia), media.ID)
		
		convertedFiles, err := ProcessMedia(media, mediaDir)
		if err != nil {
			fmt.Printf("Error processing media %s: %v\n", media.ID, err)
			continue
		}
		
		// Skip nil results (non-image files)
		if convertedFiles == nil {
			skippedCount++
			continue
		}
		
		allConvertedFiles = append(allConvertedFiles, convertedFiles...)
		processedCount++
	}
	
	// Create the media files map
	WriteMediaInfoJSON(allConvertedFiles, outputDir)
	fmt.Printf("Image processing complete: %d processed, %d skipped\n", processedCount, skippedCount)
}

// WriteMediaInfoJSON creates and writes the media info JSON file
func WriteMediaInfoJSON(allConvertedFiles []ConvertedFileInfo, outputDir string) {
	// Create a temporary map to organize the files
	tempMap := make(map[string]struct {
		Original ConvertedFileInfo
		Versions map[string]ConvertedFileInfo
		Timestamp string // Added timestamp for sorting
	})
	
	// First pass: collect all original files and initialize versions map
	for _, file := range allConvertedFiles {
		if file.Original {
			tempMap[file.MediaID] = struct {
				Original ConvertedFileInfo
				Versions map[string]ConvertedFileInfo
				Timestamp string
			}{
				Original: file,
				Versions: make(map[string]ConvertedFileInfo),
				Timestamp: file.Timestamp, // Store timestamp for sorting
			}
		}
	}
	
	// Second pass: add converted versions to their originals with named sizes
	for _, file := range allConvertedFiles {
		if !file.Original {
			entry := tempMap[file.MediaID]
			
			// Find the corresponding size name
			sizeName := fmt.Sprintf("size_%d", file.Width) // default
			for _, size := range imageSizes {
				if size.Width == file.Width {
					sizeName = size.Name
					break
				}
			}
			
			// Remove timestamp from version files
			versionFile := file
			versionFile.Timestamp = "" // Clear timestamp from version
			
			entry.Versions[sizeName] = versionFile
			tempMap[file.MediaID] = entry
		}
	}
	
	// Convert map to array format and include original in versions
	mediaFilesArray := make(MediaFilesArray, 0, len(tempMap))
	for mediaID, entry := range tempMap {
		// Include original in the versions map but without timestamp
		originalWithoutTimestamp := entry.Original
		originalWithoutTimestamp.Timestamp = "" // Clear timestamp from the version
		entry.Versions["original"] = originalWithoutTimestamp
		
		mediaFilesArray = append(mediaFilesArray, MediaFileEntry{
			MediaID:   mediaID,
			Timestamp: entry.Timestamp,
			Versions:  entry.Versions,
		})
	}
	
	// Sort the final array by timestamp to maintain chronological order
	sort.Slice(mediaFilesArray, func(i, j int) bool {
		// Get timestamps
		timeI := mediaFilesArray[i].Timestamp
		timeJ := mediaFilesArray[j].Timestamp
		
		// Parse timestamps
		parsedTimeI, errI := time.Parse(time.RFC3339, timeI)
		parsedTimeJ, errJ := time.Parse(time.RFC3339, timeJ)
		
		// If parsing fails, move items to the end
		if errI != nil {
			return false
		}
		if errJ != nil {
			return true
		}
		
		// Sort in descending order (newest first)
		return parsedTimeI.After(parsedTimeJ)
	})
	
	// Create the output directory
	if err := EnsureDirectoryExists(outputDir); err != nil {
		fmt.Printf("Error creating output directory: %v\n", err)
		return
	}

	// Write the JSON file
	mediaInfoPath := filepath.Join(outputDir, "converted_media.json")
	mediaInfoJSON, err := json.MarshalIndent(mediaFilesArray, "", "  ")
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
