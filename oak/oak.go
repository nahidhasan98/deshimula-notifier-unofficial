package oak

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/nahidhasan98/deshimula-notifier-unofficial/base"
	"github.com/nahidhasan98/deshimula-notifier-unofficial/config"
	"github.com/nahidhasan98/deshimula-notifier-unofficial/errorhandling"
	"github.com/nahidhasan98/deshimula-notifier-unofficial/interfacer"
)

// cleanScriptContent removes escape sequences from the script content
func cleanScriptContent(content string) string {
	replacements := map[string]string{
		"\\u003c":      "<",  // Unicode for <
		"\\u003e":      ">",  // Unicode for >
		"\\n":          "\n", // Newline
		"\\\\\\\"":     "\"", // Escaped quote (\\\")
		"\\\"":         "\"", // Escaped quote (\")
		"\\u0026lt;":   "<",  // HTML < escaped as \u0026lt;
		"\\u0026gt;":   ">",  // HTML > escaped as \u0026gt;
		"\\u0026quot;": "\"", // HTML " escaped as \u0026quot;
		"\\u0026#x2F;": "/",  // HTML / escaped as \u0026#x2F;
	}

	for old, new := range replacements {
		content = strings.ReplaceAll(content, old, new)
	}
	return content
}

type Oak struct {
	*base.BaseService
}

func New() (interfacer.Service, error) {
	if os.Getenv("WEBHOOK_ID_OAK") == "" || os.Getenv("WEBHOOK_TOKEN_OAK") == "" ||
		os.Getenv("WEBHOOK_ID_ERROR") == "" || os.Getenv("WEBHOOK_TOKEN_ERROR") == "" {
		return nil, errorhandling.NewError(errorhandling.ConfigError, "Missing webhook configuration", nil)
	}

	webhookID := os.Getenv("WEBHOOK_ID_OAK")
	webhookToken := os.Getenv("WEBHOOK_TOKEN_OAK")
	if os.Getenv("MODE") == "DEVELOPMENT" {
		webhookID = os.Getenv("WEBHOOK_ID_ERROR")
		webhookToken = os.Getenv("WEBHOOK_TOKEN_ERROR")
	}

	baseService, err := base.NewBaseService(
		config.OakStorageFile,
		config.OakURL,
		webhookID,
		webhookToken,
		0x0D9488, // Teal color
	)
	if err != nil {
		return nil, err
	}

	return &Oak{
		BaseService: baseService,
	}, nil
}

func (m *Oak) FetchAndProcessStories() error {
	return m.BaseService.FetchAndProcessStories(m.fetchStoryLinks, m.processStory)
}

func (m *Oak) fetchStoryLinks() ([]string, error) {
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

	scriptContent = cleanScriptContent(scriptContent)

	// Extract story IDs using regex
	idPattern := regexp.MustCompile(`"id":"([^"]+)"`)
	matches := idPattern.FindAllStringSubmatch(scriptContent, -1)

	for _, match := range matches {
		if len(match) > 1 {
			storyLink := m.BaseService.BaseURL + "/story/" + match[1]
			links = append(links, storyLink)
		}
	}

	return links, nil
}

func (m *Oak) processStory(link string) error {
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

func (m *Oak) fetchAndParseStory(link string) (*base.Story, error) {
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

	scriptContent = cleanScriptContent(scriptContent)

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
