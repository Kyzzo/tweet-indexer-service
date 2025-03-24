package cycle

import (
	"context"
	"sync"

	"github.com/jeetdotso/tweet-indexer-service/config"
	"github.com/jeetdotso/tweet-indexer-service/logg"
	"github.com/jeetdotso/tweet-indexer-service/persistence/mongodb"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Service interface {
	UpdateOrInsert(ctx context.Context, pl *ScrapeCycle) error
	FindById(ctx context.Context, id primitive.ObjectID) (*ScrapeCycle, error)
	FindLatest(ctx context.Context) (*ScrapeCycle, error)
	CreateNewCycle(ct context.Context, totalUsers, removedUsers, newUsers int) error
	AddRollback(ctx context.Context) error
	AddScrapedTweets(ctx context.Context) error
	EndCycle(ctx context.Context) error
}

var (
	nextMu sync.Mutex
)

type ServiceDefaultImpl struct {
	repo Repository
}

func NewCycleService(conn *mongodb.MongoConnection, cfg *config.Config) Service {
	repos := NewRepository(conn)
	return &ServiceDefaultImpl{
		repo: repos,
	}
}

func (s *ServiceDefaultImpl) UpdateOrInsert(ctx context.Context, pl *ScrapeCycle) error {
	err := s.repo.UpdateOrInsert(ctx, pl)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error updating or inserting cycle.")
		return err
	}
	return nil
}

func (s *ServiceDefaultImpl) FindById(ctx context.Context, id primitive.ObjectID) (*ScrapeCycle, error) {
	cycle, err := s.repo.FindById(ctx, id)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error finding cycle by id.")
		return nil, err
	}
	return cycle, nil
}

func (s *ServiceDefaultImpl) FindLatest(ctx context.Context) (*ScrapeCycle, error) {
	cycle, err := s.repo.FindLatest(ctx)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error finding latest cycle.")
		return nil, err
	}
	return cycle, nil
}

func (s *ServiceDefaultImpl) CreateNewCycle(ct context.Context, totalUsers, removedUsers, newUsers int) error {
	cycle := NewScrapeCycle()
	cycle.TotalUsers = totalUsers
	cycle.RemovedUsers = removedUsers
	cycle.NewUsers = newUsers
	err := s.UpdateOrInsert(context.Background(), cycle)
	if err != nil {
		return err
	}
	return nil
}

func (s *ServiceDefaultImpl) AddRollback(ctx context.Context) error {
	cycle, err := s.FindLatest(ctx)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error finding latest cycle.")
		return err
	}
	cycle.RollbacksTriggered++
	err = s.UpdateOrInsert(ctx, cycle)
	if err != nil {
		return err
	}
	return nil
}

func (s *ServiceDefaultImpl) AddScrapedTweets(ctx context.Context) error {
	cycle, err := s.FindLatest(ctx)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error finding latest cycle.")
		return err
	}
	cycle.TweetsScraped++
	err = s.UpdateOrInsert(ctx, cycle)
	if err != nil {
		return err
	}
	return nil
}

func (s *ServiceDefaultImpl) EndCycle(ctx context.Context) error {
	cycle, err := s.FindLatest(ctx)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error finding latest cycle.")
		return err
	}
	cycle.EndCycle()
	err = s.UpdateOrInsert(ctx, cycle)
	if err != nil {
		return err
	}
	return nil
}
