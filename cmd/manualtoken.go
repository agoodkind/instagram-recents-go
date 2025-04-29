package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/agoodkind/instagram-recents-go/lib"

	"github.com/spf13/cobra"
)

var fetchMedia bool


// runManualTokenProcess executes the manual token process directly
func runManualTokenProcess(outputDir string) ([]lib.Media, error) {
	// Get env variable INSTAGRAM_DEVELOPMENT_ACCESS_TOKEN
	accessToken := os.Getenv("INSTAGRAM_DEVELOPMENT_ACCESS_TOKEN")
	if accessToken == "" {
		return nil, fmt.Errorf("INSTAGRAM_DEVELOPMENT_ACCESS_TOKEN is not set")
	}

	userId, err := lib.GetUserIdFromToken(accessToken)
	if err != nil {
		return nil, fmt.Errorf("error getting user ID from token: %w", err)
	}

	recentMedia, err := lib.FetchRecentMedia(userId, accessToken)
	if err != nil {
		return nil, fmt.Errorf("error fetching recent media: %w", err)
	}

	recentMediaJSON, err := json.Marshal(recentMedia)
	if err != nil {
		return nil, fmt.Errorf("error marshalling recent media: %w", err)
	}

	// Ensure output directory exists
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("error creating output directory %s: %w", outputDir, err)
	}

	// Write JSON to file
	if err := os.WriteFile(filepath.Join(outputDir, "recent_media.json"), recentMediaJSON, 0644); err != nil {
		return nil, fmt.Errorf("error writing to file %s: %w", outputDir, err)
	}

	fmt.Printf("Successfully wrote recent media data to %s\n", filepath.Join(outputDir, "recent_media.json"))
	return recentMedia, nil
}


// manualTokenCmd represents the manual-token command
var manualTokenCmd = &cobra.Command{
	Use:   "manual-token",
	Short: "Run the manual token process directly",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Running manual token process...")
		recentMedia, err := runManualTokenProcess(outputDir)
		if err != nil {
			fmt.Println("Error running manual token process:", err)
			os.Exit(1)
		}
		if fetchMedia {
			fmt.Println("Fetching and transforming media...")
			lib.FetchAndTransformImages(recentMedia, mediaDir, outputDir)
		}
	},
}

func init() {
	rootCmd.AddCommand(manualTokenCmd)
	
	// Add local flags for this command
	manualTokenCmd.Flags().BoolVar(&fetchMedia, "fetch-media", false, "Fetch and transform media after getting token")
} 