package main

import "os"

type InstagramConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
}

func LoadConfig() InstagramConfig {
	return InstagramConfig{
		ClientID:     os.Getenv("INSTAGRAM_APP_ID"),
		ClientSecret: os.Getenv("INSTAGRAM_APP_SECRET"),
		RedirectURI:  os.Getenv("REDIRECT_URI"),
	}
}
