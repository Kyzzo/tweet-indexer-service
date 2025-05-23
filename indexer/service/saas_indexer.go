package service

import (
	"context"
	"fmt"
	"runtime/debug"

	"github.com/jeetdotso/tweet-indexer-service/config"
	"github.com/jeetdotso/tweet-indexer-service/indexer/model"
	"github.com/jeetdotso/tweet-indexer-service/indexer/repository"
	"github.com/jeetdotso/tweet-indexer-service/logg"
	"github.com/jeetdotso/tweet-indexer-service/persistence/mongodb"
	"github.com/jeetdotso/tweet-indexer-service/stream"
	"github.com/jeetdotso/tweet-indexer-service/stream/kafka"
	"google.golang.org/protobuf/proto"
)

type SaasIndexer struct {
	grokRepo  repository.GrokEventRepository
	tweetRepo repository.TweetEventRepository
	producer  stream.Producer
}

const (
	SAAS_TOPIC_NAME = "shill"
)

var (
	SAAS_TOPIC_CON = fmt.Sprintf("%s.%s", stream.StreamTweetPrefixTopic, SAAS_TOPIC_NAME)
)

func NewSaasIndexer(cfg config.Config, conn *mongodb.MongoConnection) SaasIndexer {
	grokRepo := repository.NewGrokIndexRepo(conn)
	tweetRepo := repository.NewTweetIndexRepo(conn)
	producer := kafka.NewProducer(cfg.Stream, SAAS_TOPIC_CON)
	return SaasIndexer{
		grokRepo:  grokRepo,
		tweetRepo: tweetRepo,
		producer:  producer,
	}
}

func (si *SaasIndexer) TopicName() string {
	return SAAS_TOPIC_CON
}

func (si *SaasIndexer) Start(ctx context.Context, grokEvent *model.GrokEvent) {
	// Check if the user is already in the database
	tweet, err := si.tweetRepo.FindByTweetId(ctx, grokEvent.ID)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Failed to fetch tweet event by ID.")
		return
	}
	if tweet == nil {
		logg.Log.Warn().Caller().Ctx(ctx).Str("tweetID", grokEvent.ID).Msg("Tweet event does not exists.")
		return
	}
	saasEvent := model.SaasEvent{
		ID:            grokEvent.ID,
		TweetId:       tweet.TweetID,
		Text:          tweet.Text,
		CreatedAt:     tweet.CreatedAt,
		UserId:        tweet.UserID,
		TweetType:     string(tweet.TweetType),
		TweetUrl:      tweet.GetURL(),
		TokensShilled: grokEvent.TokensShilled,
	}

	go si.streamTweet(ctx, saasEvent)
}
func (si *SaasIndexer) StartTweet(ctx context.Context, gr *model.GrokEvent, tweetEvent *model.TweetEvent) {
	// Check if the user is already in the database

	saasEvent := model.SaasEvent{
		ID:            tweetEvent.ID,
		TweetId:       tweetEvent.TweetID,
		Text:          tweetEvent.Text,
		CreatedAt:     tweetEvent.CreatedAt,
		UserId:        tweetEvent.UserID,
		TweetType:     string(tweetEvent.TweetType),
		TweetUrl:      tweetEvent.GetURL(),
		TokensShilled: gr.TokensShilled,
	}

	go si.streamTweet(ctx, saasEvent)
}

func (si *SaasIndexer) streamTweet(ctx context.Context, saasEvt model.SaasEvent) {
	// catch any panics inside the goroutine
	defer func() {
		if r := recover(); r != nil {
			logg.Log.Error().
				Caller().
				Ctx(ctx).
				Msgf("Recovered from panic in GetGrokCheck for tweet %s: %v\n%s",
					saasEvt.ID, r, debug.Stack(),
				)
		}
	}()

	saasProto := saasEvt.ToProto()
	b, err := proto.Marshal(saasProto)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Failed to marshal proto message.")
		return
	}
	// Produce message to Kafka
	if err = si.producer.Produce([]byte(LIVE_KEY), b); err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Failed to produce stream message.")
		return
	}

	logg.Log.Info().Caller().Ctx(ctx).Str("tweetId", saasEvt.ID).Msg("Saas Tweet event sent!")

}
