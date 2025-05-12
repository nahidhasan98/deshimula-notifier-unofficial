package oak

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/PuerkitoBio/goquery"
	"github.com/nahidhasan98/deshimula-notifier-unofficial/config"
	"github.com/nahidhasan98/deshimula-notifier-unofficial/errorhandling"
	"github.com/nahidhasan98/deshimula-notifier-unofficial/interfacer"
	"github.com/nahidhasan98/deshimula-notifier-unofficial/storage"
	discordtexthook "github.com/nahidhasan98/discord-text-hook"
)

type Story struct {
	ID        string `json:"id"`
	Link      string `json:"link"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	Company   string `json:"company_name"`
	Review    string `json:"review_type"`
	Status    string `json:"status"`
	VotesUp   int    `json:"votes_up"`
	VotesDown int    `json:"votes_down"`
	BrowserID string `json:"browser_id"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	IP        string `json:"ip"`
	Browser   string `json:"browser"`
}

type Oak struct {
	httpConfig *config.HTTPConfig
	storage    *storage.StoryStorage
	mu         sync.Mutex
	discordMu  sync.Mutex // Add mutex for Discord message synchronization
}

func New() (interfacer.Service, error) {
	if os.Getenv("WEBHOOK_ID_OAK") == "" || os.Getenv("WEBHOOK_TOKEN_OAK") == "" ||
		os.Getenv("WEBHOOK_ID_ERROR") == "" || os.Getenv("WEBHOOK_TOKEN_ERROR") == "" {
		return nil, errorhandling.NewError(errorhandling.ConfigError, "Missing webhook configuration", nil)
	}

	storageDir := filepath.Join(config.StorageDir, config.OakStorageFile)
	storyStorage, err := storage.NewStoryStorage(storageDir)
	if err != nil {
		return nil, errorhandling.NewError(errorhandling.ConfigError, "Failed to initialize storage", err)
	}

	return &Oak{
		httpConfig: config.NewHTTPConfig(),
		storage:    storyStorage,
	}, nil
}

var isFirstRun = true

func (m *Oak) FetchAndProcessStories() error {
	token, err := m.fetchToken()
	if err != nil {
		return err
	}

	stories, err := m.fetchStories(token)
	if err != nil {
		return err
	}

	var wg sync.WaitGroup

	if isFirstRun {
		if len(stories) > 0 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if err := m.processStory(&stories[0]); err != nil {
					errorhandling.HandleError(err)
				}
			}()

			for _, story := range stories[1:] {
				if !m.storage.HasStory(story.ID) {
					if err := m.storage.AddStory(story.ID); err != nil {
						errorhandling.HandleError(err)
					}
				}
			}
		}
		isFirstRun = false
	} else {
		wg.Add(len(stories))
		for _, story := range stories {
			go func(story Story) {
				defer wg.Done()
				if err := m.processStory(&story); err != nil {
					errorhandling.HandleError(err)
				}
			}(story)
		}
	}

	wg.Wait()
	return nil
}

func (m *Oak) fetchToken() (string, error) {
	resp, err := m.httpConfig.Client.Get(config.OakURL)
	if err != nil {
		return "", errorhandling.NewError(errorhandling.NetworkError, "Failed to fetch token", err)
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return "", errorhandling.NewError(errorhandling.ScrapingError, "Failed to parse token", err)
	}

	var token string

	doc.Find("script").Each(func(i int, s *goquery.Selection) {
		src, exists := s.Attr("src")
		if exists && strings.HasPrefix(src, "/assets/index-") {
			resp, err := m.httpConfig.Client.Get(config.OakURL + src)
			if err != nil {
				return
			}
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return
			}
			respBody := string(body)

			if strings.Contains(respBody, "QC=\"") {
				token = strings.Split(respBody, "QC=\"")[1]
				token = strings.Split(token, "\"")[0]
				return
			}
		}
	})

	return token, nil
}

