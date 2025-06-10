package twitterAPI

import (
	"bytes"
	"fmt"
	"io"
	"math/rand"
	hh "net/http"
	"strings"
	"time"

	http "github.com/bogdanfinn/fhttp"
	tls_client "github.com/bogdanfinn/tls-client"
	profiles "github.com/bogdanfinn/tls-client/profiles"

	utils "github.com/jeetdotso/tweet-indexer-service/utils"
)

type TwitterClient struct {
	BaseURL string
	Client  *tls_client.HttpClient
	Headers http.Header
	Cookies []*http.Cookie
}

type NewTwitterClientOptions struct {
	BaseURL       string
	CustomHeaders http.Header
	Cookies       []*http.Cookie
	ProxyURL      string
}

func NewTwitterClient(opts *NewTwitterClientOptions) (*TwitterClient, error) {
	if opts.BaseURL == "" {
		opts.BaseURL = "https://x.com"
	}
	options := []tls_client.HttpClientOption{
		tls_client.WithTimeoutSeconds(30),
		tls_client.WithClientProfile(profiles.Chrome_133_PSK),
		tls_client.WithNotFollowRedirects(),
	}
	if opts.ProxyURL != "" {
		options = append(options, tls_client.WithProxyUrl(opts.ProxyURL))
	}
	client, err := tls_client.NewHttpClient(tls_client.NewNoopLogger(), options...)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize TLS client: %w", err)
	}
	headers := http.Header{
		"authority":                 {"x.com"},
		"accept":                    {"*/*"},
		"accept-language":           {"en-US,en;q=0.9"},
		"authorization":             {"Bearer AAAAAAAAAAAAAAAAAAAAANRILgAAAAAAnNwIzUejRCOuH5E6I8xnZz4puTs%3D1Zv7ttfk8LF81IUq16cHjhLTvJu4FA33AGWWjCpTnA"},
		"Host":                      {"x.com"},
		"origin":                    {"https://x.com"},
		"referer":                   {"https://x.com/"},
		"Content-Type":              {"application/json"},
		"sec-fetch-dest":            {"empty"},
		"sec-fetch-mode":            {"cors"},
		"sec-fetch-site":            {"same-origin"},
		"sec-gpc":                   {"1"},
		"user-agent":                {"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:133.0) Gecko/20100101 Firefox/133.0"},
		"x-twitter-active-user":     {"yes"},
		"x-twitter-auth-type":       {"OAuth2Session"},
		"x-twitter-client-language": {"en"},
	}
	headers["x-csrf-token"] = []string{*utils.GetCookieByName(&opts.Cookies, "ct0")}
	if opts.CustomHeaders != nil {
		for k, v := range opts.CustomHeaders {
			headers[k] = v
		}
	}
	return &TwitterClient{
		BaseURL: opts.BaseURL,
		Client:  &client,
		Headers: headers,
		Cookies: opts.Cookies,
	}, nil
}

func (tc *TwitterClient) MakeRequest(method, url string, body io.Reader, headers *http.Header) (*[]byte, int, error) {
	// Clone the body so it can be read and re-used
	var bodyBytes []byte
	if body != nil {
		var err error
		bodyBytes, err = io.ReadAll(body)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to read request body: %w", err)
		}
		body = bytes.NewReader(bodyBytes) // Reset body reader after reading
	}

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create %s request: %w", method, err)
	}

	// Clone and optionally override headers
	req.Header = tc.Headers.Clone()
	if headers != nil {
		for k, v := range *headers {
			req.Header[k] = v
		}
	}

	// Attach cookies
	for _, cookie := range tc.Cookies {
		req.AddCookie(cookie)
	}

	// Perform request
	resp, err := (*tc.Client).Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("%s request failed: %w", method, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("failed to read response body: %w", err)
	}

	return &respBody, resp.StatusCode, nil
}

