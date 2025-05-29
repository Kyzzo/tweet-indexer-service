# Twitter Monitoring Packages Overview

## 📦 Packages in This Directory

This repository contains two main packages for Twitter monitoring and analysis:

### 1. **Twitter Following Monitor**
**Purpose:** Track and analyze Twitter following relationships  
**Key Features:**  
- Concurrent processing with worker pools  
- Two operation modes: single-page or full pagination processing  
- Automatic retry mechanism for API calls  
- Context-aware cancellation  
- Detailed logging and resource cleanup  

**Use Cases:**  
- Monitoring account followings  
- Building relationship graphs  
- Tracking changes in followings over time  

---

### 2. **Twitter Grok Monitor**
**Purpose:** Analyze tweets for cryptocurrency promotions using Grok AI  
**Key Features:**  
- Proxy rotation for reliable requests  
- Grok conversation management  
- Specialized crypto promotion detection  
- Structured response parsing  
- Automatic retry mechanism  

**Use Cases:**  
- Detecting crypto shilling  
- Monitoring promotional activity  
- Analyzing tweet content at scale  

---

## 🛠 Usage Patterns

### **Following Monitor**
```go
MonitorFollowingsPipeline(
    ctx,
    accountIDs,
    3,              // concurrent fetchers
    10,             // concurrent handlers
    3,              // max retries
    2*time.Second,  // retry delay
    handleFollowing,
    dummyAccountService,
)
```

### **Grok Monitor**
```go
GetGrokCheck(
    context.Background(), 
    tweet, 
    dummyAccountService, 
    "https://twitter.com", 
    handleResponse
)
```

---

## 📚 Detailed Documentation

For complete documentation on each package:  
- [Following Monitor Documentation](#)  
- [Grok Monitor Documentation](#)  

---

## 🔗 Dependencies

Both packages share these common dependencies:  
- `github.com/tidwall/gjson` for JSON parsing  
- Custom Twitter API client  
- Dummy account service  
- Logging package (`logg`)  

---

## 🏗 Architecture

The packages follow similar architectural patterns:  
- Worker pool model for concurrent processing  
- Channel-based communication  
- Context-aware operations  
- Comprehensive error handling  
- Resource cleanup guarantees  

---

## 📈 Best Practices

For both packages:  
- Monitor API rate limits and adjust concurrency accordingly  
- Implement robust error handling in your callback functions  
- Use contexts for proper cancellation  
- Review logs regularly to identify issues  
- Monitor resource usage, especially when processing at scale  
