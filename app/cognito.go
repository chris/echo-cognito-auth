package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
)

var (
	cognitoUserPoolClientID     = os.Getenv("COGNITO_USER_POOL_CLIENT_ID")
	cognitoUserPoolClientSecret = os.Getenv("COGNITO_USER_POOL_CLIENT_SECRET")
	cognitoBaseUrl              = os.Getenv("COGNITO_BASE_URL")
	cognitoRedirectUri          = os.Getenv("COGNITO_REDIRECT_URI")
)

// CognitoTokenResponse represents the response from Cognito token endpoint
type CognitoTokenResponse struct {
	AccessToken  string `json:"access_token"`
	IDToken      string `json:"id_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

// CognitoUserInfo represents the user info from Cognito userInfo endpoint
type CognitoUserInfo struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified string `json:"email_verified"`
	Name          string `json:"name"`
	Username      string `json:"username"`
}

// exchangeCodeForTokens exchanges the authorization code for access and ID tokens
func exchangeCodeForTokens(code string) (*CognitoTokenResponse, error) {
	tokenEndpoint := cognitoBaseUrl + "/oauth2/token"

	// Prepare form data
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("client_id", cognitoUserPoolClientID)
	data.Set("code", code)
	data.Set("redirect_uri", cognitoRedirectUri)
	data.Set("client_secret", cognitoUserPoolClientSecret)

	// Create request
	req, err := http.NewRequest("POST", tokenEndpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create token request: %w", err)
	}

	// Set headers
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	// Send request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read token response: %w", err)
	}

	// Check for error response
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token request returned non-200 status: %d, body: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var tokenResponse CognitoTokenResponse
	if err := json.Unmarshal(body, &tokenResponse); err != nil {
		return nil, fmt.Errorf("failed to parse token response: %w", err)
	}

	return &tokenResponse, nil
}

// getUserInfo retrieves user information using the access token
func getUserInfo(accessToken string) (*CognitoUserInfo, error) {
	userInfoEndpoint := cognitoBaseUrl + "/oauth2/userInfo"

	// Create request
	req, err := http.NewRequest("GET", userInfoEndpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create userInfo request: %w", err)
	}

	// Add authorization header
	req.Header.Add("Authorization", "Bearer "+accessToken)

	// Send request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("userInfo request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read userInfo response: %w", err)
	}

	// Check for error response
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("userInfo request returned non-200 status: %d, body: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var userInfo CognitoUserInfo
	if err := json.Unmarshal(body, &userInfo); err != nil {
		return nil, fmt.Errorf("failed to parse userInfo response: %w", err)
	}

	return &userInfo, nil
}

func cognitoHostedLoginURL() string {
	u, err := url.Parse(cognitoBaseUrl + "/login")
	if err != nil {
		logger.Error("Error parsing Cognito baseURL", "error", err)
		return ""
	}

	params := url.Values{}
	params.Add("response_type", "code")
	params.Add("client_id", cognitoUserPoolClientID)
	params.Add("redirect_uri", cognitoRedirectUri)
	params.Add("state", "EchoCognitoAuth")
	params.Add("scope", "openid email profile")
	u.RawQuery = params.Encode()

	return u.String()
}

func cognitoHostedLogoutURL(host string) string {
	u, err := url.Parse(cognitoBaseUrl + "/logout")
	if err != nil {
		logger.Error("Error parsing Cognito baseURL", "error", err)
		return ""
	}

	// This will send the user back to the home page of the app after logging out.
	// Alternatively, if you want to send them back to the login/signup page, you
	// would instead set all the same params as the login URL, and remove the
	// logout_uri parameter.
	var logoutURI string
	if strings.Contains(host, "localhost") {
		logoutURI = "http://" + host
	} else {
		logoutURI = "https://" + host
	}

	params := url.Values{}
	params.Add("client_id", cognitoUserPoolClientID)
	params.Add("logout_uri", logoutURI)
	u.RawQuery = params.Encode()

	return u.String()
}
