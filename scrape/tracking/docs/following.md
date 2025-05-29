### Twitter Following Monitor Package ###

## Overview
This package provides a concurrent pipeline for monitoring Twitter following relationships. It offers two main operational modes:

1. **Single Page Processing**: Fetches and processes just the first page of followings for each account.
2. **Full Pagination Processing**: Fetches all pages of followings until completion for each account.

---

## Features
- Concurrent processing with configurable worker pools.
- Automatic retry mechanism for failed API calls.
- Proper resource cleanup (account logout).
- Context-aware cancellation.
- Detailed logging throughout the pipeline.
- Flexible handling of parsed following users.

---

## Usage

### Initialization
```go
monitor, account, err := NewFollowingMonitor(ctx, dummyAccountService)
if err != nil {
    // handle error
}
defer account.LogOut(ctx)
```

### Pipeline Modes

#### 1. Single Page Processing
```go
MonitorSingleFollowingPagePipeline(
    ctx,
    accountIDs,          // map[string]struct{} of account IDs
    fetcherCount,        // number of concurrent fetchers
    handlerCount,        // number of concurrent handlers
    maxRetries,          // max retry attempts
    retryDelay,          // delay between retries
    handleFollowingFunc, // your processing function
    dummyAccountService,
)
```

#### 2. Full Pagination Processing
```go
MonitorFollowingsPipeline(
    ctx,
    accountIDs,          // map[string]struct{} of account IDs
    fetcherCount,        // number of concurrent fetchers
    handlerCount,        // number of concurrent handlers
    maxRetries,          // max retry attempts
    retryDelay,          // delay between retries
    handleFollowingFunc, // your processing function
    dummyAccountService,
)
```

---

### Handler Function Signature
```go
func(ctx context.Context, sourceID string, user twitterTypes.FollowingUser) {
    // Process the following relationship
}
```

---

## Configuration

| Parameter       | Description                                      |
|------------------|--------------------------------------------------|
| `fetcherCount`   | Number of concurrent account processors (API callers). |
| `handlerCount`   | Number of concurrent following user processors. |
| `maxRetries`     | Maximum number of retry attempts for failed API calls. |
| `retryDelay`     | Delay between retry attempts.                   |
| `accountIDs`     | Set of account IDs to process (`map[string]struct{}`). |

---

## Internals

### Key Components
- **Worker Pools**:
  - **Fetchers**: Handle API calls and initial processing.
  - **Handlers**: Process the parsed following users.

- **Channel Architecture**:
  - `accountJobs`: Feed accounts to fetchers.
  - `followingJobs`: Pass parsed followings to handlers.

- **Error Handling**:
  - Automatic retries with backoff.
  - Detailed error logging.
  - Resource cleanup on failure.

- **Pagination**:
  - Automatic cursor handling.
  - End-of-list detection.

---

### Helper Functions
- `fetchWithRetries`: Handles API calls with retry logic.
- `processEntries`: Processes raw API responses into `FollowingUser` structs.
- `extractEntriesAndCursor`: Parses Twitter API response format.
- `cleanupAccount`: Ensures proper resource cleanup.

---

## Best Practices

### Concurrency Tuning
- Set `fetcherCount` based on API rate limits.
- Set `handlerCount` based on your processing requirements.

### Error Handling
- Implement robust error handling in your `handleFollowing` function.
- Monitor the logs for retry attempts.

### Resource Management
- Always use context for cancellation.
- Ensure your handler function doesn't block unnecessarily.

### Monitoring
- Track processing metrics in your handler.
- Log important milestones.

---

## Example
```go
accountIDs := map[string]struct{}{
    "12345": {},
    "67890": {},
}

handleFollowing := func(ctx context.Context, sourceID string, user twitterTypes.FollowingUser) {
    fmt.Printf("%s follows %s (@%s)\n", sourceID, user.UserName, user.Handle)
    // Add your custom logic here
}

MonitorSingleFollowingPagePipeline(
    context.Background(),
    accountIDs,
    3,              // 3 concurrent fetchers
    10,             // 10 concurrent handlers
    3,              // max retries
    2*time.Second,  // retry delay
    handleFollowing,
    dummyAccountService,
)
```

---

## Dependencies
- `github.com/tidwall/gjson` for JSON parsing.
- Custom Twitter API client (`twitterAPI` package).
- Dummy account service for authentication.
- Logging package (`logg`).