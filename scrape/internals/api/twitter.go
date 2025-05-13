package twitterAPI

import (
	"bytes"
	"compress/gzip"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	hh "net/http"
	"net/url"
	"strings"

	realhttp "net/http"

	http "github.com/bogdanfinn/fhttp"
	utils "github.com/jeetdotso/tweet-indexer-service/utils"
	"github.com/tidwall/gjson"
)

func (tc *TwitterClient) commonRequest(endpoint string, params url.Values) (*gjson.Result, error) {
	fullURL := fmt.Sprintf("%s%s?%s", tc.BaseURL, endpoint, params.Encode())
	bodyBytes, statusCode, err := tc.Get(fullURL, nil)
	if bodyBytes == nil {
		return nil, fmt.Errorf("cannot do the request right now")
	}
	r := gjson.ParseBytes(*bodyBytes)
	if err != nil {
		return nil, fmt.Errorf("cannot do the request right now")
	} else if statusCode != 200 {
		return &r, fmt.Errorf("bad request")
	}
	return &r, nil
}

func (tc *TwitterClient) commonPostRequestJSON(endpoint string, payload interface{}, header *http.Header) (*gjson.Result, error) {
	var fullURL string
	if strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://") {
		fullURL = endpoint
	} else if strings.HasPrefix(endpoint, "/") {
		fullURL = fmt.Sprintf("%s%s", tc.BaseURL, endpoint)
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON payload: %w", err)
	}

	// Create reader for the POST body
	bodyReader := bytes.NewReader(bodyBytes)
	var h *http.Header
	// If header is provided, use it; otherwise, use nil to let default headers be used
	if header != nil {
		h = header
	} else {
		h = &http.Header{
			"Content-Type": {"application/json"}, // Set default content-type if not provided
		}
	}
	// Let default headers be used (tc.Headers already has correct content-type, etc.)
	respBody, statusCode, err := tc.Post(fullURL, bodyReader, h)
	if err != nil {
		return nil, fmt.Errorf("POST request failed: %w", err)
	}
	result := gjson.ParseBytes(*respBody)
	if statusCode != 200 {
		return &result, fmt.Errorf("non-200 response: %d", statusCode)
	}
	return &result, nil
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
func (tc *TwitterClient) FullUser(username string) (*gjson.Result, error) {
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
		return nil, err
	}

	return result, nil
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
		"creator_subscriptions_quote_tweet_preview_enabled":                       false,
		"freedom_of_speech_not_reach_fetch_enabled":                               true,
		"standardized_nudges_misinfo":                                             true,
		"tweet_with_visibility_results_prefer_gql_limited_actions_policy_enabled": true,
		"rweb_video_timestamps_enabled":                                           true,
		"longform_notetweets_rich_text_read_enabled":                              true,
		"longform_notetweets_inline_media_enabled":                                true,
		"responsive_web_grok_image_annotation_enabled":                            true,
		"responsive_web_enhance_cards_enabled":                                    false,
	})
	fieldToggles, _ := json.Marshal(map[string]bool{"withArticlePlainText": false})
	params := url.Values{}
	params.Add("variables", string(variablesJSON))
	params.Add("features", string(features))
	params.Add("fieldToggles", string(fieldToggles))

	return tc.commonRequest("/i/api/graphql/MpOINUGH_YVb2BKjYZOPaQ/UserTweets", params)
}

