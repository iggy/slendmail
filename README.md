# Slendmail

A portmanteau of slack and sendmail

## sendmail meets modern notifications (nee Slack)

A cron compatible sendmail alternative that sends messages to multiple notification backends instead of email.

Since most people can't actually send email from their computers these days, send the messages via Slack, Mattermost, Discord, or any webhook-compatible service instead.

This is especially useful for homelabbers that can't/don't want to setup something like SES or run their own MTA that can actually send somewhere useful.

## Features

- **Multiple Backends**: Support for Slack, webhooks (Mattermost, Discord, etc.)
- **Flexible Configuration**: TOML-based configuration with support for multiple instances of each backend
- **Webhook Formats**: Built-in support for Mattermost, Slack-compatible, and generic webhook formats
- **Cron Compatible**: Drop-in replacement for sendmail in cron jobs
- **Reliable**: Continues sending to other backends even if one fails
- **Logging**: Comprehensive syslog integration for monitoring and debugging

## Supported Backends

### Slack
- Native Slack API integration
- Rich message formatting with subject and hostname
- Support for multiple Slack workspaces/channels

### Webhook (Generic)
- **Mattermost**: Full compatibility with Mattermost incoming webhooks
- **Discord**: Slack-compatible webhook format
- **Custom APIs**: Generic JSON format for custom integrations
- **Flexible Headers**: Custom HTTP headers for authentication

### Future Backends
The architecture is designed to easily support additional backends like Matrix, MQTT, Pushbullet, IRC, Signal, etc.

## Configuration

Configuration file should be placed at `/etc/slendmail.conf` in TOML format.

### Basic Example

```toml
# Optional syslog tag (defaults to "slendmail")
syslog_tag = "slendmail"

[backends]

# Slack backend
[[backends.slack]]
name = "alerts"
token = "xoxb-your-slack-bot-token-here"
channel = "#alerts"

# Mattermost webhook
[[backends.webhook]]
name = "mattermost"
url = "https://your-mattermost.com/hooks/xxx-generatedkey-xxx"
format = "mattermost"
username = "slendmail"
```

### Complete Example

```toml
syslog_tag = "slendmail"

[backends]

# Multiple Slack configurations
[[backends.slack]]
name = "production-alerts"
token = "xoxb-prod-token"
channel = "#alerts"

[[backends.slack]]
name = "dev-notifications"
token = "xoxb-dev-token"
channel = "#dev-alerts"

# Mattermost webhook
[[backends.webhook]]
name = "mattermost-alerts"
url = "https://mattermost.example.com/hooks/xxx-key-xxx"
format = "mattermost"
username = "slendmail"
channel = "alerts"

# Discord webhook (using Slack compatibility)
[[backends.webhook]]
name = "discord"
url = "https://discord.com/api/webhooks/123/token/slack"
format = "slack"
username = "Server Bot"

# Custom API with authentication
[[backends.webhook]]
name = "custom-api"
url = "https://api.example.com/notifications"
format = "generic"
[backends.webhook.headers]
"Authorization" = "Bearer your-api-token"
"Content-Type" = "application/json"
```

## Backend Configuration

### Slack Backend

```toml
[[backends.slack]]
name = "optional-name"          # Optional: friendly name for logging
token = "xoxb-your-bot-token"   # Required: Slack bot token
channel = "#channel-name"       # Required: Target channel
```

### Webhook Backend

```toml
[[backends.webhook]]
name = "optional-name"          # Optional: friendly name for logging
url = "https://webhook.url"     # Required: webhook endpoint URL
format = "mattermost"           # Required: "mattermost", "slack", or "generic"
username = "bot-name"           # Optional: override webhook username
channel = "#channel"            # Optional: override webhook channel
[backends.webhook.headers]      # Optional: custom HTTP headers
"Authorization" = "Bearer token"
```

#### Webhook Formats

- **`mattermost`**: Mattermost-compatible format with Markdown formatting
- **`slack`**: Slack-compatible format (works with Discord webhooks ending in `/slack`)
- **`generic`**: Simple JSON with separate `subject`, `body`, and `hostname` fields

## Installation & Setup

1. **Build the binary**:
   ```bash
   go build -o slendmail main.go
   ```

2. **Install system-wide**:
   ```bash
   sudo cp slendmail /usr/local/bin/
   sudo chmod +x /usr/local/bin/slendmail
   ```

3. **Create configuration**:
   ```bash
   sudo cp slendmail.conf.example /etc/slendmail.conf
   sudo chmod 600 /etc/slendmail.conf  # Protect tokens
   ```

4. **Configure cron**:

   **Alpine/BusyBox**: Add to `/etc/crontabs/root`:
   ```bash
   MAILTO=""
   SENDMAIL="/usr/local/bin/slendmail"
   ```

   **Ubuntu/Debian**: Add to `/etc/crontab`:
   ```bash
   MAILTO=""
   SENDMAIL="/usr/local/bin/slendmail"
   ```

## Usage

Slendmail is designed as a drop-in replacement for sendmail. It reads email messages from stdin and parses the subject and body to send via configured backends.

### Direct Usage
```bash
echo -e "Subject: Test Message\n\nThis is a test" | ./slendmail
```

### Cron Integration
Once configured as the system sendmail alternative, any cron job output will automatically be sent through slendmail:

```bash
# This cron job's output will be sent via slendmail
0 2 * * * /path/to/backup-script.sh
```

## Message Format

Slendmail parses standard email format from stdin:

```
Subject: Your Subject Here

Message body goes here.
Multiple lines are supported.
```

The parsed message includes:
- **Subject**: Extracted from email headers
- **Body**: Email message content
- **Hostname**: Automatically detected system hostname

## Logging

Slendmail uses syslog for comprehensive logging:

- **Debug**: Successful backend operations and message content
- **Error**: Backend failures and configuration issues
- **Info**: General operational messages

View logs with:
```bash
# System logs
journalctl -t slendmail

# Traditional syslog
tail -f /var/log/messages | grep slendmail
```

## Error Handling

- **Partial Failures**: If some backends fail, slendmail continues with successful backends
- **Complete Failure**: Only exits with error if ALL configured backends fail
- **Logging**: All failures are logged to syslog with detailed error messages

## Backend Development

Adding new backends is straightforward. Implement the `Backend` interface:

```go
type Backend interface {
    Send(msg *EmailMessage) error
    Name() string
}
```

See the existing `SlackBackend` and `WebhookBackend` implementations as examples.

## TODO

- Look for config in more/configurable places
- More documentation on how to setup the Slack app
- Better compatibility with different cron daemons and other utils that call sendmail directly
- Matrix backend
- MQTT backend
- Pushbullet backend
- IRC backend
- Signal backend
- Message templates/formatting options
- Retry functionality (not really sure how this could work since it's not a running daemon, maybe write out to a file or something)
- Configuration validation

## Compatibility Notes

- **Alpine/BusyBox cron**: Calls sendmail with `sendmail -ti`
  - `-i`: ignore dots alone (finish processing at end of input)
  - `-t`: read headers for to/cc/bcc (parsed but not used)
- **Standard cron**: Compatible with most cron implementations

## Security Considerations

- Store the configuration file with restricted permissions when possible (`chmod 600`)
- Webhook URLs should be treated as secrets
