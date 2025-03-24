package controller

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jeetdotso/tweet-indexer-service/indexer/service"
)

type TweetEventController struct {
	api service.TweetAPI
}

func NewTweetIndexApi(api service.TweetAPI) *TweetEventController {
	beventCrtl := TweetEventController{api: api}
	return &beventCrtl
}

func (ctrl TweetEventController) ReplayEvents(c *gin.Context) {
	ctx := c.Request.Context()
	idx, _ := strconv.ParseInt(c.DefaultQuery("index", "0"), 10, 64)
	ctrl.api.ReplayEvents(ctx, idx)
	c.JSON(http.StatusCreated, gin.H{"response": "Events emitted!"})
}

func (ctrl TweetEventController) ReplayEventsByUser(c *gin.Context) {
	ctx := c.Request.Context()
	userId := c.Param("user_id")
	idx, _ := strconv.ParseInt(c.DefaultQuery("index", "0"), 10, 64)
	ctrl.api.ReplayEventsByUserID(ctx, userId, idx)
	c.JSON(http.StatusCreated, gin.H{"response": "Events emitted!"})
}

func (ctrl TweetEventController) ReplayEventsByID(c *gin.Context) {
	ctx := c.Request.Context()
	id := c.Param("id")
	ctrl.api.ReplayEventByID(ctx, id)
	c.JSON(http.StatusCreated, gin.H{"response": "Events emitted!"})
}
