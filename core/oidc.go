package core

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

const (
	// ActionsIDTokenRequestTokenEnvName is the env var containing the OIDC request token.
	ActionsIDTokenRequestTokenEnvName = "ACTIONS_ID_TOKEN_REQUEST_TOKEN"
	// ActionsIDTokenRequestURLEnvName is the env var containing the OIDC request URL.
	ActionsIDTokenRequestURLEnvName = "ACTIONS_ID_TOKEN_REQUEST_URL"
)

// GetIDToken gets the JSON Web Token (JWT) for the current workflow run.
// audience is an optional audience for the token; pass empty string for the default.
// The token is automatically masked from logs.
func GetIDToken(audience string) (string, error) {
	requestToken, ok := lookupEnv(ActionsIDTokenRequestTokenEnvName)
	if !ok || requestToken == "" {
		return "", fmt.Errorf("unable to get %s env variable", ActionsIDTokenRequestTokenEnvName)
	}

	requestURL, ok := lookupEnv(ActionsIDTokenRequestURLEnvName)
	if !ok || requestURL == "" {
		return "", fmt.Errorf("unable to get %s env variable", ActionsIDTokenRequestURLEnvName)
	}

	if audience != "" {
		u, err := url.Parse(requestURL)
		if err != nil {
			return "", fmt.Errorf("invalid OIDC request URL: %w", err)
		}
		q := u.Query()
		q.Set("audience", audience)
		u.RawQuery = q.Encode()
		requestURL = u.String()
	}

	Debugf("ID token url is %s", requestURL)

	req, err := http.NewRequest(http.MethodGet, requestURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create ID token request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+requestToken)
	req.Header.Set("Accept", "application/json; api-version=2.0")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to get ID token: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read ID token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to get ID token. Status code: %d. Error: %s", resp.StatusCode, string(body))
	}

	var tokenResponse struct {
		Value string `json:"value"`
	}
	if err := json.Unmarshal(body, &tokenResponse); err != nil {
		return "", fmt.Errorf("failed to parse ID token response: %w", err)
	}
	if tokenResponse.Value == "" {
		return "", fmt.Errorf("response json body does not have ID token field")
	}

	SetSecret(tokenResponse.Value)
	return tokenResponse.Value, nil
}
