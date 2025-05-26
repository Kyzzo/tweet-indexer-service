package service

import (
	"context"
	"runtime/debug"
	"strings"

	"github.com/jeetdotso/tweet-indexer-service/config"
	"github.com/jeetdotso/tweet-indexer-service/dummyaccount"
	"github.com/jeetdotso/tweet-indexer-service/indexer/model"
	"github.com/jeetdotso/tweet-indexer-service/indexer/repository"
	"github.com/jeetdotso/tweet-indexer-service/logg"
	"github.com/jeetdotso/tweet-indexer-service/persistence/mongodb"
	twitterTypes "github.com/jeetdotso/tweet-indexer-service/scrape/internals/api/types"
	"github.com/jeetdotso/tweet-indexer-service/scrape/tracking"
)

type GrokIndexer struct {
	grokRepo      repository.GrokEventRepository
	retweetRepo   repository.RetweetEventRepository
	saasSvc       SaasIndexer
	dummSvc       dummyaccount.Service
	grokTokenRepo repository.GrokTokenEventRepository
	grokUrl       string
}

func NewGrokIndexer(cfg config.Config, conn *mongodb.MongoConnection, dummSvc dummyaccount.Service, saasInx SaasIndexer) GrokIndexer {
	grokRepo := repository.NewGrokIndexRepo(conn)
	retweetRepo := repository.NewRetweetIndexRepo(conn)

	grokTokenRepo := repository.NewGrokTokenRepo(conn)
	url := cfg.Services.GrokAPIServices.Host
	return GrokIndexer{
		grokRepo:      grokRepo,
		retweetRepo:   retweetRepo,
		dummSvc:       dummSvc,
		saasSvc:       saasInx,
		grokTokenRepo: grokTokenRepo,
		grokUrl:       url,
	}
}
func (gi *GrokIndexer) Start(ctx context.Context, tweet *model.TweetEvent, done chan struct{}) {
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
	// Check if the user is already in the database
	if tweet.TweetType == twitterTypes.Reply || tweet.TweetType == twitterTypes.Retweet || tweet.TweetType == twitterTypes.QuoteRetweet {
		return
	}

	exist, err := gi.grokRepo.FindAllByID(ctx, tweet.TweetID)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Failed to fetch tweet event by ID.")
		return
	}
	if exist != nil {
		if !exist.IsShill {
			return
		}
		gi.saasSvc.StartTweet(ctx, exist, tweet)
		return
	}

	tracking.GetGrokCheck(ctx, tweet, gi.dummSvc, gi.grokUrl, gi.HandleGrokEvent)
}

// ✅ **Handler for new tweets (passed as a callback)**
func (gi GrokIndexer) HandleGrokEvent(ctx context.Context, grokEvent *model.GrokEvent) {
	if grokEvent == nil {
		logg.Log.Error().Msg("HandleGrokEvent received a nil tweet")
		return
	}

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

	gi.streamGrok(ctx, *grokEvent)

}
func (gi GrokIndexer) HandleGrokTokenEvent(ctx context.Context, userId string, grokEvent *model.GrokEvent, selAccountUsername string) {
	if grokEvent == nil {
		logg.Log.Error().Msg("HandleGrokEvent received a nil tweet")
		return
	}

	//Check if tweet already exists
	exist, err := gi.grokTokenRepo.FindAllByID(ctx, grokEvent.ID)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Failed to fetch tweet event by ID.")
		return
	}
	if exist != nil {
		logg.Log.Warn().Caller().Ctx(ctx).Str("tweetID", grokEvent.ID).Msg("Tweet event already exists.")
		return
	}

	gi.streamGrok(ctx, *grokEvent)

}

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
		return
	}

	///Start saas service
	gi.saasSvc.Start(ctx, &grokEvent)
}

// func (gi GrokIndexer) streamTokenGrok(ctx context.Context, grokEvent model.GrokTokenEvent) {

// 	_, err := gi.grokTokenRepo.SaveOrUpdate(ctx, grokEvent)
// 	if err != nil {
// 		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Failed to save tweet event.")
// 		return
// 	}
// 	if grokEvent.IsPromoting {
// 		if err = gi.listSvc.Clicked(ctx, grokEvent.UserId); err != nil {
// 			logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Failed to save tweet event.")
// 			return
// 		}
// 	}
// }

func (gi GrokIndexer) streamRollbackGrok(ctx context.Context, grokEvent model.GrokEvent) {
	_, err := gi.grokRepo.SaveOrUpdate(ctx, grokEvent)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Failed to save tweet event.")
		return
	}

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
