package cycle

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ScrapeCycle struct {
	ID                 primitive.ObjectID     `bson:"_id"`
	StartTime          time.Time              `bson:"start_time"`
	CycleType          string                 `bson:"cycle_type"`
	EndTime            time.Time              `bson:"end_time"`
	TotalUsers         int                    `bson:"total_users"`
	NewUsers           int                    `bson:"new_users"`
	RemovedUsers       int                    `bson:"removed_users"`
	TweetsScraped      int                    `bson:"tweets_scraped"`
	RollbacksTriggered int                    `bson:"rollbacks_triggered"`
	Status             int                    `bson:"status"`
	Metadata           map[string]interface{} `bson:"metadata"`
}

const (
	Running = iota
	Completed
	Failed
)

type CycleType string

const (
	KOL_INDEXER         CycleType = "kol_indexer"
	TOKEN_GROUP_INDEXER CycleType = "token_group_indexer"
)

var StatusMap = map[int]string{
	Running:   "running",
	Completed: "completed",
	Failed:    "failed",
}

func (sc *ScrapeCycle) GetStatus() string {
	return StatusMap[sc.Status]
}

func NewScrapeCycle(ct CycleType) *ScrapeCycle {
	return &ScrapeCycle{
		ID:        primitive.NewObjectID(),
		StartTime: time.Now(),
		Status:    Running,
		CycleType: string(ct),
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

type UpdateCycle struct {
	ID primitive.ObjectID `bson:"_id"`
}

// /////////////////////////////////////Health Cycle///////////////////////////////////////
type HealthCycle struct {
	ID               primitive.ObjectID `bson:"_id"`
	StartTime        time.Time          `bson:"start_time"`
	EndTime          time.Time          `bson:"end_time"`
	PreWorkingCount  int                `bson:"pre_working_count"`
	PostWorkingCount int                `bson:"post_working_count"`
}

func (hc *HealthCycle) GetID() any {
	return hc.ID
}
func NewHealthCycle(wokringCount int) *HealthCycle {
	return &HealthCycle{
		ID:              primitive.NewObjectID(),
		StartTime:       time.Now(),
		PreWorkingCount: wokringCount,
	}
}

func (hc *HealthCycle) EndCycle(postWorkingCount int) {
	hc.EndTime = time.Now()
	hc.PostWorkingCount = postWorkingCount
}
