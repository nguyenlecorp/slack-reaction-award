package handler

import (
	"github.com/mochisuna/slack-reaction-award/domain"
)

const NominateSize = 10
const ParallelRanking = 100

type RankingHandler interface {
	GetRanking(messages []domain.SlackMessage) *domain.Ranking
}
