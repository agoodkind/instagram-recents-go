package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	// Common flags
	outputDir string
	mediaDir  string
	jsonFile  string
	picsumLimit int
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "instagram-recents-go",
	Short: "A tool to download and transform Instagram recent media",
	Long: `Instagram Recents Go is a tool to manage your Instagram media.
It can authenticate with Instagram, download your recent media,
transform the images, and display them in a web interface.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	// Define common flags that can be used by multiple commands
	rootCmd.PersistentFlags().StringVar(&outputDir, "output-dir", "./output", "Directory to save output files")
	rootCmd.PersistentFlags().StringVar(&mediaDir, "media-dir", "./output/media", "Directory to save media files")
	rootCmd.PersistentFlags().StringVar(&jsonFile, "json-file", "./output/recent_media.json", "Path to recent_media.json file")
	rootCmd.PersistentFlags().IntVar(&picsumLimit, "picsum-limit", 10, "Number of images to fetch from Picsum Photos API (max 100)")
} 