package dummyaccount

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	// scrapedtweets "github.com/jeetdotso/tweet-indexer-service/legacytweets"
	"github.com/jeetdotso/tweet-indexer-service/logg"
	"github.com/tebeka/selenium"
	"github.com/tebeka/selenium/chrome"
)

type Helper interface {
	CheckSignUpElement(wd selenium.WebDriver) (bool, error)
	GetRandomSleepTime() int
	CheckRetryButton(wd selenium.WebDriver) bool
	WaitForElement(wd selenium.WebDriver, selector string) (selenium.WebElement, error)
	LogInPassword(ctx context.Context, wd selenium.WebDriver, sessionPassword string) error
	LogInUserName(ctx context.Context, wd selenium.WebDriver, sessionUsername string) error
	LogIn2Fa(ctx context.Context, wd selenium.WebDriver, session2Fa string) error
	LogInAndGetCookies(ctx context.Context, username, password string, wd selenium.WebDriver) ([]selenium.Cookie, error)
	LoadWebDriver(ctx context.Context, cookies []selenium.Cookie) (selenium.WebDriver, error)
	// ParseTweetAdvanced(ctx context.Context, html, userName, userTweetId, searchedTweetId, tweetType string) ([]*scrapedtweets.Tweet, error)
	LoadWebDriverNoCookies() (selenium.WebDriver, error)
	LogInAndGetCookies2fa(ctx context.Context, username, password, session2fa string, wd selenium.WebDriver) ([]selenium.Cookie, error)
	ExtractQuoteAndText(WQuote, WText string) (string, string)
	Get2FAToken(wd selenium.WebDriver, secret string) (string, error)
}

type HelperDefaultImpl struct {
}

func NewSeleniumHelper() Helper {
	return &HelperDefaultImpl{}
}

func (hlp *HelperDefaultImpl) CheckSignUpElement(wd selenium.WebDriver) (bool, error) {
	signUpElement, err := wd.FindElement(selenium.ByXPATH, "//span[contains(text(),'Sign up')]")
	if err != nil {
		// If the element is not found, the user might be logged in
		return false, nil
	}

	// If the element is found, return true indicating that the user is not logged in
	if signUpElement != nil {
		return true, nil
	}

	return false, nil
}

func GetTweetID(tweet *goquery.Selection, userName string) (string, string) {
	var realTweetID, retweet string

	tweet.Find("a[aria-label]").EachWithBreak(func(index int, item *goquery.Selection) bool {
		tweetID, exists := item.Attr("href")
		if !exists {
			return true // No link found, skip this iteration
		}

		var check = ""

		parts := strings.Split(tweetID, "/")
		if len(parts) < 2 {
			fmt.Println("Invalid link structure")
			return true // Invalid structure, skip this iteration
		}

		check = parts[2]
		// Handle history case
		tweetID = parts[len(parts)-1]
		if tweetID == "history" || tweetID == "analytics" {
			tweetID = parts[len(parts)-2]
		}

		// Detect retweets

		retweet = parts[1]

		// If it's not a tweet, skip
		if check != "status" {
			fmt.Println("Not a tweet, checking next... ", parts)
			fmt.Println("-------------------")
			return true
		}

		// At this point, you've found a valid tweet
		fmt.Println("Valid tweet found with ID: ", tweetID)
		realTweetID = tweetID
		return false
	})

	return realTweetID, retweet
}

func extractTweetText(tweet *goquery.Selection) string {
	var tweetText string

	tweet.Find("div[data-testid='tweetText']").Contents().Each(func(i int, s *goquery.Selection) {
		if goquery.NodeName(s) != "img" {
			// This is normal text
			tweetText += s.Text()
		} else if goquery.NodeName(s) == "img" {
			// This is an emoji represented as an <img> element
			emojiAlt, exists := s.Attr("alt")
			if exists {
				tweetText += emojiAlt
			}
		}
	})

	return tweetText
}

func (hlp *HelperDefaultImpl) CheckRetryButton(wd selenium.WebDriver) bool {
	// Try to find the Retry button using XPath or CSS selector
	button, err := wd.FindElement(selenium.ByXPATH, "//button[.//span[text()='Retry']]")

	if err == nil && button != nil {
		// The Retry button is found
		return true
	}

	return false
}

func (hlp *HelperDefaultImpl) GetRandomSleepTime() int {
	rand.Seed(time.Now().UnixNano())
	return rand.Intn(4) + 2 // Generates a random number between 2 and 6
}

