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
	"github.com/jeetdotso/tweet-indexer-service/scrape/proxy"
	"github.com/tidwall/gjson"
)

type GrokMonitor struct {
	twitterAPI *twitterAPI.GrokClient
	ss         *dummyaccount.SelAccount
}

var (
	// msgToSend = "Hello there"
	profileMsg = "@%s \n\nWould you classify this profile as a Trader, Memecoin Speculator, Influencer, Company, or None?Respond with only one. Output in raw text strictly adhere to the following schema {\"isCategorized\"}"
	memeMsg    = "%s\n\nFor a memecoin token trader, there are two types of shilling: hard shilling and soft shilling. Hard shilling involves the author explicitly mentioning one or more specific memecoins (by name, ticker, or contract address). A hard shill also includes one of the following: 1.  The author of the tweet states that they purchased the memecoin(s) or has a position in said memecoin(s) 2.  The author is explicitly bullish about the memecoin's past price performance 3.  The author is explicitly bullish about the memecoin's future price appreciation potential 4.  The author of the tweet posts the contract address of the memecoin(s) Soft shilling is a subtle or indirect form of promoting a memecoin — usually without making it look like an obvious advertisement or endorsement. A soft shill includes one of the following: 1.  Speaks positively about the project’s technology 2.  Posts a chart image of the memecoin(s) 3.  Includes subtle language about how the memecoin(s) could perform well, if various circumstances or factors align 4.  Posts artwork, imagery, or memes related to the memecoin(s) Important Clarification: •  A tweet must reference a specific, identifiable memecoin (via name, ticker, or contract address) to be classified as a shill. •  If the tweet uses generic, hypothetical, or non-specific phrases (e.g., “streamer coin,” “influencer token,” “dog coin”) that do not clearly refer to an existing memecoin, it should not be classified as a shill. Your job is to analyze the tweet and determine whether it is a shill or not. If yes, the tweet will be defined as a shill. If the tweet is a shill, specify whether it is Hard or Soft, and list the token(s) being referenced. If there is no specific token referenced, classify it as not a shill. Produce a response in raw text strictly adhering to the following schema:{\"isShill\": (None, \"Hard\", \"Soft\"), \"tokensMentioned\": list or null}"
	traderMsg  = "%s\n\nFor a cryptocurrency trader, useful analysis typically involves commentary on any of these 6 factors: price movements, volume trends, technical indicators, onchain indicators, fundamental catalysts, or the mere presence of a price chart image. Your job is to analyze this tweet and determine whether any of the 6 criteria are being met. If so, the tweet will be defined as useful analysis.  If the tweet is being defined as useful analysis, we what to know what token ticker is being referenced. If there are multiple tokens being referenced, list them all. Produce a response in raw text strictly adhering to the following schema {\"isUsefulAnalysis\": bool,\"tokensMentioned\": list or null}"
	msgToSend  = "Is this tweet explicitly promoting a specific cryptocurrency token(s)? If yes, I want to know what token(s) the author is promoting in the tweet. Do not look at data from the comments section to answer my question. Respond in this format:\n\nisPromoting: true| false \ntokensPromoted: tokens (separated by commas))"
)

func NewGrokMonitor(ctx context.Context, svc dummyaccount.Service, url string) (*GrokMonitor, *dummyaccount.SelAccount, error) {
	// Fetch the next available Selenium account
	// selAccount, err := svc.LogInToNext(ctx)
	selAccount, err := svc.LogInToNextGrok(ctx, dummyaccount.WORKING_GROUP)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error retrieving Selenium account.")
		return nil, nil, err
	}

	// Create Twitter API client using the cookies from the Selenium account
	twitterAPIClient, _ := twitterAPI.CreateGrokClientWithCookies(selAccount.GetCookiesRealHttp(), url)
	acc := &GrokMonitor{
		twitterAPI: twitterAPIClient,
		ss:         selAccount,
	}

	return acc, selAccount, nil
}

