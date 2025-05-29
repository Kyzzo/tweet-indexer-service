# Twitter API Client Package

## Overview
This package provides a comprehensive HTTP client for interacting with Twitter's API, featuring:

- **TLS fingerprinting bypass**
- **Cookie and session management**
- **Request randomization techniques**
- **Dual request methods (standard and advanced)**
- **Proxy support**

---

## Features
- 🔒 **TLS Fingerprinting** - Uses Chrome 133 profile to avoid detection
- 🍪 **Cookie Management** - Automatic cookie handling and rotation
- 🎲 **Request Randomization** - Random client transaction IDs
- 🔄 **Dual Request Methods** - Standard and advanced implementations
- 🌐 **Proxy Support** - Built-in proxy configuration

---

## Installation
```go
import "github.com/your-repo/twitterAPI"
```

---

## Initialization

### Basic Initialization
```go
client, err := CreateTwitterClient()
```

### With Custom Cookies
```go
client, err := CreateTwitterClientWithCookies(yourCookies)
```

### Advanced Configuration
```go
client, err := NewTwitterClient(&NewTwitterClientOptions{
    BaseURL:  "https://x.com",
    Cookies:  yourCookies,
    ProxyURL: "http://proxy:port",
})
```

---

## Key Methods

### Request Methods
```go
// Standard methods
Get(url string, headers *http.Header) (*[]byte, int, error)
Post(url string, body io.Reader, headers *http.Header) (*[]byte, int, error)

// Advanced methods with additional randomization
Get2(url string, body io.Reader, headers *http.Header) (*[]byte, int, error) 
Post2(url string, body io.Reader, headers *http.Header) ([]byte, int, error)
```

---

## Usage Examples

### Basic GET Request
```go
response, status, err := client.Get("https://x.com/api/endpoint", nil)
```

### POST with Custom Headers
```go
headers := http.Header{}
headers.Set("Custom-Header", "value")

body := strings.NewReader(`{"key":"value"}`)
response, status, err := client.Post("https://x.com/api/post", body, &headers)
```

### Advanced POST with Full Control
```go
responseBytes, status, err := client.Post2(
    "https://x.com/api/advanced",
    bytes.NewBuffer(jsonData),
    &customHeaders,
)
```

---

## Configuration Options

### NewTwitterClientOptions
| Parameter       | Description              | Default         |
|-----------------|--------------------------|-----------------|
| **BaseURL**     | Base API URL             | `"https://x.com"` |
| **Cookies**     | Session cookies          | Required        |
| **ProxyURL**    | Proxy server URL         | `""`            |
| **CustomHeaders** | Additional headers      | `nil`           |

---

## Request Headers
Automatically includes:
- Authorization tokens
- CSRF protection
- Randomized client IDs
- Proper content types
- Browser-like headers

---

## Error Handling
All methods return:
- **Response body** (bytes)
- **HTTP status code**
- **Error object** (nil on success)

---

## Best Practices

### Cookie Rotation
- Use `utils.BuildCookiesPool()` for cookie management
- Rotate cookies regularly

### Request Randomization
- Prefer `Get2`/`Post2` methods for better anonymity
- Let the client handle transaction IDs

### Proxy Usage
- Distribute requests across multiple proxies
- Monitor for IP blocks

### Error Recovery
- Check for session expiration
- Implement retry logic

---

## Dependencies
- `github.com/bogdanfinn/fhttp` - HTTP client
- `github.com/bogdanfinn/tls-client` - TLS fingerprinting
- Internal utility packages