package errorhandling

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"sync"
	"time"

	discordtexthook "github.com/nahidhasan98/discord-text-hook"
)

type ErrorType int

const (
	ConfigError ErrorType = iota
	NetworkError
	ScrapingError
	DiscordError
	StorageError
)

type AppError struct {
	Type    ErrorType
	Message string
	Err     error
}

func (e *AppError) Error() string {
	return fmt.Sprintf("%s: %v", e.Message, e.Err)
}

func NewError(errType ErrorType, message string, err error) *AppError {
	return &AppError{
		Type:    errType,
		Message: message,
		Err:     err,
	}
}

func getStackTrace() string {
	buf := make([]byte, 1024)
	n := runtime.Stack(buf, false)
	return string(buf[:n])
}

func formatErrorMessage(err error) string {
	msg := "Error Details:\n"
	if appErr, ok := err.(*AppError); ok {
		msg += fmt.Sprintf("Type: %v\n", appErr.Type)
		msg += fmt.Sprintf("Message: %s\n", appErr.Message)
		msg += fmt.Sprintf("Error: %v\n", appErr.Err)
	} else {
		msg += fmt.Sprintf("Error: %v\n", err)
	}
	msg += "\nStack Trace:\n"
	msg += getStackTrace()
	return msg
}

func sendToDiscord(msg string) error {
	webhookID := os.Getenv("WEBHOOK_ID_ERROR")
	webhookToken := os.Getenv("WEBHOOK_TOKEN_ERROR")

	if webhookID == "" || webhookToken == "" {
		return fmt.Errorf("discord webhook configuration missing")
	}

	formattedMsg := fmt.Sprintf("```md\n%s```", msg)
	webhook := discordtexthook.NewDiscordTextHookService(webhookID, webhookToken)

	_, err := webhook.SendMessage(formattedMsg)
	return err
}

type ErrorTracker struct {
	errors   map[string]time.Time
	mu       sync.RWMutex
	cooldown time.Duration
}

var (
	tracker = &ErrorTracker{
		errors:   make(map[string]time.Time),
		cooldown: time.Hour,
	}
)

func (t *ErrorTracker) shouldSendError(err error) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	errKey := err.Error()
	lastSent, exists := t.errors[errKey]

	if !exists {
		t.errors[errKey] = time.Now()
		return true
	}

	if time.Since(lastSent) > t.cooldown {
		t.errors[errKey] = time.Now()
		return true
	}

	return false
}

func (t *ErrorTracker) cleanup() {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	for errKey, lastSent := range t.errors {
		if now.Sub(lastSent) > t.cooldown {
			delete(t.errors, errKey)
		}
	}
}

func HandleError(err error) {
	if err == nil {
		return
	}

	log.Printf("ERROR: %v\n", err)

	// Check if we should send this error to Discord
	if !tracker.shouldSendError(err) {
		log.Printf("Skipping error notification (cooldown): %v\n", err)
		return
	}

	msg := formatErrorMessage(err)
	if discordErr := sendToDiscord(msg); discordErr != nil {
		log.Printf("Failed to send error to Discord: %v\n", discordErr)
	}

	// Cleanup old errors periodically
	tracker.cleanup()
}