func (gm *GrokMonitor) GetResponse(ctx context.Context, tweet *model.TweetEvent, handleResponse func(ctx context.Context, twt *model.GrokEvent)) {
	startTime := time.Now()
	// Step 1: Create a new Grok conversation
	px := proxy.NextProxy()
	convId, err := gm.twitterAPI.CreateConversation(ctx, px)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Str("url", gm.twitterAPI.BaseURL).Str("proxy", px).Msg("Error creating Grok conversation.")
		return
	}
	duration := time.Since(startTime)
	minutes := int(duration.Minutes())
	seconds := int(duration.Seconds()) % 60
	logg.Log.Info().
		Caller().
		Ctx(ctx).
		Str("tweetID", tweet.ID).
		Str("SelAcct", gm.ss.UserName).
		Str("time taken", fmt.Sprintf("%dm%ds", minutes, seconds)).
		Msg("Created Grok conversation.")

	// Step 4: Send the actual Grok prompt
	const maxPromptRetries = 2
	var (
		err2 error
		resp string
	)

	// TODO: change to intial
	var (
		msgToSend string
		key       string
		tokenRec  string
	)
	switch tweet.Category {
	case model.CategoryTypeMEME:
		msgToSend = memeMessage(tweet)
		key = "isShill"
		tokenRec = "tokensMentioned"
		logg.Log.Info().Caller().Ctx(ctx).Str("tweetID", tweet.ID).Msg("Sending Grok meme message.")
	default:
		msgToSend = traderMessage(tweet)
		key = "isUsefulAnalysis"
		tokenRec = "tokensMentioned"
		logg.Log.Info().Caller().Ctx(ctx).Str("tweetID", tweet.ID).Msg("Sending Grok trader message.")
	}

	resp, _, err2 = gm.twitterAPI.SendMessage(ctx, msgToSend, convId, "grok-3-latest", px, false)
	duration = time.Since(startTime)
	minutes = int(duration.Minutes())
	seconds = int(duration.Seconds()) % 60
	logg.Log.Info().Caller().Ctx(ctx).Str("tweetID", tweet.ID).Int("Category", int(tweet.Category)).Str("SelAcct", gm.ss.UserName).Str("time taken", fmt.Sprintf("%dm%ds", minutes, seconds)).Msg("Completed sending Grok message.")
	promV := gjson.Parse(resp).Get(key)
	if resp == "" || !promV.Exists() {
		//retry by geting convo history
		for attempt := 1; attempt <= maxPromptRetries; attempt++ {
			resp, err2 = gm.twitterAPI.GetConversationHistory(ctx, convId, px)
			if resp != "" {
				break
			}
			time.Sleep(5 * time.Second)
		}

	}

	if err2 != nil {
		duration := time.Since(startTime)
		minutes := int(duration.Minutes())
		seconds := int(duration.Seconds()) % 60
		logg.Log.
			Error().
			Caller().
			Ctx(ctx).
			Err(err2).
			Str("tweetID", tweet.ID).
			Str("SelAcct", gm.ss.UserName).
			Str("time taken", fmt.Sprintf("%dm%ds", minutes, seconds)).
			Msg("Failed to send Grok prompt initial after retries")
		return
	}
	promV = gjson.Parse(resp).Get(key)
	if !promV.Exists() {
		_ = gm.ProcessResponse(ctx, key, tokenRec, convId, tweet, px, handleResponse)
		return
	}
	var gr *model.GrokEvent
	switch tweet.Category {
	case model.CategoryTypeMEME:
		shillType := gjson.Parse(resp).Get(key).String()
		shillType = strings.ReplaceAll(shillType, "\"", "")
		trimmed := strings.ToLower(strings.TrimSpace(shillType))
		if trimmed != "none" && trimmed != "hard" && trimmed != "soft" {
			_ = gm.ProcessResponse(ctx, key, tokenRec, convId, tweet, px, handleResponse)
		}
		gr = &model.GrokEvent{
			ID: tweet.TweetID,
			IsShill: func() bool {
				return trimmed == "hard" || trimmed == "soft"
			}(),
			ShillType: trimmed,
			TokensShilled: func() []string {
				results := gjson.Parse(resp).Get(tokenRec).Array()
				tokens := make([]string, len(results))
				for i, result := range results {
					tokens[i] = result.String()
				}
				return tokens
			}(),
			UserId:       tweet.UserID,
			CreatedAt:    time.Now(),
			DummyAccount: gm.ss.UserName,
			Category:     model.CategoryTypeMEME,
			ConvoID:      convId,
		}
		logg.Log.Info().Caller().Ctx(ctx).Str("shillType", shillType).Str("trimmed", trimmed).Msg("Parsed shill type.")

	default:

		gr = &model.GrokEvent{
			ID:      tweet.TweetID,
			IsShill: gjson.Parse(resp).Get(key).Bool(),
			TokensShilled: func() []string {
				results := gjson.Parse(resp).Get(tokenRec).Array()
				tokens := make([]string, len(results))
				for i, result := range results {
					tokens[i] = result.String()
				}
				return tokens
			}(),
			UserId:       tweet.UserID,
			ShillType:    "N/A",
			CreatedAt:    time.Now(),
			DummyAccount: gm.ss.UserName,
			ConvoID:      convId,
		}
	}
	handleResponse(ctx, gr)
	duration = time.Since(startTime)
	minutes = int(duration.Minutes())
	seconds = int(duration.Seconds()) % 60
	logg.Log.Info().Caller().Ctx(ctx).Str("tweetID", tweet.ID).Str("SelAcct", gm.ss.UserName).Str("time taken", fmt.Sprintf("%dm%ds", minutes, seconds)).Msg("Completed Grok Response.")

}
func (gm *GrokMonitor) GetProfileResponse(ctx context.Context, prfId string, profileName string, handleResponse func(ctx context.Context, id string, tag string)) {
	// Step 1: Create a new Grok conversation
	px := proxy.NextProxy()
	convId, err := gm.twitterAPI.CreateConversation(ctx, px)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Str("url", gm.twitterAPI.BaseURL).Str("proxy", px).Msg("Error creating Grok conversation.")
		return
	}

	// Step 4: Send the actual Grok prompt
	const maxPromptRetries = 5
	var (
		err2 error
		resp string
	)

	// TODO: change to intial
	resp, _, err2 = gm.twitterAPI.SendMessage(ctx, getProfileMsg(profileName), convId, "grok-3-latest", px, false)
	promV := gjson.Parse(resp).Get("isCategorized")
	if resp == "" || !promV.Exists() {
		//retry by geting convo history
		for attempt := 1; attempt <= maxPromptRetries; attempt++ {
			resp, err2 = gm.twitterAPI.GetConversationHistory(ctx, convId, px)
			if resp != "" {
				break
			}
			time.Sleep(time.Duration(attempt) * time.Second)
		}

	}

	if err2 != nil {
		logg.Log.
			Error().
			Caller().
			Ctx(ctx).
			Err(err2).
			Str("profileID", prfId).
			Str("SelAcct", gm.ss.UserName).
			Msg("Failed to send Grok prompt initial after retries")
		return
	}
	promV = gjson.Parse(resp).Get("isCategorized")
	if !promV.Exists() {
		_ = gm.ProcessProfileResponse(ctx, convId, prfId, profileName, px, handleResponse)
		return
	}
	gr := gjson.Parse(resp).Get("isCategorized").String()

	handleResponse(ctx, prfId, gr)

}

