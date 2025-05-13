package dummyaccount

import (
	"context"
	"sync"
	"time"

	"github.com/jeetdotso/tweet-indexer-service/config"
	"github.com/jeetdotso/tweet-indexer-service/logg"
	"github.com/jeetdotso/tweet-indexer-service/persistence/mongodb"
	"github.com/tebeka/selenium"
)

type Service interface {
	NewAccount(ctx context.Context, username, password string, group int) error
	JustAddCookies(ctx context.Context, username string, cookies []selenium.Cookie) error
	FindInUse(ctx context.Context, group int) (*SelAccount, error)
	RemoveCookies(ctx context.Context, username string) error
	FindGroups(ctx context.Context) (int, error)
	LogInToNext(ctx context.Context) (*SelAccount, error)
	LogInToNextGroup(ctx context.Context, group int) (*SelAccount, error)
	LogOut(ctx context.Context, userName string) error
	SwitchGroup(ctx context.Context, username string, group int) error
	FindByUsername(ctx context.Context, username string) (*SelAccount, error)
	ResetAll(ctx context.Context) error
	FindAndAddTokens(ctx context.Context, username, bearer, csrf string) error
	FindAllByGroup(ctx context.Context, group int) ([]*SelAccount, error)
	NewTwoFaAccount(ctx context.Context, username, password string, group int, twoPreFa string) error
	Update2FaCode(ctx context.Context, username, code string) error
}

var (
	nextMu      sync.Mutex
	nextGroupMu sync.Mutex
)

type ServiceDefaultImpl struct {
	hlp  Helper
	repo Repository
}

func NewSeleniumService(conn *mongodb.MongoConnection, cfg *config.Config) Service {
	repos := NewRepository(conn)
	hlp := NewSeleniumHelper()
	return &ServiceDefaultImpl{
		repo: repos,
		hlp:  hlp,
	}
}

func (s *ServiceDefaultImpl) Update2FaCode(ctx context.Context, username, code string) error {
	sl, err := s.repo.FindById(ctx, username)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error finding sel account.")
		return err
	}
	if sl == nil {
		return ErrUserNotFound
	}
	sl.TwoFACode = code
	err = s.repo.UpdateOrInsert(ctx, sl)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error updating sel.")
		return err
	}
	return nil
}

func (s *ServiceDefaultImpl) FindAndAddTokens(ctx context.Context, username, bearer, csrf string) error {
	sl, err := s.repo.FindById(ctx, username)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error finding sel account.")
		return err
	}
	if sl == nil {
		return ErrUserNotFound
	}
	sl.AddTokens(bearer, csrf)
	err = s.repo.UpdateOrInsert(ctx, sl)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error updating sel.")
		return err
	}
	return nil
}

func (s *ServiceDefaultImpl) FindInUse(ctx context.Context, group int) (*SelAccount, error) {
	sl, err := s.repo.FindInUse(ctx, group)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error getting in use.")
		return nil, err
	}
	if sl == nil {
		return nil, errEmptyAccount
	}

	return sl, nil
}

func (s *ServiceDefaultImpl) JustAddCookies(ctx context.Context, username string, cookies []selenium.Cookie) error {
	sl, err := s.repo.FindById(ctx, username)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error inserting selenium account.")
		return err
	}
	if sl == nil {
		return errEmptyAccount
	}

	sl.InUse = true
	sl.AddSeleniumCookies(cookies)
	err = s.repo.UpdateOrInsert(ctx, sl)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error inserting selenium account.")
		return err
	}

	return nil
}

func (s *ServiceDefaultImpl) NewAccount(ctx context.Context, username, password string, group int) error {
	sa := NewAccount(username, password, group)
	err := s.repo.UpdateOrInsert(ctx, sa)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error inserting selenium account.")
		return err
	}
	return nil
}
func (s *ServiceDefaultImpl) NewTwoFaAccount(ctx context.Context, username, password string, group int, twoPreFa string) error {
	sa := NewTwoFaAccount(username, password, group, twoPreFa)
	err := s.repo.UpdateOrInsert(ctx, sa)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error inserting selenium account.")
		return err
	}
	return nil
}

