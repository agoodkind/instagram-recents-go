package main

import (
	"fmt"

	"net"
	"os"
	"strconv"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

func main() {
	cfg := LoadConfig()

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
