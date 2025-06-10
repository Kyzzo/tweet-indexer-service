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
	msgToSend2    = "Is this tweet explicitly promoting a specific cryptocurrency token(s)? If yes, I want to know what token(s) the author is promoting in the tweet. Do not look at data from the comments section to answer my question.\n\nText: %s\n %s. output in raw txt strictly follow the following schema {\"isPromoting\": bool,\"tokensPromoted\": list or null}"
	launchCoinMsg = "https://x.com/%s/status/%s\nDoes this tweet reference the cryptocurrency token ‘Launchcoin’ at all? Respond in this format\n\nisPromoting: true| false"
	msgToSend     = "Is this tweet explicitly promoting a specific cryptocurrency token(s)? If yes, I want to know what token(s) the author is promoting in the tweet. Do not look at data from the comments section to answer my question. Respond in this format:\n\nisPromoting: true| false \ntokensPromoted: tokens (separated by commas))"
)

func NewGrokMonitor(ctx context.Context, svc dummyaccount.Service, url string) (*GrokMonitor, *dummyaccount.SelAccount, error) {
	// Fetch the next available Selenium account
	// selAccount, err := svc.LogInToNext(ctx)
	selAccount, err := svc.LogInToNextGroup(ctx, dummyaccount.WORKING_GROUP)
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
	// Step 1: Create a new Grok conversation
	px := proxy.NextProxy()
	convId, err := gm.twitterAPI.CreateConversation(px)
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
	resp, _, err2 = gm.twitterAPI.SendMessage(getMsg2(tweet), convId, "grok-3", px, false)
	promV := gjson.Parse(resp).Get("isPromoting")
	if resp == "" || !promV.Exists() {
		//retry by geting convo history
		for attempt := 1; attempt <= maxPromptRetries; attempt++ {
			resp, err2 = gm.twitterAPI.GetConversationHistory(convId, px)
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
			Str("tweetID", tweet.ID).
			Str("SelAcct", gm.ss.UserName).
			Msg("Failed to send Grok prompt initial after retries")
		return
	}
	promV = gjson.Parse(resp).Get("isPromoting")
	if !promV.Exists() {
		_ = gm.ProcessResponse(ctx, convId, tweet, px, handleResponse)
		return
	}
	gr := model.GrokEvent{
		ID:      tweet.TweetID,
		IsShill: gjson.Parse(resp).Get("isPromoting").Bool(),
		TokensShilled: func() []string {
			results := gjson.Parse(resp).Get("tokensPromoted").Array()
			tokens := make([]string, len(results))
			for i, result := range results {
				tokens[i] = result.String()
			}
			return tokens
		}(),
		TokensMentioned: func() []string {
			results := gjson.Parse(resp).Get("tokensMentioned").Array()
			tokens := make([]string, len(results))
			for i, result := range results {
				tokens[i] = result.String()
			}
			return tokens
		}(),
		UserId:       tweet.UserID,
		CreatedAt:    time.Now(),
		DummyAccount: gm.ss.UserName,
		ConvoID:      convId,
	}
	handleResponse(ctx, &gr)

}

func (gm *GrokMonitor) ProcessResponse(ctx context.Context, grokId string, tweet *model.TweetEvent, px string, handleResponse func(ctx context.Context, twt *model.GrokEvent)) string {
	maxAttempts := 5
	retryDelay := 5 * time.Second
	defer func() {
		recover := recover()
		if recover != nil {
			logg.Log.Error().Caller().Ctx(ctx).Msgf("Recovered from panic: %v", recover)
		}
	}()
	for attempt := 1; attempt <= maxAttempts; attempt++ {

		resp, err := gm.twitterAPI.GetConversationHistory(grokId, px)
		if err != nil {
			logg.Log.Error().Caller().Ctx(ctx).Err(err).Msgf("Attempt %d: Error fetching Grok response.", attempt)
			time.Sleep(retryDelay)
			continue
		}
		if resp == "" {
			time.Sleep(retryDelay)
			continue
		}
		promV := gjson.Parse(resp).Get("isPromoting")
		if !promV.Exists() {
			time.Sleep(retryDelay)
			continue
		}
		gr := &model.GrokEvent{
			ID:      tweet.TweetID,
			IsShill: gjson.Parse(resp).Get("isPromoting").Bool(),
			TokensShilled: func() []string {
				results := gjson.Parse(resp).Get("tokensPromoted").Array()
				tokens := make([]string, len(results))
				for i, result := range results {
					tokens[i] = result.String()
				}
				return tokens
			}(),
			TokensMentioned: func() []string {
				results := gjson.Parse(resp).Get("tokensMentioned").Array()
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
		handleResponse(ctx, gr)
		return ""

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
	if len(m) >= 0 {
		media = fmt.Sprintf("Media:%s\n", media)
	} else {
		media = ""
	}
	return fmt.Sprintf(msgToSend2, tweet.Text, media)
}
func getUrl(tweet *model.TweetEvent) string {
	return fmt.Sprintf("https://x.com/%s/status/%s", tweet.UserName, tweet.ID)
}

func GetGrokCheck(ctx context.Context, tweet *model.TweetEvent, svc dummyaccount.Service, url string, handleResponse func(ctx context.Context, twt *model.GrokEvent)) {
	// Fetch a Selenium account
	monitor, selAccount, err := NewGrokMonitor(ctx, svc, url)
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
