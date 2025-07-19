package infrastructure

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gkbiswas/hotel-reviews-microservice/pkg/logger"
	"gopkg.in/gomail.v2"
)

// LoggerAlertChannel sends alerts to the logger
type LoggerAlertChannel struct {
	logger  *logger.Logger
	enabled bool
}

// NewLoggerAlertChannel creates a new logger alert channel
func NewLoggerAlertChannel(logger *logger.Logger) *LoggerAlertChannel {
	return &LoggerAlertChannel{
		logger:  logger,
		enabled: true,
	}
}

// Send sends an alert to the logger
func (lac *LoggerAlertChannel) Send(ctx context.Context, alert *Alert) error {
	if !lac.enabled {
		return nil
	}

	// Log based on alert level
	switch alert.Level {
	case AlertLevelInfo:
		lac.logger.InfoContext(ctx, fmt.Sprintf("ALERT: %s", alert.Title),
			"alert_id", alert.ID,
			"alert_type", alert.Type,
			"alert_level", alert.Level,
			"message", alert.Message,
			"details", alert.Details,
		)
	case AlertLevelWarning:
		lac.logger.WarnContext(ctx, fmt.Sprintf("ALERT: %s", alert.Title),
			"alert_id", alert.ID,
			"alert_type", alert.Type,
			"alert_level", alert.Level,
			"message", alert.Message,
			"details", alert.Details,
		)
	case AlertLevelError, AlertLevelCritical:
		lac.logger.ErrorContext(ctx, fmt.Sprintf("ALERT: %s", alert.Title),
			"alert_id", alert.ID,
			"alert_type", alert.Type,
			"alert_level", alert.Level,
			"message", alert.Message,
			"details", alert.Details,
			"actions", alert.Actions,
			"runbook", alert.Runbook,
		)
	}

	return nil
}

// Name returns the channel name
func (lac *LoggerAlertChannel) Name() string {
	return "logger"
}

// IsEnabled returns true if the channel is enabled
func (lac *LoggerAlertChannel) IsEnabled() bool {
	return lac.enabled
}

// SetEnabled sets the channel enabled state
func (lac *LoggerAlertChannel) SetEnabled(enabled bool) {
	lac.enabled = enabled
}

// EmailAlertChannel sends alerts via email
type EmailAlertChannel struct {
	host     string
	port     int
	username string
	password string
	from     string
	to       []string
	enabled  bool
	logger   *logger.Logger
	dialer   *gomail.Dialer
}

// EmailConfig represents email configuration
type EmailConfig struct {
	Host     string   `json:"host"`
	Port     int      `json:"port"`
	Username string   `json:"username"`
	Password string   `json:"password"`
	From     string   `json:"from"`
	To       []string `json:"to"`
}

// NewEmailAlertChannel creates a new email alert channel
func NewEmailAlertChannel(config *EmailConfig, logger *logger.Logger) *EmailAlertChannel {
	dialer := gomail.NewDialer(config.Host, config.Port, config.Username, config.Password)

	return &EmailAlertChannel{
		host:     config.Host,
		port:     config.Port,
		username: config.Username,
		password: config.Password,
		from:     config.From,
		to:       config.To,
		enabled:  true,
		logger:   logger,
		dialer:   dialer,
	}
}

// Send sends an alert via email
func (eac *EmailAlertChannel) Send(ctx context.Context, alert *Alert) error {
	if !eac.enabled {
		return nil
	}

	// Create email message
	message := gomail.NewMessage()
	message.SetHeader("From", eac.from)
	message.SetHeader("To", eac.to...)
	message.SetHeader("Subject", fmt.Sprintf("[%s] %s", strings.ToUpper(string(alert.Level)), alert.Title))

	// Create email body
	body := eac.createEmailBody(alert)
	message.SetBody("text/html", body)

	// Send email
	if err := eac.dialer.DialAndSend(message); err != nil {
		eac.logger.ErrorContext(ctx, "Failed to send email alert",
			"alert_id", alert.ID,
			"error", err,
		)
		return err
	}

	eac.logger.InfoContext(ctx, "Email alert sent successfully",
		"alert_id", alert.ID,
		"recipients", eac.to,
	)

	return nil
}

