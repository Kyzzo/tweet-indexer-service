package tracking

import (
	"context"
	"strings"
	"time"

	"fmt"

	dummyaccount "github.com/jeetdotso/tweet-indexer-service/dummyaccount"
	"github.com/jeetdotso/tweet-indexer-service/indexer/model"
	"github.com/jeetdotso/tweet-indexer-service/logg"
	twitterAPI "github.com/jeetdotso/tweet-indexer-service/scrape/internals/api"
	twitterTypes "github.com/jeetdotso/tweet-indexer-service/scrape/internals/api/types"
	grokparser "github.com/jeetdotso/tweet-indexer-service/scrape/internals/parsers/grok"
)

type GrokMonitor struct {
	twitterAPI *twitterAPI.TwitterClient
	ss         *dummyaccount.SelAccount
}

var (
	// msgToSend = "Hello there"
	msgToSend2 = "Try to understand the tweet via the text and media\n\nText: %s\n Media:%s\n Is this message promoting a specific cryptocurrency token(s)? If yes, I want to know what token(s) the author is promoting in the message.  Respond in this format\n\nisPromoting: true| false \ntokensPromoted: tokens (separated by commas)"

	msgToSend = "Is this tweet explicitly promoting a specific cryptocurrency token(s)? If yes, I want to know what token(s) the author is promoting in the tweet. Do not look at data from the comments section to answer my question. Respond in this format:\n\nisPromoting: true| false \ntokensPromoted: tokens (separated by commas))"
)

func NewGrokMonitor(ctx context.Context, svc dummyaccount.Service) (*GrokMonitor, *dummyaccount.SelAccount, error) {
	// Fetch the next available Selenium account
	// selAccount, err := svc.LogInToNext(ctx)
	selAccount, err := svc.LogInToNextGroup(ctx, dummyaccount.WORKING_GROUP)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error retrieving Selenium account.")
		return nil, nil, err
	}

	// Create Twitter API client using the cookies from the Selenium account
	twitterAPIClient, _ := twitterAPI.CreateTwitterClientWithCookies(selAccount.GetCookiesHttp())
	acc := &GrokMonitor{
		twitterAPI: twitterAPIClient,
		ss:         selAccount,
	}

	return acc, selAccount, nil
}

func (gm *GrokMonitor) GetResponse(ctx context.Context, tweet *model.TweetEvent, handleResponse func(ctx context.Context, userID string, twt *twitterTypes.GrokResponse, sellAcct string)) {
	// Step 1: Create a new Grok conversation
	convId, err := gm.twitterAPI.CreateGrokConversation()
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error creating Grok conversation.")
		return
	}
	// Step 2: Build URL for add_response.json with correct domain
	url := "https://grok.x.com/2/grok/add_response.json"

	// Step 3: Send OPTIONS preflight
	if err := gm.twitterAPI.SendGrokPreflight(url); err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Preflight OPTIONS request failed.")
		return
	}
	// // Small delay for safety
	time.Sleep(2 * time.Second)
	// Step 4: Send the actual Grok prompt
	const maxPromptRetries = 5
	var (
		err2 error
		err3 error
	)
	for attempt := 1; attempt <= maxPromptRetries; attempt++ {
		_, err2 = gm.twitterAPI.SendGrokPromptInitial(convId, getUrl(tweet))
		if err2 == nil {
			break
		}

		time.Sleep(time.Duration(attempt*2) * time.Second)
	}
	if err2 != nil {
		logg.Log.
			Error().
			Caller().
			Ctx(ctx).
			Err(err2).
			Str("tweetID", tweet.ID).
			Str("SelAcct", gm.ss.UserName).
			Msg("Failed to send Grok prompt initial after retries")
		return
	}
	time.Sleep(12 * time.Second)

	// Step 6: Fetch and process the agent’s response
	responseMsg := gm.ProcessResponse(ctx, convId, tweet.ID, tweet.UserID, false, handleResponse)
	trimmed := strings.TrimLeft(responseMsg, " \r\n\t")
	if strings.HasPrefix(trimmed,
		"This post cannot be analyzed because some critical content is deleted") {
		fmt.Println("------------\nHitting deleted stuff: ", responseMsg, "\n-------------")
		gm.GetResponseDeleted(ctx, tweet, convId, handleResponse)
		return

	}
	for attempt := 1; attempt <= maxPromptRetries; attempt++ {
		_, err3 = gm.twitterAPI.SendGrokPrompt(convId, getUrl(tweet), responseMsg, getMsg(tweet))
		if err3 == nil {
			break
		}

		time.Sleep(time.Duration(attempt*2) * time.Second)
	}
	if err3 != nil {
		logg.Log.
			Error().
			Caller().
			Ctx(ctx).
			Err(err2).
			Str("tweetID", tweet.ID).
			Str("SelAcct", gm.ss.UserName).
			Msg("Failed to send Grok prompt after retries")
		return
	}

	// … proceed with processing using resp if needed …
	// Step 5: Allow some time for Grok to generate response
	time.Sleep(8 * time.Second)

	// Step 6: Fetch and process the agent’s response
	gm.ProcessResponse(ctx, convId, tweet.ID, tweet.UserID, true, handleResponse)
}

