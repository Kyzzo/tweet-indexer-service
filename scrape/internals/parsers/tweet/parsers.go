package tweetParsers

import (
	"strings"

	"github.com/tidwall/gjson"

	twitterTypes "github.com/jeetdotso/tweet-indexer-service/scrape/internals/api/types"
)

func ParseTweet(r *gjson.Result, tweetID, trackedUserId string) *twitterTypes.TweetInfo {
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
			if tweetInfo.TweetType == twitterTypes.Tweet && tweetInfo.UserID != trackedUserId {
				tweetInfo.TweetType = twitterTypes.Retweet
				tweetInfo.RetweetedByID = trackedUserId
			}
			break
		}
	}

	return &tweetInfo
}
func ParseTweetFromEntry(entry *gjson.Result, trackedUserID string) *twitterTypes.TweetInfo {

	t := entry.Get("content.itemContent.tweet_results.result")
	// 2) when you passed in the whole module entry
	if !t.Exists() {
		t = entry.Get("item.itemContent.tweet_results.result")
	}
	// 3) when you stripped off to just the `item` object
	if !t.Exists() {
		t = entry.Get("itemContent.tweet_results.result")
	}
	// still nothing? bail
	if !t.Exists() {
		return nil
	}
	// basics
	created := t.Get("legacy.created_at").String()
	// …otherwise fall back into the embedded "tweet" object:
	if created == "" {
		created = t.Get("tweet.legacy.created_at").String()
		t = t.Get("tweet")

	}
	id := t.Get("rest_id").String()
	text := t.Get("legacy.full_text").String()
	userID := t.Get("core.user_results.result.rest_id").String()
	convoID := t.Get("legacy.conversation_id_str").String()
	replyTo := t.Get("legacy.in_reply_to_status_id_str").String()

	// media attachments
	mediaArr := t.Get("legacy.entities.media").Array()
	var attachments []interface{}
	if len(mediaArr) != 0 {
		for _, m := range mediaArr {
			attachments = append(attachments, m.Value())
		}
	}
	// determine type
	typ := twitterTypes.Tweet
	if t.Get("legacy.is_quote_status").Bool() {
		typ = twitterTypes.Quote
		if strings.HasPrefix(text, "RT @") {
			typ = twitterTypes.QuoteRetweet
		}
	} else if strings.HasPrefix(text, "RT @") {
		typ = twitterTypes.Retweet
	} else if replyTo != "" {
		typ = twitterTypes.Reply
	}
	if typ == twitterTypes.Quote || typ == twitterTypes.QuoteRetweet {
		qr := t.Get("quoted_status_result.result")
		if qr.Exists() {
			// the quoted tweet may have both the simple `entities.media` _and_ an
			// `extended_entities.media` (for video, GIFs, etc.)
			for _, m := range qr.Get("legacy.entities.media").Array() {
				attachments = append(attachments, m.Value())
			}
			for _, m := range qr.Get("legacy.extended_entities.media").Array() {
				attachments = append(attachments, m.Value())
			}
		}
	}

	// who retweeted it (if at all)
	retweetedBy := ""
	if typ == twitterTypes.Retweet {
		// in a “single‐entry” payload, we assume the trackedUser did the RT
		retweetedBy = trackedUserID
	}
	return &twitterTypes.TweetInfo{
		ID:               id,
		CreatedAt:        created,
		Text:             text,
		MediaAttachments: &attachments,
		UserID:           userID,
		ConversationID:   convoID,
		ReplyToID:        replyTo,
		TweetType:        typ,
		RetweetedByID:    retweetedBy,
	}
}
