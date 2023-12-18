# Slendmail

A portmanteau of slack and sendmail

## sendmail meets slack

A cron compatible sendmail alternative that sends messages to slack instead of
email.

Since most people can't actually send email from their computers these days,
send the messages via slack instead.

This is especially useful for homelabbers that can't/don't want to setup
something like SES or run their own MTA that can actually send somewhere useful

## Configuration

Example toml config file. Should be placed at `/etc/slendmail.conf`

```toml
# best to quote all the strings below. I don't think it's strictly necessary
# but I had weird issues with the go-toml library otherwise
slack_token = "xoxb-123456789012-12345678901234-l;iqwjecacwiejfQWERoifqjwQWE"  # gitleaks:allow this isn't a real token
channel = "#notifications-cron"
```

Alpine: Add `MAILTO` to /etc/crontabs/root
Ubuntu: Add `MAILTO` to /etc/crontab

## TODO

* More compatibility
* Look for config in more/configurable places
* config docs (the slack side)

## Extra Info

* alpine's (busybox's) cron calls sendmail with `sendmail -ti`
  * -i - ignore dots alone - i.e. finish processing at end of input
  * -t - read headers for to/cc/bcc - not applicable