func (s *ServiceDefaultImpl) LogInToNext(ctx context.Context) (*SelAccount, error) {

	nextMu.Lock()
	// Getting next
	sl, err := s.repo.FindNext(ctx)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error finding next selenium account.")
		nextMu.Unlock()
		return nil, err
	}
	if sl == nil {
		nextMu.Unlock()
		return nil, errEmptyAccount
	}
	lastUpdated := sl.UpdatedAt
	sl.InUse = true
	sl.UpdatedAt = time.Now()
	sl.LastLoggedIn = time.Now()

	err = s.repo.UpdateOrInsert(ctx, sl)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error updating selenium account.")
		nextMu.Unlock()
		return nil, err
	}
	nextMu.Unlock()

	timeToWait := lastUpdated.Add(1 * time.Minute)

	// Calculate the remaining duration to wait
	remainingTime := time.Until(timeToWait)

	// If there's still time to wait, sleep for the remaining duration
	if remainingTime > 0 {
		time.Sleep(remainingTime)
	}

	return sl, nil
}
func (s *ServiceDefaultImpl) LogInToNextGroup(ctx context.Context, group int) (*SelAccount, error) {

	nextGroupMu.Lock()
	defer nextGroupMu.Unlock()
	// Getting next
	sl, err := s.repo.FindNextByGroup(ctx, group)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error finding next selenium account.")
		return nil, err
	}
	if sl == nil {
		return nil, errEmptyAccount
	}
	sl.InUse = true
	sl.UpdatedAt = time.Now()
	sl.LastLoggedIn = time.Now()

	err = s.repo.UpdateOrInsert(ctx, sl)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error updating selenium account.")
		return nil, err
	}

	return sl, nil
}

func (s *ServiceDefaultImpl) LogOut(ctx context.Context, userName string) error {

	//get in use first

	sl, err := s.repo.FindById(ctx, userName)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error finding next selenium account.")
		return err
	}
	if sl == nil {
		return errEmptyAccount
	}

	sl.InUse = false
	sl.UpdatedAt = time.Now()
	sl.LastLoggedIn = time.Now()
	err = s.repo.UpdateOrInsert(ctx, sl)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error updating selenium account.")
		return err
	}
	return nil
}

func (svc *ServiceDefaultImpl) FindGroups(ctx context.Context) (int, error) {
	groups, err := svc.repo.FindGroupNumber(ctx)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error grtting groups.")
		return 0, err
	}
	return groups, nil
}

func (svc *ServiceDefaultImpl) SwitchGroup(ctx context.Context, username string, group int) error {
	sel, err := svc.repo.FindById(ctx, username)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error finding sel account.")
		return err
	}
	if sel == nil {
		return ErrUserNotFound
	}
	sel.Group = group
	err = svc.repo.UpdateOrInsert(ctx, sel)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error updating sel.")
		return err
	}
	return nil

}
func (svc *ServiceDefaultImpl) FindByUsername(ctx context.Context, username string) (*SelAccount, error) {
	sel, err := svc.repo.FindById(ctx, username)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error getting sel.")
		return nil, err
	}
	return sel, nil
}

func (s *ServiceDefaultImpl) ResetAll(ctx context.Context) error {
	return s.repo.ResetAll(ctx)
}

func (s *ServiceDefaultImpl) FindAllByGroup(ctx context.Context, group int) ([]*SelAccount, error) {
	return s.repo.FindAllByGroup(ctx, group)
}

func (s *ServiceDefaultImpl) RemoveCookies(ctx context.Context, username string) error {
	sl, err := s.repo.FindById(ctx, username)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error finding sel account.")
		return err
	}
	if sl == nil {
		return ErrUserNotFound
	}
	sl.Cookies = []selenium.Cookie{}
	err = s.repo.UpdateOrInsert(ctx, sl)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error updating sel.")
		return err
	}
	return nil
}
