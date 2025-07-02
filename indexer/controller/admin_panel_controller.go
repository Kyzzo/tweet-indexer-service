package controller

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jeetdotso/tweet-indexer-service/config"
	"github.com/jeetdotso/tweet-indexer-service/cycle"
	"github.com/jeetdotso/tweet-indexer-service/dummyaccount"
	"github.com/jeetdotso/tweet-indexer-service/indexer/repository"
	"github.com/jeetdotso/tweet-indexer-service/indexer/service"
	"github.com/jeetdotso/tweet-indexer-service/persistence/mongodb"
	"github.com/jeetdotso/tweet-indexer-service/twitterprofile"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type AdminPanleController struct {
	ftcPrf   twitterprofile.Repository
	fllIdx   service.FollowingIndexer
	grokRepo repository.GrokEventRepository
	cycSvc   cycle.Service
}

func NewAdminPanleController(cfg *config.Config, conn *mongodb.MongoConnection, dummySvc dummyaccount.Service) *AdminPanleController {

	ftcPrf := twitterprofile.NewRepository(conn)
	fllIdx := service.NewFollowingIndexer(*cfg, conn, dummySvc) // Assuming dummy account service is not needed here
	grokRepo := repository.NewGrokIndexRepo(conn)
	cycSvc := cycle.NewCycleService(conn, cfg)
	return &AdminPanleController{
		ftcPrf:   ftcPrf,
		fllIdx:   fllIdx,
		grokRepo: grokRepo,
		cycSvc:   cycSvc,
	}
}

func (ctrl AdminPanleController) FindByCycleTypeAndLimit(ctx *gin.Context) {

	cycType := ctx.Param("type")
	if cycType == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "cycType is required"})
		return
	}
	if cycType != "kol_indexer" && cycType != "daily_scrape_data" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid cycle type. Allowed values are: kol_indexer, daily_scrape_data"})
		return
	}
	// kol_indexer and daily_scrape_data are the only allowed cycle types
	limit := ctx.Query("limit")
	if limit == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "limit is required"})
		return
	}

	limitInt, err := strconv.Atoi(limit)
	if err != nil || limitInt <= 0 {
		limitInt = 10 // Default limit if parsing fails or is invalid
	}
	cycles, err := ctrl.cycSvc.GetCycleByTypeAndLimit(ctx.Request.Context(), cycle.CycleType(cycType), limitInt)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})

		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "Cycles retrieved successfully", "cycType": cycType, "cycles": cycles})
}

func (ctrl AdminPanleController) GetLatestScrapeCycle(ctx *gin.Context) {
	cycle, err := ctrl.cycSvc.FindLatest(ctx.Request.Context())
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "Latest Cycle retrieved successfully", "cycle": *cycle})
}

// //////////////////Following cycles/////////////////////
// DailyUpdate   CycleType = "daily_update"
// MonthlyUpdate CycleType = "monthly_update"
func (ctrl AdminPanleController) GetFollowingCycle(ctx *gin.Context) {
	cycType := ctx.Param("type")
	if cycType == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "cycType is required"})
		return
	}
	if cycType != "daily_update" && cycType != "monthly_update" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid cycle type. Allowed values are: daily_update, monthly_update"})
		return
	}
	// Get the latest following cycle by type
	latestFollowingCycle, err := ctrl.cycSvc.FindFollowingLatest(ctx.Request.Context(), cycle.CycleType(cycType))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"message": "Latest Following Cycle retrieved successfully", "cycle": *latestFollowingCycle})
}
func (ctrl AdminPanleController) GetAllFollowingCycles(ctx *gin.Context) {
	cycles, err := ctrl.cycSvc.FindAllFollowing(ctx.Request.Context())
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"message": "All Following Cycles retrieved successfully", "cycles": cycles})
}

func (ctrl AdminPanleController) GetFollowingCycleById(ctx *gin.Context) {
	id := ctx.Param("id")
	if id == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "id is required"})
		return
	}

	objectId, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid id format"})
		return
	}
	followingCycle, err := ctrl.cycSvc.FindFollowingById(ctx.Request.Context(), objectId)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"message": "Following Cycle retrieved successfully", "cycle": followingCycle})
}

// //////Health cycles/////////////////
func (ctrl AdminPanleController) GetHealthCycle(ctx *gin.Context) {
	healthCycle, err := ctrl.cycSvc.FindLatestHealth(ctx.Request.Context())
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"message": "Health Cycle retrieved successfully", "cycle": *healthCycle})
}

func (ctrl AdminPanleController) GetAllHealthCycles(ctx *gin.Context) {
	healthCycles, err := ctrl.cycSvc.FindAllHealth(ctx.Request.Context())
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"message": "All Health Cycles retrieved successfully", "cycles": healthCycles})
}

// ///////////////Grok Data Cycles/////////////////
type GetGrokChartRequest struct {
	Start      time.Time `form:"start"     time_format:"2006-01-02T15:04:05Z07:00"  binding:"required"`
	End        time.Time `form:"end"       time_format:"2006-01-02T15:04:05Z07:00"  binding:"required"`
	Interval   string    `form:"interval"  binding:"required,oneof=5min 1H 2H 3H 1D 2D 1W"`
	Categories []int     `form:"category"`
}

func (ctrl AdminPanleController) GetGrokDataCycles(c *gin.Context) {
	var req GetGrokChartRequest

	// bind & validate query params
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	chart, err := ctrl.grokRepo.GetGrokChart(
		c.Request.Context(),
		req.Start,
		req.End,
		req.Interval,
		req.Categories,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// return the full chart response
	c.JSON(http.StatusOK, chart)
}
