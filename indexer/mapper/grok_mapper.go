package mapper

import (
	"time"

	"github.com/jeetdotso/tweet-indexer-service/indexer/model"
	twitterTypes "github.com/jeetdotso/tweet-indexer-service/scrape/internals/api/types"
)

// ✅ Convert Twitter API response to TweetEvent
func MapGrokToEvent(userID string, gr *twitterTypes.GrokResponse, selAccount string) model.GrokEvent {
	return model.GrokEvent{
		ID:              gr.TweetId,
		IsShill:         gr.IsShill,
		TokensMentioned: gr.TokensMentioned,
		TokensShilled:   gr.TokensShilled,
		UserId:          userID,
		CreatedAt:       time.Now(),
		DummyAccount:    selAccount,
	}
}
func MapGrokTokenToEvent(userID string, gr *twitterTypes.GrokTokenResponse, selAccount string) model.GrokTokenEvent {
	return model.GrokTokenEvent{
		ID:           gr.TweetId,
		IsPromoting:  gr.IsPromoting,
		UserId:       userID,
		CreatedAt:    time.Now(),
		DummyAccount: selAccount,
	}
}
