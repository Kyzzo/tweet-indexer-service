package service

import (
	"context"
	"runtime/debug"
	"strings"

	"github.com/jeetdotso/tweet-indexer-service/config"
	"github.com/jeetdotso/tweet-indexer-service/dummyaccount"
	"github.com/jeetdotso/tweet-indexer-service/indexer/mapper"
	"github.com/jeetdotso/tweet-indexer-service/indexer/model"
	"github.com/jeetdotso/tweet-indexer-service/indexer/repository"
	"github.com/jeetdotso/tweet-indexer-service/logg"
	"github.com/jeetdotso/tweet-indexer-service/persistence/mongodb"
	twitterTypes "github.com/jeetdotso/tweet-indexer-service/scrape/internals/api/types"
	"github.com/jeetdotso/tweet-indexer-service/scrape/tracking"
)

type GrokIndexer struct {
	grokRepo    repository.GrokEventRepository
	retweetRepo repository.RetweetEventRepository
	saasSvc     SaasIndexer
	dummSvc     dummyaccount.Service
}

func NewGrokIndexer(cfg config.Config, conn *mongodb.MongoConnection, dummSvc dummyaccount.Service) GrokIndexer {
	grokRepo := repository.NewGrokIndexRepo(conn)
	retweetRepo := repository.NewRetweetIndexRepo(conn)
	saasSvc := NewSaasIndexer(cfg, conn)
	return GrokIndexer{
		grokRepo:    grokRepo,
		retweetRepo: retweetRepo,
		dummSvc:     dummSvc,
		saasSvc:     saasSvc,
	}
}
func (gi *GrokIndexer) Start(ctx context.Context, tweet *model.TweetEvent, done chan struct{}) {
	// Check if the user is already in the database
	if tweet.TweetType == twitterTypes.QuoteRetweet || tweet.TweetType == twitterTypes.Retweet {
		return
	}
	exist, err := gi.grokRepo.FindAllByID(ctx, tweet.ID)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Failed to fetch tweet event by ID.")
		return
	}
	if exist != nil {
		// logg.Log.Warn().Caller().Ctx(ctx).Str("tweetID", tweetId).Msg("Grok event already exists.")
		return
	}
	// catch any panics inside the goroutine
	defer func() {
		if r := recover(); r != nil {
			logg.Log.Error().
				Caller().
				Ctx(ctx).
				Msgf("Recovered from panic in GetGrokCheck for tweet %s: %v\n%s",
					tweet.ID, r, debug.Stack(),
				)
		}
		if done != nil {
			close(done) // Signal completion
		}
	}()

	tracking.GetGrokCheck(ctx, tweet, gi.dummSvc, gi.HandleGrokEvent)

}

// ✅ **Handler for new tweets (passed as a callback)**
func (gi GrokIndexer) HandleGrokEvent(ctx context.Context, userId string, gr *twitterTypes.GrokResponse, selAccountUsername string) {
	if gr == nil {
		logg.Log.Error().Msg("HandleGrokEvent received a nil tweet")
		return
	}

	grokEvent := mapper.MapGrokToEvent(userId, gr, selAccountUsername)

	//Check if tweet already exists
	exist, err := gi.grokRepo.FindAllByID(ctx, grokEvent.ID)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Failed to fetch tweet event by ID.")
		return
	}
	if exist != nil {
		logg.Log.Warn().Caller().Ctx(ctx).Str("tweetID", grokEvent.ID).Msg("Tweet event already exists.")
		return
	}

	gi.streamGrok(ctx, grokEvent)

}

// func (gi GrokIndexer) HandleGrokEventRollback(ctx context.Context, userId string, grok *twitterTypes.GrokResponse) {
// 	if grok == nil {
// 		logg.Log.Error().Msg("HandleGrokEvent received a nil Grok")
// 		return
// 	}

// 	//Check if Grok already exists
// 	exist, err := gi.grokRepo.FindAllByID(ctx, grok.TweetId)
// 	if err != nil {
// 		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Failed to fetch Grok event by ID.")
// 		return
// 	}
// 	if exist != nil {
// 		logg.Log.Warn().Caller().Ctx(ctx).Str("GrokID", grok.TweetId).Msg("Grok event already exists.")
// 		return
// 	}

// 	grokEvent := mapper.MapGrokToEvent(userId, grok)
// 	gi.streamRollbackGrok(ctx, grokEvent)

// }

func (gi GrokIndexer) streamGrok(ctx context.Context, grokEvent model.GrokEvent) {
	if grokEvent.TokensShilled == nil {
		logg.Log.Error().Msg("HandleGrokEvent received a nil GrokEvent")
		return
	}
	_, err := gi.grokRepo.SaveOrUpdate(ctx, grokEvent)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Failed to save tweet event.")
		return
	}
	if !grokEvent.IsShill {
		grokEvent.TokensMentioned = []string{"NOT A SHILL"}
	}

	// if grokEvent.IsShill {
	///Start saas service
	gi.saasSvc.Start(ctx, &grokEvent)
	// }

	// tweetProto := tweetEvent.ToProto()

	// // Marshal to binary format
	// b, err := proto.Marshal(tweetProto)
	// if err != nil {
	// 	logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Failed to marshal proto message.")
	// 	return
	// }
	// // Produce message to Kafka
	// if err = gi.producer.Produce([]byte(LIVE_KEY), b); err != nil {
	// 	logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Failed to produce stream message.")
	// 	return
	// }

	// logg.Log.Info().Caller().Ctx(ctx).Str("tweetId", tweetEvent.ID).Msg("Tweet event sent!")

}
func (gi GrokIndexer) streamRollbackGrok(ctx context.Context, grokEvent model.GrokEvent) {
	_, err := gi.grokRepo.SaveOrUpdate(ctx, grokEvent)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Failed to save tweet event.")
		return
	}
	// tweetProto := tweetEvent.ToProto()

	// // Marshal to binary format
	// b, err := proto.Marshal(tweetProto)
	// if err != nil {
	// 	logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Failed to marshal proto message.")
	// 	return
	// }
	// // Produce message to Kafka
	// if err = gi.producer.Produce([]byte(ROLLBACK_KEY), b); err != nil {
	// 	logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Failed to produce stream message.")
	// 	return
	// }

	// logg.Log.Info().Caller().Ctx(ctx).Str("tweetId", tweetEvent.ID).Msg("Tweet event sent!")

}

func ReduceString(str string) string {
	reduced := strings.ReplaceAll(str, "$", "")
	reduced = strings.ReplaceAll(reduced, "#", "")
	reduced = strings.ReplaceAll(reduced, "!", "")
	reduced = strings.ReplaceAll(reduced, "@", "")
	reduced = strings.Trim(reduced, " ") // Remove extra spaces
	// Convert to lowercase
	reduced = strings.ToLower(reduced)

	return reduced
}

func MatchSpx(str string) bool {
	if str == "spx" || str == "spx6900" || strings.EqualFold(str, "0xe0f63a424a4439cbe457d80e4f4b51ad25b2c56c") || strings.EqualFold(str, strings.ToLower("J3NKxxXZcnNiMjKw9hYb2K4LUxgwB6t1FtPtQVsv3KFr")) || strings.EqualFold(str, strings.ToLower("0x50dA645f148798F68EF2d7dB7C1CB22A6819bb2C")) {
		return true
	}
	return false
}
