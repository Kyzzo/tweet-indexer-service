package mapper

import (
	"time"

	"github.com/jeetdotso/tweet-indexer-service/indexer/model"
	twitterTypes "github.com/jeetdotso/tweet-indexer-service/scrape/internals/api/types"
)

// ✅ Convert Twitter API response to TweetEvent
func MapTweetToEvent(tweet *twitterTypes.TweetInfo) model.TweetEvent {
	return model.TweetEvent{
		ID:                tweet.ID,
		UserID:            tweet.UserID,
		TweetType:         tweet.TweetType,
		UserName:          tweet.Username,
		TweetID:           tweet.TweetID,
		RetweetedUsername: tweet.RetweetedUsername,
		CreatedAt:         parseTime(tweet.CreatedAt),
		ConversationID:    tweet.ConversationID,
		ReplyToID:         tweet.ReplyToID,
		Text:              tweet.Text,
		RetweetedByID:     tweet.RetweetedByID,
		CapturedAt:        time.Now(),
		MediaAttachments:  flattenAttachments(tweet.MediaAttachments),
	}
}

// Helper function to parse time safely
func parseTime(ts string) time.Time {
	t, err := time.Parse("Mon Jan 02 15:04:05 -0700 2006", ts)
	if err != nil {
		return time.Time{}
	}
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
