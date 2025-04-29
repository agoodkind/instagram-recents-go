package cmd

import (
	"fmt"
	"net"
	"os"
	"strconv"

	"github.com/agoodkind/instagram-recents-go/lib"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
)

// runServer starts the web server with all routes
func runServer(cfg lib.InstagramConfig) {
	router := gin.Default()
	sessionStore := cookie.NewStore([]byte(os.Getenv("SESSION_SECRET")))
	router.Use(sessions.Sessions("instagram-recents-go", sessionStore))
	router.LoadHTMLGlob("templates/*")

	// Define routes
	router.GET("/", lib.IndexHandler(cfg))
	router.GET("/auth/callback", lib.AuthCallbackHandler(cfg))

	// Add new routes for manual token handling
	router.GET("/manual-token", lib.ManualTokenFormHandler())
	router.POST("/manual-token", lib.ProcessManualTokenHandler())

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



// serverCmd represents the server command
var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Run the web server",
	Run: func(cmd *cobra.Command, args []string) {
		cfg := lib.LoadConfig()
		runServer(cfg)
	},
}

func init() {
	rootCmd.AddCommand(serverCmd)
} 