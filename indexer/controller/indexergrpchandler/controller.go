package indexergrpchandler

import (
	"context"
	"errors"

	"github.com/jeetdotso/tweet-indexer-service/config"
	"github.com/jeetdotso/tweet-indexer-service/dummyaccount"
	"github.com/jeetdotso/tweet-indexer-service/indexer/repository"
	"github.com/jeetdotso/tweet-indexer-service/indexer/service"
	"github.com/jeetdotso/tweet-indexer-service/logg"
	"github.com/jeetdotso/tweet-indexer-service/persistence/mongodb"
	"github.com/jeetdotso/tweet-indexer-service/proto/grpcsaasentry"
	"github.com/jeetdotso/tweet-indexer-service/shared"
	"github.com/jeetdotso/tweet-indexer-service/twitterprofile"
	"github.com/jeetdotso/tweet-indexer-service/util"
)

type SaasGRPCHandler struct {
	grpcsaasentry.UnimplementedSaasEntryServiceServer
	followSvc service.FollowingIndexer
	prfSvc    service.Profile
	follRepo  repository.FollowingRepository
	twtSvc    twitterprofile.Repository
	rawRepo   repository.RawTweetRepository
	svc       dummyaccount.Service
}

var (
	errProfileNotFound = errors.New("profile not found")
)

func NewSaasGRPCHandler(cfg *config.Config, conn *mongodb.MongoConnection, dummSvc dummyaccount.Service) *SaasGRPCHandler {
	follRepo := repository.NewFollowingRepository(conn)
	followSvc := service.NewFollowingIndexer(*cfg, conn, dummSvc)
	prfSvc := service.NewProfile(*cfg, conn, dummSvc)
	twtSvc := twitterprofile.NewRepository(conn)
	rawRepo := repository.NewRawTweetRepo(conn)

	return &SaasGRPCHandler{
		followSvc: followSvc,
		prfSvc:    prfSvc,
		follRepo:  follRepo,
		svc:       dummSvc,
		twtSvc:    twtSvc,
		rawRepo:   rawRepo,
	}
}

func (h *SaasGRPCHandler) GetProfile(ctx context.Context, req *grpcsaasentry.GetProfileRequest) (*grpcsaasentry.GetProfileResponse, error) {
	profile, err := h.prfSvc.GetProfileByUsername(ctx, req.Username)
	if err != nil {
		return nil, err
	}
	if profile == nil {
		resp := &grpcsaasentry.GetProfileResponse{
			Profile: nil,
			Success: false,
			Message: "Profile not found",
		}
		return resp, errProfileNotFound
	}

	resp := &grpcsaasentry.GetProfileResponse{
		Profile: &grpcsaasentry.Profile{
			Id:         profile.ID,
			UserId:     profile.ID,
			UserName:   profile.UserName,
			Handle:     profile.Handle,
			ProfileImg: profile.ProfileImg,
			Followers:  int32(profile.Followers),
			Following:  int32(profile.Following),
			Verified:   profile.Verified,
		},
		Success: true,
	}
	return resp, nil

}

func (h *SaasGRPCHandler) CheckHealth(ctx context.Context, req *grpcsaasentry.EmptyRequest) (*grpcsaasentry.CheckHealthResponse, error) {
	resp := &grpcsaasentry.CheckHealthResponse{
		IsOk: true,
	}
	return resp, nil
}

func (h *SaasGRPCHandler) AddFollowing(ctx context.Context, req *grpcsaasentry.AddFollowingRequest) (*grpcsaasentry.CheckHealthResponse, error) {
	if req == nil || req.Id == "" {
		return nil, errors.New("invalid request: profile ID is required")
	}
	ctl := util.CreateBackgroundContextFromExisting(ctx)
	go h.followSvc.ScrapeFollowingListById(ctl, req.Id)

	resp := &grpcsaasentry.CheckHealthResponse{
		IsOk: true,
	}
	return resp, nil
}

func (h *SaasGRPCHandler) GetFollowListByID(ctx context.Context, req *grpcsaasentry.FollowedListRequest) (*grpcsaasentry.FollowListResponse, error) {
	if req == nil || req.KolId == "" {
		return nil, errors.New("invalid request: profile ID is required")
	}

	followingList, err := h.follRepo.FindAllByFollowingID(ctx, req.KolId)
	if err != nil {
		return nil, err
	}

	resp := &grpcsaasentry.FollowListResponse{
		Ids: followingList,
	}
	return resp, nil

}

func (h *SaasGRPCHandler) GetFollowedListWithExcluding(ctx context.Context, req *grpcsaasentry.FollowListWithExclusionsRequest) (*grpcsaasentry.FollowListResponse, error) {
	if req == nil || req.KolId == "" {
		return nil, errors.New("invalid request: profile ID is required")
	}

	followingList, err := h.follRepo.FollowedByWithExclusions(ctx, req.KolId, req.ExcludeIds)
	if err != nil {
		return nil, err
	}

	resp := &grpcsaasentry.FollowListResponse{
		Ids: followingList,
	}
	return resp, nil
}

