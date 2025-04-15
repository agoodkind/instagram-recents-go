package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/sessions"
)

var store = sessions.NewCookieStore([]byte(os.Getenv("SESSION_SECRET")))

type InstagramConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
}

type TokenResponse struct {
	AccessToken string `json:"access_token"`
	UserID      string `json:"user_id"`
	ExpiresIn   int    `json:"expires_in"`
}

func main() {
	cfg := InstagramConfig{
		ClientID:     os.Getenv("INSTAGRAM_APP_ID"),
		ClientSecret: os.Getenv("INSTAGRAM_APP_SECRET"),
		RedirectURI:  os.Getenv("REDIRECT_URI"),
	}

	r := gin.Default()
	r.LoadHTMLGlob("templates/*")

	r.GET("/", func(c *gin.Context) {
		authURL := fmt.Sprintf(
			"https://api.instagram.com/oauth/authorize?client_id=%s&redirect_uri=%s&scope=user_profile,user_media&response_type=code",
			cfg.ClientID, cfg.RedirectURI,
		)
		c.HTML(http.StatusOK, "index.html", gin.H{"AuthURL": authURL})
	})

	r.GET("/auth/callback", func(c *gin.Context) {
		session, _ := store.Get(c.Request, "instagram-session")
		code := c.Query("code")

		// Exchange code for short-lived token
		token, err := exchangeCodeForToken(cfg, code)
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}

		// Exchange for long-lived token
		longToken, err := getLongLivedToken(cfg, token.AccessToken)
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}

		session.Values["access_token"] = longToken.AccessToken
		session.Values["user_id"] = token.UserID
		session.Values["expires_at"] = time.Now().Add(time.Duration(longToken.ExpiresIn) * time.Second).Unix()
		session.Save(c.Request, c.Writer)

		c.Redirect(http.StatusFound, "/recent-posts")
	})

	r.GET("/recent-posts", func(c *gin.Context) {
		session, _ := store.Get(c.Request, "instagram-session")
		accessToken, ok := session.Values["access_token"].(string)
		userID, ok2 := session.Values["user_id"].(string)

		if !ok || !ok2 {
			c.Redirect(http.StatusFound, "/")
			return
		}

		// Refresh token if needed
		if time.Now().Unix() > session.Values["expires_at"].(int64)-604800 {
			newToken, err := refreshToken(accessToken)
			if err != nil {
				c.AbortWithError(http.StatusInternalServerError, err)
				return
			}
			session.Values["access_token"] = newToken.AccessToken
			session.Values["expires_at"] = time.Now().Add(time.Duration(newToken.ExpiresIn) * time.Second).Unix()
			session.Save(c.Request, c.Writer)
			accessToken = newToken.AccessToken
		}

		// Fetch media
		media, err := fetchRecentMedia(userID, accessToken)
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}

		c.HTML(http.StatusOK, "posts.html", gin.H{"Media": media})
	})

	r.Run(":8080")
}

func exchangeCodeForToken(cfg InstagramConfig, code string) (*TokenResponse, error) {
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

func getLongLivedToken(cfg InstagramConfig, shortToken string) (*TokenResponse, error) {
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

func refreshToken(currentToken string) (*TokenResponse, error) {
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

func fetchRecentMedia(userID, accessToken string) ([]map[string]interface{}, error) {
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
