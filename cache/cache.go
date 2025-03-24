package cache

import (
	twitterAPI "github.com/jeetdotso/tweet-indexer-service/scrape/internals/api"
	types "github.com/jeetdotso/tweet-indexer-service/scrape/tracking/types"
	utils "github.com/jeetdotso/tweet-indexer-service/utils"
)

var (
	logger       = utils.SetupLogger()
	TweetsCache  = make(map[string]*types.Tweet)
	userIDsCache = make(map[string]string)
)

func GetUserID(username string) (string, error) {
	UserID, exists := userIDsCache[username]
	if !exists {
		logger.Debugf("[%s] UserID not found in cache, fetching from API", username)
		var err error
		client, _ := twitterAPI.CreateTwitterClient()
		UserID, err = client.UserID(username)
		if err != nil {
			return "", err
		}
		userIDsCache[username] = UserID
		logger.Debugf("[%s] Cached UserID [%s]", username, UserID)
	}
	return UserID, nil
}

func GetTweet(username string) *types.Tweet {
	tweet, exists := TweetsCache[username]
	if !exists {
		return nil
	}
	return tweet
}

func UpdateTweet(username string, tweet *types.Tweet) {
	TweetsCache[username] = tweet
}

func UpdateUserID(username, userID string) {
	userIDsCache[username] = userID
}

func RemoveUserID(username string) {
	delete(userIDsCache, username)
}
