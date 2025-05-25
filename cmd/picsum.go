package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/agoodkind/instagram-recents-go/lib"

	"github.com/spf13/cobra"
)

type PicsumPhoto struct {
	ID          string `json:"id"`
	Author      string `json:"author"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	URL         string `json:"url"`
	DownloadURL string `json:"download_url"`
}

// fetchPicsumPhotos fetches images from the Picsum Photos API
func fetchPicsumPhotos(limit int) ([]PicsumPhoto, error) {
	url := fmt.Sprintf("https://picsum.photos/v2/list?limit=%d", limit)
	
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status: %s", resp.Status)
	}
	
	var photos []PicsumPhoto
	if err := json.NewDecoder(resp.Body).Decode(&photos); err != nil {
		return nil, fmt.Errorf("failed to decode JSON: %w", err)
	}
	
	return photos, nil
}

// convertPicsumToMedia converts Picsum Photos to Media format
func convertPicsumToMedia(photos []PicsumPhoto) []lib.Media {
	var media []lib.Media
	
	for _, photo := range photos {
		// Create a timestamp for the current time minus a random offset
		// This simulates having photos from different times
		randomOffset := time.Duration(len(media) * 24) * time.Hour
		timestamp := time.Now().Add(-randomOffset).Format(time.RFC3339)
		
		media = append(media, lib.Media{
			ID:        photo.ID,
			MediaType: "IMAGE",
			MediaURL:  photo.DownloadURL,
			Permalink: photo.URL,
			Timestamp: timestamp,
		})
	}
	
	return media
}


// picsumCmd represents the picsum command
var picsumCmd = &cobra.Command{
	Use:   "picsum",
	Short: "Use Picsum Photos API for test images instead of Instagram",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Fetching images from Picsum Photos API...")
		// Limit the number of images to fetch (max 100)
		limit := min(picsumLimit, 100)
		
		picsumPhotos, err := fetchPicsumPhotos(limit)
		if err != nil {
			fmt.Printf("Error fetching images from Picsum Photos API: %v\n", err)
			os.Exit(1)
		}
		
		// Convert Picsum Photos to Media format
		media := convertPicsumToMedia(picsumPhotos)
		
		// Create output directory if it doesn't exist
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			fmt.Printf("Error creating output directory %s: %v\n", outputDir, err)
			os.Exit(1)
		}
		
		// Write JSON to file for reference
		mediaJSON, err := json.MarshalIndent(media, "", "  ")
		if err != nil {
			fmt.Printf("Error marshalling media data: %v\n", err)
			os.Exit(1)
		}
		
		if err := os.WriteFile(filepath.Join(outputDir, "picsum_media.json"), mediaJSON, 0644); err != nil {
			fmt.Printf("Error writing to file %s: %v\n", filepath.Join(outputDir, "picsum_media.json"), err)
			os.Exit(1)
		}
		
		fmt.Println("Fetching and transforming Picsum Photos images...")
		lib.FetchAndTransformImages(media, mediaDir, outputDir)
	},
}

func init() {
	rootCmd.AddCommand(picsumCmd)
} 