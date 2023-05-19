package main

import (
	"testing"

	"github.com/slack-go/slack/slackevents"
)

func TestShouldSkipMessage(t *testing.T) {
	me := BotInfo{
		ID: "U123456",
	}

	tests := []struct {
		name  string
		event *slackevents.MessageEvent
		skip  bool
	}{
		{
			name: "Skip channel topic",
			event: &slackevents.MessageEvent{
				SubType: "channel_topic",
			},
			skip: true,
		},
		{
			name: "Skip quote",
			event: &slackevents.MessageEvent{
				Text: "&gt; This is a quote",
			},
			skip: true,
		},
		{
			name: "Skip message with other user's username",
			event: &slackevents.MessageEvent{
				Text: "Hello, <@U654321>!",
			},
			skip: true,
		},
		{
			name: "Don't skip message with bot's username",
			event: &slackevents.MessageEvent{
				Text: "Hello, <@U123456>!",
			},
			skip: false,
		},
		{
			name: "Skip message with bot's username and other user's username",
			event: &slackevents.MessageEvent{
				Text: "Hello, <@U123456>! My name is <@U654321>.",
			},
			skip: true,
		},
		{
			name: "Skip message with URL",
			event: &slackevents.MessageEvent{
				Text: "Check out this website: https://example.com",
			},
			skip: true,
		},
		{
			name: "Don't skip normal message",
			event: &slackevents.MessageEvent{
				Text: "This is a normal message.",
			},
			skip: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldSkipMessage(tt.event, me); got != tt.skip {
				t.Errorf("shouldSkipMessage() = %v, want %v", got, tt.skip)
			}
		})
	}
}
