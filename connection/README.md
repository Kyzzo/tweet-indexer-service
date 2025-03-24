# Connection Package

The `connection` package serves as a bridge between the **scraping service** and the **accountable service**. It handles communication via gRPC and provides services for managing tweets, user profiles, and Twitter KOL accounts. The package is modularized into several sub-services for focused functionality.

---

# Components

## 1. **Tweets Service**
Handles uploading scraped tweets to the **accountable service** and managing related operations.

## 2. **Site Profile Service**
Manages user profiles and interacts with the **accountable service** to fetch user information.

## 3. **Twitter User Service**
Retrieves KOL (Key Opinion Leaders) Twitter accounts and their associated details from the **accountable service**.

## 4. **Controller**
Combines the services into a centralized interface for health checks and endpoint access.

---

# Features

## Tweets Service
- **Upload Tweets**: Uploads scraped tweets to the accountable service.
- **Get Token Addresses**: Converts symbols to token addresses and updates tweets.
- **Error Handling**: Handles upload and address-fetching failures with detailed reporting.

## Site Profile Service
- **User Profiles**: Fetches user profiles from the accountable service.
- **Health Check**: Verifies the connection to the profile service.

## Twitter User Service
- **Find Users**: Fetches KOL accounts based on Twitter handles or user IDs.
- **Retrieve All Users**: Retrieves all raw user data.
- **Health Check**: Verifies the connection to the user service.

## Controller
- Provides HTTP endpoints for health checks across the `connection` services.

---

# Configuration

Set up `config.yaml` or equivalent configuration files to define:
- gRPC endpoints for the accountable service.
- MongoDB connection details.

# API Endpoints

## Twitter User Health Check
- **Path**: `/check-twitter-health`
- **Method**: `GET`
- **Description**: Verifies the health of the Twitter user service.

## Site Profile Health Check
- **Path**: `/check-site-health`
- **Method**: `GET`
- **Description**: Verifies the health of the site profile service.

## General Health Check
- **Path**: `/health`
- **Method**: `GET`
- **Description**: Verifies the health of all services.

## Tweet Service Health Check
- **Path**: `/check-tweet-health`
- **Method**: `GET`
- **Description**: Verifies the health of the tweet service.

---

# Planned Enhancements

## Enhanced Error Handling
- Ability to update tweets and tokens.
- Centralize error handling for gRPC interactions.

## Batch Processing
- Enable bulk uploads for tweets and user data.

## Performance Optimization
- Improve gRPC connection pooling and reuse.

## Retry Logic
- Implement retry mechanisms for intermittent failures.

## Logging Improvements
- Add structured logging for better debugging and monitoring.

---

# Example Workflow

## Scraping Tweets
- Tweets are scraped and stored in the database.

## Uploading to Accountable
- Tweets are uploaded via `TweetService.FullUpload`.

## Fetching User Profiles
- Use `SiteProfileService.FindProfileById` to fetch user information.

## Updating Tweets with Addresses
- Tokens are converted to addresses using `TweetService.GetAndUpdateAddresses`.

## Monitoring Services
- Health checks are performed regularly using the controller endpoints.