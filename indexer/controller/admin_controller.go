package controller

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jeetdotso/tweet-indexer-service/config"
	"github.com/jeetdotso/tweet-indexer-service/dummyaccount"
	"github.com/jeetdotso/tweet-indexer-service/indexer/service"
	"github.com/jeetdotso/tweet-indexer-service/logg"
	"github.com/jeetdotso/tweet-indexer-service/persistence/mongodb"
	"github.com/jeetdotso/tweet-indexer-service/shared"
	"github.com/jeetdotso/tweet-indexer-service/twitterprofile"
	"github.com/jeetdotso/tweet-indexer-service/util"
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

func (ctrl AdminController) AddMultiManual(ctx *gin.Context) {
	var req struct {
		Usernames []string `json:"usernames"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if len(req.Usernames) == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "usernames cannot be empty"})
		return
	}
	ctl := util.CreateBackgroundContextFromExisting(ctx.Request.Context())
	go func(ctx context.Context, usernames []string) {
		err := ctrl.fllIdx.GetByUsernamesAndCheckMulti(ctx, req.Usernames)
		if err != nil {
			logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error adding multiple following lists")
		}
	}(ctl, req.Usernames)

	ctx.JSON(http.StatusCreated, gin.H{"message": "Following lists added successfully", "success": true})
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

	tags := ctx.Query("tag")        // ?tag=x&tag=y
	tag := strings.Split(tags, ",") // split by comma if multiple tags are provided
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
		Tags:         tag,
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

func (ctrl AdminController) ChangeStatusMultiple(ctx *gin.Context) {
	var req struct {
		Ids    []string `json:"ids"`
		Status int      `json:"status"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if len(req.Ids) == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "ids cannot be empty"})
		return
	}

	err := ctrl.ftcPrf.ChangeStatusMultiple(ctx.Request.Context(), req.Ids, req.Status)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "Statuses updated successfully", "ids": req.Ids})
}

func (ctrl AdminController) GetTop1500(ctx *gin.Context) {

	// 1) grab & default your page/size/sort/asc
	page, err := strconv.ParseInt(ctx.DefaultQuery("page", "1"), 10, 64)
	if err != nil || page < 1 {
		page = 1
	}
	size, err := strconv.ParseInt(ctx.DefaultQuery("size", "1500"), 10, 64)
	if err != nil || size < 1 {
		size = shared.MAX_ACCOUNTS_RUN
	}

	tag := ctx.Query("tag")         // ?tag=x&tag=y
	tags := strings.Split(tag, ",") // split by comma if multiple tags are provided

	// 2) call your service / repo
	resultPage, err := ctrl.ftcPrf.GetTopHit(ctx.Request.Context(), page, size, tags)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 3) return it
	ctx.JSON(http.StatusOK, resultPage)
}

func (ctrl AdminController) FindTagOptions(ctx *gin.Context) {
	uniqueTags, err := ctrl.ftcPrf.GetUniqueTags(ctx.Request.Context())
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, uniqueTags)
}

func (ctrl AdminController) ChangeTagByID(ctx *gin.Context) {
	id := ctx.Param("id")
	if id == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "id is required"})
		return
	}

	tag := ctx.Query("tag")
	if tag == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "tag is required"})
		return
	}

	err := ctrl.ftcPrf.UpdateTagById(ctx.Request.Context(), id, tag)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "Tag updated successfully", "id": id})
}
