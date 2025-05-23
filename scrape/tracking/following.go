package tracking

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	dummyaccount "github.com/jeetdotso/tweet-indexer-service/dummyaccount"
	"github.com/jeetdotso/tweet-indexer-service/logg"
	twitterAPI "github.com/jeetdotso/tweet-indexer-service/scrape/internals/api"
	twitterTypes "github.com/jeetdotso/tweet-indexer-service/scrape/internals/api/types"
	followingParsers "github.com/jeetdotso/tweet-indexer-service/scrape/internals/parsers/following"
	"github.com/tidwall/gjson"
)

type FollowingMonitor struct {
	twitterAPI *twitterAPI.TwitterClient
}

func NewFollowingMonitor(ctx context.Context, svc dummyaccount.Service) (*FollowingMonitor, *dummyaccount.SelAccount, error) {
	sel, err := svc.LogInToNextGroup(ctx, dummyaccount.WORKING_GROUP)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error retrieving Selenium account.")
		return nil, nil, err
	}
	client, _ := twitterAPI.CreateTwitterClientWithCookies(sel.GetCookiesHttp())
	return &FollowingMonitor{twitterAPI: client}, sel, nil
}

// ExtractEntries pulls out the array of “user” entries from the GraphQL payload
func (fm *FollowingMonitor) ExtractEntries(r *gjson.Result) []gjson.Result {
	instrs := r.
		Get("data.user.result.timeline.timeline.instructions").
		Array()

	for _, ins := range instrs {
		if ins.Get("type").String() == "TimelineAddEntries" {
			allEntries := ins.Get("entries").Array()
			// strip off the last two, which are the "cursor" entries
			if len(allEntries) >= 2 {
				return allEntries[:len(allEntries)-2]
			}
			return allEntries
		}
	}
	return nil
}

// MonitorSingleAccount fetches *all* pages of “following” for the given user,
// parses every entry into FollowingUser, and invokes handleFollowing on each.
func (fm *FollowingMonitor) MonitorSingleAccount(
	ctx context.Context,
	id string,
	maxRetries int,
	retryDelay time.Duration,
	handleFollowing func(context.Context, string, twitterTypes.FollowingUser),
) error {
	// 1) look up userID (cache or however you store it)

	cursor := ""
	batch := 1

	for {
		// 2) fetch page with retries
		var resp *gjson.Result
		for attempt := 1; attempt <= maxRetries; attempt++ {
			r, err := fm.twitterAPI.Following(id, cursor)
			if err == nil {
				resp = r
				break
			}
			logg.Log.Error().Caller().Ctx(ctx).Err(err).Msgf("id %s: fetching following page %d attempt %d/%d failed", id, batch, attempt, maxRetries)
			time.Sleep(retryDelay)
		}
		if resp == nil {
			return fmt.Errorf("max retries reached for %s page %d", id, batch)
		}

		// 3) extract entries
		entries, next := extractEntriesAndCursor(resp)
		if len(entries) == 0 {
			logg.Log.Info().Caller().Ctx(ctx).
				Msgf("id %q: no entries on page %d → stopping", id, batch)
			break
		}
		fmt.Println("Lenght of entries: ", len(entries))
		for _, e := range entries {
			u := followingParsers.ParseFollowing(&e)
			if u == nil {
				logg.Log.Error().Caller().Ctx(ctx).
					Msgf("id %q: failed to parse entry %s", id, e.Raw)
				continue
			}
			if u.Handle == "" {
				logg.Log.Error().Caller().Ctx(ctx).
					Msgf("id %q: empty handle for entry %s", id, e.Raw)
				continue
			}
			handleFollowing(ctx, id, *u)
		}

		// if Twitter returns the “end” sentinel, we’re done
		if next == "" || strings.HasPrefix(next, "0|") {
			logg.Log.Info().Caller().Ctx(ctx).
				Msgf("id %q: end of list reached on batch %d", id, batch)
			break
		}

		// otherwise, use it for the next round
		cursor = next
		batch++
	}

	return nil
}

// extractEntriesAndCursor will scan the “instructions” array,
// find the TimelineAddEntries block, and return:
//   - a slice of just the *user* entries
//   - the next‐page cursor string (or empty if you’ve hit the end)
func extractEntriesAndCursor(r *gjson.Result) (users []gjson.Result, nextCursor string) {
	// 1) locate the “AddEntries” instruction
	instr := r.Get("data.user.result.timeline.timeline.instructions").Array()
	var add gjson.Result
	for _, in := range instr {
		if in.Get("type").String() == "TimelineAddEntries" {
			add = in
			break
		}
	}
	if !add.Exists() {
		return nil, ""
	}

	all := add.Get("entries").Array()
	if len(all) < 2 {
		// nothing to do
		return nil, ""
	}

	// the second‐to‐last entry is always the bottom‐cursor
	bottom := all[len(all)-2]
	nextCursor = bottom.Get("content.value").String()

	// everything *before* the last two entries are the real user items
	users = all[:len(all)-2]
	return users, nextCursor
}

// MonitorMultiple does the same across many usernames, up to maxWorkers concurrently.
func MonitorMultiple(
	ctx context.Context,
	usernames []string,
	maxWorkers, maxRetries int,
	retryDelay time.Duration,
	svc dummyaccount.Service,
	handleFollowing func(context.Context, string, twitterTypes.FollowingUser),
) {
	var wg sync.WaitGroup
	sem := make(chan struct{}, maxWorkers)

	for _, username := range usernames {
		wg.Add(1)
		sem <- struct{}{}

		go func(user string) {
			defer wg.Done()
			defer func() { <-sem }()

			fm, sel, err := NewFollowingMonitor(ctx, svc)
			if err != nil {
				logg.Log.Error().Caller().Ctx(ctx).Err(err).
					Msgf("username %s: unable to init FollowingMonitor", user)
				return
			}
			defer svc.LogOut(ctx, sel.UserName)

			if err := fm.MonitorSingleAccount(ctx, user, maxRetries, retryDelay, handleFollowing); err != nil {
				logg.Log.Error().Caller().Ctx(ctx).Err(err).
					Msgf("username %s: error during MonitorSingleAccount", user)
			}
		}(username)
		time.Sleep(50 * time.Millisecond) // throttle between launches
	}

	wg.Wait()
}
