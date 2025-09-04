// Binary slendmail - see README.md
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/syslog"
	"net/http"
	"net/mail"
	"os"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"github.com/slack-go/slack"
)

// EmailMessage represents a parsed email
type EmailMessage struct {
	Subject  string
	Body     string
	Hostname string
}

// Backend interface for notification backends
type Backend interface {
	Send(msg *EmailMessage) error
	Name() string
}

// Config - toml config struct
type Config struct {
	SyslogTag string         `toml:"syslog_tag"`
	Backends  BackendsConfig `toml:"backends"`
}

// BackendsConfig contains configuration for all backends
type BackendsConfig struct {
	Slack   []SlackConfig   `toml:"slack"`
	Webhook []WebhookConfig `toml:"webhook"`
}

// SlackConfig configuration for Slack backend
type SlackConfig struct {
	Token   string `toml:"token"`
	Channel string `toml:"channel"`
	Name    string `toml:"name,omitempty"`
}

// WebhookConfig configuration for generic webhook backend
type WebhookConfig struct {
	URL      string            `toml:"url"`
	Name     string            `toml:"name,omitempty"`
	Headers  map[string]string `toml:"headers,omitempty"`
	Username string            `toml:"username,omitempty"`
	Channel  string            `toml:"channel,omitempty"`
	Format   string            `toml:"format,omitempty"` // "mattermost", "slack", "generic"
}

// SlackBackend implements Backend for Slack notifications
type SlackBackend struct {
	config SlackConfig
	api    *slack.Client
}

// NewSlackBackend creates a new Slack backend
func NewSlackBackend(config SlackConfig) *SlackBackend {
	return &SlackBackend{
		config: config,
		api:    slack.New(config.Token),
	}
}

// Name returns the backend name
func (s *SlackBackend) Name() string {
	if s.config.Name != "" {
		return fmt.Sprintf("Slack (%s)", s.config.Name)
	}
	return "Slack"
}

// Send sends the email message to Slack
func (s *SlackBackend) Send(msg *EmailMessage) error {
	// setup slack message
	attach := &slack.Attachment{
		Text: msg.Body,
	}

	subjText := slack.NewTextBlockObject("mrkdwn", "*Subject:* "+msg.Subject, false, false)
	hostText := slack.NewTextBlockObject("mrkdwn", "*Hostname:* "+msg.Hostname, false, false)
	hdrBlock := []*slack.TextBlockObject{subjText, hostText}

	smsg := slack.MsgOptionBlocks(
		slack.NewSectionBlock(
			nil,
			hdrBlock,
			nil,
		),
	)

	_, _, err := s.api.PostMessage(
		s.config.Channel,
		smsg,
		slack.MsgOptionAttachments(*attach),
	)
	return err
}

// WebhookBackend implements Backend for generic webhook notifications
type WebhookBackend struct {
	config WebhookConfig
	client *http.Client
}

// NewWebhookBackend creates a new webhook backend
func NewWebhookBackend(config WebhookConfig) *WebhookBackend {
	// Set default format if not specified
	if config.Format == "" {
		config.Format = "mattermost"
	}

	return &WebhookBackend{
		config: config,
		client: &http.Client{},
	}
}

// Name returns the backend name
func (w *WebhookBackend) Name() string {
	if w.config.Name != "" {
		return fmt.Sprintf("Webhook (%s)", w.config.Name)
	}
	return "Webhook"
}