func (tc *TwitterClient) GetTweet(tweetID, username string) (*gjson.Result, error) {
	// vars := map[string]interface{}{
	// 	"focalTweetId":                           tweetID,
	// 	"rankingMode":                            "Relevance",
	// 	"includePromotedContent":                 true,
	// 	"with_rux_injections":                    false,
	// 	"referrer":                               "profile",
	// 	"controller_data":                        "DAACDAABDAABCgABAAAAAAAAAAAKAAkTJQ1goZZABwAAAAA=",
	// 	"withCommunity":                          true,
	// 	"withQuickPromoteEligibilityTweetFields": true,
	// 	"withBirdwatchNotes":                     true,
	// 	"withVoice":                              true,
	// }
	// feats := map[string]interface{}{
	// 	"rweb_video_screen_enabled":                                               false,
	// 	"profile_label_improvements_pcf_label_in_post_enabled":                    true,
	// 	"rweb_tipjar_consumption_enabled":                                         true,
	// 	"verified_phone_label_enabled":                                            false,
	// 	"creator_subscriptions_tweet_preview_api_enabled":                         true,
	// 	"responsive_web_graphql_timeline_navigation_enabled":                      true,
	// 	"responsive_web_graphql_skip_user_profile_image_extensions_enabled":       false,
	// 	"premium_content_api_read_enabled":                                        false,
	// 	"communities_web_enable_tweet_community_results_fetch":                    true,
	// 	"c9s_tweet_anatomy_moderator_badge_enabled":                               true,
	// 	"responsive_web_grok_analyze_button_fetch_trends_enabled":                 false,
	// 	"responsive_web_grok_analyze_post_followups_enabled":                      true,
	// 	"responsive_web_jetfuel_frame":                                            false,
	// 	"responsive_web_grok_share_attachment_enabled":                            true,
	// 	"articles_preview_enabled":                                                true,
	// 	"responsive_web_edit_tweet_api_enabled":                                   true,
	// 	"graphql_is_translatable_rweb_tweet_is_translatable_enabled":              true,
	// 	"view_counts_everywhere_api_enabled":                                      true,
	// 	"longform_notetweets_consumption_enabled":                                 true,
	// 	"responsive_web_twitter_article_tweet_consumption_enabled":                true,
	// 	"tweet_awards_web_tipping_enabled":                                        false,
	// 	"responsive_web_grok_show_grok_translated_post":                           false,
	// 	"responsive_web_grok_analysis_button_from_backend":                        true,
	// 	"rweb_video_timestamps_enabled":                                           true,
	// 	"standardized_nudges_misinfo":                                             true,
	// 	"tweet_with_visibility_results_prefer_gql_limited_actions_policy_enabled": true,
	// 	"longform_notetweets_rich_text_read_enabled":                              true,
	// 	"longform_notetweets_inline_media_enabled":                                true,
	// 	"responsive_web_graphql_exclude_directive_enabled":                        true,
	// 	"responsive_web_grok_image_annotation_enabled":                            true, // browser had true
	// 	"responsive_web_enhance_cards_enabled":                                    false,
	// }
	// toggles := map[string]bool{
	// 	"withArticleRichContentState": true,
	// 	"withArticlePlainText":        false,
	// 	"withGrokAnalyze":             false,
	// 	"withDisallowedReplyControls": false,
	// }
	variables := "variables=%7B%22focalTweetId%22%3A%22REPLACEME%22%2C%22with_rux_injections%22%3Afalse%2C%22rankingMode%22%3A%22Relevance%22%2C%22includePromotedContent%22%3Atrue%2C%22withCommunity%22%3Atrue%2C%22withQuickPromoteEligibilityTweetFields%22%3Atrue%2C%22withBirdwatchNotes%22%3Atrue%2C%22withVoice%22%3Atrue%7D&features=%7B%22rweb_video_screen_enabled%22%3Afalse%2C%22profile_label_improvements_pcf_label_in_post_enabled%22%3Atrue%2C%22rweb_tipjar_consumption_enabled%22%3Atrue%2C%22verified_phone_label_enabled%22%3Afalse%2C%22creator_subscriptions_tweet_preview_api_enabled%22%3Atrue%2C%22responsive_web_graphql_timeline_navigation_enabled%22%3Atrue%2C%22responsive_web_graphql_skip_user_profile_image_extensions_enabled%22%3Afalse%2C%22premium_content_api_read_enabled%22%3Afalse%2C%22communities_web_enable_tweet_community_results_fetch%22%3Atrue%2C%22c9s_tweet_anatomy_moderator_badge_enabled%22%3Atrue%2C%22responsive_web_grok_analyze_button_fetch_trends_enabled%22%3Afalse%2C%22responsive_web_grok_analyze_post_followups_enabled%22%3Atrue%2C%22responsive_web_jetfuel_frame%22%3Afalse%2C%22responsive_web_grok_share_attachment_enabled%22%3Atrue%2C%22articles_preview_enabled%22%3Atrue%2C%22responsive_web_edit_tweet_api_enabled%22%3Atrue%2C%22graphql_is_translatable_rweb_tweet_is_translatable_enabled%22%3Atrue%2C%22view_counts_everywhere_api_enabled%22%3Atrue%2C%22longform_notetweets_consumption_enabled%22%3Atrue%2C%22responsive_web_twitter_article_tweet_consumption_enabled%22%3Atrue%2C%22tweet_awards_web_tipping_enabled%22%3Afalse%2C%22responsive_web_grok_show_grok_translated_post%22%3Afalse%2C%22responsive_web_grok_analysis_button_from_backend%22%3Atrue%2C%22creator_subscriptions_quote_tweet_preview_enabled%22%3Afalse%2C%22freedom_of_speech_not_reach_fetch_enabled%22%3Atrue%2C%22standardized_nudges_misinfo%22%3Atrue%2C%22tweet_with_visibility_results_prefer_gql_limited_actions_policy_enabled%22%3Atrue%2C%22longform_notetweets_rich_text_read_enabled%22%3Atrue%2C%22longform_notetweets_inline_media_enabled%22%3Atrue%2C%22responsive_web_grok_image_annotation_enabled%22%3Atrue%2C%22responsive_web_enhance_cards_enabled%22%3Afalse%7D&fieldToggles=%7B%22withArticleRichContentState%22%3Atrue%2C%22withArticlePlainText%22%3Afalse%2C%22withGrokAnalyze%22%3Afalse%2C%22withDisallowedReplyControls%22%3Afalse%7D"
	// 2) Build the URL
	variables = strings.Replace(variables, "REPLACEME", tweetID, 1)
	// — 2) Create the GET request
	endpoint := "https://x.com/i/api/graphql/_8aYOgEDz35BrBcBal1-_w/TweetDetail"
	fullURL := endpoint +
		"?" + variables

	req, err := realhttp.NewRequest("GET", fullURL, nil)
	if err != nil {
		return nil, err
	}

	// — 3) Set only the essential headers
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br, zstd")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("Priority", "u=1, i")
	req.Header.Set("Referer", fmt.Sprintf("https://x.com/%s/status/%s", username, tweetID))
	req.Header.Set("Sec-Ch-Ua", `"Google Chrome";v="135", "Not-A.Brand";v="8", "Chromium";v="135"`)
	req.Header.Set("Sec-Ch-Ua-Mobile", "?1")
	req.Header.Set("Sec-Ch-Ua-Platform", `"Android"`)
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-origin")

	req.Header.Set("User-Agent", "Mozilla/5.0 (Linux; Android 6.0; Nexus 5 Build/MRA58N) "+
		"AppleWebKit/537.36 (KHTML, like Gecko) "+
		"Chrome/135.0.0.0 Mobile Safari/537.36")
	// csurf from your ct0 cookie:
	csrf := utils.GetCookieByName(&tc.Cookies, "ct0")
	req.Header.Set("X-Csrf-Token", *csrf)
	req.Header.Set("X-Twitter-Active-User", "yes")
	req.Header.Set("X-Twitter-Auth-Type", "OAuth2Session")
	req.Header.Set("X-Twitter-Client-Language", "en")
	req.Header.Set("authorization", "Bearer AAAAAAAAAAAAAAAAAAAAANRILgAAAAAAnNwIzUejRCOuH5E6I8xnZz4puTs%3D1Zv7ttfk8LF81IUq16cHjhLTvJu4FA33AGWWjCpTnA")
	req.Header.Set("Origin", "https://x.com")
	req.Header.Set("X-Client-Transaction-Id", "DTXZQ0WokMldDdJDg0uAKy75mCK9a13iOi6gczcMdlZtJxND0JIvhPZmPUXtAyJqjUGGzQ4L4y2rQdu60fGQP49QsBWsDg")
	// req.Header.Set("X-Client-UUID", "<copy-your-chrome-value-here>")
	txn, err := NewClientTxID(80) // pick a length around 64–80 chars
	if err != nil {
		return nil, err
	}
	// lPwdj4fWw3rwyceV4FN5AU3bnyI2lv5Ayfek61TOrYNtpJAUuqs742qRSUdMdApu
	// DTXZQ0WokMldDdJDg0uAKy75mCK9a13iOi6gczcMdlZtJxND0JIvhPZmPUXtAyJqjUGGzQ4L4y2rQdu60fGQP49QsBWsDg
	// NecpZ+T2BXqAt9o4/32Um9T8M/W4OSkkflI7f13IgN4u500lc5mwSC8pedvENNuPUfiW9TaOTIRQopx25dzawSTmFt8JNg
	fmt.Println("txn:", txn, " vs ", "DTXZQ0WokMldDdJDg0uAKy75mCK9a13iOi6gczcMdlZtJxND0JIvhPZmPUXtAyJqjUGGzQ4L4y2rQdu60fGQP49QsBWsDg")
	// req.Header.Set("X-Client-Transaction-Id", txn)
	var cookiePairs []string
	for _, c := range tc.Cookies {
		// Only include name=value; skip attributes like Path, Domain, etc.
		pair := fmt.Sprintf("%s=%s", c.Name, c.Value)
		cookiePairs = append(cookiePairs, pair)
	}
	req.Header.Set("Cookie", strings.Join(cookiePairs, "; "))
	// OPTIONAL: dump it so you can paste into curl or browser

	httpClient := &realhttp.Client{
		Transport: &DumpTransport{Base: realhttp.DefaultTransport},
		CheckRedirect: func(req *realhttp.Request, via []*realhttp.Request) error {
			// Handle redirects if needed
			return nil
		},
	}
	// — 4) Send & check status
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	reader := resp.Body
	if resp.Header.Get("Content-Encoding") == "gzip" {
		gz, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("gzip.NewReader failed: %w", err)
		}
		defer gz.Close()
		reader = gz
	}

	// read *once*
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("reading body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		fmt.Println("Response body:", resp)
		return nil, fmt.Errorf("GetTweet %d: %s", resp.StatusCode, data)
	}

	// parse from the same bytes
	result := gjson.ParseBytes(data)
	return &result, nil
}