func (gm *GrokMonitor) GetResponseDeleted(ctx context.Context, tweet *model.TweetEvent, conversationID string, handleResponse func(ctx context.Context, userID string, twt *twitterTypes.GrokResponse, sellAcct string)) {
	// Step 1: Create a new Grok conversation
	var convId string
	if conversationID == "" {

		convId, err := gm.twitterAPI.CreateGrokConversation()
		if err != nil {
			logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error creating Grok conversation.")
			return
		}
		convId = convId
		// Step 2: Build URL for add_response.json with correct domain
		url := "https://grok.x.com/2/grok/add_response.json"

		// Step 3: Send OPTIONS preflight
		if err := gm.twitterAPI.SendGrokPreflight(url); err != nil {
			logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Preflight OPTIONS request failed.")
			return
		}
		// // Small delay for safety
		time.Sleep(2 * time.Second)
	} else {
		convId = conversationID
	}
	// Step 4: Send the actual Grok prompt
	const maxPromptRetries = 5
	var (
		err2 error
	)
	for attempt := 1; attempt <= maxPromptRetries; attempt++ {
		_, err2 = gm.twitterAPI.SendGrokPromptDeleted(convId, getMsg2(tweet))
		if err2 == nil {
			break
		}

		time.Sleep(time.Duration(attempt*2) * time.Second)
	}
	if err2 != nil {
		logg.Log.
			Error().
			Caller().
			Ctx(ctx).
			Err(err2).
			Str("tweetID", tweet.ID).
			Str("SelAcct", gm.ss.UserName).
			Msg("Failed to send Grok prompt initial after retries")
		return
	}
	time.Sleep(12 * time.Second)

	// Step 6: Fetch and process the agent’s response
	gm.ProcessResponse(ctx, convId, tweet.ID, tweet.UserID, true, handleResponse)
}

func (gm *GrokMonitor) ProcessResponse(ctx context.Context, grokId, tweetId, userId string, isFinal bool, handleResponse func(ctx context.Context, userID string, twt *twitterTypes.GrokResponse, sellAcct string)) string {
	maxAttempts := 5
	retryDelay := 8 * time.Second
	defer func() {
		recover := recover()
		if recover != nil {
			logg.Log.Error().Caller().Ctx(ctx).Msgf("Recovered from panic: %v", recover)
		}
	}()
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		respJson, err := gm.twitterAPI.GetGrokConversationResponse(grokId, "")
		if err != nil {
			logg.Log.Error().Caller().Ctx(ctx).Err(err).Msgf("Attempt %d: Error fetching Grok response.", attempt)
			time.Sleep(retryDelay)
			continue
		}

		allItems := respJson.Get("data.grok_conversation_items_by_rest_id.items").Array()

		// Try to find the latest message from the Agent
		var latestAgentMessage string
		var agentMsgCount int
		for i := len(allItems) - 1; i >= 0; i-- {
			item := allItems[i]
			if item.Get("sender_type").String() == "Agent" {
				if isFinal {

					agentMsgCount++
					if agentMsgCount > 1 {
						latestAgentMessage = item.Get("message").String()
						break
					}
				} else {
					latestAgentMessage = item.Get("message").String()
					break
				}
			}
		}

		if latestAgentMessage == "" {
			logg.Log.Debug().Caller().Ctx(ctx).Str("tweetID", tweetId).Msgf("Attempt %d: Agent has not responded yet. Retrying...", attempt)
			time.Sleep(retryDelay)
			continue
		}

		// logg.Log.Debug().Caller().Ctx(ctx).Msgf("Latest Grok Agent message: %s", latestAgentMessage)
		if isFinal {
			fmt.Println("Latest Grok Agent message: ", latestAgentMessage)
			parsed := grokparser.ParseGrokMessage(tweetId, latestAgentMessage)
			if parsed != nil {
				handleResponse(ctx, userId, parsed, gm.ss.UserName)
				return ""
			}
		} else {
			return latestAgentMessage
		}
		logg.Log.Error().Caller().Ctx(ctx).Msg("Failed to parse Grok message.")
		time.Sleep(retryDelay)
	}

	logg.Log.Error().Caller().Ctx(ctx).Msg("Max retries reached. Grok response not available.")
	return ""
}

func getMsg(tweet *model.TweetEvent) string {
	return msgToSend
	// m := tweet.GetMediaUrls()

	// media := strings.Join(m, ",")

	// return fmt.Sprintf(msgToSend2, tweet.Text, media)
}
func getMsg2(tweet *model.TweetEvent) string {
	// return msgToSend
	m := tweet.GetMediaUrls()

	media := strings.Join(m, ",")

	return fmt.Sprintf(msgToSend2, tweet.Text, media)
}
func getUrl(tweet *model.TweetEvent) string {
	return fmt.Sprintf("https://x.com/%s/status/%s", tweet.UserName, tweet.ID)
}

func GetGrokCheck(ctx context.Context, tweet *model.TweetEvent, svc dummyaccount.Service, handleResponse func(ctx context.Context, userID string, twt *twitterTypes.GrokResponse, sellAcct string)) {
	// Fetch a Selenium account
	monitor, selAccount, err := NewGrokMonitor(ctx, svc)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Failed to get account monitor.")
		return
	}
	defer func() {
		_ = svc.LogOut(ctx, selAccount.UserName)
	}()

	// Start monitoring tweets
	monitor.GetResponse(ctx, tweet, handleResponse)

}