func (hlp *HelperDefaultImpl) WaitForElement(wd selenium.WebDriver, selector string) (selenium.WebElement, error) {
	var elem selenium.WebElement
	err := wd.WaitWithTimeoutAndInterval(func(wd selenium.WebDriver) (bool, error) {
		var err error
		elem, err = wd.FindElement(selenium.ByXPATH, selector)
		if err != nil {
			return false, nil
		}
		return true, nil
	}, 10*time.Second, 500*time.Millisecond)
	return elem, err
}

func (hlp *HelperDefaultImpl) LogInUserName(ctx context.Context, wd selenium.WebDriver, sessionUsername string) error {
	url := "https://x.com/login"
	if err := wd.Get(url); err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error navigating to url")
		return err
	}

	// Wait for the username input box to be visible
	usernameField, err := hlp.WaitForElement(wd, "//input[@name='text']")
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error finding username input box")
		return err
	}

	if err := usernameField.SendKeys(sessionUsername); err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error sending keys to username input box")
		return err
	}

	// Wait for the "Next" button to be visible and click it
	nextButton, err := hlp.WaitForElement(wd, "//button//span[text()='Next']")
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error finding the next button")
		return err
	}

	if err := nextButton.Click(); err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error clicking the next button")
		return err
	}

	return nil
}

func (hlp *HelperDefaultImpl) LogIn2Fa(ctx context.Context, wd selenium.WebDriver, session2Fa string) error {
	twoFAField, err := hlp.WaitForElement(wd, "//input[@data-testid='ocfEnterTextTextInput']")
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error finding the 2FA input field")
		return err
	}

	// Send the 2FA code
	if err := twoFAField.SendKeys(session2Fa); err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error entering the 2FA code")
		return err
	}

	// Wait for the "Next" or "Confirm" button (assume button contains "Next" text)
	nextButton, err := hlp.WaitForElement(wd, "//button//span[text()='Next']")
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error finding the Next button after entering 2FA")
		return err
	}

	// Click the "Next" button to submit the 2FA code
	if err := nextButton.Click(); err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error clicking the Next button after entering 2FA")
		return err
	}

	// Add a delay to allow processing (optional)
	time.Sleep(5 * time.Second)
	return nil
}

func (hlp *HelperDefaultImpl) LogInPassword(ctx context.Context, wd selenium.WebDriver, sessionPassword string) error {
	// Wait for the password input field to be visible and enter the password
	passwordField, err := hlp.WaitForElement(wd, "//input[@name='password']")
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error finding the password input")
		return err
	}

	if err := passwordField.SendKeys(sessionPassword); err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error inserting the password input")
		return err
	}

	// Wait for the "Log In" or "Next" button to be visible and click it
	nextButton, err := hlp.WaitForElement(wd, "//button//span[text()='Log in']")
	if err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error finding the log in button")
		return err
	}

	if err := nextButton.Click(); err != nil {
		logg.Log.Error().Caller().Ctx(ctx).Err(err).Msg("Error clicking the log in button")
		return err
	}

	// Add a delay to see the result (optional)
	time.Sleep(5 * time.Second)
	return nil
}

func (hlp *HelperDefaultImpl) LogInAndGetCookies(ctx context.Context, username, password string, wd selenium.WebDriver) ([]selenium.Cookie, error) {
	err := hlp.LogInUserName(ctx, wd, username)
	if err != nil {
		return nil, err
	}
	err = hlp.LogInPassword(ctx, wd, password)
	if err != nil {
		return nil, err
	}
	return wd.GetCookies()
}
func (hlp *HelperDefaultImpl) LogInAndGetCookies2fa(ctx context.Context, username, password, session2fa string, wd selenium.WebDriver) ([]selenium.Cookie, error) {
	err := hlp.LogInUserName(ctx, wd, username)
	if err != nil {
		return nil, err
	}
	err = hlp.LogInPassword(ctx, wd, password)
	if err != nil {
		return nil, err
	}

	err = hlp.LogIn2Fa(ctx, wd, session2fa)
	if err != nil {
		return nil, err
	}
	return wd.GetCookies()
}