type grokRequest struct {
	ConversationId       string            `json:"conversationId"`
	EnableSideBySide     bool              `json:"enableSideBySide"`
	GrokModelOptionId    string            `json:"grokModelOptionId"`
	ImageGenerationCount int               `json:"imageGenerationCount"`
	IsDeepsearch         bool              `json:"isDeepsearch"`
	IsReasoning          bool              `json:"isReasoning"`
	PromptMetadata       map[string]string `json:"promptMetadata"`
	RequestFeatures      map[string]bool   `json:"requestFeatures"`
	Responses            []struct {
		FileAttachments []interface{} `json:"fileAttachments"`
		Message         string        `json:"message"`
		PromptSource    string        `json:"promptSource"`
		Sender          int           `json:"sender"`
	} `json:"responses"`
	ReturnCitations     bool           `json:"returnCitations"`
	ReturnSearchResults bool           `json:"returnSearchResults"`
	SystemPromptName    string         `json:"systemPromptName"`
	ToolOverrides       map[string]any `json:"toolOverrides"`
}
type grokRequest2 struct {
	ConversationId       string                 `json:"conversationId"`
	EnableSideBySide     bool                   `json:"enableSideBySide"`
	GrokModelOptionId    string                 `json:"grokModelOptionId"`
	ImageGenerationCount int                    `json:"imageGenerationCount"`
	PromptMetadata       map[string]interface{} `json:"promptMetadata"`
	RequestFeatures      map[string]bool        `json:"requestFeatures"`
	Responses            []struct {
		FileAttachments []interface{} `json:"fileAttachments"`
		Message         string        `json:"message"`
		PromptSource    string        `json:"promptSource"`
		Sender          int           `json:"sender"`
	} `json:"responses"`
	ReturnCitations     bool                   `json:"returnCitations"`
	ReturnSearchResults bool                   `json:"returnSearchResults"`
	SystemPromptName    string                 `json:"systemPromptName"`
	ToolOverrides       map[string]interface{} `json:"toolOverrides"`
}

