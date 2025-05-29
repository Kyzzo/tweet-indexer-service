# Twitter Tweets Rollback Package

## Overview
This package provides a robust solution for fetching and processing historical tweets from Twitter accounts. It implements:

- **Tweet collection** with pagination support  
- **Account rotation** for rate limit handling  
- **Concurrent processing pipeline**  
- **Date-based filtering** of tweets  

---

## Features
- 🚀 **Concurrent processing** with worker pools  
- ♻️ **Automatic account rotation** when rate limited  
- ⏳ **Historical tweet collection** with date filtering  
- 🔄 **Retry mechanism** for failed requests  
- 📊 **Detailed logging** throughout operations  

---

## Installation
```go
import "github.com/jeetdotso/tweet-indexer-service/rollback"
```

---

## Usage

### Basic Example
```go
// Initialize rollback client
trb, account, err := NewTweetsRollBack(ctx, 3, 2*time.Second, dummyAccountService)
if err != nil {
    // handle error
}
defer account.LogOut(ctx)

// Fetch tweets since specific date
tweets, err := trb.Tweets(ctx, "username", time.Now().Add(-24*time.Hour))
if err != nil {
    // handle error
}
```

### Pipeline Processing
```go
TweetsPipeline(
    ctx,
    users,          // map[string]time.Time of usernames and their cutoff dates
    3,              // concurrent fetchers
    10,             // concurrent handlers
    3,              // max retries
    2*time.Second,  // retry delay
    dummyAccountService,
    func(ctx context.Context, twt *twitterTypes.TweetInfo) {
        // process each tweet
    },
    syncFunction,   // optional sync function
)
```

---

## Key Components

### `TweetsRollBack` Struct
Main service object that handles tweet fetching operations.

#### Methods:
- **`Tweets()`** - Fetch tweets for a user since a specific date  
- **`fetchTweetsWithRetry()`** - Retry wrapper for API calls  
- **`SwitchAccounts()`** - Rotate to a new account when rate limited  
- **`processTweetEntries()`** - Parse and filter tweet entries  

### Pipeline Functions
- **`TweetsPipeline()`** - Concurrent processing pipeline  
  - Worker pools for fetching and handling tweets  

---

## Configuration

| Parameter       | Description                              |
|-----------------|------------------------------------------|
| `maxRetries`    | Maximum retry attempts for failed requests |
| `retryDelay`    | Delay between retry attempts             |
| `fetcherCount`  | Number of concurrent fetchers            |
| `handlerCount`  | Number of concurrent handlers            |

---

## Response Format
Returns a slice of `twitterTypes.TweetInfo` containing:

- **Tweet ID** and content  
- **Creation timestamp**  
- **Author information**  
- **Media attachments**  

---

## Error Handling
- Automatic retries with backoff  
- Account rotation on failures  
- Detailed error logging  
- Context cancellation support  

---

## Dependencies
- **Twitter API client**  
- **Dummy account service**  
- **Logging package (`logg`)**  
- `github.com/tidwall/gjson` for JSON parsing  

---

## Best Practices

### Rate Limiting
- Monitor API responses  
- Adjust concurrency based on rate limits  

### Resource Management
- Always call `LogOut` on accounts  
- Use context for cancellation  

### Error Handling
- Implement robust error handling  
- Monitor logs for retry attempts  