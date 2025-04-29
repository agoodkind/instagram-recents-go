package lib

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type TokenResponse struct {
	AccessToken string `json:"access_token"`
	UserID      string `json:"user_id"`
	ExpiresIn   int    `json:"expires_in"`
}

type Media struct {
	ID           string `json:"id"`
	MediaType    string `json:"media_type"`
	MediaURL     string `json:"media_url"`
	Permalink    string `json:"permalink"`
	Timestamp    string `json:"timestamp"`
	ThumbnailURL string `json:"thumbnail_url,omitempty"`
	IsSharedToFeed bool `json:"is_shared_to_feed,omitempty"`
}

type MediaResponse struct {
	Data []Media `json:"data"`
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

func FetchRecentMedia(userID, accessToken string) ([]Media, error) {
	fields := []string{
		"id",
		"media_type",
		"media_url",
		"permalink",
		"timestamp",
		"thumbnail_url",
		"is_shared_to_feed",
	}
	fieldsString := strings.Join(fields, ",")
	url := fmt.Sprintf(
		"https://graph.instagram.com/%s/media?fields=%s&access_token=%s",
		userID, fieldsString, accessToken,
	)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result MediaResponse
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
