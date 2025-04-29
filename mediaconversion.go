package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/disintegration/imaging"
	"github.com/kolesa-team/go-webp/encoder"
	"github.com/kolesa-team/go-webp/webp"
	"github.com/relvacode/iso8601"
)

// ImageVersionEntry represents information about a converted file
type ImageVersionEntry struct {
	FileName string `json:"file_name"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
}

// MediaFileEntry represents a single media entry with original and versions
type MediaFileEntry struct {
	MediaID   string                       `json:"media_id"`
	Timestamp string                       `json:"timestamp"`
	Versions  map[string]ImageVersionEntry `json:"versions"`
}

// Standard image sizes to generate
var imageVersions = []struct {
	Width int
	Name  string
}{
	{Width: 1024, Name: "large"},
	{Width: 768, Name: "medium"},
	{Width: 384, Name: "small"},
	{Width: 256, Name: "thumb"},
}

func timestampCompare(i, j MediaFileEntry) int {
	// converrt timestamp to int
	// timestamp is in format 2025-04-16T15:58:54+0000
	timestampI, err := iso8601.ParseString(i.Timestamp)
	if err != nil {
		return 1 // i comes after j if i's timestamp is invalid
	}
	timestampJ, err := iso8601.ParseString(j.Timestamp)
	if err != nil {
		return -1 // j comes after i if j's timestamp is invalid
	}

	if timestampI.After(timestampJ) {
		return -1 // i comes before j (descending order)
	} else if timestampI.Before(timestampJ) {
		return 1 // j comes before i (descending order)
	}
	return 0 // equal timestamps
}

// downloadImageToBytes downloads a file from a URL into memory
func downloadImageToBytes(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status: %s", resp.Status)
	}

	return io.ReadAll(resp.Body)
}

// ResizeByWidthWebP resizes an image and converts it to WebP format
// Write the image to the destination path
// Returns the actual height of the resized image
type ResizeRes struct {
	Height   int
	Width    int
	FileName string
	Error    error
}

// resizeImageBytesByWidthWebP resizes an in-memory image and converts it to WebP
func resizeImageBytesByWidthWebP(imageData []byte, width, height int, baseFileName, outputDir, name string) ResizeRes {
	// Open the source image from memory
	src, err := imaging.Decode(bytes.NewReader(imageData))
	if err != nil {
		return ResizeRes{0, 0, "", fmt.Errorf("failed to decode image: %w", err)}
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

	destFileName := fmt.Sprintf("%s_%dw_%s.webp", baseFileName, width, name)
	destPath := filepath.Join(outputDir, destFileName)

	// Create output file
	output, err := os.Create(destPath)
	if err != nil {
		return ResizeRes{0, 0, "", fmt.Errorf("failed to create output file: %w", err)}
	}
	defer output.Close()

	// Configure WebP encoder and save the image
	options, err := encoder.NewLossyEncoderOptions(encoder.PresetDefault, 80)
	if err != nil {
		return ResizeRes{0, 0, "", fmt.Errorf("failed to create encoder options: %w", err)}
	}

	if err := webp.Encode(output, resized, options); err != nil {
		return ResizeRes{actualHeight, width, destFileName, fmt.Errorf("failed to encode to WebP: %w", err)}
	}

	return ResizeRes{actualHeight, width, destFileName, nil}
}

// EnsureDirectoryExists creates a directory if it doesn't exist
func EnsureDirectoryExists(path string) error {
	return os.MkdirAll(path, 0755)
}

// processImage downloads an image and converts it to multiple WebP sizes
func processImage(url, mediaID, mediaDir string) ([]ImageVersionEntry, error) {
	var versions []ImageVersionEntry

	// Ensure media directory exists
	if err := EnsureDirectoryExists(mediaDir); err != nil {
		return nil, err
	}

	// Download original file to memory
	imageData, err := downloadImageToBytes(url)
	if err != nil {
		return nil, fmt.Errorf("download failed: %w", err)
	}

	// Process each image size directly from memory
	for _, size := range imageVersions {
		resizeRes := resizeImageBytesByWidthWebP(imageData, size.Width, 0, mediaID, mediaDir, size.Name)
		if resizeRes.Error != nil {
			return nil, fmt.Errorf("failed to resize and convert to WebP: %w", resizeRes.Error)
		}

		// Create file info for this size
		webpInfo := ImageVersionEntry{
			FileName: resizeRes.FileName,
			Width:    size.Width,
			Height:   resizeRes.Height,
		}

		versions = append(versions, webpInfo)
		fmt.Printf("Created %s (%dx%d)\n", webpInfo.FileName, webpInfo.Width, webpInfo.Height)
	}

	return versions, nil
}

// processMedia handles downloading, converting, and tracking a single media item
func processMedia(media Media, mediaDir string) ([]ImageVersionEntry, error) {
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
	files, err := processImage(url, media.ID, mediaDir)
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

			convertedFiles, err := processMedia(media, mediaDir)
			if err != nil {
				fmt.Printf("Error processing media %s: %v\n", media.ID, err)
				return
			}

			// Skip nil results (non-image files)
			if convertedFiles == nil {
				atomic.AddInt32(&skippedCountAtomic, 1)
				return
			}

			versionMap := make(map[string]ImageVersionEntry)
			for _, file := range convertedFiles {
				// Find and store the corresponding size name
				for _, size := range imageVersions {
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
	slices.SortFunc(mediaFilesArray, timestampCompare)

	// Update the counts
	skippedCount := int(skippedCountAtomic)
	processedCount := int(processedCountAtomic)

	// Create the media files map
	writeMediaInfoJSON(mediaFilesArray, outputDir)
	fmt.Printf("Image processing complete: %d processed, %d skipped\n", processedCount, skippedCount)
}

// writeMediaInfoJSON creates and writes the media info JSON file
func writeMediaInfoJSON(mediaFilesArray []MediaFileEntry, outputDir string) {
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
