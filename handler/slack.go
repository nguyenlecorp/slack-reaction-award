package handler

import (
	"github.com/mochisuna/slack-reaction-award/domain"
)

type SlackHandler interface {
	GetChannels() ([]domain.SlackChannel, error)
	GetChannelHistory(channel domain.SlackChannel) ([]domain.SlackMessage, error)
	GetPermalink(channelID, timestamp string) (string, error)
	PostMessage(channelID, text string) error
}
