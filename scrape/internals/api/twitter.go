package twitterAPI

import (
	"encoding/json"
	"net/url"
	"fmt"

	"github.com/tidwall/gjson"
)

func (tc *TwitterClient) commonRequest(endpoint string, params url.Values) (*gjson.Result, error) {
	fullURL := fmt.Sprintf("%s%s?%s", tc.BaseURL, endpoint, params.Encode())
	bodyBytes, statusCode, err := tc.Get(fullURL, nil)
	r := gjson.ParseBytes(*bodyBytes)
	if err != nil {
		return nil, fmt.Errorf("cannot do the request right now")
	} else if statusCode != 200 {
		return &r, fmt.Errorf("bad request")
	}
	return &r, nil
}

func (tc *TwitterClient) UserID(username string) (string, error) {
	variables, _ := json.Marshal(map[string]string{"screen_name": username})
	features, _ := json.Marshal(map[string]interface{}{
		"hidden_profile_subscriptions_enabled":                              true,
		"profile_label_improvements_pcf_label_in_post_enabled":              true,
		"rweb_tipjar_consumption_enabled":                                   true,
		"responsive_web_graphql_exclude_directive_enabled":                  true,
		"verified_phone_label_enabled":                                      false,
		"subscriptions_verification_info_is_identity_verified_enabled":      true,
		"subscriptions_verification_info_verified_since_enabled":            true,
		"highlights_tweets_tab_ui_enabled":                                  true,
		"responsive_web_twitter_article_notes_tab_enabled":                  true,
		"subscriptions_feature_can_gift_premium":                            true,
		"creator_subscriptions_tweet_preview_api_enabled":                   true,
		"responsive_web_graphql_skip_user_profile_image_extensions_enabled": false,
		"responsive_web_graphql_timeline_navigation_enabled":                true,
	})
	fieldToggles, _ := json.Marshal(map[string]bool{"withAuxiliaryUserLabels": false})
	params := url.Values{}
	params.Add("variables", string(variables))
	params.Add("features", string(features))
	params.Add("fieldToggles", string(fieldToggles))

	result, err := tc.commonRequest("/i/api/graphql/32pL5BWe9WKeSK1MoPvFQQ/UserByScreenName", params)
	if err != nil {
		return "", err
	}
	return result.Get("data.user.result.rest_id").String(), nil
}

func (tc *TwitterClient) Following(userID string, cursor string) (*gjson.Result, error) {
	variables, _ := json.Marshal(map[string]interface{}{
		"userId":                 userID,
		"count":                  20,
		"cursor":                 cursor,
		"includePromotedContent": false,
	})
	features, _ := json.Marshal(map[string]interface{}{
		"profile_label_improvements_pcf_label_in_post_enabled":                    false,
		"rweb_tipjar_consumption_enabled":                                         true,
		"responsive_web_graphql_exclude_directive_enabled":                        true,
		"verified_phone_label_enabled":                                            false,
		"creator_subscriptions_tweet_preview_api_enabled":                         true,
		"responsive_web_graphql_timeline_navigation_enabled":                      true,
		"responsive_web_graphql_skip_user_profile_image_extensions_enabled":       false,
		"premium_content_api_read_enabled":                                        false,
		"communities_web_enable_tweet_community_results_fetch":                    true,
		"c9s_tweet_anatomy_moderator_badge_enabled":                               true,
		"responsive_web_grok_analyze_button_fetch_trends_enabled":                 true,
		"articles_preview_enabled":                                                true,
		"responsive_web_edit_tweet_api_enabled":                                   true,
		"graphql_is_translatable_rweb_tweet_is_translatable_enabled":              true,
		"view_counts_everywhere_api_enabled":                                      true,
		"longform_notetweets_consumption_enabled":                                 true,
		"responsive_web_twitter_article_tweet_consumption_enabled":                true,
		"tweet_awards_web_tipping_enabled":                                        false,
		"creator_subscriptions_quote_tweet_preview_enabled":                       false,
		"freedom_of_speech_not_reach_fetch_enabled":                               true,
		"standardized_nudges_misinfo":                                             true,
		"tweet_with_visibility_results_prefer_gql_limited_actions_policy_enabled": true,
		"rweb_video_timestamps_enabled":                                           true,
		"longform_notetweets_rich_text_read_enabled":                              true,
		"longform_notetweets_inline_media_enabled":                                true,
		"responsive_web_enhance_cards_enabled":                                    false,
	})
	params := url.Values{}
	params.Add("variables", string(variables))
	params.Add("features", string(features))

	return tc.commonRequest("/i/api/graphql/gsxNGYhRKA6iYYSInE9qew/Following", params)
}

