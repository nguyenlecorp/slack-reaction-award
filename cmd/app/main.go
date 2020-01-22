package main

import (
	"flag"
	"fmt"

	app "github.com/mochisuna/slack-reaction-award/application"
	"github.com/mochisuna/slack-reaction-award/config"
)

func main() {
	env := flag.String("e", "local", "environment")
	flag.Parse()
	// import config
	conf, err := config.New(*env)
	if err != nil {
		panic(fmt.Sprintf("Loading config failed. err: %+v", err))
	}
	oldestTimestamp, latestTimestamp, err := app.GetDatetime(conf.Slack.Year)
	// init handler
	sh, err := app.NewSlackHandler(conf.Slack.Token, oldestTimestamp, latestTimestamp)
	if err != nil {
		panic(fmt.Sprintf("Error on new SlackgetHandler: %+v", err))
	}
	rh, err := app.NewRankingHandler(oldestTimestamp)
	if err != nil {
		panic(fmt.Sprintf("Error on new RankingHandler: %+v", err))
	}

	app.Run(sh, rh, conf.Slack.PostChannel)
}
