package model

import (
	"time"

	proto "github.com/jeetdotso/tweet-indexer-service/proto"
	twitterTypes "github.com/jeetdotso/tweet-indexer-service/scrape/internals/api/types"
)

type TweetEvent struct {
	ID               string                 `bson:"_id"`
	CreatedAt        time.Time              `bson:"created_at"`
	Text             string                 `bson:"text"`
	UserID           string                 `bson:"user_id"`
	ConversationID   string                 `bson:"conversation_id"`
	ReplyToID        string                 `bson:"reply_to_id"`
	TweetType        twitterTypes.TweetType `bson:"tweet_type"`
	RetweetedByID    string                 `bson:"retweeted_by_id"`
	MediaAttachments []interface{}          `bson:"media_attachments"`
}

type UserCheckPoint struct {
	ID                 string    `bson:"_id"`
	LastTweet          string    `bson:"last_tweet"`
	LastTweetCreatedAt time.Time `bson:"last_tweet_created_at"`
	UpdatedAt          time.Time `bson:"updated_at"`
}

func (te *TweetEvent) GetID() any {
	return te.ID
}

func (uch *UserCheckPoint) GetID() any {
	return uch.ID
}

// ✅ Converts `TweetEvent` to `proto.TweetEvent`
func (te *TweetEvent) ToProto() *proto.TweetEvent {
	// Convert `[]interface{}` to `[]string` for Protobuf compatibility
	var mediaAttachments []string
	for _, media := range te.MediaAttachments {
		mediaAttachments = append(mediaAttachments, media.(string))
	}

	return &proto.TweetEvent{
		Id:               te.ID,
		CreatedAt:        te.CreatedAt.Format(time.RFC3339),
		Text:             te.Text,
		UserId:           te.UserID,
		ConversationId:   te.ConversationID,
		ReplyToId:        te.ReplyToID,
		TweetType:        string(te.TweetType),
		RetweetedById:    te.RetweetedByID,
		MediaAttachments: mediaAttachments,
	}
}
