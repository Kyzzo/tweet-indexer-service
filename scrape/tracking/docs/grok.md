## Twitter Grok Monitoring Package

### Overview
This package provides functionality for analyzing Twitter tweets using Grok AI to detect cryptocurrency promotions. It handles:

- **Grok conversation management**
- **Proxy rotation for request distribution**
- **Response parsing and processing**
- **Error handling with retries**

---

### Features
- **Proxy Rotation**: Uses round-robin proxy distribution for reliable requests.
- **Grok Integration**: Creates and manages Grok conversations for tweet analysis.
- **Cryptocurrency Detection**: Specialized prompts to identify crypto promotions.
- **Retry Mechanism**: Automatic retries for failed requests.
- **Structured Responses**: Parses Grok responses into a standardized format.

---

### Installation
```go
import "github.com/jeetdotso/tweet-indexer-service/tracking"
```

---

### Usage

#### Basic Example
```go
tweet := &model.TweetEvent{
    ID:       "12345",
    TweetID:  "12345",
    UserID:   "67890",
    UserName: "cryptouser",
    Text:     "Check out this new token!",
}

handleResponse := func(ctx context.Context, grokEvent *model.GrokEvent) {
    if grokEvent.IsShill {
        fmt.Printf("Detected crypto promotion: %v\n", grokEvent.TokensShilled)
    }
}

GetGrokCheck(context.Background(), tweet, dummyAccountService, "https://twitter.com", handleResponse)
```

---

### Key Components
- **GrokMonitor**: Main service object that handles Grok interactions.
- **NewGrokMonitor()**: Creates a new monitor instance.
- **GetResponse()**: Processes a single tweet.
- **ProcessResponse()**: Handles response parsing.
- **Proxy Management**: Round-robin proxy rotation.
  - `nextProxy()`: Gets the next proxy in sequence.
- **Response Handling**: Callback-based processing with a custom `handleResponse` function.

---

### Configuration

#### Proxy List
Edit the `proxyList` slice to include your proxy endpoints:
```go
var proxyList = []string{
    "http://user:pass@proxy1:port",
    "http://user:pass@proxy2:port",
    // ...
}
```

#### Prompt Messages
Customize the Grok prompts by modifying:
```go
var msgToSend = "Is this tweet explicitly promoting..."
var msgToSend2 = "{\"isPromoting\": bool,\"tokensPromoted\": list or null}"
```

---

### Methods

#### `NewGrokMonitor(ctx, svc, url)`
Creates a new Grok monitor instance.

**Parameters**:
- `ctx`: Context for cancellation.
- `svc`: Dummy account service.
- `url`: Base Twitter URL.

**Returns**:
- `*GrokMonitor`: Monitor instance.
- `*SelAccount`: Selected account.
- `error`: Any initialization error.

#### `GetGrokCheck(ctx, tweet, svc, url, handleResponse)`
Main entry point to analyze a tweet.

**Parameters**:
- `ctx`: Context for cancellation.
- `tweet`: Tweet to analyze.
- `svc`: Account service.
- `url`: Twitter URL.
- `handleResponse`: Callback for processed results.

---

### Response Format
The package returns parsed responses in this structure:
```go
type GrokEvent struct {
    ID             string    // Tweet ID
    IsShill        bool      // If promoting crypto
    TokensShilled  []string  // Promoted tokens
    TokensMentioned []string // Mentioned tokens
    UserId         string    // Author ID
    CreatedAt      time.Time // Analysis time
    DummyAccount   string    // Account used
    ConvoID        string    // Grok conversation ID
}
```

---

### Error Handling
The package includes:
- Automatic retries for failed requests.
- Detailed error logging.
- Panic recovery.
- Proxy rotation on failures.

---

### Best Practices

#### Rate Limiting
- Space out requests when processing multiple tweets.
- Use sufficient proxies to avoid blocks.

#### Error Handling
- Implement robust error handling in your callback.
- Monitor logs for retry attempts.

#### Resource Management
- Always use context for cancellation.
- Ensure proper logout of dummy accounts.

---

### Dependencies
- `github.com/tidwall/gjson` for JSON parsing.
- Custom Twitter API client.
- Dummy account service.
- Logging package (`logg`).