// createEmailBody creates the email body for an alert
func (eac *EmailAlertChannel) createEmailBody(alert *Alert) string {
	var body strings.Builder

	body.WriteString("<html><body>")
	body.WriteString(fmt.Sprintf("<h2>Alert: %s</h2>", alert.Title))
	body.WriteString(fmt.Sprintf("<p><strong>Level:</strong> %s</p>", alert.Level))
	body.WriteString(fmt.Sprintf("<p><strong>Type:</strong> %s</p>", alert.Type))
	body.WriteString(fmt.Sprintf("<p><strong>Time:</strong> %s</p>", alert.Timestamp.Format(time.RFC3339)))
	body.WriteString(fmt.Sprintf("<p><strong>Message:</strong> %s</p>", alert.Message))

	if len(alert.Details) > 0 {
		body.WriteString("<h3>Details:</h3>")
		body.WriteString("<ul>")
		for key, value := range alert.Details {
			body.WriteString(fmt.Sprintf("<li><strong>%s:</strong> %v</li>", key, value))
		}
		body.WriteString("</ul>")
	}

	if len(alert.Actions) > 0 {
		body.WriteString("<h3>Suggested Actions:</h3>")
		body.WriteString("<ul>")
		for _, action := range alert.Actions {
			body.WriteString(fmt.Sprintf("<li>%s</li>", action))
		}
		body.WriteString("</ul>")
	}

	if alert.Runbook != "" {
		body.WriteString(fmt.Sprintf("<p><strong>Runbook:</strong> <a href=\"%s\">%s</a></p>", alert.Runbook, alert.Runbook))
	}

	body.WriteString("</body></html>")

	return body.String()
}

// Name returns the channel name
func (eac *EmailAlertChannel) Name() string {
	return "email"
}

// IsEnabled returns true if the channel is enabled
func (eac *EmailAlertChannel) IsEnabled() bool {
	return eac.enabled
}

// SetEnabled sets the channel enabled state
func (eac *EmailAlertChannel) SetEnabled(enabled bool) {
	eac.enabled = enabled
}

// WebhookAlertChannel sends alerts to a webhook
type WebhookAlertChannel struct {
	url     string
	method  string
	headers map[string]string
	client  *http.Client
	enabled bool
	logger  *logger.Logger
}

// WebhookConfig represents webhook configuration
type WebhookConfig struct {
	URL     string            `json:"url"`
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers"`
	Timeout time.Duration     `json:"timeout"`
}

// NewWebhookAlertChannel creates a new webhook alert channel
func NewWebhookAlertChannel(config *WebhookConfig, logger *logger.Logger) *WebhookAlertChannel {
	if config.Method == "" {
		config.Method = "POST"
	}

	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	client := &http.Client{
		Timeout: config.Timeout,
	}

	return &WebhookAlertChannel{
		url:     config.URL,
		method:  config.Method,
		headers: config.Headers,
		client:  client,
		enabled: true,
		logger:  logger,
	}
}

