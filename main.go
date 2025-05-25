package main

import (
	"github.com/agoodkind/instagram-recents-go/cmd"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env file if it exists
	_ = godotenv.Load()

	// Execute the root command //
	cmd.Execute()
}