func (hlp *HelperDefaultImpl) LoadWebDriver(ctx context.Context, cookies []selenium.Cookie) (selenium.WebDriver, error) {
	chromeCaps := chrome.Capabilities{
		Args: []string{
			"--headless",
			"--disable-gpu",
			"--window-size=1920x1080",
			"--no-sandbox",
			"--disable-dev-shm-usage",
			"--disable-software-rasterizer",
		},
	}

	caps := selenium.Capabilities{
		"browserName":   "chrome",
		"chromeOptions": chromeCaps,
	}

	wd, err := selenium.NewRemote(caps, fmt.Sprintf("http://localhost:%d/wd/hub", port))
	if err != nil {
		return nil, err
	}
	url := "https://twitter.com"
	if err := wd.Get(url); err != nil {
		return nil, err
	}
	time.Sleep(time.Duration(hlp.GetRandomSleepTime()) * time.Second)
	// Add cookies to the WebDriver instance
	for _, cook := range cookies {
		err = wd.AddCookie(&cook)
		if err != nil {
			logg.Log.Error().Caller().Ctx(ctx).Err(err).Str("cookie domain", cook.Domain).Str("account", cook.Name).Msg("Error adding cookies")
			return nil, err
		}
	}
	time.Sleep(1 * time.Second)

	return wd, nil
}
func (hlp *HelperDefaultImpl) LoadWebDriverNoCookies() (selenium.WebDriver, error) {
	chromeCaps := chrome.Capabilities{
		Args: []string{
			"--headless",
			"--disable-gpu",
			"--window-size=1920x1080",
			"--no-sandbox",
			"--disable-dev-shm-usage",
			"--disable-software-rasterizer",
		},
	}
	// Set Chrome capabilities
	caps := selenium.Capabilities{
		"browserName":   "chrome",
		"chromeOptions": chromeCaps,
	}
	wd, err := selenium.NewRemote(caps, fmt.Sprintf("http://localhost:%d/wd/hub", port))
	if err != nil {
		return nil, err
	}
	time.Sleep(1 * time.Second)

	return wd, nil
}

func (hlp *HelperDefaultImpl) ExtractQuoteAndText(WQuote, WText string) (string, string) {
	// Step 1: The real text is always at the start of WText.
	// The real quote is the remainder of WText after the real text.
	possibleTextEnd := strings.LastIndex(WText, strings.TrimSpace(WQuote[:min(len(WQuote), 10)])) // Find a match of the start of the quote in WText.

	// Step 2: If we find where the quote starts in WText, we split.
	if possibleTextEnd != -1 {
		realText := strings.TrimSpace(WText[:possibleTextEnd])  // Real text is before the quote starts.
		realQuote := strings.TrimSpace(WText[possibleTextEnd:]) // Real quote is the remaining part of WText.

		return realQuote, realText
	}

	// Step 3: Fallback to basic extraction
	return strings.TrimSpace(WQuote), strings.TrimSpace(WText)
}

func (hlp *HelperDefaultImpl) Get2FAToken(wd selenium.WebDriver, secret string) (string, error) {

	// Open TOTP Generator website
	if err := wd.Get("https://totp.danhersam.com/"); err != nil {
		return "", fmt.Errorf("failed to open TOTP site: %v", err)
	}

	// Find and input the Secret Key
	secretInput, err := wd.FindElement(selenium.ByCSSSelector, "input[placeholder='The secret key (in base-32 format)']")
	if err != nil {
		return "", fmt.Errorf("failed to find secret key input: %v", err)
	}

	secretInput.Clear()
	// Find and input Token Period (90 seconds)
	// periodInput, err := wd.FindElement(selenium.ByCSSSelector, "input[placeholder='Usually 30']")
	// if err != nil {
	// 	return "", fmt.Errorf("failed to find token period input: %v", err)
	// }
	// periodInput.Clear()
	// err = periodInput.SendKeys("90")
	// if err != nil {
	// 	return "", fmt.Errorf("failed to enter token period: %v", err)
	// }
	time.Sleep(2 * time.Second)
	secretInput.Clear()
	err = secretInput.SendKeys(secret)
	if err != nil {
		return "", fmt.Errorf("failed to enter secret key: %v", err)
	}

	// Wait a few seconds for the page to update the TOTP
	time.Sleep(3 * time.Second)

	// Get the generated TOTP code
	totpElement, err := wd.FindElement(selenium.ByID, "token")
	if err != nil {
		return "", fmt.Errorf("failed to find TOTP token: %v", err)
	}
	totpCode, err := totpElement.Text()
	if err != nil {
		return "", fmt.Errorf("failed to retrieve TOTP code: %v", err)
	}

	return totpCode, nil
}
