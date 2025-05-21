package oak

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/PuerkitoBio/goquery"
	"github.com/nahidhasan98/deshimula-notifier-unofficial/config"
	"github.com/nahidhasan98/deshimula-notifier-unofficial/errorhandling"
	"github.com/nahidhasan98/deshimula-notifier-unofficial/interfacer"
	"github.com/nahidhasan98/deshimula-notifier-unofficial/storage"
	discordtexthook "github.com/nahidhasan98/discord-text-hook"
)

type Story struct {
	Title       string
	Company     string
	Tag         string
	Description string
	Link        string
	Author      string
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
	links, err := m.fetchStoryLinks()
	if err != nil {
		return err
	}

	var wg sync.WaitGroup

	if isFirstRun {
		if len(links) > 0 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if err := m.processStory(links[0]); err != nil {
					errorhandling.HandleError(err)
				}
			}()

			for _, link := range links[1:] {
				if !m.storage.HasStory(strings.TrimPrefix(link, "https://oakthu.com/story/")) {
					if err := m.storage.AddStory(strings.TrimPrefix(link, "https://oakthu.com/story/")); err != nil {
						errorhandling.HandleError(err)
					}
				}
			}
		}
		isFirstRun = false
	} else {
		wg.Add(len(links))
		for _, link := range links {
			go func(storyLink string) {
				defer wg.Done()
				if err := m.processStory(storyLink); err != nil {
					errorhandling.HandleError(err)
				}
			}(link)
		}
	}

	wg.Wait()
	return nil
}

func (m *Oak) fetchStoryLinks() ([]string, error) {
	req, err := http.NewRequest("GET", config.OakURL, nil)
	if err != nil {
		return nil, errorhandling.NewError(errorhandling.NetworkError, "Failed to create request", err)
	}

	for key, value := range m.httpConfig.Headers {
		req.Header.Set(key, value)
	}
	resp, err := m.httpConfig.Client.Do(req)
	if err != nil {
		return nil, errorhandling.NewError(errorhandling.NetworkError, "Failed to fetch story links", err)
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	var links []string
	doc.Find("div.w-full a.transition-colors").Each(func(i int, s *goquery.Selection) {
		if link, exists := s.Attr("href"); exists {
			if link[0] == '/' {
				link = config.OakURL + link
				links = append(links, link)
			}
		}
	})

	return links, nil
}

func (m *Oak) processStory(link string) error {
	if m.storage.HasStory(strings.TrimPrefix(link, "https://oakthu.com/story/")) {
		log.Println("Found no new story, skipping:", strings.TrimPrefix(link, "https://oakthu.com/story/"))
		return nil
	}

	story, err := m.fetchAndParseStory(link)
	if err != nil {
		return errorhandling.NewError(errorhandling.ScrapingError, "Failed to fetch story", err)
	}

	if err := m.sendToDiscord(story); err != nil {
		return err
	}

	if err := m.storage.AddStory(strings.TrimPrefix(link, "https://oakthu.com/story/")); err != nil {
		return errorhandling.NewError(errorhandling.StorageError, "Failed to mark story as sent", err)
	}
	return nil
}

func (m *Oak) fetchAndParseStory(link string) (*Story, error) {
	req, err := http.NewRequest("GET", link, nil)
	if err != nil {
		return nil, err
	}

	for key, value := range m.httpConfig.Headers {
		req.Header.Set(key, value)
	}

	resp, err := m.httpConfig.Client.Do(req)
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

	// Find the script tag containing the story data
	var scriptContent string
	doc.Find("script").Each(func(i int, s *goquery.Selection) {
		content := s.Text()
		if strings.Contains(content, "self.__next_f.push") {
			scriptContent += content[len("self.__next_f.push([1,\"") : len(content)-2]
		}
	})

	if scriptContent == "" {
		return nil, errors.New("no script content found with company information")
	}

	scriptContent = strings.ReplaceAll(scriptContent, "\\u003c", "<")
	scriptContent = strings.ReplaceAll(scriptContent, "\\u003e", ">")
	scriptContent = strings.ReplaceAll(scriptContent, "\\n", "\n")
	scriptContent = strings.ReplaceAll(scriptContent, "\\\"", "\"")

	// Extract title
	if titleMatch := regexp.MustCompile(`"title":"([^"]+)"`).FindStringSubmatch(scriptContent); len(titleMatch) > 1 {
		story.Title = titleMatch[1]
	}

	// Extract company name
	if companyMatch := regexp.MustCompile(`"company_name":"([^"]+)"`).FindStringSubmatch(scriptContent); len(companyMatch) > 1 {
		story.Company = companyMatch[1]
	}

	// Extract review type
	if tagMatch := regexp.MustCompile(`"review_type":"([^"]+)"`).FindStringSubmatch(scriptContent); len(tagMatch) > 1 {
		story.Tag = tagMatch[1]
	}

	// Extract content
	ind1 := strings.Index(scriptContent, "title")
	ind2 := strings.Index(scriptContent, "company_name")
	contentDoc, err := goquery.NewDocumentFromReader(strings.NewReader(scriptContent[ind1:ind2]))
	if err != nil {
		return nil, err
	}
	var description strings.Builder
	contentDoc.Find("p, ol li").Each(func(i int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		if text != "" {
			description.WriteString(text + "\n\n")
		}
	})
	story.Description = strings.TrimSpace(description.String())

	if len(story.Company) == 0 {
		return nil, errors.New("empty company name")
	}

	if len(story.Description) == 0 {
		return nil, errors.New("empty description")
	}

	return story, nil
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
		Title: "📢  " + truncateString(story.Title, 256),
		Description: fmt.Sprintf("**Company:** %s\n**Tag:** %s\n**Link:** %s",
			truncateString(story.Company, 1024),
			truncateString(story.Tag, 1024),
			story.Link),
		Color: 0x0D9488,
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
				lastNewline = maxContentLength
			}
			chunk = description[:lastNewline]
			description = description[lastNewline:]
		} else {
			description = ""
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
			Color:       0x0D9488,
		}

		if _, err := webhook.SendEmbed(contentEmbed); err != nil {
			return errorhandling.NewError(errorhandling.DiscordError, "Failed to send description chunk to Discord", err)
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
