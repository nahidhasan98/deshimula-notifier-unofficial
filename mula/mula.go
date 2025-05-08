package mula

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/PuerkitoBio/goquery"
	"github.com/nahidhasan98/deshimula-notifier-unofficial/config"
	"github.com/nahidhasan98/deshimula-notifier-unofficial/errorhandling"
	"github.com/nahidhasan98/deshimula-notifier-unofficial/storage"
	discordtexthook "github.com/nahidhasan98/discord-text-hook"
)

type Story struct {
	Title       string
	Company     string
	Tag         string
	Description string
	Link        string
}

type Mula struct {
	httpConfig *config.HTTPConfig
	storage    *storage.StoryStorage
	mu         sync.Mutex
}

func New(httpConfig *config.HTTPConfig) (*Mula, error) {
	if os.Getenv("WEBHOOK_ID") == "" || os.Getenv("WEBHOOK_TOKEN") == "" {
		return nil, errorhandling.NewError(errorhandling.ConfigError, "Missing webhook configuration", nil)
	}

	storageDir := filepath.Join("storage", "sent_stories.json")
	storyStorage, err := storage.NewStoryStorage(storageDir)
	if err != nil {
		return nil, errorhandling.NewError(errorhandling.ConfigError, "Failed to initialize storage", err)
	}

	return &Mula{
		httpConfig: httpConfig,
		storage:    storyStorage,
	}, nil
}

func (m *Mula) FetchAndProcessStories() error {
	links, err := m.fetchStoryLinks()
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	for _, link := range links {
		wg.Add(1)
		go func(storyLink string) {
			defer wg.Done()
			if err := m.processStory(storyLink); err != nil {
				errorhandling.HandleError(err)
			}
		}(link)
	}
	wg.Wait()
	return nil
}

func (m *Mula) fetchStoryLinks() ([]string, error) {
	resp, err := m.httpConfig.Client.Get(config.BaseURL + "/stories/1")
	if err != nil {
		return nil, errorhandling.NewError(errorhandling.NetworkError, "Failed to fetch stories", err)
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	var links []string
	doc.Find(".mula-list").Each(func(i int, s *goquery.Selection) {
		if i != 1 {
			return
		}
		if link, exists := s.Find("a").Attr("href"); exists {
			if link[0] == '/' {
				link = config.BaseURL + link
			}
			links = append(links, link)
		}
	})
	return links, nil
}

func (m *Mula) processStory(link string) error {

	if m.storage.HasStory(link) {
		return nil
	}

	story, err := m.fetchAndParseStory(link)
	if err != nil {
		return errorhandling.NewError(errorhandling.ScrapingError, "Failed to fetch story", err)
	}

	if err := m.sendToDiscord(story); err != nil {
		return err
	}

	if err := m.storage.AddStory(link); err != nil {
		return errorhandling.NewError(errorhandling.StorageError, "Failed to mark story as sent", err)
	}
	return nil
}

func (m *Mula) fetchAndParseStory(link string) (*Story, error) {
	resp, err := m.httpConfig.Client.Get(link)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	story := &Story{
		Link: link,
	}

	story.Title = strings.TrimSpace(doc.Find("h3").First().Text())

	doc.Find(".badge").Each(func(i int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		if i == 0 {
			story.Company = text
		} else if i == 1 {
			story.Tag = text
		}
	})

	var description strings.Builder
	doc.Find("main").Find("p, ol li").Each(func(i int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		if text != "" {
			if s.Is("li") {
				description.WriteString("- ")
			}
			description.WriteString(text + "\n")
		}
	})
	story.Description = description.String()

	return story, nil
}

func (m *Mula) sendToDiscord(story *Story) error {
	webhookID := os.Getenv("WEBHOOK_ID")
	webhookToken := os.Getenv("WEBHOOK_TOKEN")

	webhook := discordtexthook.NewDiscordTextHookService(webhookID, webhookToken)

	var message strings.Builder
	message.WriteString(fmt.Sprintf("**%s**\n", story.Title))
	message.WriteString(fmt.Sprintf("Company: %s\n", story.Company))
	message.WriteString(fmt.Sprintf("Tag: %s\n", story.Tag))
	message.WriteString(fmt.Sprintf("Link: %s\n\n", story.Link))
	message.WriteString(fmt.Sprintf("Description:\n%s", story.Description))

	_, err := webhook.SendMessage(message.String())
	if err != nil {
		return errorhandling.NewError(errorhandling.DiscordError, "Failed to send to Discord", err)
	}
	return nil
}
