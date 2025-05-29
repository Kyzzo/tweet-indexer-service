# Tweet Indexer Service API Clients

## Overview
This repository contains Go packages for interacting with Twitter's APIs, including standard API endpoints, Grok AI, and advanced HTTP client implementations. Each package is designed to simplify integration with Twitter's services while providing robust features like authentication, proxy support, and error handling.

---

## Packages Summary

### Twitter API Client
- **Purpose**: Interact with Twitter's standard API endpoints.
- **Features**:
    - TLS fingerprinting bypass
    - Cookie and session management
    - Proxy support
    - Request randomization techniques
- **Key Methods**:
    - `Get` and `Post` for standard requests
    - `Get2` and `Post2` for advanced randomized requests
- **Best Practices**:
    - Rotate cookies regularly
    - Use proxies for distributed requests
    - Implement retry logic for error recovery

### Grok API Client
- **Purpose**: Communicate with Twitter's Grok AI API for conversation management and message exchange.
- **Features**:
    - Create and manage conversations
    - Send messages and parse responses
    - Proxy and authentication support
- **Key Methods**:
    - `CreateConversation` for initiating conversations
    - `SendMessage` for exchanging messages
    - `GenerateTransactionID` for API requests
- **Best Practices**:
    - Reuse conversation IDs for context
    - Rotate proxies to avoid rate limits
    - Parse raw responses for advanced use cases

### Twitter API Twitter Package
- **Purpose**: Access Twitter's user profiles, relationships, tweets, and GraphQL API.
- **Features**:
    - Retrieve user profiles and tweets
    - Manage follower/following relationships
    - Post tweets and retweets
- **Key Methods**:
    - `UserID` and `FullUser` for user operations
    - `Following` and `Followers` for relationship management
    - `AccountTweets` and `Retweet` for tweet operations
- **Best Practices**:
    - Monitor rate limits during batch operations
    - Implement error recovery for session expiration
    - Reuse client instances for efficiency

---

## Dependencies
All packages leverage Go's standard library and external dependencies like:
- `github.com/tidwall/gjson` for JSON parsing
- `github.com/bogdanfinn/fhttp` for HTTP client functionality
- `github.com/bogdanfinn/tls-client` for TLS fingerprinting

---

## Conclusion
These packages provide a unified solution for interacting with Twitter's APIs, offering flexibility, security, and ease of use. Refer to individual package documentation for detailed usage instructions and examples.
