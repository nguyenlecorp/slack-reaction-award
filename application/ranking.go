package application

import (
	"sort"
	"strconv"
	"sync"

	"github.com/mochisuna/slack-reaction-award/domain"
	"github.com/mochisuna/slack-reaction-award/handler"
)

const NominateSize = 10
const ParallelRanking = 100

type rankingHandler struct {
	Messages        []domain.SlackMessage
	oldestTimestamp float64
	ranking         *domain.Ranking
	reactionMap     map[string]int
	wg              *sync.WaitGroup
	mu              *sync.Mutex
}

func newCategory() *domain.Category {
	nom := &domain.Category{
		Omoro:   make([]domain.Nominate, NominateSize),
		Variety: make([]domain.Nominate, NominateSize),
		Amount:  make([]domain.Nominate, NominateSize),
	}
	for i := 0; i < NominateSize; i++ {
		nom.Omoro[i].Count = -1
		nom.Variety[i].Count = -1
		nom.Amount[i].Count = -1
	}
	return nom
}

func NewRankingHandler(oldestTimestamp string) (handler.RankingHandler, error) {
	ts, err := strconv.ParseFloat(oldestTimestamp, 64)
	if err != nil {
		return nil, err
	}
	return &rankingHandler{
		ranking: &domain.Ranking{
			Category: newCategory(),
		},
		oldestTimestamp: ts,
		reactionMap:     make(map[string]int),
		wg:              new(sync.WaitGroup),
		mu:              &sync.Mutex{},
	}, nil
}

// より良い投稿が見つかった場合はランキングを更新
func compare(nom []domain.Nominate, msg domain.SlackMessage, count int, oldestTimestamp float64) {
	// 随時ソートしているので末端だけ見ればOK
	if nom[NominateSize-1].Count < count {
		nom[NominateSize-1].Count = count
		nom[NominateSize-1].Message = msg
		sort.SliceStable(nom, func(i, j int) bool { return nom[i].Count > nom[j].Count })
	}
}

// 結果をセット
func (rh rankingHandler) set(ch chan domain.SlackMessage) {
	// chで受付け並行処理で片付ける
	for msg := range ch {
		omoro := 0
		variety := 0
		amount := 0
		for _, reac := range msg.Reactions {
			amount += reac.Count
			variety++
			rh.mu.Lock()
			rh.reactionMap[reac.Name] += reac.Count
			rh.mu.Unlock()
			if reac.IsOmoro() {
				omoro += reac.Count
			}
		}

		compare(rh.ranking.Category.Omoro, msg, omoro, rh.oldestTimestamp)
		compare(rh.ranking.Category.Variety, msg, variety, rh.oldestTimestamp)
		compare(rh.ranking.Category.Amount, msg, amount, rh.oldestTimestamp)
		rh.wg.Done()
	}
}

// mapだと取り回しにくいので配列として取り扱えるように変形
func (rh rankingHandler) setReaction() {
	reac := make([]domain.Reaction, 0, 1000)
	for k, v := range rh.reactionMap {
		reac = append(reac, domain.Reaction{k, v})
	}

	sort.Slice(reac, func(i, j int) bool {
		return reac[i].Value > reac[j].Value
	})
	rh.ranking.Reactions = reac
}

func (rh rankingHandler) GetRanking(messages []domain.SlackMessage) *domain.Ranking {
	msgCh := make(chan domain.SlackMessage, 1000)
	defer close(msgCh)
	// 数が多いので来た順に一気に片付ける
	for i := 0; i < ParallelRanking; i++ {
		go rh.set(msgCh)
	}
	for _, m := range messages {
		i, _ := strconv.ParseFloat(m.Timestamp, 64)
		if i < rh.oldestTimestamp {
			// 1000件単位でとるためか、たまに古い奴が来てしまうことがあったため対策を入れる
			// fmt.Printf("too old: %#v\n", m)
			continue
		}
		rh.wg.Add(1)
		msgCh <- m
	}
	rh.wg.Wait()
	rh.setReaction()
	return rh.ranking
}
