package connection

import (
	"github.com/gin-gonic/gin"
	"github.com/jeetdotso/tweet-indexer-service/config"
	"github.com/jeetdotso/tweet-indexer-service/connection/proto/grpcclient"
	"github.com/jeetdotso/tweet-indexer-service/connection/twitteruser/user"
	"github.com/jeetdotso/tweet-indexer-service/http/httputil"
	"github.com/jeetdotso/tweet-indexer-service/persistence/mongodb"
)

type Controller struct {
	twtUsr user.TwitteruserService
}

func NewController(cfg *config.Config, conn *mongodb.MongoConnection, grpcconn *grpcclient.GrpcClientMap) *Controller {
	var (
		twtUsr = user.NewTwitterUserService(grpcconn, cfg.Services)
	)
	return &Controller{
		twtUsr: twtUsr,
	}
}

// check twitter health
func (ctrl *Controller) CheckTwitterHealth(c *gin.Context) {
	ctx := c.Request.Context()
	err := ctrl.twtUsr.CheckHealth(ctx)
	if err != nil {
		c.JSON(500, httputil.InternalServerError(err.Error()))
		return
	}
	c.JSON(200, httputil.OK("Accountable services Twitter connection is alive!", true))
}

// check both health
func (ctrl *Controller) CheckHealth(c *gin.Context) {
	ctx := c.Request.Context()
	err := ctrl.twtUsr.CheckHealth(ctx)
	if err != nil {
		c.JSON(500, httputil.InternalServerError(err.Error()))
		return
	}
	c.JSON(200, httputil.OK("Accountable services connection is alive!", true))
}