func (gm *GrokMonitor) ProcessResponse(ctx context.Context, key, tokenRec, grokId string, tweet *model.TweetEvent, px string, handleResponse func(ctx context.Context, twt *model.GrokEvent)) string {
	startTime := time.Now()
	maxAttempts := 3
	retryDelay := 3 * time.Second
	defer func() {
		recover := recover()
		if recover != nil {
			logg.Log.Error().Caller().Ctx(ctx).Msgf("Recovered from panic: %v", recover)
		}
	}()
	for attempt := 1; attempt <= maxAttempts; attempt++ {

		resp, err := gm.twitterAPI.GetConversationHistory(ctx, grokId, px)
		if err != nil {
			logg.Log.Error().Caller().Ctx(ctx).Err(err).Msgf("Attempt %d: Error fetching Grok response.", attempt)
			time.Sleep(retryDelay)
			continue
		}
		if resp == "" {
			time.Sleep(retryDelay)
			continue
		}
		promV := gjson.Parse(resp).Get(key)
		if !promV.Exists() {
			time.Sleep(retryDelay)
			continue
		}
		var gr *model.GrokEvent
		if key == "isShill" {
			shillType := gjson.Parse(resp).Get(key).String()
			shillType = strings.ReplaceAll(shillType, "\"", "")
			trimmed := strings.ToLower(strings.TrimSpace(shillType))
			gr = &model.GrokEvent{
				ID: tweet.TweetID,
				IsShill: func() bool {
					return trimmed == "hard" || trimmed == "soft"
				}(),
				ShillType: trimmed,
				TokensShilled: func() []string {
					results := gjson.Parse(resp).Get(tokenRec).Array()
					tokens := make([]string, len(results))
					for i, result := range results {
						tokens[i] = result.String()
					}
					return tokens
				}(),
				UserId:       tweet.UserID,
				CreatedAt:    time.Now(),
				DummyAccount: gm.ss.UserName,
				Category:     model.CategoryTypeMEME,
				ConvoID:      grokId,
			}
			logg.Log.Info().Caller().Ctx(ctx).Str("shillType", shillType).Str("trimmed", trimmed).Msg("Parsed shill type.")

		} else {

			gr = &model.GrokEvent{
				ID:        tweet.TweetID,
				IsShill:   gjson.Parse(resp).Get(key).Bool(),
				ShillType: "N/A",
				TokensShilled: func() []string {
					results := gjson.Parse(resp).Get(tokenRec).Array()
					tokens := make([]string, len(results))
					for i, result := range results {
						tokens[i] = result.String()
					}
					return tokens
				}(),
				UserId:       tweet.UserID,
				CreatedAt:    time.Now(),
				DummyAccount: gm.ss.UserName,
				ConvoID:      grokId,
			}
		}
		handleResponse(ctx, gr)

		duration := time.Since(startTime)
		minutes := int(duration.Minutes())
		seconds := int(duration.Seconds()) % 60
		logg.Log.Info().Caller().Ctx(ctx).Str("tweetID", tweet.ID).Str("SelAcct", gm.ss.UserName).Str("time taken", fmt.Sprintf("%dm%ds", minutes, seconds)).Msg("Completed Grok Response.")
		return ""

	}

	duration := time.Since(startTime)
	minutes := int(duration.Minutes())
	seconds := int(duration.Seconds()) % 60
	logg.Log.Error().Caller().Ctx(ctx).Str("SelAcct", gm.ss.UserName).Str("tweetID", tweet.ID).Str("time taken", fmt.Sprintf("%dm%ds", minutes, seconds)).Msg("Max retries reached. Grok response not available.")
	return ""
}
func (gm *GrokMonitor) ProcessProfileResponse(ctx context.Context, grokId string, prfId string, profileName string, px string, handleResponse func(ctx context.Context, id string, tag string)) string {
	startTime := time.Now()
	maxAttempts := 4
	retryDelay := 2 * time.Second
	defer func() {
		recover := recover()
		if recover != nil {
			logg.Log.Error().Caller().Ctx(ctx).Msgf("Recovered from panic: %v", recover)
		}
	}()
	for attempt := 1; attempt <= maxAttempts; attempt++ {

		resp, err := gm.twitterAPI.GetConversationHistory(ctx, grokId, px)
		if err != nil {
			logg.Log.Error().Caller().Ctx(ctx).Err(err).Msgf("Attempt %d: Error fetching Grok response.", attempt)
			time.Sleep(retryDelay)
			continue
		}
		if resp == "" {
			time.Sleep(retryDelay)
			continue
		}
		promV := gjson.Parse(resp).Get("isCategorized")
		if !promV.Exists() {
			time.Sleep(retryDelay)
			continue
		}
		gr := gjson.Parse(resp).Get("isCategorized").String()

		handleResponse(ctx, prfId, gr)
		return ""

	}
	duration := time.Since(startTime)
	minutes := int(duration.Minutes())
	seconds := int(duration.Seconds()) % 60

	logg.Log.Error().Caller().Ctx(ctx).Str("SelAcct", gm.ss.UserName).Str("profile_id", prfId).Str("time taken", fmt.Sprintf("%dm%ds", minutes, seconds)).Msg("Max retries reached. Grok Profile response not available.")
	return ""
}