// Send sends the email message to the webhook endpoint
func (w *WebhookBackend) Send(msg *EmailMessage) error {
	var payload interface{}

	switch w.config.Format {
	case "mattermost", "slack":
		// Mattermost/Slack compatible format
		text := fmt.Sprintf("**Subject:** %s\n**Hostname:** %s\n\n%s",
			msg.Subject, msg.Hostname, msg.Body)

		payload = map[string]interface{}{
			"text": text,
		}

		// Add optional fields
		if w.config.Username != "" {
			payload.(map[string]interface{})["username"] = w.config.Username
		}
		if w.config.Channel != "" {
			payload.(map[string]interface{})["channel"] = w.config.Channel
		}

	case "generic":
		// Generic format with separate fields
		payload = map[string]interface{}{
			"subject":  msg.Subject,
			"body":     msg.Body,
			"hostname": msg.Hostname,
		}

	default:
		return fmt.Errorf("unsupported webhook format: %s", w.config.Format)
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal webhook payload: %w", err)
	}

	req, err := http.NewRequest("POST", w.config.URL, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("failed to create webhook request: %w", err)
	}

	// Set default content type
	req.Header.Set("Content-Type", "application/json")

	// Add custom headers
	for key, value := range w.config.Headers {
		req.Header.Set(key, value)
	}

	resp, err := w.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send webhook request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("webhook request failed with status %d: %s",
			resp.StatusCode, string(body))
	}

	return nil
}

func main() {
	var config Config

	// set a default for syslogtag
	config.SyslogTag = "slendmail"

	// read config
	cfgFile, err := os.ReadFile("/etc/slendmail.conf")
	if err != nil {
		log.Fatal("failed to read config file ", err)
	}
	err = toml.Unmarshal(cfgFile, &config)
	if err != nil {
		log.Fatal("failed to unmarshal config file ", err)
	}

	// setup syslogger, we use this instead of regular output so we can see the output in the
	// case of being called from crond
	sl, err := syslog.New(syslog.LOG_WARNING|syslog.LOG_MAIL, config.SyslogTag)
	if err != nil {
		_ = sl.Err(fmt.Sprintln("failed to setup syslog connection ", err))
		log.Fatal("failed to setup syslog connection ", err)
	}

	// parse stdin, check RFC5321 for specifics of the format
	// this probably needs to be beefed up a bit to handle other
	// callers. So far only tested with busybox/Alpine crond
	stdin, err := io.ReadAll(os.Stdin)
	if err != nil {
		// this really shouldn't fail
		log.Fatal("failed to read stdin", err)
	}
	_ = sl.Debug(string(stdin))

	msg, err := mail.ReadMessage(bytes.NewReader(stdin))
	if err != nil {
		_ = sl.Err(fmt.Sprintln("failed to read stdin email format ", err))
		log.Fatal("failed to read stdin email format", err)
	}

	body, err := io.ReadAll(msg.Body)
	if err != nil {
		_ = sl.Err(fmt.Sprintln("failed to read email body ", err))
		log.Fatal("failed to read  email body", err)
	}

	hostname, _ := os.Hostname()
	emailMsg := &EmailMessage{
		Subject:  msg.Header.Get("Subject"),
		Body:     strings.TrimSpace(string(body)),
		Hostname: hostname,
	}

	// Initialize backends
	var backends []Backend

	// Add Slack backends
	for _, slackConfig := range config.Backends.Slack {
		backends = append(backends, NewSlackBackend(slackConfig))
	}

	// Add Webhook backends
	for _, webhookConfig := range config.Backends.Webhook {
		backends = append(backends, NewWebhookBackend(webhookConfig))
	}

	if len(backends) == 0 {
		_ = sl.Err("no backends configured")
		log.Fatal("no backends configured")
	}

	// Send to all backends
	var errors []string
	for _, backend := range backends {
		err := backend.Send(emailMsg)
		if err != nil {
			errMsg := fmt.Sprintf("failed to send via %s: %v", backend.Name(), err)
			errors = append(errors, errMsg)
			_ = sl.Err(errMsg)
		} else {
			_ = sl.Debug(fmt.Sprintf("successfully sent via %s", backend.Name()))
		}
	}

	// If all backends failed, exit with error
	if len(errors) == len(backends) {
		log.Fatal("all backends failed: ", strings.Join(errors, "; "))
	}

	// Log summary
	successCount := len(backends) - len(errors)
	_ = sl.Debug(fmt.Sprintf("sent to %d/%d backends successfully - argv: %v",
		successCount, len(backends), os.Args))
}
