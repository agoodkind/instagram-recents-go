package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

func IndexHandler(cfg InstagramConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		authURL := "https://api.instagram.com/oauth/authorize?client_id=" + cfg.ClientID +
			"&redirect_uri=" + cfg.RedirectURI +
			"&scope=user_profile,user_media&response_type=code"
		c.HTML(http.StatusOK, "index.html", gin.H{
			"AuthURL": authURL,
			"DevMode": true, // Flag to show manual token option
		})
	}
}

func AuthCallbackHandler(cfg InstagramConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		code := c.Query("code")

		tokenRes, err := ExchangeCodeForToken(cfg, code)
		if err != nil {
			c.HTML(http.StatusBadRequest, "index.html", gin.H{
				"Error": "Failed to exchange code for token",
			})
			return
		}

		longTokenRes, err := GetLongLivedToken(cfg, tokenRes.AccessToken)
		if err != nil {
			c.HTML(http.StatusBadRequest, "index.html", gin.H{
				"Error": "Failed to get long-lived token",
			})
			return
		}

		accessToken := longTokenRes.AccessToken
		userId := longTokenRes.UserID

		c.JSON(http.StatusOK, gin.H{
			"AccessToken": accessToken,
			"UserID":      userId,
			"ExpiresIn":   longTokenRes.ExpiresIn,
		})
	}
}

// ManualTokenFormHandler New handler for manual token entry form
func ManualTokenFormHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.HTML(http.StatusOK, "manual.html", nil)
	}
}

// ProcessManualTokenHandler New handler to process manually entered token
// Updated handler to process manually entered token
func ProcessManualTokenHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get form data - now only need access token
		accessToken := c.PostForm("access_token")

		if accessToken == "" {
			c.HTML(http.StatusBadRequest, "manual.html", gin.H{
				"Error": "Access token is required",
			})
			return
		}

		// Automatically retrieve user ID using the token
		userId, err := GetUserIdFromToken(accessToken)
		if err != nil {
			c.HTML(http.StatusBadRequest, "manual.html", gin.H{
				"Error": fmt.Sprintf("Invalid token: %v", err),
			})
			return
		}

		recentMedia, nil := FetchRecentMedia(userId, accessToken) // Fetch media to validate token
		recentMediaJSON, err := json.Marshal(recentMedia)
		if !errors.Is(nil, err) {
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}

		c.JSON(http.StatusOK, string(recentMediaJSON))
	}
}
