// Binary slendmail - see README.md
package main

import (
	"fmt"
	"io"
	"log"
	"log/syslog"
	"os"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"github.com/slack-go/slack"
)

// Config - toml config struct
type Config struct {
	SlackToken string `toml:"slack_token"`
	Channel    string
	SyslogTag  string `toml:"syslog_tag"`
}

func main() {
	var subject string
	var body []string
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

	api := slack.New(config.SlackToken)

	// setup syslogger, we use this instead of regular output so we can see the output in the
	// case of being called from crond
	sl, err := syslog.New(syslog.LOG_WARNING|syslog.LOG_MAIL, config.SyslogTag)
	if err != nil {
		log.Fatal("failed to setup syslog connection ", err)
	}

	// parse stdin, check RFC5321 for specifics of the format
	// this probably needs to be beefed up a bit to handle other
	// callers. So far only tested with busybox/Alpine crond
	stdin, err := io.ReadAll(os.Stdin)
	if err != nil {
		log.Fatal("failed to read anything from stdin ", err)
	}
	lines := strings.Split(string(stdin), "\n")
	for n, line := range lines {
		if strings.HasPrefix(line, "Subject: ") {
			subject = strings.SplitAfterN(line, " ", 2)[1]
		}
		if line == "" {
			body = lines[n:]
			break
		}
	}

	// setup slack message
	smsg := slack.MsgOptionBlocks(
		slack.NewSectionBlock(
			slack.NewTextBlockObject("mrkdwn", "*Subject:* "+subject, false, false), nil, nil,
		),
		slack.NewDividerBlock(),
		slack.NewSectionBlock(
			slack.NewTextBlockObject("mrkdwn", strings.Join(body, "\n"), false, false), nil, nil,
		),
		slack.NewDividerBlock(),
	)

	msgchan, msgts, err := api.PostMessage(
		config.Channel,
		smsg,
	)
	if err != nil {
		log.Fatal("failed to post message ", err)
	}

	sl.Debug(fmt.Sprintf("channel: %s - ts: %s - argv: %v", msgchan, msgts, os.Args)) //nolint:errcheck
	sl.Debug(string(stdin))                                                           //nolint:errcheck
}
