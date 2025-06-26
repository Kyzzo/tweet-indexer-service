package cycle

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ScrapeCycle struct {
	ID                 primitive.ObjectID     `bson:"_id"`
	StartTime          time.Time              `bson:"start_time"`
	CycleType          string                 `bson:"cycle_type"`
	PodsRunning        int                    `bson:"pods_running"`
	EndTime            time.Time              `bson:"end_time"`
	TotalUsers         int                    `bson:"total_users"`
	UniqueUsers        int                    `bson:"unique_users"`
	TweetsScraped      int                    `bson:"tracked_tweets_scraped"`
	NoneTrackedScraped int                    `bson:"none_tracked_scraped"`
	GrokErrors         int                    `bson:"grok_errors"`
	AvgGrokTime        float64                `bson:"avg_grok_time"`
	AvgScrapeTime      float64                `bson:"avg_scrape_time"`
	Shills             int                    `bson:"shills"`
	Status             int                    `bson:"status"`
	Tags               []string               `bson:"tags"`
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
	TOKEN_GROUP_INDEXER CycleType = "daily_scrape_data"

	//////Following Cycle Types
	DailyUpdate   CycleType = "daily_update"
	MonthlyUpdate CycleType = "monthly_update"
)

var StatusMap = map[int]string{
	Running:   "RUNNING",
	Completed: "COMPLETED",
	Failed:    "FAILED",
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

// /////////////////////////Following Cycle///////////////////////////////////////
type FollowingCycle struct {
	ID                      primitive.ObjectID `bson:"_id"`
	StartTime               time.Time          `bson:"start_time"`
	EndTime                 time.Time          `bson:"end_time"`
	TotalInfluencerAccounts int                `bson:"total_influencer_accounts"`
	TotalUserAccounts       int                `bson:"total_user_accounts"`
	Status                  int                `bson:"status"`
	CycleType               CycleType          `bson:"cycle_type"`
}

func (fc *FollowingCycle) GetID() any {
	return fc.ID
}
func NewFollowingCycle(ct CycleType) *FollowingCycle {
	return &FollowingCycle{
		ID:        primitive.NewObjectID(),
		StartTime: time.Now(),
		CycleType: ct,
		Status:    Running,
	}
}
