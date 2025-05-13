package dummyaccount

import (
	"context"
	"sync"

	"github.com/rs/zerolog/log"
)

type Onboard interface {
	Run(ctx context.Context)
}

type OnboardDefaultImpl struct {
	hlp  Helper
	repo Repository
	svc  Service
}

func NewOnboardDefaultImpl(hlp Helper, repo Repository, svc Service) *OnboardDefaultImpl {
	return &OnboardDefaultImpl{
		hlp:  hlp,
		repo: repo,
		svc:  svc,
	}
}
func (s *OnboardDefaultImpl) Run(ctx context.Context) {

	auth, err := s.svc.FindAllByGroup(ctx, NEW_GROUP)
	if err != nil {
		log.Error().Err(err).Msg("Error getting accounts")
		return
	}
	maxThreads := 8

	var wg sync.WaitGroup
	sem := make(chan struct{}, maxThreads) // Limit concurrent WebDrivers
	for _, a := range auth {
		wg.Add(1)
		sem <- struct{}{} // Block if too many WebDrivers are running
		////////////////////////////////////For 2fa accounts////////////////////////////////////
		if a.TwoFAPre != "" {
			go func(account *SelAccount) {
				defer wg.Done()
				defer func() { <-sem }() // Release semaphore slot

				s.check2fa(ctx, account)
			}(a)
		} else {

			////////////////////////////////////For non 2fa accounts////////////////////////////////////
			go func(account *SelAccount) {
				defer wg.Done()
				defer func() { <-sem }() // Release semaphore slot

				// Create a WebDriver for this worker
				s.check(ctx, account)
			}(a)
		}
	}
	wg.Wait()
}

func (s *OnboardDefaultImpl) check(ctx context.Context, account *SelAccount) {

	// Create a WebDriver for this worker
	wd, err := s.hlp.LoadWebDriverNoCookies()
	if err != nil {
		log.Printf("Error loading WebDriver: %v\n", err)
		return
	}
	defer wd.Quit() // Ensure WebDriver closes after this account
	// Log in and fetch cookies
	cookies, err := s.hlp.LogInAndGetCookies(ctx, account.UserName, account.Password, wd)
	if err != nil {
		log.Printf("Error logging in for user: %s\n", account.UserName)
		return
	}

	// Save cookies to DB
	account.Cookies = cookies
	account.Group = TEST_GROUP

	err = s.repo.UpdateOrInsert(ctx, account)
	if err != nil {
		log.Printf("Error updating cookies for user: %s\n", account.UserName)
	}
}

func (s *OnboardDefaultImpl) check2fa(ctx context.Context, account *SelAccount) {
	// Create a WebDriver for this worker

	wd, err := s.hlp.LoadWebDriverNoCookies()
	if err != nil {
		log.Printf("Error loading WebDriver: %v\n", err)
		return
	}
	defer wd.Quit() // Ensure WebDriver closes after this account

	session2fa, err := s.hlp.Get2FAToken(wd, account.TwoFAPre)
	if err != nil {
		log.Printf("Error getting 2fa token: %v\n", err)
		return
	}

	// Log in and fetch cookies
	cookies, err := s.hlp.LogInAndGetCookies2fa(ctx, account.UserName, account.Password, session2fa, wd)
	if err != nil {
		log.Printf("Error logging in for user: %s\n", account.UserName)
		return
	}

	// Save cookies to DB
	account.Cookies = cookies
	account.Group = TEST_GROUP

	account.TwoFACode = session2fa

	err = s.repo.UpdateOrInsert(ctx, account)
	if err != nil {
		log.Printf("Error updating cookies for user: %s\n", account.UserName)
	}

}
