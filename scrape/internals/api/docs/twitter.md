# Twitter API Twitter Package

## Overview
This package provides a comprehensive client for interacting with Twitter's API, including endpoints for:

- **User profile information**
- **Following/follower relationships**
- **Tweet retrieval and posting**
- **GraphQL API interactions**

## Features
- 📡 **Complete API coverage** - User profiles, tweets, relationships
- 🔐 **Authentication handling** - Cookie and token management
- 🔄 **Retry logic** - Built-in error handling and retries
- 📊 **Response parsing** - Automatic JSON parsing with `gjson`
- 🛡 **Request security** - CSRF protection and proper headers

## Installation
```go
import "github.com/jeetdotso/tweet-indexer-service/scrape/internals/api/twitter"
```

## Key Methods

### User Operations
```go
// Get user ID by username
UserID(username string) (string, error)

// Get complete user profile
FullUser(username string) (*gjson.Result, error)
```

### Relationship Operations
```go
// Get user's following list
Following(userID string, cursor string) (*gjson.Result, error)

// Get user's followers
Followers(userID string, cursor string) (*gjson.Result, error)
```

### Tweet Operations
```go
// Get tweets from user account
AccountTweets(userID string, cursor string) (*gjson.Result, error)

// Get specific tweet details 
GetTweet(tweetID, username string) (*gjson.Result, error)

// Post a retweet
Retweet(tweetID string) (bool, error)
```

## Usage Example

### Basic User Lookup
```go
client := NewTwitterClient(cookies, baseURL)

// Get user ID
userID, err := client.UserID("twitterhandle")
if err != nil {
    // handle error
}

// Get full profile
profile, err := client.FullUser("twitterhandle")
```

### Paginated Following List
```go
cursor := ""
for {
    result, err := client.Following(userID, cursor)
    if err != nil {
        // handle error
    }
    
    // Process followers
    followers := result.Get("data.user.result.timeline.timeline.instructions").Array()
    
    // Get next page cursor
    cursor = result.Get("data.user.result.timeline.timeline.instructions.0.entries.-1.content.value").String()
    if cursor == "" {
        break
    }
}
```

## Request Configuration
The client handles all necessary headers and parameters including:

- **Authentication tokens**
- **CSRF protection**
- **GraphQL variables and features**
- **Proper content encoding**

## Error Handling
Methods return:

- `*gjson.Result` for successful responses
- `error` for failed requests with details

## Dependencies
- `github.com/tidwall/gjson` - JSON parsing
- `github.com/bogdanfinn/fhttp` - HTTP client
- **Internal utility packages**

## Best Practices

### Rate Limiting
- Implement delays between batch operations
- Monitor API responses for rate limit headers

### Error Recovery
- Check for expired sessions
- Implement retry logic for transient failures

### Resource Management
- Reuse client instances when possible
- Properly handle response bodies
