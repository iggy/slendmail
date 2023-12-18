// Binary slendmail - see README.md
package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"log/syslog"
	"net/mail"
	"os"

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

	// setup slack message
	attach := new(slack.Attachment)
	attach.Text = string(body)
	hostname, _ := os.Hostname()
	subjText := slack.NewTextBlockObject("mrkdwn", "*Subject:* "+msg.Header.Get("Subject"), false, false)
	hostText := slack.NewTextBlockObject("mrkdwn", "*Hostname:* "+hostname, false, false)
	hdrBlock := make([]*slack.TextBlockObject, 0)
	hdrBlock = append(hdrBlock, subjText)
	hdrBlock = append(hdrBlock, hostText)
	smsg := slack.MsgOptionBlocks(
		slack.NewSectionBlock(
			nil,
			hdrBlock,
			nil,
		),
	)

	msgchan, msgts, err := api.PostMessage(
		config.Channel,
		smsg,
		slack.MsgOptionAttachments(*attach),
	)
	if err != nil {
		_ = sl.Err(fmt.Sprintln("failed to post message ", err))
		log.Println("body", string(body))
		log.Fatal("failed to post message ", err)
	}

	_ = sl.Debug(fmt.Sprintf("channel: %s - ts: %s - argv: %v", msgchan, msgts, os.Args)) //nolint:errcheck
}
