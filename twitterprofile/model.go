package twitterprofile

import "time"

type TwitterProfile struct {
	ID         string    `bson:"_id" json:"id"`
	UserName   string    `bson:"user_name" json:"user_name"`
	Handle     string    `bson:"handle"   json:"handle"`
	ProfileImg string    `bson:"profile_img" json:"profile_img"`
	Followers  int       `bson:"follower_count"  json:"follower_count"`
	Following  int       `bson:"following_count" json:"following_count"`
	Verified   bool      `bson:"verified"        json:"verified"`
	IsCrypto   bool      `bson:"is_crypto"       json:"is_crypto"`
	Tracking   bool      `bson:"tracking"        json:"tracking"`
	CapturedAt time.Time `bson:"captured_at"     json:"captured_at"`
	UpdatedAt  time.Time `bson:"updated_at"      json:"updated_at"`
}

func (f *TwitterProfile) GetID() any {
	return f.ID
}

type TagType string

const (
	TAG_TRADER     TagType = "trader"
	TAG_MEME       TagType = "memecoin speculator"
	TAG_INFLUENCER TagType = "influencer"
)
const (
	NEW_PROFILE     = 0
	REMOVED_PROFILE = 1
	ADDED_PROFILE   = 2
	NEUTRAL_PROFILE = 3
)

func (t TagType) String() string {
	return string(t)
}

type FetchedProfile struct {
	ID         string    `bson:"_id" json:"id"`
	UserName   string    `bson:"user_name" json:"user_name"`
	Handle     string    `bson:"handle"   json:"handle"`
	ProfileImg string    `bson:"profile_img" json:"profile_img"`
	Followers  int       `bson:"follower_count"  json:"follower_count"`
	Following  int       `bson:"following_count" json:"following_count"`
	Verified   bool      `bson:"verified"        json:"verified"`
	Tag        string    `bson:"tag"            json:"tag"`
	Status     int       `bson:"status"         json:"status"` // 0: not tracked, 1: tracking, 2: not tracking
	CapturedAt time.Time `bson:"captured_at"     json:"captured_at"`
	UpdatedAt  time.Time `bson:"updated_at"      json:"updated_at"`
}
type FetchedProfileFull struct {
	ID              string    `bson:"_id" json:"id"`
	UserName        string    `bson:"user_name" json:"user_name"`
	Handle          string    `bson:"handle"   json:"handle"`
	ProfileImg      string    `bson:"profile_img" json:"profile_img"`
	Followers       int       `bson:"follower_count"  json:"follower_count"`
	Following       int       `bson:"following_count" json:"following_count"`
	Verified        bool      `bson:"verified"        json:"verified"`
	Tag             string    `bson:"tag"            json:"tag"`
	Status          int       `bson:"status"         json:"status"` // 0: not tracked, 1: tracking, 2: not tracking
	CapturedAt      time.Time `bson:"captured_at"     json:"captured_at"`
	UpdatedAt       time.Time `bson:"updated_at"      json:"updated_at"`
	FollowedByCount int       `bson:"followed_by_count" json:"followed_by_count"` // Number of users following this profile
}

func (f *FetchedProfile) GetID() any {
	return f.ID
}

// in your persistence or repo package
type FetchedProfileQuery struct {
	Page         int64 // 1‐based
	Size         int64
	SortField    string // e.g. "captured_at" or "follower_count"
	Asc          bool
	Status       *int     // nil = no filter
	Tags         []string // empty = no filter
	FollowersMin *int     // nil = no filter
	FollowersMax *int     // nil = no filter
}
