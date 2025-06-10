package tweetParsers

import (
	"fmt"
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
func extractFullText(obj gjson.Result) string {
	// 1) If it’s a note-tweet, grab the note text
	if note := obj.Get("note_tweet.note_tweet_results.result"); note.Exists() {
		if t := note.Get("text"); t.Exists() {
			return t.String()
		}
		if t := note.Get("legacy.full_text"); t.Exists() {
			return t.String()
		}
	}
	// 2) Else fall back to the main full_text
	if t := obj.Get("legacy.full_text"); t.Exists() {
		return t.String()
	}
	// 3) Some payloads nest the tweet under `tweet`
	if t := obj.Get("tweet.legacy.full_text"); t.Exists() {
		return t.String()
	}
	return ""
}
func ParseTweetFromEntry(entry *gjson.Result, trackedUserID, trackedUsername string) *twitterTypes.TweetInfo {
	if entry.Get("content.entryType").String() == "TimelineTimelineModule" {
		items := entry.Get("content.items").Array()
		if len(items) == 0 {
			return nil
		}

		// Gather all texts & media
		var texts []string
		var media []interface{}
		for _, mod := range items {
			child := mod.Get("item")
			// reuse your existing single‐tweet logic by slicing off the module flag
			if ti := ParseTweetFromEntry(&child, trackedUserID, trackedUsername); ti != nil {
				texts = append(texts, ti.Text)
				if ti.MediaAttachments != nil {
					media = append(media, *ti.MediaAttachments...)
				}
			}
		}

		// Use the first tweet in the module as the "root" for the ID, CreatedAt, user, etc.
		first := items[0].Get("item.itemContent.tweet_results.result")
		rootID := first.Get("rest_id").String()
		created := first.Get("legacy.created_at").String()
		// all the other fields (replyTo, retweets, etc.) we leave blank for a thread

		return &twitterTypes.TweetInfo{
			ID:               rootID,
			TweetID:          rootID,
			CreatedAt:        created,
			Text:             strings.Join(texts, "\n\n"),
			MediaAttachments: &media,
			FullInterface:    entry.String(), // store the full module for later use
			UserID:           trackedUserID,
			Username:         trackedUsername,
			TweetType:        twitterTypes.Thread, // make sure you’ve defined this new type
		}
	}
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

			if textField := qr.Get("note_tweet.note_tweet_results.result.text"); textField.Exists() {
				quoteText := textField.String()
				text = fmt.Sprintf("%s\nQuote: %s", text, quoteText)
			}
		}
	}
	if typ != twitterTypes.Quote && typ != twitterTypes.QuoteRetweet {
		if note := t.Get("note_tweet.note_tweet_results.result"); note.Exists() {
			if textField := note.Get("text"); textField.Exists() {
				text = textField.String()
			} else if legacyField := note.Get("legacy.full_text"); legacyField.Exists() {
				text = legacyField.String()
			}
		} else {
			fullText := t.Get("legacy.full_text").String()
			if fullText != "" {
				text = fullText
			} else {
				text = t.Get("tweet.legacy.full_text").String()
			}
		}

	}
	if typ == twitterTypes.Retweet || typ == twitterTypes.QuoteRetweet {
		if ft := t.Get("legacy.retweeted_status_result.result"); ft.Exists() {
			if note := ft.Get("note_tweet.note_tweet_results.result"); note.Exists() {
				if textField := note.Get("text"); textField.Exists() {
					text = textField.String()
				} else if legacyField := note.Get("legacy.full_text"); legacyField.Exists() {
					text = legacyField.String()
				}
			} else {
				fullText := ft.Get("legacy.full_text").String()
				if fullText != "" {
					text = fullText
				} else {
					text = ft.Get("tweet.legacy.full_text").String()
				}
			}
		} else if ft := t.Get("retweeted_status_result.result"); ft.Exists() {
			if note := ft.Get("note_tweet.note_tweet_results.result"); note.Exists() {
				if textField := note.Get("text"); textField.Exists() {
					text = textField.String()
				} else if legacyField := note.Get("legacy.full_text"); legacyField.Exists() {
					text = legacyField.String()
				}
			} else {
				fullText := ft.Get("legacy.full_text").String()
				if fullText != "" {
					text = fullText
				} else {
					text = ft.Get("tweet.legacy.full_text").String()
				}
			}

		}
	}
	if typ == twitterTypes.QuoteRetweet {
		qr := t.Get("retweeted_status_result.result.quoted_status_result.result")
		var quoteText string
		if qr.Exists() {
			if textField := qr.Get("note_tweet.note_tweet_results.result.text"); textField.Exists() {
				quoteText = textField.String()
			} else if legacyField := qr.Get("legacy.full_text"); legacyField.Exists() {
				quoteText = legacyField.String()
			}
			if quoteText != "" {
				text = fmt.Sprintf("%s\nQuote: %s", text, quoteText)
			}
			for _, m := range qr.Get("legacy.entities.media").Array() {
				attachments = append(attachments, m.Value())
			}
			for _, m := range qr.Get("legacy.extended_entities.media").Array() {
				attachments = append(attachments, m.Value())
			}

		} else if qr = t.Get("legacy.retweeted_status_result.result.quoted_status_result.result"); qr.Exists() {
			if textField := qr.Get("note_tweet.note_tweet_results.result.text"); textField.Exists() {
				quoteText = textField.String()
			} else if legacyField := qr.Get("legacy.full_text"); legacyField.Exists() {
				quoteText = legacyField.String()
			}
			if quoteText != "" {
				text = fmt.Sprintf("%s\nQuote: %s", text, quoteText)
			}
			for _, m := range qr.Get("legacy.entities.media").Array() {
				attachments = append(attachments, m.Value())
			}
			for _, m := range qr.Get("legacy.extended_entities.media").Array() {
				attachments = append(attachments, m.Value())
			}

		}
	}
	var (
		uniqueId           = id
		retweeted_username = ""
		retweetedBy        = ""
	)

	// who retweeted it (if at all)
	if typ == twitterTypes.Retweet || typ == twitterTypes.QuoteRetweet {
		// in a “single‐entry” payload, we assume the trackedUser did the RT
		uniqueId = fmt.Sprintf("%s_%s", trackedUserID, id)
		if mentions := t.Get("legacy.entities.user_mentions").Array(); len(mentions) > 0 {
			retweeted_username = mentions[0].Get("screen_name").String()
			retweetedBy = mentions[0].Get("id_str").String()
		}

	}
	return &twitterTypes.TweetInfo{
		ID:                uniqueId,
		CreatedAt:         created,
		Text:              text,
		MediaAttachments:  &attachments,
		UserID:            userID,
		ConversationID:    convoID,
		ReplyToID:         replyTo,
		FullInterface:     entry.String(), // store the full module for later use
		TweetType:         typ,
		RetweetedByID:     retweetedBy,
		TweetID:           id,
		Username:          trackedUsername,
		RetweetedUsername: retweeted_username,
	}
}
