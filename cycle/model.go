package cycle

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ScrapeCycle struct {
	ID                 primitive.ObjectID     `bson:"_id,omitempty"`
	StartTime          time.Time              `bson:"start_time"`
	EndTime            time.Time              `bson:"end_time"`
	TotalUsers         int                    `bson:"total_users"`
	NewUsers           int                    `bson:"new_users"`
	RemovedUsers       int                    `bson:"removed_users"`
	TweetsScraped      int                    `bson:"tweets_scraped"`
	RollbacksTriggered int                    `bson:"rollbacks_triggered"`
	Status             int                    `bson:"status"`
	Metadata           map[string]interface{} `bson:"metadata,omitempty"`
}

const (
	Running = iota
	Completed
	Failed
)

var StatusMap = map[int]string{
	Running:   "running",
	Completed: "completed",
	Failed:    "failed",
}

func (sc *ScrapeCycle) GetStatus() string {
	return StatusMap[sc.Status]
}

func NewScrapeCycle() *ScrapeCycle {
	return &ScrapeCycle{
		ID:        primitive.NewObjectID(),
		StartTime: time.Now(),
		Status:    Running,
	}
}

func (sc *ScrapeCycle) End() {
	sc.EndTime = time.Now()
	sc.Status = Completed
}

func (sc *ScrapeCycle) GetID() any {
	return sc.ID
}

func (sc *ScrapeCycle) EndCycle() {
	sc.EndTime = time.Now()
	sc.Status = Completed
}