func (tc *TwitterClient) SendGrokPrompt(conversationId string, url, respondedMsg, message string) (*gjson.Result, error) {
	fullURL := "https://grok.x.com/2/grok/add_response.json"

	payload := grokRequest{
		ConversationId:       conversationId,
		EnableSideBySide:     false,
		GrokModelOptionId:    "grok-3",
		ImageGenerationCount: 4,
		IsDeepsearch:         false,
		IsReasoning:          false,
		PromptMetadata: map[string]string{
			"action":       "INPUT",
			"promptSource": "NATURAL",
		},
		RequestFeatures: map[string]bool{
			"eagerTweets":   true,
			"serverHistory": true,
		},
		Responses: []struct {
			FileAttachments []interface{} `json:"fileAttachments"`
			Message         string        `json:"message"`
			PromptSource    string        `json:"promptSource"`
			Sender          int           `json:"sender"`
		}{
			{
				FileAttachments: nil,
				Message:         url,
				Sender:          1,
			},
			{
				FileAttachments: nil,
				Message:         respondedMsg,
				Sender:          2,
			},
			{
				FileAttachments: nil,
				Message:         message,
				Sender:          1,
			}},

		ReturnCitations:     true,
		ReturnSearchResults: true,
		SystemPromptName:    "",
		ToolOverrides:       map[string]any{},
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal grok payload: %w", err)
	}

	req, _ := hh.NewRequest(http.MethodPost, "https://grok.x.com/2/grok/add_response.json", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "text/plain;charset=UTF-8")
	// … set all your other headers …

	// dump if you still want to inspect:
	// if dump, err := httputil.DumpRequestOut(req, true); err == nil {
	// 	log.Debug().Msgf("Grok request:\n%s", dump)
	// }

	// fmt.Printf("Sending raw JSON: %s\n", string(bodyBytes))

	respBody, statusCode, err := tc.Post2(fullURL, bytes.NewReader(bodyBytes), nil)
	if err != nil {
		return nil, fmt.Errorf("POST request failed: %w", err)
	}
	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d: %s", statusCode, string(respBody))
	}

	// success but no body
	if len(respBody) == 0 {
		return nil, nil
	}

	// parse and return real JSON if there ever is any
	r := gjson.ParseBytes(respBody)
	return &r, nil
}
func (tc *TwitterClient) SendGrokPromptDeleted(conversationId, message string) (*gjson.Result, error) {
	fullURL := "https://grok.x.com/2/grok/add_response.json"

	payload := grokRequest{
		ConversationId:       conversationId,
		EnableSideBySide:     false,
		GrokModelOptionId:    "grok-3",
		ImageGenerationCount: 4,
		IsDeepsearch:         false,
		IsReasoning:          false,
		PromptMetadata: map[string]string{
			"action":       "INPUT",
			"promptSource": "NATURAL",
		},
		RequestFeatures: map[string]bool{
			"eagerTweets":   true,
			"serverHistory": true,
		},
		Responses: []struct {
			FileAttachments []interface{} `json:"fileAttachments"`
			Message         string        `json:"message"`
			PromptSource    string        `json:"promptSource"`
			Sender          int           `json:"sender"`
		}{
			{
				FileAttachments: nil,
				Message:         message,
				Sender:          1,
			}},

		ReturnCitations:     true,
		ReturnSearchResults: true,
		SystemPromptName:    "",
		ToolOverrides:       map[string]any{},
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal grok payload: %w", err)
	}

	req, _ := hh.NewRequest(http.MethodPost, "https://grok.x.com/2/grok/add_response.json", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "text/plain;charset=UTF-8")
	// … set all your other headers …

	// dump if you still want to inspect:
	// if dump, err := httputil.DumpRequestOut(req, true); err == nil {
	// 	log.Debug().Msgf("Grok request:\n%s", dump)
	// }

	// fmt.Printf("Sending raw JSON: %s\n", string(bodyBytes))

	respBody, statusCode, err := tc.Post2(fullURL, bytes.NewReader(bodyBytes), nil)
	if err != nil {
		return nil, fmt.Errorf("POST request failed: %w", err)
	}
	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d: %s", statusCode, string(respBody))
	}

	// success but no body
	if len(respBody) == 0 {
		return nil, nil
	}

	// parse and return real JSON if there ever is any
	r := gjson.ParseBytes(respBody)
	return &r, nil
}
func (tc *TwitterClient) SendGrokPromptInitial(conversationId string, url string) (*gjson.Result, error) {
	fullURL := "https://grok.x.com/2/grok/add_response.json"

	payload := grokRequest2{
		ConversationId:       conversationId,
		EnableSideBySide:     false,
		GrokModelOptionId:    "grok-3",
		ImageGenerationCount: 4,
		PromptMetadata: map[string]interface{}{
			"action":       "INPUT",
			"promptSource": "GROK_ANALYZE",
			"properties": map[string]interface{}{
				"element": "BUTTON",
			},
		},
		RequestFeatures: map[string]bool{
			"eagerTweets":   true,
			"serverHistory": true,
		},
		Responses: []struct {
			FileAttachments []interface{} `json:"fileAttachments"`
			Message         string        `json:"message"`
			PromptSource    string        `json:"promptSource"`
			Sender          int           `json:"sender"`
		}{{
			FileAttachments: nil,
			Message:         url,
			PromptSource:    "post_analysis_details_page",
			Sender:          1,
		}},
		ReturnCitations:     true,
		ReturnSearchResults: true,
		SystemPromptName:    "",
		ToolOverrides:       map[string]any{},
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal grok payload: %w", err)
	}

	req, _ := hh.NewRequest(http.MethodPost, "https://grok.x.com/2/grok/add_response.json", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "text/plain;charset=UTF-8")
	// … set all your other headers …

	// dump if you still want to inspect:
	// if dump, err := httputil.DumpRequestOut(req, true); err == nil {
	// 	log.Debug().Msgf("Grok request:\n%s", dump)
	// }

	// fmt.Printf("Sending raw JSON: %s\n", string(bodyBytes))

	respBody, statusCode, err := tc.Post2(fullURL, bytes.NewReader(bodyBytes), nil)
	if err != nil {
		return nil, fmt.Errorf("POST request failed: %w", err)
	}
	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d: %s", statusCode, string(respBody))
	}

	// success but no body
	if len(respBody) == 0 {
		return nil, nil
	}

	// parse and return real JSON if there ever is any
	r := gjson.ParseBytes(respBody)
	return &r, nil
}

