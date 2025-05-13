package twitteruser

import (
	"time"
)

const (
	CREATED_AT = "created_at"
)

type User struct {
	ID           string    `bson:"_id" json:"id"`
	UserId       string    `bson:"user_id" json:"user_id"`
	UserName     string    `bson:"user_name" json:"user_name"`
	Handle       string    `bson:"handle" json:"handle"`
	ProfileImg   string    `bson:"profile_img" json:"profile_img"`
	ProfileImgS3 string    `bson:"profile_img_s3" json:"profile_img_s3"`
	Followers    int       `json:"follower_count" bson:"follower_count"`
	Following    int       `json:"following_count" bson:"following_count"`
	Verified     bool      `bson:"verified" json:"verified"`
	CreatedAt    time.Time `bson:"created_at" json:"created_at"`
	UpdatedAt    time.Time `bson:"updated_at" json:"updated_at"`
}
