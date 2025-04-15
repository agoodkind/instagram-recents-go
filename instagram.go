package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
	"time"
)

type TokenResponse struct {
	AccessToken string `json:"access_token"`
	UserID      string `json:"user_id"`
	ExpiresIn   int    `json:"expires_in"`
}

// Validate a manually entered token by making a test API call
func ValidateManualToken(accessToken string) (bool, error) {
	url := fmt.Sprintf(
		"https://graph.instagram.com/me?fields=id,username&access_token=%s",
		accessToken,
	)
	resp, err := http.Get(url)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("invalid token, API returned status: %d", resp.StatusCode)
	}

	return true, nil
}

func ExchangeCodeForToken(cfg InstagramConfig, code string) (*TokenResponse, error) {
	resp, err := http.PostForm("https://api.instagram.com/oauth/access_token", map[string][]string{
		"client_id":     {cfg.ClientID},
		"client_secret": {cfg.ClientSecret},
		"grant_type":    {"authorization_code"},
		"redirect_uri":  {cfg.RedirectURI},
		"code":          {code},
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var token TokenResponse
	err = json.NewDecoder(resp.Body).Decode(&token)
	return &token, err
}

func GetLongLivedToken(cfg InstagramConfig, shortToken string) (*TokenResponse, error) {
	url := fmt.Sprintf(
		"https://graph.instagram.com/access_token?grant_type=ig_exchange_token&client_secret=%s&access_token=%s",
		cfg.ClientSecret, shortToken,
	)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var token TokenResponse
	err = json.NewDecoder(resp.Body).Decode(&token)
	return &token, err
}

func RefreshToken(currentToken string) (*TokenResponse, error) {
	url := fmt.Sprintf(
		"https://graph.instagram.com/refresh_access_token?grant_type=ig_refresh_token&access_token=%s",
		currentToken,
	)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var token TokenResponse
	err = json.NewDecoder(resp.Body).Decode(&token)
	return &token, err
}

func FetchRecentMedia(userID, accessToken string) ([]map[string]interface{}, error) {
	url := fmt.Sprintf(
		"https://graph.instagram.com/%s/media?fields=id,caption,media_type,media_url,permalink,timestamp&access_token=%s",
		userID, accessToken,
	)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data []map[string]interface{} `json:"data"`
	}
	err = json.NewDecoder(resp.Body).Decode(&result)
	return result.Data, err
}

func ShouldRefreshToken(expiresAt int64) bool {
	return time.Now().Unix() > expiresAt-604800 // 7 days before expiry
}

// GetUserIdFromToken makes a call to the /me endpoint to get the user ID
func GetUserIdFromToken(accessToken string) (string, error) {
	url := fmt.Sprintf(
		"https://graph.instagram.com/me?fields=id,username&access_token=%s",
		accessToken,
	)
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API returned status: %d", resp.StatusCode)
	}

	var result struct {
		ID       string `json:"id"`
		Username string `json:"username"`
	}

	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return "", err
	}

	if result.ID == "" {
		return "", fmt.Errorf("no user ID returned from API")
	}

	return result.ID, nil
}

func RenderRecentPostsJSON(c *gin.Context, userId, accessToken string) {
	recentMedia, nil := FetchRecentMedia(userId, accessToken) // Fetch media to validate token
	recentMediaJSON, err := json.Marshal(recentMedia)
	if !errors.Is(nil, err) {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusInternalServerError, gin.H{
		"accessToken": accessToken,
		"userId":      userId,
		"recentMedia": string(recentMediaJSON),
	})
	return
}
