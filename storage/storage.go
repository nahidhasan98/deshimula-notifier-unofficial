package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

type StoryStorage struct {
	filepath string
	stories  sync.Map
	mu       sync.Mutex
}

func NewStoryStorage(storagePath string) (*StoryStorage, error) {
	if err := os.MkdirAll(filepath.Dir(storagePath), 0755); err != nil {
		return nil, err
	}

	s := &StoryStorage{
		filepath: storagePath,
	}

	if _, err := os.Stat(storagePath); err == nil {
		data, err := os.ReadFile(storagePath)
		if err != nil {
			return nil, err
		}

		var stories map[string]bool
		if err := json.Unmarshal(data, &stories); err != nil {
			return nil, err
		}

		for link := range stories {
			s.stories.Store(link, true)
		}
	}

	return s, nil
}

func (s *StoryStorage) HasStory(link string) bool {
	_, exists := s.stories.Load(link)
	return exists
}

func (s *StoryStorage) AddStory(link string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.stories.Store(link, true)

	stories := make(map[string]bool)
	s.stories.Range(func(key, value interface{}) bool {
		stories[key.(string)] = true
		return true
	})

	data, err := json.Marshal(stories)
	if err != nil {
		return err
	}

	return os.WriteFile(s.filepath, data, 0644)
}
