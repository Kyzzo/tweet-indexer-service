package tracking

import (
	"context"
	"fmt"
	"time"

	"github.com/jeetdotso/tweet-indexer-service/logg"
	"github.com/tidwall/gjson"
)

func fetchWithRetry(ctx context.Context, fetchMethod func(string, string) (*gjson.Result, error), userID string, username string, cursor string, maxRetries int, retryDelay time.Duration, dataLabel string) (*gjson.Result, error) {
	for attempt := 1; attempt <= maxRetries; attempt++ {
		response, err := fetchMethod(userID, cursor)
		if err == nil {
			return response, nil
		}
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg(fmt.Sprintf("username: [%s] Attempt [%d/%d for %s failed", username, attempt, maxRetries, dataLabel))
		if attempt < maxRetries {
			logg.Log.Warn().Caller().Ctx(ctx).Msg(fmt.Sprintf("username: [%s] Retrying in %v seconds...", username, retryDelay.Seconds()))

			time.Sleep(retryDelay)
		} else {
			logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg(fmt.Sprintf("username: [%s] Max retries reached for %s. Aborting request.", username, dataLabel))
			return nil, err
		}
	}
	return nil, fmt.Errorf("max retries reached for %s", dataLabel)
}

func ScrapePaginatedData(ctx context.Context, username string, userID string, fetchMethod func(string, string) (*gjson.Result, error), dataLabel string, maxRetries int, retryDelay time.Duration) []map[string]interface{} {
	logg.Log.Info().Caller().Ctx(ctx).Msg(fmt.Sprintf("username: [%s] Starting to scrape %s...", username, dataLabel))
	results := []map[string]interface{}{}
	cursor := ""
	batchNumber := 1

	for {
		response, err := fetchWithRetry(ctx, fetchMethod, userID, username, cursor, maxRetries, retryDelay, dataLabel)
		if err != nil {
			logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg(fmt.Sprintf("username: [%s] Failed to fetch batch #%d of %s", username, batchNumber, dataLabel))
			break
		}

		instructions := response.Get("data.user.result.timeline.timeline.instructions").Array()
		if len(instructions) == 0 {
			logg.Log.Error().Caller().Ctx(ctx).Msg(fmt.Sprintf("username: [%s] No instructions found in the response. Ending pagination for %s.", username, dataLabel))
			break
		}

		entries := instructions[len(instructions)-1].Get("entries").Array()
		if len(entries) < 2 {
			logg.Log.Error().Caller().Ctx(ctx).Msg(fmt.Sprintf("username: [%s] Not enough entries found in batch #%d; ending pagination for %s.", username, batchNumber, dataLabel))
			break
		}
		for _, entry := range entries[:len(entries)-2] {
			item := entry.Get("content.itemContent.user_results.result").Value().(map[string]interface{})
			if item != nil {
				results = append(results, item)
			}
		}

		newCursor := entries[len(entries)-2].Get("content.value").String()
		logg.Log.Debug().Caller().Ctx(ctx).Msg(fmt.Sprintf("username: [%s] Scraped %d %s from batch #%d", username, len(results), dataLabel, batchNumber))

		if newCursor == "" || newCursor[0:2] == "0|" {
			logg.Log.Info().Caller().Ctx(ctx).Msg(fmt.Sprintf("username: [%s] Reached the end of the %s list. Scraping complete.", username, dataLabel))
			break
		}

		cursor = newCursor
		batchNumber++
	}

	return results
}