func memeMessage(tweet *model.TweetEvent) string {
	// return msgToSend
	url := fmt.Sprintf("https://x.com/%s/status/%s", tweet.UserName, tweet.TweetID)

	return fmt.Sprintf(memeMsg, url)
}
func traderMessage(tweet *model.TweetEvent) string {
	// return msgToSend
	url := fmt.Sprintf("https://x.com/%s/status/%s", tweet.UserName, tweet.TweetID)

	return fmt.Sprintf(traderMsg, url)
}
func getUrl(tweet *model.TweetEvent) string {
	return fmt.Sprintf("https://x.com/%s/status/%s", tweet.UserName, tweet.ID)
}
func getProfileMsg(profileName string) string {
	return fmt.Sprintf(profileMsg, profileName)
}

func GetGrokCheck(ctx context.Context, tweet *model.TweetEvent, svc dummyaccount.Service, url string, handleResponse func(ctx context.Context, twt *model.GrokEvent)) {
	startTime := time.Now()
	// Fetch a Selenium account
	monitor, selAccount, err := NewGrokMonitor(ctx, svc, url)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Failed to get account monitor.")
		return
	}
	// ensure we always attempt logout, regardless of ctx timeout/cancel
	if selAccount != nil {
		defer func(user string) {
			logoutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := svc.LogGrokOut(logoutCtx, user); err != nil {
				logg.Log.Error().Err(err).Str("SelAcct", user).Msg("LogGrokOut failed")
			}
		}(selAccount.UserName)
	}

	// Start monitoring tweets
	monitor.GetResponse(ctx, tweet, handleResponse)
	duration := time.Since(startTime)
	minutes := int(duration.Minutes())
	seconds := int(duration.Seconds()) % 60
	logg.Log.Info().Caller().Ctx(ctx).Str("tweetID", tweet.ID).Str("SelAcct", selAccount.UserName).Str("time taken", fmt.Sprintf("%dm%ds", minutes, seconds)).Msg("Completed Grok check for tweet.")

}

func GetGrokProfile(ctx context.Context, prfID, prfUsername string, svc dummyaccount.Service, url string, handleResponse func(ctx context.Context, id string, tag string)) {
	// Fetch a Selenium account
	monitor, selAccount, err := NewGrokMonitor(ctx, svc, url)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Failed to get account monitor.")
		return
	}
	if selAccount != nil {
		defer func(user string) {
			logoutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := svc.LogGrokOut(logoutCtx, user); err != nil {
				logg.Log.Error().Err(err).Str("SelAcct", user).Msg("LogGrokOut failed")
			}
		}(selAccount.UserName)
	}

	// Start monitoring tweets
	monitor.GetProfileResponse(ctx, prfID, prfUsername, handleResponse)
}