// Send sends an alert to a webhook
func (wac *WebhookAlertChannel) Send(ctx context.Context, alert *Alert) error {
	if !wac.enabled {
		return nil
	}

	// Create webhook payload
	payload := map[string]interface{}{
		"alert":     alert,
		"timestamp": time.Now(),
		"source":    "hotel-reviews-error-handler",
	}

	// Marshal payload to JSON
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		wac.logger.ErrorContext(ctx, "Failed to marshal webhook payload",
			"alert_id", alert.ID,
			"error", err,
		)
		return err
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, wac.method, wac.url, strings.NewReader(string(jsonPayload)))
	if err != nil {
		wac.logger.ErrorContext(ctx, "Failed to create webhook request",
			"alert_id", alert.ID,
			"error", err,
		)
		return err
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	for key, value := range wac.headers {
		req.Header.Set(key, value)
	}

	// Send request
	resp, err := wac.client.Do(req)
	if err != nil {
		wac.logger.ErrorContext(ctx, "Failed to send webhook request",
			"alert_id", alert.ID,
			"url", wac.url,
			"error", err,
		)
		return err
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode >= 400 {
		wac.logger.ErrorContext(ctx, "Webhook request failed",
			"alert_id", alert.ID,
			"url", wac.url,
			"status_code", resp.StatusCode,
		)
		return fmt.Errorf("webhook request failed with status %d", resp.StatusCode)
	}

	wac.logger.InfoContext(ctx, "Webhook alert sent successfully",
		"alert_id", alert.ID,
		"url", wac.url,
		"status_code", resp.StatusCode,
	)

	return nil
}

// Name returns the channel name
func (wac *WebhookAlertChannel) Name() string {
	return "webhook"
}

// IsEnabled returns true if the channel is enabled
func (wac *WebhookAlertChannel) IsEnabled() bool {
	return wac.enabled
}

// SetEnabled sets the channel enabled state
func (wac *WebhookAlertChannel) SetEnabled(enabled bool) {
	wac.enabled = enabled
}

// SlackAlertChannel sends alerts to Slack
type SlackAlertChannel struct {
	webhookURL string
	channel    string
	username   string
	client     *http.Client
	enabled    bool
	logger     *logger.Logger
}

// SlackConfig represents Slack configuration
type SlackConfig struct {
	WebhookURL string        `json:"webhook_url"`
	Channel    string        `json:"channel"`
	Username   string        `json:"username"`
	Timeout    time.Duration `json:"timeout"`
}

// NewSlackAlertChannel creates a new Slack alert channel
func NewSlackAlertChannel(config *SlackConfig, logger *logger.Logger) *SlackAlertChannel {
	if config.Username == "" {
		config.Username = "Error Handler"
	}

	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	client := &http.Client{
		Timeout: config.Timeout,
	}

	return &SlackAlertChannel{
		webhookURL: config.WebhookURL,
		channel:    config.Channel,
		username:   config.Username,
		client:     client,
		enabled:    true,
		logger:     logger,
	}
}

// Send sends an alert to Slack
func (sac *SlackAlertChannel) Send(ctx context.Context, alert *Alert) error {
	if !sac.enabled {
		return nil
	}

	// Create Slack message
	message := sac.createSlackMessage(alert)

	// Marshal message to JSON
	jsonMessage, err := json.Marshal(message)
	if err != nil {
		sac.logger.ErrorContext(ctx, "Failed to marshal Slack message",
			"alert_id", alert.ID,
			"error", err,
		)
		return err
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", sac.webhookURL, strings.NewReader(string(jsonMessage)))
	if err != nil {
		sac.logger.ErrorContext(ctx, "Failed to create Slack request",
			"alert_id", alert.ID,
			"error", err,
		)
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	// Send request
	resp, err := sac.client.Do(req)
	if err != nil {
		sac.logger.ErrorContext(ctx, "Failed to send Slack request",
			"alert_id", alert.ID,
			"error", err,
		)
		return err
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode >= 400 {
		sac.logger.ErrorContext(ctx, "Slack request failed",
			"alert_id", alert.ID,
			"status_code", resp.StatusCode,
		)
		return fmt.Errorf("slack request failed with status %d", resp.StatusCode)
	}

	sac.logger.InfoContext(ctx, "Slack alert sent successfully",
		"alert_id", alert.ID,
		"channel", sac.channel,
	)

	return nil
}

// createSlackMessage creates a Slack message for an alert
func (sac *SlackAlertChannel) createSlackMessage(alert *Alert) map[string]interface{} {
	// Determine color based on alert level
	color := "good"
	switch alert.Level {
	case AlertLevelWarning:
		color = "warning"
	case AlertLevelError:
		color = "danger"
	case AlertLevelCritical:
		color = "danger"
	}

	// Create attachment
	attachment := map[string]interface{}{
		"color":     color,
		"title":     alert.Title,
		"text":      alert.Message,
		"timestamp": alert.Timestamp.Unix(),
		"fields": []map[string]interface{}{
			{
				"title": "Level",
				"value": string(alert.Level),
				"short": true,
			},
			{
				"title": "Type",
				"value": string(alert.Type),
				"short": true,
			},
			{
				"title": "Alert ID",
				"value": alert.ID,
				"short": true,
			},
			{
				"title": "Count",
				"value": fmt.Sprintf("%d", alert.Count),
				"short": true,
			},
		},
	}

	// Add actions if present
	if len(alert.Actions) > 0 {
		actionsText := strings.Join(alert.Actions, "\n• ")
		attachment["fields"] = append(attachment["fields"].([]map[string]interface{}), map[string]interface{}{
			"title": "Suggested Actions",
			"value": "• " + actionsText,
			"short": false,
		})
	}

	// Add runbook if present
	if alert.Runbook != "" {
		attachment["actions"] = []map[string]interface{}{
			{
				"type": "button",
				"text": "View Runbook",
				"url":  alert.Runbook,
			},
		}
	}

	// Create message
	message := map[string]interface{}{
		"username":    sac.username,
		"attachments": []map[string]interface{}{attachment},
	}

	if sac.channel != "" {
		message["channel"] = sac.channel
	}

	return message
}

// Name returns the channel name
func (sac *SlackAlertChannel) Name() string {
	return "slack"
}

// IsEnabled returns true if the channel is enabled
func (sac *SlackAlertChannel) IsEnabled() bool {
	return sac.enabled
}

// SetEnabled sets the channel enabled state
func (sac *SlackAlertChannel) SetEnabled(enabled bool) {
	sac.enabled = enabled
}

// ConsoleAlertChannel sends alerts to the console
type ConsoleAlertChannel struct {
	enabled bool
	logger  *logger.Logger
}

// NewConsoleAlertChannel creates a new console alert channel
func NewConsoleAlertChannel(logger *logger.Logger) *ConsoleAlertChannel {
	return &ConsoleAlertChannel{
		enabled: true,
		logger:  logger,
	}
}

// Send sends an alert to the console
func (cac *ConsoleAlertChannel) Send(ctx context.Context, alert *Alert) error {
	if !cac.enabled {
		return nil
	}

	// Create console output
	output := cac.createConsoleOutput(alert)

	// Print to console
	fmt.Print(output)

	return nil
}

// createConsoleOutput creates console output for an alert
func (cac *ConsoleAlertChannel) createConsoleOutput(alert *Alert) string {
	var output strings.Builder

	// Add header
	output.WriteString(repeat("=", 80) + "\n")
	output.WriteString(fmt.Sprintf("ALERT: %s\n", alert.Title))
	output.WriteString(repeat("=", 80) + "\n")

	// Add details
	output.WriteString(fmt.Sprintf("Level: %s\n", alert.Level))
	output.WriteString(fmt.Sprintf("Type: %s\n", alert.Type))
	output.WriteString(fmt.Sprintf("Time: %s\n", alert.Timestamp.Format(time.RFC3339)))
	output.WriteString(fmt.Sprintf("Message: %s\n", alert.Message))

	if len(alert.Details) > 0 {
		output.WriteString("\nDetails:\n")
		for key, value := range alert.Details {
			output.WriteString(fmt.Sprintf("  %s: %v\n", key, value))
		}
	}

	if len(alert.Actions) > 0 {
		output.WriteString("\nSuggested Actions:\n")
		for _, action := range alert.Actions {
			output.WriteString(fmt.Sprintf("  - %s\n", action))
		}
	}

	if alert.Runbook != "" {
		output.WriteString(fmt.Sprintf("\nRunbook: %s\n", alert.Runbook))
	}

	output.WriteString(repeat("=", 80) + "\n\n")

	return output.String()
}

// Name returns the channel name
func (cac *ConsoleAlertChannel) Name() string {
	return "console"
}

// IsEnabled returns true if the channel is enabled
func (cac *ConsoleAlertChannel) IsEnabled() bool {
	return cac.enabled
}

// SetEnabled sets the channel enabled state
func (cac *ConsoleAlertChannel) SetEnabled(enabled bool) {
	cac.enabled = enabled
}

// Helper function to repeat string
func repeat(s string, count int) string {
	return strings.Repeat(s, count)
}