func (tc *TwitterClient) GetGrokConversationResponse(conversationID, _ string) (*gjson.Result, error) {
	endpoint := "/i/api/graphql/y1Lkc52sobXGzJ8ADjNEIw/GrokConversationItemsByRestId"

	variables := map[string]string{
		"restId": conversationID,
	}
	features := map[string]interface{}{
		"creator_subscriptions_tweet_preview_api_enabled":                         true,
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
		"responsive_web_grok_show_grok_translated_post":                           false,
		"responsive_web_grok_analysis_button_from_backend":                        true,
		"creator_subscriptions_quote_tweet_preview_enabled":                       false,
		"freedom_of_speech_not_reach_fetch_enabled":                               true,
		"standardized_nudges_misinfo":                                             true,
		"tweet_with_visibility_results_prefer_gql_limited_actions_policy_enabled": true,
		"longform_notetweets_rich_text_read_enabled":                              true,
		"longform_notetweets_inline_media_enabled":                                true,
		"profile_label_improvements_pcf_label_in_post_enabled":                    true,
		"rweb_tipjar_consumption_enabled":                                         true,
		"responsive_web_graphql_exclude_directive_enabled":                        true,
		"verified_phone_label_enabled":                                            false,
		"responsive_web_grok_image_annotation_enabled":                            true,
		"responsive_web_graphql_skip_user_profile_image_extensions_enabled":       false,
		"responsive_web_graphql_timeline_navigation_enabled":                      true,
		"responsive_web_enhance_cards_enabled":                                    false,
	}

	// Encode to JSON for URL
	variablesJSON, _ := json.Marshal(variables)
	featuresJSON, _ := json.Marshal(features)

	fieldToggles, _ := json.Marshal(map[string]bool{"withArticlePlainText": false})
	params := url.Values{}
	params.Add("variables", string(variablesJSON))
	params.Add("features", string(featuresJSON))
	params.Add("fieldToggles", string(fieldToggles))

	return tc.commonRequest(endpoint, params)
}

