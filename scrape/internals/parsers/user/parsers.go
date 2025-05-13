package user

import (
	"time"

	twitterTypes "github.com/jeetdotso/tweet-indexer-service/scrape/internals/api/types"
	"github.com/tidwall/gjson"
)

func ParseUserFromGJSON(res *gjson.Result) (*twitterTypes.User, error) {
	u := res.Get("data.user.result")

	// grab the two “IDs”:
	id := u.Get("rest_id").String()     // numeric Twitter ID
	userID := u.Get("rest_id").String() // internal base64‐style

	// name + handle
	name := u.Get("legacy.name").String()
	handle := u.Get("legacy.screen_name").String()

	// profile image URL
	img := u.Get("legacy.profile_image_url_https").String()

	// follower / following counts
	followers := int(u.Get("legacy.followers_count").Int())
	following := int(u.Get("legacy.friends_count").Int())

	// verified flag
	verified := u.Get("legacy.verified").Bool()

	// parse the created_at timestamp
	createdAtStr := u.Get("legacy.created_at").String()
	createdAt, err := time.Parse(time.RubyDate, createdAtStr)
	if err != nil {
		createdAt = time.Time{} // default to zero value if parsing fails
	}

	return &twitterTypes.User{
		ID:         id,
		UserId:     userID,
		UserName:   name,
		Handle:     handle,
		ProfileImg: img,
		Followers:  followers,
		Following:  following,
		Verified:   verified,
		CreatedAt:  createdAt,
		UpdatedAt:  time.Now(),
	}, nil
}
