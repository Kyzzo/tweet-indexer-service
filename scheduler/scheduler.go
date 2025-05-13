package scheduler

type Scheduler interface {
	Execute()
	Expression() string
}
