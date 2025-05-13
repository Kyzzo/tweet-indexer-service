package twitterTypes

type TweetType string

const (
	Quote        TweetType = "Quote"
	Thread       TweetType = "Thread"
	Reply        TweetType = "Reply"
	Tweet        TweetType = "Tweet"
	Retweet      TweetType = "Retweet"
	QuoteRetweet TweetType = "QuoteRetweet"
)

type TweetInfo struct {
	ID               string         `json:"id" bson:"id"`
	CreatedAt        string         `json:"created_at" bson:"created_at"`
	Text             string         `json:"text" bson:"text"`
	UserID           string         `json:"user_id" bson:"user_id"`
	ConversationID   string         `json:"conversation_id" bson:"conversation_id"`
	ReplyToID        string         `json:"reply_to_id" bson:"reply_to_id"`
	TweetType        TweetType      `json:"tweet_type" bson:"tweet_type"`
	RetweetedByID    string         `json:"retweeted_by_id" bson:"retweeted_by_id"`
	MediaAttachments *[]interface{} `json:"media_attachments" bson:"media_attachments"`
}
