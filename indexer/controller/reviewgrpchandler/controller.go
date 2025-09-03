package reviewgrpchandler

import (
	"context"
	"time"

	"github.com/jeetdotso/tweet-indexer-service/config"
	"github.com/jeetdotso/tweet-indexer-service/indexer/repository"
	"github.com/jeetdotso/tweet-indexer-service/persistence/mongodb"
	"github.com/jeetdotso/tweet-indexer-service/proto/grpcreviewentry"
)

type ReviewGRPCHandler struct {
	grpcreviewentry.UnimplementedReviewEntryServiceServer
	twtRepo repository.TweetEventRepository
}

func NewReviewGRPCHandler(cfg *config.Config, conn *mongodb.MongoConnection) *ReviewGRPCHandler {
	twtRepo := repository.NewTweetIndexRepo(conn)

	return &ReviewGRPCHandler{
		twtRepo: twtRepo,
	}
}

func (h *ReviewGRPCHandler) FetchDummyTweets(ctx context.Context, req *grpcreviewentry.GetDummyTweets) (*grpcreviewentry.GetDummyTweetsResponse, error) {
	tweets, err := h.twtRepo.FindAllFalseGrok(ctx, int64(req.Count))
	if err != nil {
		return nil, err
	}

	var formattedTweets []*grpcreviewentry.DummyTweet
	for _, tweet := range tweets {
		formattedTweets = append(formattedTweets, &grpcreviewentry.DummyTweet{
			Id:        tweet.ID,
			UserId:    tweet.UserID,
			Handle:    tweet.UserName,
			TweetUrl:  tweet.GetURL(),
			Text:      tweet.Text,
			CreatedAt: tweet.CreatedAt.Format(time.RFC3339),
		})
	}

	return &grpcreviewentry.GetDummyTweetsResponse{
		Tweets: formattedTweets,
	}, nil
}
