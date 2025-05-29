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
