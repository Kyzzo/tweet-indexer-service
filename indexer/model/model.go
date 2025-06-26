package model

import (
	"fmt"
	"time"

	proto "github.com/jeetdotso/tweet-indexer-service/proto"
	twitterTypes "github.com/jeetdotso/tweet-indexer-service/scrape/internals/api/types"
)

type CategoryType int

const (
	CategoryTypeMEME CategoryType = iota
	CategoryTypeTRADER
	CategoryTypeGrokToken
)

func (c CategoryType) String() string {
	switch c {
	case CategoryTypeMEME:
		return "MEME"
	case CategoryTypeTRADER:
		return "TRADER"
	case CategoryTypeGrokToken:
		return "GrokToken"
	default:
		return "UNKNOWN"
	}
}

func ToCategoryString(cat CategoryType) string {
	return cat.String()
}

type TweetEvent struct {
	ID                string                 `bson:"_id"`
	TweetID           string                 `bson:"tweet_id"`
	CreatedAt         time.Time              `bson:"created_at"`
	CapturedAt        time.Time              `bson:"captured_at"`
	Text              string                 `bson:"text"`
	UserID            string                 `bson:"user_id"`
	UserName          string                 `bson:"user_name"`
	ConversationID    string                 `bson:"conversation_id"`
	ReplyToID         string                 `bson:"reply_to_id"`
	TweetType         twitterTypes.TweetType `bson:"tweet_type"`
	RetweetedByID     string                 `bson:"retweet_user_id"`
	RetweetedUsername string                 `bson:"retweet_username"`
	Category          CategoryType           `bson:"category"`
	MediaAttachments  []interface{}          `bson:"media_attachments"`
}

type RawTweet struct {
	ID       string `bson:"_id"`
	TweetID  string `bson:"tweet_id"`
	UserID   string `bson:"user_id"`
	RawEntry string `bson:"raw_entry"`
}

type UserCheckPoint struct {
	ID                 string    `bson:"_id"`
	LastTweet          string    `bson:"last_tweet"`
	LastTweetCreatedAt time.Time `bson:"last_tweet_created_at"`
	UpdatedAt          time.Time `bson:"updated_at"`
}

type GrokEvent struct {
	ID              string       `bson:"_id"`
	IsShill         bool         `bson:"is_shill"`
	TokensMentioned []string     `bson:"tokens_mentioned"`
	TokensShilled   []string     `bson:"tokens_shilled"`
	UserId          string       `bson:"user_id"`
	CreatedAt       time.Time    `bson:"created_at"`
	DummyAccount    string       `bson:"dummy_account"`
	Category        CategoryType `bson:"category"`
	ConvoID         string       `bson:"convo_id"`
}
type GrokTokenEvent struct {
	ID           string    `bson:"_id"`
	IsPromoting  bool      `bson:"is_promoting"`
	UserId       string    `bson:"user_id"`
	CreatedAt    time.Time `bson:"created_at"`
	DummyAccount string    `bson:"dummy_account"`
}

type RetweetEvent struct {
	ID        string    `bson:"_id"`
	CreatedAt time.Time `bson:"created_at"`
	UserId    string    `bson:"user_id"`
}

func (rt *RawTweet) GetID() any {
	return rt.ID
}

func (te *TweetEvent) GetID() any {
	return te.ID
}

func (te *TweetEvent) GetURL() string {
	return fmt.Sprintf("https://vxtwitter.com/%s/status/%s", te.UserName, te.TweetID)
}

func (te *GrokTokenEvent) GetID() any {
	return te.ID
}

func (rt *RetweetEvent) GetID() any {
	return rt.ID
}

func (uch *UserCheckPoint) GetID() any {
	return uch.ID
}
func (ge *GrokEvent) GetID() any {
	return ge.ID
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

func (ge *GrokEvent) ToProto() *proto.GrokEvent {
	return &proto.GrokEvent{
		Id:              ge.ID,
		IsShill:         ge.IsShill,
		TokensMentioned: ge.TokensMentioned,
		TokensShilled:   ge.TokensShilled,
		UserId:          ge.UserId,
	}
}

type SaasEvent struct {
	ID            string       `bson:"_id"`
	TweetId       string       `bson:"tweet_id"`
	TweetUrl      string       `bson:"tweet_url"`
	Text          string       `bson:"text"`
	UserId        string       `bson:"user_id"`
	UserName      string       `bson:"username"`
	TweetType     string       `bson:"tweet_type"`
	TokensShilled []string     `bson:"tokens_shilled"`
	CreatedAt     time.Time    `bson:"created_at"`
	Category      CategoryType `bson:"category_type"`
}

func (se *SaasEvent) ToProto() *proto.TweetEventInterface {
	return &proto.TweetEventInterface{
		Id:            se.ID,
		Text:          se.Text,
		TweetId:       se.TweetId,
		UserId:        se.UserId,
		TweetUrl:      se.TweetUrl,
		TweetType:     se.TweetType,
		TokensShilled: se.TokensShilled,
		CreatedAt:     se.CreatedAt.Format(time.RFC3339),
		Username:      se.UserName,
		Category:      se.Category.String(),
	}
}

func (tw *TweetEvent) GetMediaUrls() []string {
	var urls []string

	for _, raw := range tw.MediaAttachments {
		// each attachment should be a map[string]interface{}
		m, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}

		// look at the “type” field
		typ, _ := m["type"].(string)
		switch typ {
		case "photo":
			if u, _ := m["media_url_https"].(string); u != "" {
				urls = append(urls, u)
			}
		case "video":
			if u, _ := m["expanded_url"].(string); u != "" {
				urls = append(urls, u)
			}
		default:
			if u, _ := m["expanded_url"].(string); u != "" {
				urls = append(urls, u)
			}
		}

	}

	return urls
}

type Following struct {
	ID          string    `json:"id" bson:"_id"`
	UserID      string    `json:"user_id" bson:"user_id"`
	FollowingID string    `json:"following_id" bson:"following_id"`
	UpdatedAt   time.Time `json:"updated_at" bson:"updated_at"`
}

func NewFollowing(userID, followingID string) *Following {
	return &Following{
		ID:          fmt.Sprintf("%s_%s", userID, followingID),
		UserID:      userID,
		FollowingID: followingID,
		UpdatedAt:   time.Now(),
	}
}

func (f *Following) GetID() any {
	return f.ID
}
