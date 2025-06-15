package base

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/nahidhasan98/deshimula-notifier-unofficial/config"
	"github.com/nahidhasan98/deshimula-notifier-unofficial/errorhandling"
	"github.com/nahidhasan98/deshimula-notifier-unofficial/storage"
	discordtexthook "github.com/nahidhasan98/discord-text-hook"
)

// Story represents a common story structure
type Story struct {
	Title       string
	Company     string
	Tag         string
	Description string
	Link        string
	Author      string
}

// BaseService provides common functionality for story services
type BaseService struct {
	HTTPConfig   *config.HTTPConfig
	Storage      *storage.StoryStorage
	mu           sync.Mutex
	discordMu    sync.Mutex
	BaseURL      string
	WebhookID    string
	WebhookToken string
	EmbedColor   int
	isFirstRun   bool
}

// NewBaseService creates a new base service
func NewBaseService(storageFile string, baseURL string, webhookID string, webhookToken string, embedColor int) (*BaseService, error) {
	storageDir := filepath.Join(config.StorageDir, storageFile)
	storyStorage, err := storage.NewStoryStorage(storageDir)
	if err != nil {
		return nil, errorhandling.NewError(errorhandling.ConfigError, "Failed to initialize storage", err)
	}

	return &BaseService{
		HTTPConfig:   config.NewHTTPConfig(),
		Storage:      storyStorage,
		BaseURL:      baseURL,
		WebhookID:    webhookID,
		WebhookToken: webhookToken,
		EmbedColor:   embedColor,
		isFirstRun:   true,
	}, nil
}

// FetchAndProcessStories is the common implementation for fetching and processing stories
func (b *BaseService) FetchAndProcessStories(fetchLinks func() ([]string, error), processStory func(string) error) error {
	links, err := fetchLinks()
	if err != nil {
		return err
	}

	var wg sync.WaitGroup

	if b.isFirstRun {
		if len(links) > 0 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if err := processStory(links[0]); err != nil {
					errorhandling.HandleError(err)
				}
			}()

			for _, link := range links[1:] {
				storyID := strings.TrimPrefix(link, b.BaseURL+"/story/")
				if !b.HasStory(storyID) {
					if err := b.AddStory(storyID); err != nil {
						errorhandling.HandleError(err)
					}
				}
			}
		}
		b.isFirstRun = false
	} else {
		wg.Add(len(links))
		for _, link := range links {
			go func(storyLink string) {
				defer wg.Done()
				if err := processStory(storyLink); err != nil {
					errorhandling.HandleError(err)
				}
			}(link)
		}
	}

	wg.Wait()
	return nil
}

// SendToDiscord sends a story to Discord
func (b *BaseService) SendToDiscord(story *Story) error {
	// Validate required fields
	if story.Company == "" {
		return errorhandling.NewError(errorhandling.ValidationError, "Cannot send story with empty company name", nil)
	}
	if story.Description == "" {
		return errorhandling.NewError(errorhandling.ValidationError, "Cannot send story with empty description", nil)
	}

	b.discordMu.Lock()
	defer b.discordMu.Unlock()

	webhook := discordtexthook.NewDiscordTextHookService(b.WebhookID, b.WebhookToken)

	embed := discordtexthook.Embed{
		Title: "📢  " + truncateString(story.Title, 256),
		Description: fmt.Sprintf("**Author:** %s\n**Company:** %s\n**Tag:** %s\n**Link:** %s",
			truncateString(story.Author, 1024),
			truncateString(story.Company, 1024),
			truncateString(story.Tag, 1024),
			story.Link),
		Color: b.EmbedColor,
	}

	if _, err := webhook.SendEmbed(embed); err != nil {
		return errorhandling.NewError(errorhandling.DiscordError, "Failed to send main embed to Discord", err)
	}

	const maxContentLength = 4000
	description := story.Description
	chunkNumber := 1

	for len(description) > 0 {
		chunk := description
		if len(description) > maxContentLength {
			lastNewline := strings.LastIndex(description[:maxContentLength], "\n")
			if lastNewline == -1 {
				// If no newline found, cut at maxLength
				chunk = description[:maxContentLength]
				description = description[maxContentLength:]
			} else {
				// Cut at the last newline
				chunk = description[:lastNewline]
				description = description[lastNewline+1:] // Skip the newline character
			}
		} else {
			// This is the last chunk
			chunk = description
			description = ""
		}

		// Skip empty chunks
		chunk = strings.TrimSpace(chunk)
		if chunk == "" {
			continue
		}

		var title string
		if chunkNumber > 1 || len(description) > 0 {
			title = fmt.Sprintf("Review/Description (Part %d)", chunkNumber)
		} else {
			title = "Review/Description"
		}

		contentEmbed := discordtexthook.Embed{
			Title:       title,
			Description: chunk,
			Color:       b.EmbedColor,
		}

		if _, err := webhook.SendEmbed(contentEmbed); err != nil {
			return errorhandling.NewError(errorhandling.DiscordError, "Failed to send description chunk to Discord", err)
		}

		chunkNumber++
	}

	return nil
}

// HasStory checks if a story exists in storage
func (b *BaseService) HasStory(storyID string) bool {
	return b.Storage.HasStory(storyID)
}

// AddStory adds a story to storage
func (b *BaseService) AddStory(storyID string) error {
	return b.Storage.AddStory(storyID)
}

// truncateString truncates a string to the specified maximum length
func truncateString(s string, maxLength int) string {
	if len(s) <= maxLength {
		return s
	}
	return s[:maxLength-3] + "..."
}