// AddFollowingsByUsername
func (h *SaasGRPCHandler) AddFollowingsByUsername(ctx context.Context, req *grpcsaasentry.AddFollowingsByUsernameRequest) (*grpcsaasentry.AddFollowingsUsernameResponse, error) {
	if req == nil || req.Username == "" {
		return nil, errors.New("invalid request: usernames are required")
	}
	id, err := h.prfSvc.GetProfileByUsername(ctx, req.Username)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error retrieving profile by username.")
		resp := &grpcsaasentry.AddFollowingsUsernameResponse{
			Id:      "",
			Message: "Profile not found",
			Success: false,
		}
		return resp, err
	}
	if id == nil || id.ID == "" {
		resp := &grpcsaasentry.AddFollowingsUsernameResponse{
			Id:      "",
			Message: "Profile not found",
			Success: false,
		}
		return resp, errProfileNotFound
	}

	ctl := util.CreateBackgroundContextFromExisting(ctx)
	go h.followSvc.ScrapeFollowingListById(ctl, id.ID)

	resp := &grpcsaasentry.AddFollowingsUsernameResponse{
		Message: "Followings are being added for the profile",
		Id:      id.ID,
		Success: true,
	}
	return resp, nil
}

func (h *SaasGRPCHandler) GetFollowingFullList(ctx context.Context, req *grpcsaasentry.FollowedListRequest) (*grpcsaasentry.FollowingFullListResponse, error) {
	if req == nil || req.KolId == "" {
		return nil, errors.New("invalid request: profile ID is required")
	}

	followingList, err := h.follRepo.FindAllByUserID(ctx, req.KolId)
	if err != nil {
		return nil, err
	}
	if len(followingList) == 0 {
		resp := &grpcsaasentry.FollowingFullListResponse{
			Profiles: []*grpcsaasentry.Profile{},
		}
		return resp, nil
	}
	prfs, err := h.twtSvc.FindAllByIDs(ctx, followingList)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error retrieving profiles by IDs.")
		return nil, err
	}
	profiles := make([]*grpcsaasentry.Profile, len(prfs))
	for i, prf := range prfs {
		profiles[i] = &grpcsaasentry.Profile{
			Id:         prf.ID,
			UserId:     prf.ID,
			UserName:   prf.UserName,
			Handle:     prf.Handle,
			ProfileImg: prf.ProfileImg,
			Followers:  int32(prf.Followers),
			Following:  int32(prf.Following),
			Verified:   prf.Verified,
		}
	}
	resp := &grpcsaasentry.FollowingFullListResponse{
		Profiles: profiles,
	}
	return resp, nil
}

func (h *SaasGRPCHandler) GetProfilesByIDs(ctx context.Context, req *grpcsaasentry.BlockListRequest) (*grpcsaasentry.FollowingFullListResponse, error) {
	if req == nil || len(req.KolIds) == 0 {
		return nil, errors.New("invalid request: profile IDs are required")
	}

	prfs, err := h.twtSvc.FindAllByIDs(ctx, req.KolIds)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error retrieving profiles by IDs.")
		return nil, err
	}
	if len(prfs) == 0 {
		resp := &grpcsaasentry.FollowingFullListResponse{
			Profiles: []*grpcsaasentry.Profile{},
		}
		return resp, nil
	}

	profiles := make([]*grpcsaasentry.Profile, len(prfs))
	for i, prf := range prfs {
		profiles[i] = &grpcsaasentry.Profile{
			Id:         prf.ID,
			UserId:     prf.ID,
			UserName:   prf.UserName,
			Handle:     prf.Handle,
			ProfileImg: prf.ProfileImg,
			Followers:  int32(prf.Followers),
			Following:  int32(prf.Following),
			Verified:   prf.Verified,
		}
	}
	resp := &grpcsaasentry.FollowingFullListResponse{
		Profiles: profiles,
	}
	return resp, nil
}

func (h *SaasGRPCHandler) GetRawTweetEvent(ctx context.Context, req *grpcsaasentry.GetRawTweetEventRequest) (*grpcsaasentry.GetRawTweetEventResponse, error) {
	if req == nil || req.TweetId == "" {
		return nil, errors.New("invalid request: tweet ID is required")
	}

	rawTweetEvent, err := h.rawRepo.FindAllByID(ctx, req.TweetId)
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error retrieving raw tweet event.")
		return nil, err
	}

	resp := &grpcsaasentry.GetRawTweetEventResponse{
		RawTweetEvent: rawTweetEvent.RawEntry,
	}
	return resp, nil
}

func (h *SaasGRPCHandler) GetTrackedProfiles(ctx context.Context, req *grpcsaasentry.EmptyRequest) (*grpcsaasentry.TopProfileResponse, error) {
	profiles, err := h.twtSvc.GetTopHitRaw(ctx, shared.MAX_ACCOUNTS_RUN, []string{twitterprofile.TAG_TRADER.String()})
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error retrieving tracked profiles.")
		return nil, err
	}

	if len(profiles) == 0 {
		resp := &grpcsaasentry.TopProfileResponse{
			Profiles: []*grpcsaasentry.Profile{},
			Success:  true,
		}
		return resp, nil
	}

	profileList := make([]*grpcsaasentry.Profile, len(profiles))
	for i, prf := range profiles {
		profileList[i] = &grpcsaasentry.Profile{
			Id:         prf.ID,
			UserId:     prf.ID,
			UserName:   prf.UserName,
			Handle:     prf.Handle,
			ProfileImg: prf.ProfileImg,
			Followers:  int32(prf.Followers),
			Following:  int32(prf.Following),
			Verified:   prf.Verified,
		}
	}

	resp := &grpcsaasentry.TopProfileResponse{
		Profiles: profileList,
		Success:  true,
	}
	return resp, nil
}
