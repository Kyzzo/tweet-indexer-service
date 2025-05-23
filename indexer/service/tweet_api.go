package service

import (
	"context"

	"github.com/jeetdotso/tweet-indexer-service/config"
	"github.com/jeetdotso/tweet-indexer-service/indexer/model"
	"github.com/jeetdotso/tweet-indexer-service/indexer/repository"
	"github.com/jeetdotso/tweet-indexer-service/logg"
	"github.com/jeetdotso/tweet-indexer-service/persistence/mongodb"
)

type TweetAPI struct {
	repository repository.TweetEventRepository
	// producer   stream.Producer
}

func NewTweetAPI(cfg config.Config, conn *mongodb.MongoConnection) TweetAPI {
	repo := repository.NewTweetIndexRepo(conn)

	// producer := kafka.NewProducer(cfg.Stream, stream.StreamTweetIndexReplayTopic)

	return TweetAPI{
		repository: repo,
		// producer:   producer,
	}
}

// ✅ Replay all stored tweet events
func (api TweetAPI) ReplayEvents(ctx context.Context, idx int64) error {
	tweetEvents, err := api.repository.FindAll(ctx, idx)
	if err != nil {
		return err
	}
	for _, tweet := range tweetEvents {
		api.streamTweet(ctx, tweet)
	}
	return nil
}

// ✅ Replay tweets for a specific user
func (api TweetAPI) ReplayEventsByUserID(ctx context.Context, userID string, idx int64) error {
	tweetEvents, err := api.repository.FindAllByUserId(ctx, userID, idx)
	if err != nil {
		return err
	}
	for _, tweet := range tweetEvents {
		api.streamTweet(ctx, tweet)
	}
	return nil
}

// ✅ Replay a specific tweet event by ID
func (api TweetAPI) ReplayEventByID(ctx context.Context, id string) error {
	tweetEvent, err := api.repository.FindAllByID(ctx, id)
	if err != nil || tweetEvent == nil {
		return err
	}
	api.streamTweet(ctx, *tweetEvent)
	return nil
}

// ✅ Stream a tweet event to Kafka
func (api TweetAPI) streamTweet(ctx context.Context, tweet model.TweetEvent) {
	// Convert to Proto
	// tweetProto := tweet.ToProto()

	// // Marshal to binary format
	// b, err := proto.Marshal(tweetProto)
	// if err != nil {
	// 	logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Failed to marshal proto message.")
	// 	return
	// }

	// // Produce message to Kafka
	// if err = api.producer.Produce([]byte(""), b); err != nil {
	// 	logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Failed to produce stream message.")
	// 	return
	// }
	logg.Log.Info().Caller().Ctx(ctx).Str("tweetId", tweet.ID).Msg("Tweet event sent!")
}
