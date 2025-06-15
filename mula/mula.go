package mula

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/nahidhasan98/deshimula-notifier-unofficial/base"
	"github.com/nahidhasan98/deshimula-notifier-unofficial/config"
	"github.com/nahidhasan98/deshimula-notifier-unofficial/errorhandling"
	"github.com/nahidhasan98/deshimula-notifier-unofficial/interfacer"
)

type Mula struct {
	*base.BaseService
}

func New() (interfacer.Service, error) {
	if os.Getenv("WEBHOOK_ID_MULA") == "" || os.Getenv("WEBHOOK_TOKEN_MULA") == "" ||
		os.Getenv("WEBHOOK_ID_ERROR") == "" || os.Getenv("WEBHOOK_TOKEN_ERROR") == "" {
		return nil, errorhandling.NewError(errorhandling.ConfigError, "Missing webhook configuration", nil)
	}

	webhookID := os.Getenv("WEBHOOK_ID_MULA")
	webhookToken := os.Getenv("WEBHOOK_TOKEN_MULA")
	if os.Getenv("MODE") == "DEVELOPMENT" {
		webhookID = os.Getenv("WEBHOOK_ID_ERROR")
		webhookToken = os.Getenv("WEBHOOK_TOKEN_ERROR")
	}

	baseService, err := base.NewBaseService(
		config.MulaStorageFile,
		config.MulaURL,
		webhookID,
		webhookToken,
		0xFFDFBA, // Light orange color
	)
	if err != nil {
		return nil, err
	}

	return &Mula{
		BaseService: baseService,
	}, nil
}

func (m *Mula) FetchAndProcessStories() error {
	return m.BaseService.FetchAndProcessStories(m.fetchStoryLinks, m.processStory)
}

func (m *Mula) fetchStoryLinks() ([]string, error) {
	req, err := http.NewRequest("GET", m.BaseURL, nil)
	if err != nil {
		return nil, errorhandling.NewError(errorhandling.NetworkError, "Failed to create request", err)
	}

	for key, value := range m.HTTPConfig.Headers {
		req.Header.Set(key, value)
	}
	resp, err := m.HTTPConfig.Client.Do(req)
	if err != nil {
		return nil, errorhandling.NewError(errorhandling.NetworkError, "Failed to fetch story links", err)
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	var links []string
	doc.Find("a.text-decoration-none.hyper-link").Each(func(i int, s *goquery.Selection) {
		if link, exists := s.Attr("href"); exists {
			if link[0] == '/' {
				link = m.BaseURL + link
				links = append(links, link)
			}
		}
	})

	return links, nil
}

func (m *Mula) processStory(link string) error {
	if m.HasStory(strings.TrimPrefix(link, m.BaseURL+"/story/")) {
		log.Println("Found no new story, skipping:", strings.TrimPrefix(link, m.BaseURL+"/story/"))
		return nil
	}

	story, err := m.fetchAndParseStory(link)
	if err != nil {
		return errorhandling.NewError(errorhandling.ScrapingError, "Failed to fetch story", err)
	}

	if err := m.SendToDiscord(story); err != nil {
		return err
	}

	if err := m.AddStory(strings.TrimPrefix(link, m.BaseURL+"/story/")); err != nil {
		return errorhandling.NewError(errorhandling.StorageError, "Failed to mark story as sent", err)
	}
	return nil
}

func (m *Mula) fetchAndParseStory(link string) (*base.Story, error) {
	req, err := http.NewRequest("GET", link, nil)
	if err != nil {
		return nil, err
	}

	for key, value := range m.HTTPConfig.Headers {
		req.Header.Set(key, value)
	}

	resp, err := m.HTTPConfig.Client.Do(req)
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

	story := &base.Story{
		Link: link,
	}

	story.Title = strings.TrimSpace(doc.Find("h3").First().Text())

	authorText := doc.Find("h6.fw-semibold").Text()
	story.Author = strings.TrimSpace(strings.TrimPrefix(authorText, "by "))

	doc.Find(".badge").Each(func(i int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		if i == 0 {
			story.Company = text
		} else if i == 1 {
			story.Tag = text
		}
	})

	if len(story.Company) == 0 {
		return nil, errors.New("Empty company name")
	}

	var description strings.Builder
	doc.Find("main .mt-4 .row .col-12").Each(func(i int, s *goquery.Selection) {
		// Find the d-flex my-2 div first
		div := s.Find(".d-flex.my-2")

		// Get all following siblings that match your criteria
		div.NextAll().Filter("p, ol li, h3, h4").Each(func(i int, s *goquery.Selection) {
			text := strings.TrimSpace(s.Text())
			if text != "" {
				if s.Is("h3") {
					description.WriteString("\n### " + text + " ###\n")
				} else if s.Is("h4") {
					description.WriteString("\n## " + text + " ##\n")
				} else if s.Is("li") {
					description.WriteString("- " + text + "\n")
				} else {
					description.WriteString(text + "\n")
				}
			}
		})
	})
	story.Description = strings.TrimSpace(description.String())

	return story, nil
}