func (m *Oak) fetchStories(token string) ([]Story, error) {
	req, err := http.NewRequest("GET", config.OakStoriesURL, nil)
	if err != nil {
		return nil, errorhandling.NewError(errorhandling.NetworkError, "Failed to create request", err)
	}

	for key, value := range m.httpConfig.Headers {
		req.Header.Set(key, value)
	}

	req.Header.Set("Apikey", token)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := m.httpConfig.Client.Do(req)
	if err != nil {
		return nil, errorhandling.NewError(errorhandling.NetworkError, "Failed to fetch stories", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errorhandling.NewError(errorhandling.ScrapingError, "Failed to read stories", err)
	}

	var stories []Story
	if err := json.Unmarshal(body, &stories); err != nil {
		return nil, errorhandling.NewError(errorhandling.ScrapingError, "Failed to unmarshal stories", err)
	}

	return stories, nil
}

func (m *Oak) processStory(story *Story) error {
	if m.storage.HasStory(story.ID) {
		log.Println("Found no new story, skipping:", story.ID)
		return nil
	}

	err := m.parseStory(story)
	if err != nil {
		return errorhandling.NewError(errorhandling.ScrapingError, "Failed to parse story", err)
	}

	if err := m.sendToDiscord(story); err != nil {
		return err
	}

	if err := m.storage.AddStory(story.ID); err != nil {
		return errorhandling.NewError(errorhandling.StorageError, "Failed to mark story as sent", err)
	}
	return nil
}

func (m *Oak) parseStory(story *Story) error {
	story.Link = fmt.Sprintf("%s/story/%s", config.OakURL, story.ID)

	converter := md.NewConverter("", true, nil)
	var err error
	story.Content, err = converter.ConvertString(story.Content)
	if err != nil {
		return errorhandling.NewError(errorhandling.ScrapingError, "Failed to convert story content to markdown", err)
	}

	return nil
}

func (m *Oak) sendToDiscord(story *Story) error {
	m.discordMu.Lock()
	defer m.discordMu.Unlock()

	var webhookID, webhookToken string
	if os.Getenv("MODE") == "DEVELOPMENT" {
		webhookID = os.Getenv("WEBHOOK_ID_ERROR")
		webhookToken = os.Getenv("WEBHOOK_TOKEN_ERROR")
	} else {
		webhookID = os.Getenv("WEBHOOK_ID_OAK")
		webhookToken = os.Getenv("WEBHOOK_TOKEN_OAK")
	}

	webhook := discordtexthook.NewDiscordTextHookService(webhookID, webhookToken)

	embed := discordtexthook.Embed{
		Title: truncateString(story.Title, 256),
		Description: fmt.Sprintf("**Company:** %s\n**Review Type:** %s\n**Link:** %s",
			truncateString(story.Company, 1024),
			truncateString(story.Review, 1024),
			story.Link),
		Color: 0x0D9488,
	}

	if _, err := webhook.SendEmbed(embed); err != nil {
		return errorhandling.NewError(errorhandling.DiscordError, "Failed to send main embed to Discord", err)
	}

	const maxContentLength = 4000
	content := story.Content
	chunkNumber := 1

	for len(content) > 0 {
		chunk := content
		if len(content) > maxContentLength {
			lastNewline := strings.LastIndex(content[:maxContentLength], "\n")
			if lastNewline == -1 {
				lastNewline = maxContentLength
			}
			chunk = content[:lastNewline]
			content = content[lastNewline:]
		} else {
			content = ""
		}

		contentEmbed := discordtexthook.Embed{
			Title:       fmt.Sprintf("Review/Description (Part %d)", chunkNumber),
			Description: chunk,
			Color:       0x0D9488,
		}

		if _, err := webhook.SendEmbed(contentEmbed); err != nil {
			return errorhandling.NewError(errorhandling.DiscordError, "Failed to send content chunk to Discord", err)
		}

		chunkNumber++
	}

	return nil
}

func truncateString(s string, maxLength int) string {
	if len(s) <= maxLength {
		return s
	}
	return s[:maxLength-3] + "..."
}
