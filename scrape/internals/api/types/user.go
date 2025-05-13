package twitterTypes

import "time"

type User struct {
	ID         string    `bson:"_id"            json:"id"`
	UserId     string    `bson:"user_id"        json:"user_id"`
	UserName   string    `bson:"user_name"      json:"user_name"`
	Handle     string    `bson:"handle"         json:"handle"`
	ProfileImg string    `bson:"profile_img"    json:"profile_img"`
	Followers  int       `bson:"follower_count" json:"follower_count"`
	Following  int       `bson:"following_count"json:"following_count"`
	Verified   bool      `bson:"verified"       json:"verified"`
	CreatedAt  time.Time `bson:"created_at"     json:"created_at"`
	UpdatedAt  time.Time `bson:"updated_at"     json:"updated_at"`
}

func (usr *User) GetID() any {
	return usr.ID
}
