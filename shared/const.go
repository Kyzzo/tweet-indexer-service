package shared

import "github.com/jeetdotso/tweet-indexer-service/twitterprofile"

const (
	MAX_ACCOUNTS_RUN = 1500
)

var (
	RUNNING_TAGS      = []string{twitterprofile.TAG_TRADER.String(), twitterprofile.TAG_MEME.String()}
	RUNNING_TAGS_TYPE = []twitterprofile.TagType{twitterprofile.TAG_TRADER, twitterprofile.TAG_MEME}
)