func (tc *TwitterClient) Followers(userID string, cursor string) (*gjson.Result, error) {
	variables, _ := json.Marshal(map[string]interface{}{
		"userId":                 userID,
		"count":                  20,
		"cursor":                 cursor,
		"includePromotedContent": false,
	})
	features, _ := json.Marshal(map[string]interface{}{
		"profile_label_improvements_pcf_label_in_post_enabled":                    true,
		"rweb_tipjar_consumption_enabled":                                         true,
		"responsive_web_graphql_exclude_directive_enabled":                        true,
		"verified_phone_label_enabled":                                            false,
		"creator_subscriptions_tweet_preview_api_enabled":                         true,
		"responsive_web_graphql_timeline_navigation_enabled":                      true,
		"responsive_web_graphql_skip_user_profile_image_extensions_enabled":       false,
		"premium_content_api_read_enabled":                                        false,
		"communities_web_enable_tweet_community_results_fetch":                    true,
		"c9s_tweet_anatomy_moderator_badge_enabled":                               true,
		"responsive_web_grok_analyze_button_fetch_trends_enabled":                 false,
		"responsive_web_grok_analyze_post_followups_enabled":                      true,
		"responsive_web_jetfuel_frame":                                            false,
		"responsive_web_grok_share_attachment_enabled":                            true,
		"articles_preview_enabled":                                                true,
		"responsive_web_edit_tweet_api_enabled":                                   true,
		"graphql_is_translatable_rweb_tweet_is_translatable_enabled":              true,
		"view_counts_everywhere_api_enabled":                                      true,
		"longform_notetweets_consumption_enabled":                                 true,
		"responsive_web_twitter_article_tweet_consumption_enabled":                true,
		"tweet_awards_web_tipping_enabled":                                        false,
		"responsive_web_grok_analysis_button_from_backend":                        true,
		"creator_subscriptions_quote_tweet_preview_enabled":                       false,
		"freedom_of_speech_not_reach_fetch_enabled":                               true,
		"standardized_nudges_misinfo":                                             true,
		"tweet_with_visibility_results_prefer_gql_limited_actions_policy_enabled": true,
		"rweb_video_timestamps_enabled":                                           true,
		"longform_notetweets_rich_text_read_enabled":                              true,
		"longform_notetweets_inline_media_enabled":                                true,
		"responsive_web_grok_image_annotation_enabled":                            false,
		"responsive_web_enhance_cards_enabled":                                    false,
	})
	params := url.Values{}
	params.Add("variables", string(variables))
	params.Add("features", string(features))

	return tc.commonRequest("/i/api/graphql/OGScL-RC4DFMsRGOCjPR6g/Followers", params)
}

func (tc *TwitterClient) AccountTweets(userID string, cursor string) (*gjson.Result, error) {
	variables := map[string]interface{}{
		"userId":                                 userID,
		"count":                                  20,
		"includePromotedContent":                 true,
		"withQuickPromoteEligibilityTweetFields": true,
		"withVoice":                              true,
		"withV2Timeline":                         true,
	}
	if cursor != "" {
		variables["cursor"] = cursor
	}
	variablesJSON, _ := json.Marshal(variables)
	features, _ := json.Marshal(map[string]interface{}{
		"profile_label_improvements_pcf_label_in_post_enabled":        true,
		"rweb_tipjar_consumption_enabled":                             true,
		"responsive_web_graphql_exclude_directive_enabled":            true,
		"verified_phone_label_enabled":                                false,
		"creator_subscriptions_tweet_preview_api_enabled":             true,
		"responsive_web_graphql_timeline_navigation_enabled":          true,
		"responsive_web_graphql_skip_user_profile_image_extensions_enabled": false,
		"premium_content_api_read_enabled":                            false,
		"communities_web_enable_tweet_community_results_fetch":        true,
		"c9s_tweet_anatomy_moderator_badge_enabled":                   true,
		"responsive_web_grok_analyze_button_fetch_trends_enabled":     false,
		"responsive_web_grok_analyze_post_followups_enabled":          true,
		"responsive_web_jetfuel_frame":                                false,
		"responsive_web_grok_share_attachment_enabled":                true,
		"articles_preview_enabled":                                    true,
		"responsive_web_edit_tweet_api_enabled":                       true,
		"graphql_is_translatable_rweb_tweet_is_translatable_enabled":  true,
		"view_counts_everywhere_api_enabled":                          true,
		"longform_notetweets_consumption_enabled":                     true,
		"responsive_web_twitter_article_tweet_consumption_enabled":    true,
		"tweet_awards_web_tipping_enabled":                            false,
		"creator_subscriptions_quote_tweet_preview_enabled":           false,
		"freedom_of_speech_not_reach_fetch_enabled":                   true,
		"standardized_nudges_misinfo":                                 true,
		"tweet_with_visibility_results_prefer_gql_limited_actions_policy_enabled": true,
		"rweb_video_timestamps_enabled":                               true,
		"longform_notetweets_rich_text_read_enabled":                  true,
		"longform_notetweets_inline_media_enabled":                    true,
		"responsive_web_grok_image_annotation_enabled":                true,
		"responsive_web_enhance_cards_enabled":                        false,
	})
	fieldToggles, _ := json.Marshal(map[string]bool{"withArticlePlainText": false})
	params := url.Values{}
	params.Add("variables", string(variablesJSON))
	params.Add("features", string(features))
	params.Add("fieldToggles", string(fieldToggles))

	return tc.commonRequest("/i/api/graphql/MpOINUGH_YVb2BKjYZOPaQ/UserTweets", params)
}

