package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/agoodkind/instagram-recents-go/lib"
	"github.com/spf13/cobra"
)

// fetchMediaCmd represents the fetch-media command
var fetchMediaCmd = &cobra.Command{
	Use:   "fetch-media",
	Short: "Fetch and transform media from a JSON file",
	Run: func(cmd *cobra.Command, args []string) {
		var recentMedia []lib.Media

		if jsonFile != "" {
			// Read from JSON file
			jsonData, err := os.ReadFile(jsonFile)
			if err != nil {
				fmt.Printf("Error reading JSON file %s: %v\n", jsonFile, err)
				os.Exit(1)
			}

			if err := json.Unmarshal(jsonData, &recentMedia); err != nil {
				fmt.Printf("Error parsing JSON from file %s: %v\n", jsonFile, err)
				os.Exit(1)
			}
			fmt.Printf("Successfully loaded media data from %s\n", jsonFile)
		} else {
			fmt.Println("No JSON file specified. Please use --json-file flag to provide a JSON file path.")
			os.Exit(1)
		}

		fmt.Println("Fetching and transforming media...")
		lib.FetchAndTransformImages(recentMedia, mediaDir, outputDir)
	},
}

func init() {
	rootCmd.AddCommand(fetchMediaCmd)
} 