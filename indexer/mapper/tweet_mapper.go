package mapper

import (
	"time"

	"github.com/jeetdotso/tweet-indexer-service/indexer/model"
	twitterTypes "github.com/jeetdotso/tweet-indexer-service/scrape/internals/api/types"
)

// ✅ Convert Twitter API response to TweetEvent
func MapTweetToEvent(tweet *twitterTypes.TweetInfo) model.TweetEvent {
	return model.TweetEvent{
		ID:               tweet.ID,
		UserID:           tweet.UserID,
		TweetType:        tweet.TweetType,
		CreatedAt:        parseTime(tweet.CreatedAt),
		ConversationID:   tweet.ConversationID,
		ReplyToID:        tweet.ReplyToID,
		Text:             tweet.Text,
		RetweetedByID:    tweet.RetweetedByID,
		MediaAttachments: flattenAttachments(tweet.MediaAttachments),
	}
}

// Helper function to parse time safely
func parseTime(ts string) time.Time {
	t, _ := time.Parse(time.RFC1123, ts)
	return t
}

// Convert media attachments to string array
func flattenAttachments(media *[]interface{}) []interface{} {
	if media == nil {
		return nil
	}

	attachments := make([]interface{}, len(*media))
	for i, m := range *media {
		attachments[i] = m // Keep as interface{}
	}
	return attachments
}
