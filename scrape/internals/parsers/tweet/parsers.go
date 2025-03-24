package tweetParsers

import (
	"strings"

	"github.com/tidwall/gjson"

	twitterTypes "github.com/jeetdotso/tweet-indexer-service/scrape/internals/api/types"
)

func ParseTweet(r *gjson.Result, tweetID string) *twitterTypes.TweetInfo {
	if r == nil {
		return nil
	}
	var tweetInfo twitterTypes.TweetInfo
	var tweetType twitterTypes.TweetType
	var replyToID string
	entries := r.Get("data.threaded_conversation_with_injections_v2.instructions.0.entries").Array()
	for i, entry := range entries {
		tweet := entry.Get("content.itemContent.tweet_results.result")
		if tweet.Get("rest_id").String() == tweetID {
			if i == 0 {
				if tweet.Get("legacy.is_quote_status").Bool() {
					tweetType = twitterTypes.Quote
				} else if entry.Get("content.itemContent.tweetDisplayType").String() == "SelfThread" {
					tweetType = twitterTypes.Thread
				} else {
					tweetType = twitterTypes.Tweet
				}
			} else {
				tweetType = twitterTypes.Reply
				replyToID = tweet.Get("legacy.in_reply_to_status_id_str").String()
			}

			tweetInfo = twitterTypes.TweetInfo{
				ID:        tweet.Get("rest_id").String(),
				CreatedAt: tweet.Get("legacy.created_at").String(),
				Text:      tweet.Get("legacy.full_text").String(),
				MediaAttachments: func() *[]interface{} {
					media := tweet.Get("legacy.entities.media").Array()
					attachments := make([]interface{}, len(media))
					for i, m := range media {
						attachments[i] = m.Value()
					}
					return &attachments
				}(),
				UserID:         tweet.Get("core.user_results.result.rest_id").String(),
				ConversationID: tweet.Get("rest_id").String(),
				ReplyToID:      replyToID,
				TweetType:      tweetType,
				RetweetedByID:  "",
			}
			if strings.HasPrefix(tweetInfo.Text, "RT @") {
				tweetInfo.RetweetedByID = tweetInfo.UserID
				tweetInfo.TweetType = twitterTypes.Retweet
			}
			break
		}
	}

	return &tweetInfo
}
