package scheduler

import (
	"context"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/bsm/redislock"
	"github.com/jeetdotso/tweet-indexer-service/config"
	"github.com/jeetdotso/tweet-indexer-service/cycle"
	"github.com/jeetdotso/tweet-indexer-service/indexer/repository"
	"github.com/jeetdotso/tweet-indexer-service/logg"
	redis2 "github.com/jeetdotso/tweet-indexer-service/memory/redis"
	"github.com/jeetdotso/tweet-indexer-service/persistence/mongodb"
	"github.com/jeetdotso/tweet-indexer-service/scheduler"
	"github.com/jeetdotso/tweet-indexer-service/shared"
	"github.com/jeetdotso/tweet-indexer-service/twitterprofile"
	"github.com/redis/go-redis/v9"
)

type ScrapeCycle struct {
	locker      *redislock.Client
	scrapeCycle cycle.Service
	twtPrf      twitterprofile.Repository
	twtRepo     repository.TweetEventRepository
	grokRepo    repository.GrokEventRepository
	tags        []string
}

const (
	ScrapeCycleKey = "scrape-cycle-lock-sync-daily-job"
)

func NewScrapeCycleScheduler(cfg *config.Config, conn *mongodb.MongoConnection, tags []twitterprofile.TagType) scheduler.Scheduler {
	locker := redislock.New(redis.NewClient(redis2.GetRedisOptions(cfg.Memory)))
	scrapeCycle := cycle.NewCycleService(conn, cfg)
	twtPrf := twitterprofile.NewRepository(conn)
	twtRepo := repository.NewTweetIndexRepo(conn)
	grokRepo := repository.NewGrokIndexRepo(conn)
	// Convert tags to string slice
	var tagStrings []string
	for _, tag := range tags {
		tagStrings = append(tagStrings, string(tag))
	}
	return &ScrapeCycle{
		locker:      locker,
		scrapeCycle: scrapeCycle,
		twtPrf:      twtPrf,
		twtRepo:     twtRepo,
		tags:        tagStrings,
		grokRepo:    grokRepo,
	}
}

func (s *ScrapeCycle) Execute() {

	ctx := context.Background()
	lock, err := s.locker.Obtain(ctx, ScrapeCycleKey, 15*time.Minute, &redislock.Options{
		// give it up to half a second to get the lock
		RetryStrategy: redislock.LimitRetry(redislock.LinearBackoff(50*time.Millisecond), 10),
	})
	if err == redislock.ErrNotObtained {
		logg.Log.Warn().Caller().Ctx(ctx).Err(err).Msg("Job lock busy...")
		return
	} else if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error acquiring lock lb.")
		return
	}
	logg.Log.Info().Caller().Ctx(ctx).Msg("Obtained lock Daily scrape sync up")

	defer lock.Release(ctx)
	sc, err := s.scrapeCycle.FindLatest(ctx)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error finding latest scrape cycle.")
		return
	}
	if sc != nil && sc.StartTime.Add(23*time.Hour).After(time.Now()) {
		logg.Log.Info().Caller().Ctx(ctx).Msg("Scrape cycle already exists for today, skipping.")
		return
	}
	if sc != nil {

		// get function for avgs
		avgGrokTime, err := s.twtRepo.FindAvgGrokTime(ctx, sc.StartTime)
		if err != nil {
			logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error fetching average grok time.")
			return
		}
		sc.AvgGrokTime = avgGrokTime
		avgScrapeTime, err := s.twtRepo.FindAvgScrapeTime(ctx, sc.StartTime)
		if err != nil {
			logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error fetching average scrape time.")
			return
		}
		sc.AvgScrapeTime = avgScrapeTime
		//get funciton for tweet totals
		grokCount, noneGrokCount, err := s.twtRepo.CountGrokVsNonGrok(ctx, sc.StartTime)
		if err != nil {
			logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error fetching grok vs non-grok count.")
			return
		}
		sc.TweetsScraped = int(grokCount)
		sc.NoneTrackedScraped = int(noneGrokCount)
		reps, err := strconv.Atoi(os.Getenv("REPLICAS")) // injected by Helm
		if err != nil {
			log.Printf("invalid REPLICAS: %v", err)
		}
		sc.PodsRunning = reps
		shillCount, err := s.grokRepo.GrokTrueSince(ctx, sc.StartTime)
		if err != nil {
			logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error fetching shill count.")
			return
		}
		sc.Shills = int(shillCount)
		//get functions for users (unique and from scrape)
		uniqueUsers, err := s.twtRepo.FindUniqueUsersSince(ctx, sc.StartTime)
		if err != nil {
			logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error fetching unique users.")
			return
		}
		sc.UniqueUsers = int(uniqueUsers)
		var totalUsers int
		for _, tag := range s.tags {
			users, err := s.twtPrf.GetTopHitRaw(ctx, int64(shared.MAX_ACCOUNTS_RUN), []string{tag})
			if err != nil {
				logg.Log.Error().Caller().Ctx(ctx).Err(err).Msgf("Error fetching users for tag %s.", tag)
				continue
			}
			totalUsers += len(users)
		}
		sc.TotalUsers = totalUsers
		sc.Tags = s.tags
		if err = s.scrapeCycle.UpdateOrInsert(ctx, sc); err != nil {
			logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error updating scrape cycle.")
			return
		}
		// end cycle
		if err = s.scrapeCycle.EndCycle(ctx); err != nil {
			logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error ending scrape cycle.")
			return
		}
	}
	// create new cycle
	if err = s.scrapeCycle.CreateNewCycle(ctx, cycle.TOKEN_GROUP_INDEXER); err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error creating new scrape cycle.")
	}

}

func (c *ScrapeCycle) Expression() string {
	return "@daily" // equivalent to "0 0 * * *"
}
