package twitterTypes

type GrokResponse struct {
	TweetId         string   `json:"tweet_id" bson:"tweet_id"`
	IsShill         bool     `json:"is_shill" bson:"is_shill"`
	TokensMentioned []string `json:"tokens_mentioned" bson:"tokens_mentioned"`
	TokensShilled   []string `json:"tokens_shilled" bson:"tokens_shilled"`
}
type GrokTokenResponse struct {
	TweetId     string `json:"tweet_id" bson:"tweet_id"`
	IsPromoting bool   `json:"is_promoting" bson:"is_promoting"`
}
