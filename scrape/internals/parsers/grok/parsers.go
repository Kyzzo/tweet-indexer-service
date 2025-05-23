package grok

import (
	"strings"

	twitterTypes "github.com/jeetdotso/tweet-indexer-service/scrape/internals/api/types"
)

func ParseGrokMessage(tweetId, message string) *twitterTypes.GrokResponse {
	lines := strings.Split(message, "\n")

	var resp twitterTypes.GrokResponse
	resp.TweetId = tweetId

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "isPromoting:") {
			resp.IsShill = strings.Contains(line, "true")
		} else if strings.HasPrefix(line, "tokensMentioned:") {
			tokens := strings.TrimPrefix(line, "tokensMentioned:")
			resp.TokensMentioned = parseTokens(tokens)
		} else if strings.HasPrefix(line, "tokensPromoted:") {
			tokens := strings.TrimPrefix(line, "tokensPromoted:")
			resp.TokensShilled = parseTokens(tokens)
		}
	}

	return &resp
}
func ParseTokenGrokMessage(tweetId, message string) *twitterTypes.GrokTokenResponse {
	lines := strings.Split(message, "\n")

	var resp twitterTypes.GrokTokenResponse
	resp.TweetId = tweetId

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "isPromoting:") {
			resp.IsPromoting = strings.Contains(line, "true")
		}
	}

	return &resp
}

func parseTokens(tokenStr string) []string {
	tokens := strings.Split(tokenStr, ",")
	for i := range tokens {
		tokens[i] = strings.TrimSpace(tokens[i])
	}
	return tokens
}
