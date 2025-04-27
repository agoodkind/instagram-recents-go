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
	"sync"
	"sync/atomic"
	"time"

	"github.com/disintegration/imaging"
	"github.com/kolesa-team/go-webp/encoder"
	"github.com/kolesa-team/go-webp/webp"
	"github.com/relvacode/iso8601"
)

// MediaFileVersionEntry represents information about a converted file
type MediaFileVersionEntry struct {
	FileName string `json:"file_name"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
}

// MediaFileEntry represents a single media entry with original and versions
type MediaFileEntry struct {
	MediaID   string                           `json:"media_id"`
	Timestamp string                           `json:"timestamp"`
	Versions  map[string]MediaFileVersionEntry `json:"versions"`
}

// Standard image sizes to generate
var imageWidths = []struct {
	Width int
	Name  string
}{
	{Width: 1200, Name: "large"},
	{Width: 800, Name: "medium"},
	{Width: 400, Name: "thumb"},
	{Width: 160, Name: "small"},
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

// ResizeByWidthWebP resizes an image and converts it to WebP format
// Write the image to the destination path
// Returns the actual height of the resized image
type ResizeRes struct {
	Height int
	FileName string
	Error  error
}

func ResizeByWidthWebP(srcPath string, width, height int) ResizeRes {
	// srcpath without extension
	srcFileName := filepath.Base(srcPath)
	srcFileDir := filepath.Dir(srcPath)
	srcFileNameWithoutExt := strings.TrimSuffix(srcFileName, filepath.Ext(srcPath))

	// Open the source image
	src, err := imaging.Open(srcPath)
	if err != nil {
		return ResizeRes{0, "", fmt.Errorf("failed to open image: %w", err)}
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

	destFileName := fmt.Sprintf("%s_%dx%d.webp", srcFileNameWithoutExt, width, actualHeight)
	destPath := filepath.Join(srcFileDir, destFileName)

	// Create output file
	output, err := os.Create(destPath)
	if err != nil {
		return ResizeRes{0, "", fmt.Errorf("failed to create output file: %w", err)}
	}
	defer output.Close()

	// Configure WebP encoder and save the image
	options, err := encoder.NewLossyEncoderOptions(encoder.PresetDefault, 80)
	if err != nil {
		return ResizeRes{0, "", fmt.Errorf("failed to create encoder options: %w", err)}
	}

	if err := webp.Encode(output, resized, options); err != nil {
		return ResizeRes{actualHeight, destPath, fmt.Errorf("failed to encode to WebP: %w", err)}
	}

	return ResizeRes{actualHeight, destFileName, nil}
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

// EnsureDirectoryExists creates a directory if it doesn't exist
func EnsureDirectoryExists(path string) error {
	return os.MkdirAll(path, 0755)
}

// ProcessImage downloads an image and converts it to multiple WebP sizes
func ProcessImage(url, mediaID, mediaDir string) ([]MediaFileVersionEntry, error) {
	var versions []MediaFileVersionEntry

	// Determine file type and media type
	fileExt := url[strings.LastIndex(url, "."):]

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
	originalInfo := MediaFileVersionEntry{
		FileName: mediaID + fileExt,
		Width:    width,
		Height:   height,
	}

	versions = append(versions, originalInfo)

	// Process each image size sequentially
	for _, size := range imageWidths {

		resizeRes := ResizeByWidthWebP(originalPath,  size.Width, 0)
		if resizeRes.Error != nil {
			return nil, fmt.Errorf("failed to resize and convert to WebP: %w", resizeRes.Error)
		}

		// Create file info for this size
		webpInfo := MediaFileVersionEntry{
			FileName: resizeRes.FileName,
			Width:    size.Width,
			Height:   resizeRes.Height,
		}

		versions = append(versions, webpInfo)
		fmt.Printf("Created %s (%dx%d)\n", webpInfo.FileName, webpInfo.Width, webpInfo.Height)
	}

	return versions, nil
}

// ProcessMedia handles downloading, converting, and tracking a single media item
func ProcessMedia(media Media, mediaDir string) ([]MediaFileVersionEntry, error) {
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

	return files, nil
}

// FetchAndTransformMedia downloads and processes multiple image items
func FetchAndTransformMedia(recentMedia []Media, mediaDir string, outputDir string) {
	if err := EnsureDirectoryExists(mediaDir); err != nil {
		fmt.Printf("Error creating media directory: %v\n", err)
		return
	}

	fmt.Printf("Downloading and processing %d media items...\n", len(recentMedia))

	var wg sync.WaitGroup
	resultChan := make(chan MediaFileEntry, len(recentMedia))
	var skippedCountAtomic, processedCountAtomic int32

	for i, media := range recentMedia {
		wg.Add(1)
		go func(i int, media Media) {
			defer wg.Done()
			fmt.Printf("[%d/%d] Processing media ID: %s\n", i+1, len(recentMedia), media.ID)

			convertedFiles, err := ProcessMedia(media, mediaDir)
			if err != nil {
				fmt.Printf("Error processing media %s: %v\n", media.ID, err)
				return
			}

			// Skip nil results (non-image files)
			if convertedFiles == nil {
				atomic.AddInt32(&skippedCountAtomic, 1)
				return
			}

			versionMap := make(map[string]MediaFileVersionEntry)
			for _, file := range convertedFiles {
				// Find and store the corresponding size name
				for _, size := range imageWidths {
					if size.Width == file.Width {
						versionMap[size.Name] = file
						break
					}
				}
			}

			resultChan <- MediaFileEntry{
				MediaID:   media.ID,
				Timestamp: media.Timestamp,
				Versions:  versionMap,
			}
			atomic.AddInt32(&processedCountAtomic, 1)
		}(i, media)
	}

	// Close the channel once all goroutines are done
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	mediaFilesArray := make([]MediaFileEntry, 0, len(recentMedia))
	for entry := range resultChan {
		mediaFilesArray = append(mediaFilesArray, entry)
	}

	// sort mediaFilesArray by timestamp
	sort.Slice(mediaFilesArray, func(i, j int) bool {
		// converrt timestamp to int
		// timestamp is in format 2025-04-16T15:58:54+0000
		timestampI, err := iso8601.ParseString(mediaFilesArray[i].Timestamp)
		if err != nil {
			return false
		}
		timestampJ, err := iso8601.ParseString(mediaFilesArray[j].Timestamp)
		if err != nil {
			return false
		}

		return timestampI.After(timestampJ)
	})

	// Update the counts
	skippedCount := int(skippedCountAtomic)
	processedCount := int(processedCountAtomic)

	// Create the media files map
	WriteMediaInfoJSON(mediaFilesArray, outputDir)
	fmt.Printf("Image processing complete: %d processed, %d skipped\n", processedCount, skippedCount)
}

// WriteMediaInfoJSON creates and writes the media info JSON file
func WriteMediaInfoJSON(mediaFilesArray []MediaFileEntry, outputDir string) {
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
