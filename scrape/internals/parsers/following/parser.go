package following

import (
	twitterTypes "github.com/jeetdotso/tweet-indexer-service/scrape/internals/api/types"
	"github.com/tidwall/gjson"
)

func ParseFollowing(r *gjson.Result) *twitterTypes.FollowingUser {
	if r == nil {
		return nil
	}

	// 1) point at the inner "User" result
	node := r.Get("content.itemContent.user_results.result")
	if !node.Exists() {
		return nil
	}
	// 2) legacy contains most of the fields we care about
	legacy := node.Get("legacy")
	if !legacy.Exists() {
		return nil
	}

	// 3) build your struct
	restID := node.Get("rest_id").String()
	return &twitterTypes.FollowingUser{
		// use the Twitter REST ID as your Mongo _id
		ID: restID,
		// name and handle
		UserName: legacy.Get("name").String(),
		Handle:   legacy.Get("screen_name").String(),
		// profile image
		ProfileImg: legacy.Get("profile_image_url_https").String(),
		// counts
		Followers: int(legacy.Get("followers_count").Int()),
		Following: int(legacy.Get("friends_count").Int()),
		// verified (boolean in legacy)
		Verified: node.Get("is_blue_verified").Bool(),
	}
}
