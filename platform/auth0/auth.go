package auth0

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
)

// auth0TokenResponse is used to decode the token response from Auth0
type auth0TokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	TokenType   string `json:"token_type"`
}

// getManagementAPIToken requests a new access token from Auth0
func GetManagementAPIToken() (string, error) {
	domain := os.Getenv("AUTH0_DOMAIN")
	clientID := os.Getenv("AUTH0_M2M_CLIENT_ID")
	clientSecret := os.Getenv("AUTH0_M2M_CLIENT_SECRET")

	payload := map[string]string{
		"client_id":     clientID,
		"client_secret": clientSecret,
		"audience":      "https://" + domain + "/api/v2/",
		"grant_type":    "client_credentials",
	}
	payloadBytes, _ := json.Marshal(payload)

	url := "https://" + domain + "/oauth/token"
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(payloadBytes))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var tokenResponse auth0TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResponse); err != nil {
		return "", err
	}

	return tokenResponse.AccessToken, nil
}
