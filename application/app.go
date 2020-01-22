package application

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/mochisuna/slack-reaction-award/domain"
	"github.com/mochisuna/slack-reaction-award/handler"
)

const ParallelChannels = 200
const NumOfReaction = 10

func GetDatetime(year int) (string, string, error) {
	loc, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		return "", "", err
	}
	oldestTimestamp := time.Date(year, 1, 1, 0, 0, 0, 0, loc)
	latestTimestamp := time.Date(year, 12, 31, 23, 59, 59, 999999, loc)
	return strconv.FormatInt(oldestTimestamp.Unix(), 10), strconv.FormatInt(latestTimestamp.Unix(), 10), nil

}

func Run(sh handler.SlackHandler, rh handler.RankingHandler, postChannelID string) {
	t := time.Now()
	// チャンネルを全部とる（public）
	channels, err := sh.GetChannels()
	if err != nil {
		fmt.Printf("Error on GetChannels: %+v", err)
		return
	}

	// よっしゃ全部走らせるぞー(白目)
	results := make([]domain.SlackMessage, 0, 30000)
	mu := &sync.Mutex{}
	chanCh := make(chan domain.SlackChannel, ParallelChannels)
	defer close(chanCh)
	wg := new(sync.WaitGroup)
	for i := 0; i < ParallelChannels; i++ {
		go func() {
			for channel := range chanCh {
				result, err := sh.GetChannelHistory(channel)
				if err != nil {
					fmt.Printf("Error on GetChannelHistory: %+v", err)
					return
				}
				mu.Lock()
				results = append(results, result...)
				mu.Unlock()
				wg.Done()
			}
		}()
	}
	siz := len(channels)
	for i, c := range channels {
		fmt.Printf("%v/%v: %v (%v)\n", i+1, siz, c.ID, c.Name)
		wg.Add(1)
		chanCh <- c
	}
	wg.Wait()

	// 本当はこれもappendするまえに並行処理で受け取った方が良い（けど複雑になるからやらない）
	ranking := rh.GetRanking(results)
	reacs := "*最も使われたリアクションランキング！*\n"
	for i, reac := range ranking.Reactions {
		reacs += fmt.Sprintf("%v位 :%v: ： %v回\n", i+1, reac.Key, reac.Value)
		if i > NumOfReaction {
			break
		}
	}
	if err = sh.PostMessage(postChannelID, reacs); err != nil {
		panic(fmt.Sprintf("Error occured when post slack: %+v", err))
	}

	channelMsg := fmt.Sprintf("アクティブなチャンネル数: %v個\n集計した投稿数: %v個", siz, len(results))
	if err = sh.PostMessage(postChannelID, channelMsg); err != nil {
		fmt.Printf("Error occured when post slack: %v", err)
	}

	// 何度も書くのめんどいので雑に処理
	post := func(header, unit string, nominate []domain.Nominate) error {
		rank := []string{
			"優勝",
			"準優勝",
			"第3位",
		}
		// 受賞タイトルも入れとこう
		if err = sh.PostMessage(postChannelID, header); err != nil {
			return err
		}
		for i, nom := range nominate {
			if i >= len(rank) {
				break
			}
			// 参照リンクがあるとよりそれっぽい
			url, err := sh.GetPermalink(nom.Message.ChannelID, nom.Message.Timestamp)
			if err != nil {
				fmt.Printf("Error occured when getPermalink slack: %v", err)
				return err
			}
			if err = sh.PostMessage(postChannelID, fmt.Sprintf("%v： %v%v\n%v\n", rank[i], nom.Count, unit, url)); err != nil {
				fmt.Printf("Error occured when post slack: %v", err)
				return err
			}
		}
		return nil
	}
	// 多分エラーハンドリングした方がいい
	post("*reacji_omoro大賞*", "オモロ", ranking.Category.Omoro)
	post("*沢山リアクションがついた大賞（種類部門）*", "種類", ranking.Category.Variety)
	post("*沢山リアクションがついた大賞（数部門）*", "個", ranking.Category.Amount)
	fmt.Printf("実行時間: %v\n", time.Now().Sub(t))
}
