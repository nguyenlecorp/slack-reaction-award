package application

import (
	"fmt"
	"sync"
	"time"

	"github.com/mochisuna/slack-reaction-award/domain"
	"github.com/mochisuna/slack-reaction-award/handler"
	"github.com/nlopes/slack"
)

const HistoryScopeSize = 1000
const ParallelHistories = 10

type slackHandler struct {
	Client          *slack.Client
	Member          map[string]string
	oldestTimestamp string
	latestTimestamp string
}

func NewSlackHandler(token, oldestTimestamp, latestTimestamp string) (handler.SlackHandler, error) {
	cli := slack.New(token)
	users, err := cli.GetUsers()
	if err != nil {
		return nil, err
	}
	mem := make(map[string]string)
	for _, u := range users {
		mem[u.ID] = u.Name
	}
	return &slackHandler{
		Client:          cli,
		Member:          mem,
		oldestTimestamp: oldestTimestamp,
		latestTimestamp: latestTimestamp,
	}, nil
}

func (sh slackHandler) GetChannels() ([]domain.SlackChannel, error) {
	channels, err := sh.Client.GetChannels(false)
	if err != nil {
		fmt.Printf("Error in GetChannels: %s\n", err)
		return nil, err
	}
	list := []domain.SlackChannel{}
	for _, ch := range channels {
		list = append(
			list,
			domain.SlackChannel{
				ID:      ch.ID,
				Name:    ch.Name,
				IsGroup: ch.IsGroup,
			},
		)
	}
	return list, nil
}

// 取り回しやすい型に変形
func (sh slackHandler) parseSlackMessage(channelID string, history *slack.History) []domain.SlackMessage {
	msgs := make([]domain.SlackMessage, 0, 1000)
	// max:1000
	for _, msg := range history.Messages {
		reacs := []domain.SlackReaction{}
		count := 0
		// max: 50 * 27 * member
		for _, r := range msg.Reactions {
			mem := []string{}
			for _, u := range r.Users {
				mem = append(mem, sh.Member[u])
			}
			reacs = append(
				reacs,
				domain.SlackReaction{
					Name:  r.Name,
					Count: r.Count,
					Users: mem,
				},
			)
			count += r.Count
		}

		msgs = append(msgs,
			domain.SlackMessage{
				ChannelID:     channelID,
				Contributor:   sh.Member[msg.User],
				Text:          msg.Msg.Text,
				Timestamp:     msg.Timestamp,
				Reactions:     reacs,
				ReactionCount: count,
			},
		)
	}
	return msgs
}

func (sh slackHandler) GetChannelHistory(channel domain.SlackChannel) ([]domain.SlackMessage, error) {
	paramCh := make(chan slack.HistoryParameters, 5)
	msgCh := make(chan []domain.SlackMessage, 5)
	defer close(paramCh)
	defer close(msgCh)
	t := time.Now()

	paramWg := new(sync.WaitGroup)
	msgWg := new(sync.WaitGroup)
	miss := 0
	page := 1

	// キャパシティを多めにとっておく
	results := make([]domain.SlackMessage, 0, 3000)
	mu := &sync.Mutex{}
	for i := 0; i < ParallelHistories; i++ {
		go func() {
			for p := range paramCh {
				history, err := sh.Client.GetChannelHistory(channel.ID, p)
				if err != nil {
					// 定期的にリクエスト上限にぶつかるのでその場合は再検索
					// FIXME: エラーの種類でハンドリングしてないので、認証エラーとかだと詰む
					mu.Lock()
					// fmt.Printf("Error in getHistory(%s: %s): %v\n", channel.ID, channel.Name, err)
					miss++
					mu.Unlock()
					paramCh <- p
					continue
				}
				msgs := sh.parseSlackMessage(channel.ID, history)
				// ページングされている場合は期間を更新しながら再検索
				if history.HasMore {
					mu.Lock()
					page++
					mu.Unlock()
					paramWg.Add(1)
					paramCh <- slack.HistoryParameters{
						Latest: msgs[len(msgs)-1].Timestamp,
						Oldest: sh.oldestTimestamp,
						Count:  HistoryScopeSize,
					}
				}
				// バケツリレー
				msgWg.Add(1)
				msgCh <- msgs

				paramWg.Done()
			}
		}()
		// 並行処理。外に切り出したいけど面倒で・・・
		go func() {
			// リクエストがmsgChにきたら配列を合成して返す
			for msgs := range msgCh {
				results = append(results, msgs...)
				msgWg.Done()
			}
		}()
	}
	paramWg.Add(1)
	// 最初は数のみ（ここでOldestを入れると起点がOldestになってしまいやりにくい）
	paramCh <- slack.HistoryParameters{
		Count:  HistoryScopeSize,
		Latest: sh.latestTimestamp,
	}
	// パラメータ取得が全部終わるまで待つ
	paramWg.Wait()

	// slice更新が終わるまで待つ
	msgWg.Wait()
	fmt.Printf("Done: %v(%v) -> { time: %v, page: %v, miss: %v  }\n", channel.ID, channel.Name, time.Now().Sub(t), page, miss)
	return results, nil
}

func (sh slackHandler) GetPermalink(channelID, timestamp string) (string, error) {
	return sh.Client.GetPermalink(&slack.PermalinkParameters{Channel: channelID, Ts: timestamp})
}

func (sh slackHandler) PostMessage(channelID, text string) error {
	opt := slack.MsgOptionText(text, false)
	_, _, err := sh.Client.PostMessage(channelID, opt, slack.MsgOptionEnableLinkUnfurl())
	return err
}
