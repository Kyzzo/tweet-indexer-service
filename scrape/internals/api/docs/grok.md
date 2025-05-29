# Grok API Client Package

## Overview
This package provides a Go client for interacting with Twitter's Grok AI API, enabling:

- **Conversation management** with Grok AI
- **Message sending** and response handling
- **Proxy support** for distributed requests
- **Transaction ID generation** for API requests

---

## Features
- 🚀 **Conversation Management** - Create and manage Grok conversations
- 💬 **Message Exchange** - Send messages and receive AI responses
- 🔄 **Proxy Support** - Route requests through different proxies
- 🔐 **Authentication** - Handles auth tokens and cookies
- 📊 **Response Parsing** - Clean and raw response formats

---

## Installation
```go
import "github.com/jeetdotso/tweet-indexer-service/scrape/internals/api/grok"
```

---

## Initialization

### Basic Initialization
```go
client := NewGrokClient(authToken, ct0Token, baseURL)
```

### From Cookies
```go
client, err := CreateGrokClientWithCookies(cookies, baseURL)
if err != nil {
    // handle error
}
```

---

## Key Methods

### Conversation Management
```go
// Create new conversation
convID, err := client.CreateConversation(proxy)

// Get conversation history
history, err := client.GetConversationHistory(convID, proxy)
```

### Message Exchange
```go
// Send message and get response
response, raw, err := client.SendMessage(
    "Hello Grok", 
    convID, 
    "grok-1", 
    proxy, 
    false
)
```

### Utilities
```go
// Generate transaction ID
txnID, err := client.GenerateTransactionID("POST", "/path", proxy)
```

---

## Usage Examples

### Basic Message Exchange
```go
client := NewGrokClient("auth_token", "ct0_token", "https://api.example.com")

// Send message directly (creates conversation automatically)
response, _, err := client.SendMessage(
    "What's the weather today?", 
    "",  // empty conversationID creates new one
    "grok-1", 
    "",  // no proxy
    false,
)
```

### With Proxy
```go
response, _, err := client.SendMessage(
    "Analyze this tweet", 
    "conv_123", 
    "grok-3", 
    "http://proxy:port", 
    true, // get raw response
)
```

---

## Response Handling
Responses come in two formats:

- **Clean Response** - Processed answer from Grok
- **Raw Response** - Complete API response JSON

```go
clean, raw, err := client.SendMessage(...)
if err != nil {
    // handle error
}

// Use gjson to parse raw response if needed
result := gjson.Parse(raw)
```

---

## Error Handling
All methods return:

- Expected response data
- Error object (nil on success)

### Common Errors:
- Missing authentication tokens
- API request failures
- Invalid response formats

---

## Configuration

### Base URL
```go
client.SetBaseURL("https://new-api-url.com")
```

### HTTP Client
```go
client.HTTPClient = &http.Client{
    Timeout: 30 * time.Second,
}
```

---

## Best Practices
- **Reuse Conversations** - Maintain conversation IDs for context
- **Proxy Rotation** - Distribute load across multiple proxies
- **Error Checking** - Always verify success flag in responses
- **Rate Limiting** - Implement delays between requests

---

## Dependencies
### Standard Library Packages:
- `net/http`
- `encoding/json`

### External:
- `github.com/tidwall/gjson` for JSON parsing