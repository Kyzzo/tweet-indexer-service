package accounts_health

import (
	"context"

	"github.com/jeetdotso/tweet-indexer-service/dummyaccount"
	"github.com/jeetdotso/tweet-indexer-service/scheduler"
)

type HealthCheck struct {
	dummySvc dummyaccount.Service
}

const (
	SCHEDULER_THREADS = 10
)

func NewAccountHealthScheduler(dummySvc dummyaccount.Service) scheduler.Scheduler {
	return &HealthCheck{
		dummySvc: dummySvc,
	}
}

func (s *HealthCheck) Execute() {
	ctx := context.Background()
	Check(ctx, SCHEDULER_THREADS, s.dummySvc, dummyaccount.WORKING_GROUP)
}

func (c *HealthCheck) Expression() string {
	return "@every 24h"
}
