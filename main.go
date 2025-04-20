package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env file if it exists
	_ = godotenv.Load()

	// Define command line flags
	runServerCmd := flag.Bool("server", false, "Run the web server")
	manualTokenCmd := flag.Bool("manual-token", false, "Run the manual token process directly")
	outputDir := flag.String("output", ".", "Directory to save output files (for manual-token mode)")
	flag.Parse()

	// If no flag is provided, show usage
	if !*runServerCmd && !*manualTokenCmd {
		flag.Usage()
		fmt.Println("\nPlease provide a command: --server or --manual-token")
		os.Exit(1)
	}

	if *manualTokenCmd {
		// Direct manual token process
		fmt.Println("Running manual token process...")
		runManualTokenProcess(*outputDir)
		return
	}

	// Default: run the server
	cfg := LoadConfig()
	runServer(cfg)
}

// runManualTokenProcess executes the manual token process directly
func runManualTokenProcess(outputDir string) {
	// Get env variable DEVELOPMENT_ACCESS_TOKEN
	accessToken := os.Getenv("DEVELOPMENT_ACCESS_TOKEN")
	if accessToken == "" {
		fmt.Println("DEVELOPMENT_ACCESS_TOKEN is not set")
		os.Exit(1)
	}

	userId, err := GetUserIdFromToken(accessToken)
	if err != nil {
		fmt.Println("Error getting user ID from token:", err)
		os.Exit(1)
	}

	recentMedia, err := FetchRecentMedia(userId, accessToken)
	if err != nil {
		fmt.Println("Error fetching recent media:", err)
		os.Exit(1)
	}

	recentMediaJSON, err := json.Marshal(recentMedia)
	if err != nil {
		fmt.Println("Error marshalling recent media:", err)
		os.Exit(1)
	}

	// Ensure output directory exists
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		fmt.Printf("Error creating output directory %s: %v\n", outputDir, err)
		os.Exit(1)
	}

	// Write JSON to file
	outputPath := filepath.Join(outputDir, "recent_media.json")
	if err := os.WriteFile(outputPath, recentMediaJSON, 0644); err != nil {
		fmt.Printf("Error writing to file %s: %v\n", outputPath, err)
		os.Exit(1)
	}

	fmt.Printf("Successfully wrote recent media data to %s\n", outputPath)
}

// runServer starts the web server with all routes
func runServer(cfg InstagramConfig) {
	router := gin.Default()
	sessionStore := cookie.NewStore([]byte(os.Getenv("SESSION_SECRET")))
	router.Use(sessions.Sessions("instagram-recents-go", sessionStore))
	router.LoadHTMLGlob("templates/*")

	// Define routes
	router.GET("/", IndexHandler(cfg))
	router.GET("/auth/callback", AuthCallbackHandler(cfg))

	// Add new routes for manual token handling
	router.GET("/manual-token", ManualTokenFormHandler())
	router.POST("/manual-token", ProcessManualTokenHandler())

	// Automatically find an available port starting from 8080
	port := findAvailablePort(8080, 8100)
	if port == -1 {
		fmt.Fprintln(os.Stderr, "No available ports in range 8080-8099")
		os.Exit(1)
	}

	host := "localhost"
	url := fmt.Sprintf("http://%s:%d", host, port)
	fmt.Printf("Server is running at %s\n", url)

	addr := ":" + strconv.Itoa(port)
	if err := router.Run(addr); err != nil {
		panic(err)
	}
}

// findAvailablePort tries to find an available port within a range.
func findAvailablePort(start, end int) int {
	for port := start; port <= end; port++ {
		addr := ":" + strconv.Itoa(port)
		listener, err := net.Listen("tcp", addr)
		if err == nil {
			listener.Close() // Close the listener after finding a free port
			return port
		}
	}
	return -1 // No available ports found
}