func (tc *TwitterClient) GetTweet(tweetID string) (*gjson.Result, error) {
	variables, _ := json.Marshal(map[string]interface{}{
		"focalTweetId":                           tweetID,
		"with_rux_injections":                    false,
		"rankingMode":                            "Relevance",
		"includePromotedContent":                 true,
		"withCommunity":                          true,
		"withQuickPromoteEligibilityTweetFields": true,
		"withBirdwatchNotes":                     true,
		"withVoice":                              true,
	})
	features, _ := json.Marshal(map[string]interface{}{
		"profile_label_improvements_pcf_label_in_post_enabled":                    true,
		"rweb_tipjar_consumption_enabled":                                         true,
		"responsive_web_graphql_exclude_directive_enabled":                        true,
		"verified_phone_label_enabled":                                            false,
		"creator_subscriptions_tweet_preview_api_enabled":                         true,
		"responsive_web_graphql_timeline_navigation_enabled":                      true,
		"responsive_web_graphql_skip_user_profile_image_extensions_enabled":       false,
		"premium_content_api_read_enabled":                                        false,
		"communities_web_enable_tweet_community_results_fetch":                    true,
		"c9s_tweet_anatomy_moderator_badge_enabled":                               true,
		"responsive_web_grok_analyze_button_fetch_trends_enabled":                 false,
		"responsive_web_grok_analyze_post_followups_enabled":                      true,
		"responsive_web_jetfuel_frame":                                            false,
		"responsive_web_grok_share_attachment_enabled":                            true,
		"articles_preview_enabled":                                                true,
		"responsive_web_edit_tweet_api_enabled":                                   true,
		"graphql_is_translatable_rweb_tweet_is_translatable_enabled":              true,
		"view_counts_everywhere_api_enabled":                                      true,
		"longform_notetweets_consumption_enabled":                                 true,
		"responsive_web_twitter_article_tweet_consumption_enabled":                true,
		"tweet_awards_web_tipping_enabled":                                        false,
		"responsive_web_grok_analysis_button_from_backend":                        true,
		"creator_subscriptions_quote_tweet_preview_enabled":                       false,
		"freedom_of_speech_not_reach_fetch_enabled":                               true,
		"standardized_nudges_misinfo":                                             true,
		"tweet_with_visibility_results_prefer_gql_limited_actions_policy_enabled": true,
		"rweb_video_timestamps_enabled":                                           true,
		"longform_notetweets_rich_text_read_enabled":                              true,
		"longform_notetweets_inline_media_enabled":                                true,
		"responsive_web_grok_image_annotation_enabled":                            false,
		"responsive_web_enhance_cards_enabled":                                    false,
	})

	fieldToggles, _ := json.Marshal(map[string]bool{
		"withArticleRichContentState": true,
		"withArticlePlainText":        false,
		"withGrokAnalyze":             false,
		"withDisallowedReplyControls": false,
	})
	params := url.Values{}
	params.Add("variables", string(variables))
	params.Add("features", string(features))
	params.Add("fieldToggles", string(fieldToggles))

	return tc.commonRequest("/i/api/graphql/Ez6kRPyXbqNlhBwcNMpU-Q/TweetDetail", params)
}