func (tc *TwitterClient) CreateGrokConversation() (string, error) {
	payload := map[string]interface{}{
		"variables": map[string]interface{}{},
		"queryId":   "vvC5uy7pWWHXS2aDi1FZeA", // This can be dynamic if needed later
	}

	result, err := tc.commonPostRequestJSON("/i/api/graphql/vvC5uy7pWWHXS2aDi1FZeA/CreateGrokConversation", payload, nil)
	if err != nil {
		return "", err
	}

	return result.Get("data.create_grok_conversation.conversation_id").String(), nil
}

func (tc *TwitterClient) SendGrokPreflight(url string) error {
	req, err := http.NewRequest("OPTIONS", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create OPTIONS request: %w", err)
	}

	req.Host = "grok.x.com"
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Authority", "grok.x.com")
	req.Header.Set("Origin", "https://x.com")
	req.Header.Set("Referer", "https://x.com/")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/135.0.0.0 Safari/537.36")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "authorization,x-client-transaction-id,x-csrf-token,x-twitter-active-user,x-twitter-auth-type,x-xai-request-id")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Sec-Fetch-Site", "same-site")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("method", "OPTIONS")
	req.Header.Set("scheme", "https")
	req.Header.Set("Priority", "u=1, i")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br, zstd")

	resp, err := (*tc.Client).Do(req)
	if err != nil {
		return fmt.Errorf("OPTIONS request failed: %w", err)
	}
	defer resp.Body.Close()

	return nil
}

