package controller

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jeetdotso/tweet-indexer-service/config"
	"github.com/jeetdotso/tweet-indexer-service/dummyaccount"
	"github.com/jeetdotso/tweet-indexer-service/indexer/service"
	"github.com/jeetdotso/tweet-indexer-service/persistence/mongodb"
	"github.com/jeetdotso/tweet-indexer-service/twitterprofile"
)

type AdminController struct {
	ftcPrf twitterprofile.Repository
	fllIdx service.FollowingIndexer
}

func NewAdminController(cfg *config.Config, conn *mongodb.MongoConnection, dummySvc dummyaccount.Service) *AdminController {

	ftcPrf := twitterprofile.NewRepository(conn)
	fllIdx := service.NewFollowingIndexer(*cfg, conn, dummySvc) // Assuming dummy account service is not needed here
	return &AdminController{
		ftcPrf: ftcPrf,
		fllIdx: fllIdx,
	}
}

func (ctrl AdminController) AddList(ctx *gin.Context) {
	username := ctx.Param("username")
	if username == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "username is required"})
		return
	}
	err := ctrl.fllIdx.GetByUsernameAndSaveFollowings(ctx.Request.Context(), username)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "Following list added successfully", "username": username})
}

func (ctrl AdminController) GetList(ctx *gin.Context) {
	// 1) grab & default your page/size/sort/asc
	page, err := strconv.ParseInt(ctx.DefaultQuery("page", "1"), 10, 64)
	if err != nil || page < 1 {
		page = 1
	}
	size, err := strconv.ParseInt(ctx.DefaultQuery("size", "20"), 10, 64)
	if err != nil || size < 1 {
		size = 20
	}

	sortField := ctx.DefaultQuery("sort_field", "captured_at")
	asc := ctx.DefaultQuery("asc", "false") == "true"

	// 2) optional filters
	var statusPtr *int
	if s := ctx.Query("status"); s != "" {
		if si, err := strconv.Atoi(s); err == nil {
			statusPtr = &si
		}
	}

	tags := ctx.QueryArray("tag") // ?tag=x&tag=y

	var minFPtr, maxFPtr *int
	if s := ctx.Query("min_followers"); s != "" {
		if v, err := strconv.Atoi(s); err == nil {
			minFPtr = &v
		}
	}
	if s := ctx.Query("max_followers"); s != "" {
		if v, err := strconv.Atoi(s); err == nil {
			maxFPtr = &v
		}
	}

	// 3) build your query struct
	q := twitterprofile.FetchedProfileQuery{
		Page:         page,
		Size:         size,
		SortField:    sortField,
		Asc:          asc,
		Status:       statusPtr,
		Tags:         tags,
		FollowersMin: minFPtr,
		FollowersMax: maxFPtr,
	}

	// 4) call your service / repo
	resultPage, err := ctrl.ftcPrf.FindFetchedProfiles(ctx.Request.Context(), &q)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 5) return it
	ctx.JSON(http.StatusOK, resultPage)
}

func (ctrl AdminController) ChangeStatusById(ctx *gin.Context) {
	id := ctx.Param("id")
	if id == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "id is required"})
		return
	}

	statusStr := ctx.Query("status")
	if statusStr == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "status is required"})
		return
	}

	status, err := strconv.Atoi(statusStr)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid status value"})
		return
	}

	err = ctrl.ftcPrf.ChangeStatusById(ctx.Request.Context(), id, status)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "Status updated successfully", "id": id})
}
