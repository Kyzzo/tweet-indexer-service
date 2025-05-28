package accounts_health

import (
	"context"
	"sync"

	"github.com/jeetdotso/tweet-indexer-service/dummyaccount"
	"github.com/jeetdotso/tweet-indexer-service/logg"
	twitterAPI "github.com/jeetdotso/tweet-indexer-service/scrape/internals/api"
	"github.com/jeetdotso/tweet-indexer-service/util"
)

var (
	suspendedCount        int
	nonAuthenticatedCount int
	OTHER_GROUPCount      int
	notWorkingCount       int
	emptyRawCount         int
	liveCount             int
	totalCount            int
	statsMutex            sync.Mutex
)

func _check(ctx context.Context, i int, u dummyaccount.SelAccount, svc dummyaccount.Service) {
	client, _ := twitterAPI.NewTwitterClient(
		&twitterAPI.NewTwitterClientOptions{
			BaseURL: "https://x.com",
			Cookies: u.GetCookiesHttp(),
		},
	)
	r, _ := client.Following("180417061", "")
	errCode := r.Get("errors.0.code").Int()
	errMesg := r.Get("errors.0.message").String()

	statsMutex.Lock()
	totalCount++
	if errCode == 64 || errMesg == "Your account is suspended and is not permitted to access this feature." {
		suspendedCount++
		statsMutex.Unlock()
		logg.Log.Warn().Caller().Ctx(ctx).Str("username", u.UserName).Msg("Account suspended")
		if err := svc.SwitchGroup(ctx, u.UserName, dummyaccount.SUSPENDED_GROUP); err != nil {
			logg.Log.Error().Caller().Ctx(ctx).Err(err).Str("username", u.UserName).Msg("Error moving account to suspended group.")
		}
	} else if errCode == 32 || errMesg == "Could not authenticate you" {
		nonAuthenticatedCount++
		statsMutex.Unlock()
		logg.Log.Warn().Caller().Ctx(ctx).Str("username", u.UserName).Msg("Account not authenticated")
		if err := svc.SwitchGroup(ctx, u.UserName, dummyaccount.NOT_AUTHENTICATED_GROUP); err != nil {
			logg.Log.Error().Caller().Ctx(ctx).Err(err).Str("username", u.UserName).Msg("Error moving account to not authenticated group.")
		}
	} else if errCode == 88 || errMesg == "Rate limit exceeded" {
		notWorkingCount++
		statsMutex.Unlock()
		logg.Log.Warn().Caller().Ctx(ctx).Str("username", u.UserName).Msg("Account not working")
		if err := svc.SwitchGroup(ctx, u.UserName, dummyaccount.NOT_WORKING_GROUP); err != nil {
			logg.Log.Error().Caller().Ctx(ctx).Err(err).Str("username", u.UserName).Msg("Error moving account to not working group.")
		}
	} else if errMesg != "" {
		OTHER_GROUPCount++
		statsMutex.Unlock()
		logg.Log.Warn().Caller().Ctx(ctx).Str("username", u.UserName).Str("message", errMesg).Msg("Account other")
		if err := svc.SwitchGroup(ctx, u.UserName, dummyaccount.OTHER_GROUP); err != nil {
			logg.Log.Error().Caller().Ctx(ctx).Err(err).Str("username", u.UserName).Msg("Error moving account to not working group.")
		}

	} else if r.Raw == "" {
		emptyRawCount++
		statsMutex.Unlock()
		logg.Log.Warn().Caller().Ctx(ctx).Str("username", u.UserName).Msg("Account empty raw")
		if err := svc.SwitchGroup(ctx, u.UserName, dummyaccount.EMPTY_RAW); err != nil {
			logg.Log.Error().Caller().Ctx(ctx).Err(err).Str("username", u.UserName).Msg("Error moving account to not working group.")
		}
	} else {
		liveCount++
		statsMutex.Unlock()
		if err := svc.SwitchGroup(ctx, u.UserName, dummyaccount.WORKING_GROUP); err != nil {
			logg.Log.Error().Caller().Ctx(ctx).Err(err).Str("username", u.UserName).Msg("Error moving account to not working group.")
		}
	}
}

func Check(ctx context.Context, maxThreads int, svc dummyaccount.Service, group int) {
	var wg sync.WaitGroup
	sem := make(chan struct{}, maxThreads)
	selAccounts, err := svc.FindAllByGroup(ctx, group)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error retrieving Selenium accounts.")
		return
	}
	for i, u := range selAccounts {
		ctl := util.CreateBackgroundContextFromExisting(ctx)
		wg.Add(1)
		sem <- struct{}{}
		go func(ctx context.Context, i int, u dummyaccount.SelAccount, svc dummyaccount.Service) {
			defer wg.Done()
			defer func() { <-sem }()
			_check(ctx, i, u, svc)
		}(ctl, i, *u, svc)
	}
	wg.Wait()
	close(sem)

	logg.Log.Info().Caller().Ctx(ctx).Int("working accounts", liveCount).Msg("Working Accounts")
	logg.Log.Info().Caller().Ctx(ctx).Int("total accounts", totalCount).Msg("Total Accounts")
	suspendedCount = 0
	liveCount = 0
	totalCount = 0
}
