package twitterTypes

type FollowingUser struct {
	ID         string `bson:"_id" json:"id"`
	UserName   string `bson:"user_name" json:"user_name"`
	Handle     string `bson:"handle"   json:"handle"`
	ProfileImg string `bson:"profile_img" json:"profile_img"`
	Followers  int    `bson:"follower_count"  json:"follower_count"`
	Following  int    `bson:"following_count" json:"following_count"`
	Verified   bool   `bson:"verified"        json:"verified"`
	IsCrypto   bool   `bson:"is_crypto"       json:"is_crypto"`
	Tracking   bool   `bson:"tracking"        json:"tracking"`
}

func (f *FollowingUser) GetID() any {
	return f.ID
}

func (f *FollowingUser) TOTwitterUser() *TrackedUser {
	return &TrackedUser{
		ID:           f.ID,
		UserName:     f.UserName,
		Handle:       f.Handle,
		ProfileImg:   f.ProfileImg,
		Followers:    f.Followers,
		Following:    f.Following,
		Verified:     f.Verified,
		UserId:       f.ID,
		ProfileImgS3: f.ProfileImg,
	}
}