func (tc *TwitterClient) MakeRequest2(method, url string, body io.Reader, headers *http.Header) ([]byte, int, error) {
	var bodyBytes []byte
	var err error

	// Read and copy the original body
	if body != nil {
		bodyBytes, err = io.ReadAll(body)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to read request body: %w", err)
		}
	}

	// Create HTTP request
	req, err := hh.NewRequest(method, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create %s request: %w", method, err)
	}
	req.ContentLength = int64(len(bodyBytes))
	clientXIDs := []string{
		"Ts283pqN1mppW/gSSKRzsV/KFK4kx3Tx6Z0uv8xsAhDpX29ISyaJxw4PLN8yGEBg2TyZnU0WDO9VtuUW3UjDnA7TTJjmTQ",
		"1xAzpUYFh8ahCEGOWZegNiYgFf2cGfTjB9s+lwhZmJnH3EHXgplp18Wu2XuE8TIYcfMVBdRjepoLn6jnIir+LGH9nE1l1A",
		"kZWxjhCCh8Xfr4X/OxDgS2cNmzawCiL9r5HHivubaGtUy5qEvNtrrKJRYHcVCByXf0lJQpIGFk2Jc4UdaSh/bh8D0zCnkg",
		"JSEFOqQ2M3FrGzFLj6RU/9O5L4IEvpZJGyVzPk8v3N/gfy4wCG/fGBbl1MOhvKgjy3z89iYlygsa06zOpYJnwCpTTyyQJg",
		"3350gQE/b46B3Yx7qC3KuklIkR8O5bNHUkWxPNRbRxJeIABnDt7YMDy2kRlKK0oUXGcFDNxaWyYN/0udUsn4MvzkuPmW3A",
		"kW4yn7yTgHg4drZ38/hC9lmtMuGa22yum4q2Be2ThDBcyAmvNCJHmCl2bmB7JOuajY15QpLPRdnDs6ePMJ3lJe0kLHihkg",
		"iyog1VVrO9rVidgv/Hme7h0cxUtasecTBhHlaIAPE0YKdFQzWoqMZGjixU0efx5ACLhQWIjU/ix/oNDqONhfsi/z5K7+iA",
		"a5TIZUZpeoLCjEyNCQK4DKNXyBtgIZZUYXBM/xdpfsqmMvNVzti9YtOMlJqB3hFgdyyDuGgk/jLlnqmXY7QkHBWUYFtIaA",
		"/xi9QaMumDxdezmcmSYG1L5+64QskhZ3veLNqN8Bq368saMceQkfb2P5NcR3U9zJRMcPLPwhfsLI9gslRuIVXU75Outw/A",
		"ZrHZ0IW1gQgv5VliJElZ2s+fwkJL/6k3yiSTjUky8XbkhzpY2lceycHiaz1lThdtB6RhsmXOlynh59exVZY/bBpLFSFrZQ",
		"LP0VaExqwa/FlHrqYne3dIG61r+3YiVltIQ7K5+bAsDQUoPWgVF/FFl7UmiiKKvZtA0k+C/q1u0yi/aL2ZyL7/Dy8ArPLw",
	}
	n := rand.Intn(len(clientXIDs)) // uniformly 0 <= n < 8
	clientXID := clientXIDs[n]

	// Use exact headers (case-sensitive matching matters for some proxies)
	req.Header.Set("accept", "*/*")
	req.Header.Set("accept-language", "en-US,en;q=0.9")
	req.Header.Set("authorization", "Bearer AAAAAAAAAAAAAAAAAAAAANRILgAAAAAAnNwIzUejRCOuH5E6I8xnZz4puTs%3D1Zv7ttfk8LF81IUq16cHjhLTvJu4FA33AGWWjCpTnA")
	req.Header.Set("content-type", "text/plain;charset=UTF-8")
	req.Header.Set("origin", "https://x.com")
	req.Header.Set("priority", "u=1, i")
	req.Header.Set("referer", "https://x.com/")
	req.Header.Set("sec-ch-ua", `"Google Chrome";v="135", "Not-A.Brand";v="8", "Chromium";v="135"`)
	req.Header.Set("sec-ch-ua-mobile", "?0")
	req.Header.Set("sec-ch-ua-platform", `"Windows"`)
	req.Header.Set("sec-fetch-dest", "empty")
	req.Header.Set("sec-fetch-mode", "cors")
	req.Header.Set("sec-fetch-site", "same-site")
	req.Header.Set("user-agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/135.0.0.0 Safari/537.36")
	req.Header.Set("x-client-transaction-id", clientXID)
	req.Header.Set("x-csrf-token", *utils.GetCookieByName(&tc.Cookies, "ct0"))
	req.Header.Set("x-twitter-active-user", "yes")
	req.Header.Set("x-twitter-auth-type", "OAuth2Session")
	req.Header.Set("x-xai-request-id", "bf962cc8-0949-4f97-b950-4d390d06b800")

	// Raw cookie header
	var cookiePairs []string
	for _, c := range tc.Cookies {
		// Only include name=value; skip attributes like Path, Domain, etc.
		pair := fmt.Sprintf("%s=%s", c.Name, c.Value)
		cookiePairs = append(cookiePairs, pair)
	}
	req.Header.Set("Cookie", strings.Join(cookiePairs, "; "))

	// Send request
	client := &hh.Client{
		Timeout: 20 * time.Second,
		Transport: &hh.Transport{
			ForceAttemptHTTP2: true,
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("%s request failed: %w", method, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		return nil, resp.StatusCode, nil // ✅ skip reading response body
	}

	rspbod, err := io.ReadAll(resp.Body)
	if err != nil && err != io.EOF {
		return nil, resp.StatusCode, fmt.Errorf("failed to read response body: %w", err)
	}

	return rspbod, resp.StatusCode, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(rspbod))

}

func (tc *TwitterClient) Get(url string, headers *http.Header) (*[]byte, int, error) {
	return tc.MakeRequest(http.MethodGet, url, nil, headers)
}
func (tc *TwitterClient) Get2(url string, body io.Reader, headers *http.Header) (*[]byte, int, error) {
	return tc.MakeRequest(http.MethodGet, url, body, headers)
}

func (tc *TwitterClient) Post(url string, body io.Reader, headers *http.Header) (*[]byte, int, error) {
	return tc.MakeRequest(http.MethodPost, url, body, headers)
}
func (tc *TwitterClient) Post2(url string, body io.Reader, headers *http.Header) ([]byte, int, error) {
	return tc.MakeRequest2(http.MethodPost, url, body, headers)
}

func CreateTwitterClient() (*TwitterClient, error) {
	cookiesPool := utils.BuildCookiesPool()
	return NewTwitterClient(&NewTwitterClientOptions{
		BaseURL: "https://x.com",
		Cookies: *cookiesPool.GetRandomCookies(),
	})
}

func CreateTwitterClientWithCookies(cookies []*http.Cookie, proxy string) (*TwitterClient, error) {
	return NewTwitterClient(&NewTwitterClientOptions{
		BaseURL:  "https://x.com",
		Cookies:  cookies,
		ProxyURL: proxy,
	})
}
