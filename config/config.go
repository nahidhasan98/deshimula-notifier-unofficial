package config

import (
	"net/http"
	"time"
)

const (
	MulaURL         = "https://deshimula.com"
	OakURL          = "https://oakthu.com"
	OakStoriesURL   = "https://cktyiwbcjvgfbfmjcwao.supabase.co/rest/v1/stories?select=*&order=created_at.desc&limit=20"
	Interval        = 1 * time.Minute
	StorageDir      = "storage"
	MulaStorageFile = "mula_sent_stories.json"
	OakStorageFile  = "oak_sent_stories.json"
)

type HTTPConfig struct {
	Headers map[string]string
	Client  *http.Client
}

func NewHTTPConfig() *HTTPConfig {
	headers := map[string]string{
		"User-Agent":      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/113.0.0.0 Safari/537.36",
		"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8",
		"Accept-Language": "en-US,en;q=0.9",
		"Cache-Control":   "no-cache",
		"Connection":      "keep-alive",
		"Sec-Fetch-Dest":  "document",
		"Sec-Fetch-Mode":  "navigate",
		"Sec-Fetch-Site":  "none",
		"Sec-Fetch-User":  "?1",
		"Pragma":          "no-cache",
		"DNT":             "1",
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:       10,
			IdleConnTimeout:    30 * time.Second,
			DisableCompression: true,
		},
	}

	return &HTTPConfig{
		Headers: headers,
		Client:  client,
	}
}