// Retweet posts a retweet of the given tweetID and returns true if it worked.
func (tc *TwitterClient) Retweet(tweetID string) (bool, error) {
	// build the GraphQL payload
	payload := map[string]interface{}{
		"queryId": "ojPdsZsimiJrUGLR1sjUtA",
		"variables": map[string]interface{}{
			"tweet_id":     tweetID,
			"dark_request": false,
		},
	}
	headers := tc.Headers.Clone()
	headers.Set("X-Twitter-Client-Language", "en")
	headers.Set("referer", fmt.Sprintf("https://x.com/Must5/status/%s", tweetID))
	headers.Set("x-client-transaction-id", "eVmjwU/+yLo1IDH74Qgq/C7+hJnXBactjBKSJTNOocHVn6pMFA1K+g5DJzQf/fbJaCq2uXqc4o59FhVd/3ZA+ayVEdZneg")
	// fire the POST to /CreateRetweet
	res, err := tc.commonPostRequestJSON(
		"/i/api/graphql/ojPdsZsimiJrUGLR1sjUtA/CreateRetweet",
		payload,
		&headers, // no extra headers
	)
	if err != nil {
		return false, fmt.Errorf("failed to send CreateRetweet: %w", err)
	}

	// dig out the new retweet ID
	newID := res.Get("data.create_retweet.retweet_results.result.rest_id").String()
	if newID == "" {
		// something went wrong (maybe already retweeted, or auth)
		return false, fmt.Errorf("no rest_id in response: %s", res.Raw)
	}

	return true, nil
}

type DumpTransport struct {
	Base realhttp.RoundTripper
}

func (t *DumpTransport) RoundTrip(req *realhttp.Request) (*realhttp.Response, error) {
	// Dump the request for debugging
	// fmt.Printf("Request URL: %s\n", req.URL.String())
	// fmt.Printf("Request Method: %s\n", req.Method)
	// fmt.Printf("Request Headers:\n")
	for name, values := range req.Header {
		for _, value := range values {
			fmt.Printf("  %s: %s\n", name, value)
		}
	}
	fmt.Printf("Request Body:\n")
	return t.Base.RoundTrip(req)
}

const alphaNum = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"

func NewClientTxID(n int) (string, error) {
	b := make([]byte, n)
	for i := range b {
		idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphaNum))))
		if err != nil {
			return "", err
		}
		b[i] = alphaNum[idx.Int64()]
	}
	return string(b), nil
}
