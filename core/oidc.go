package core

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

const (
	// ActionsIDTokenRequestTokenEnvName is the env var containing the OIDC request token.
	ActionsIDTokenRequestTokenEnvName = "ACTIONS_ID_TOKEN_REQUEST_TOKEN"
	// ActionsIDTokenRequestURLEnvName is the env var containing the OIDC request URL.
	ActionsIDTokenRequestURLEnvName = "ACTIONS_ID_TOKEN_REQUEST_URL"

	// actionsOrchestrationIDEnvName is appended to the user agent of the OIDC request.
	actionsOrchestrationIDEnvName = "ACTIONS_ORCHESTRATION_ID"

	// the retry policy of @actions/http-client, as used by @actions/core for this request
	oidcMaxTries         = 11
	oidcBackoffCeiling   = 10
	oidcBackoffTimeSlice = 5 * time.Millisecond
)

var nonUserAgentChars = regexp.MustCompile(`[^a-zA-Z0-9_.-]`)

// GetIDToken gets the JSON Web Token (JWT) for the current workflow run.
// audience is an optional audience for the token; pass empty string for the default.
// The token is automatically masked from logs.
//
// The request, its retries and the error messages follow @actions/core.
func GetIDToken(audience string) (string, error) {
	token, err := getIDToken(audience)
	if err != nil {
		return "", fmt.Errorf("Error message: %s", err.Error()) //nolint:staticcheck
	}
	return token, nil
}

func getIDToken(audience string) (string, error) {
	requestURL, ok := lookupEnv(ActionsIDTokenRequestURLEnvName)
	if !ok || requestURL == "" {
		return "", fmt.Errorf("Unable to get %s env variable", ActionsIDTokenRequestURLEnvName) //nolint:staticcheck
	}
	if audience != "" {
		// the reference appends to whatever query the URL has, without parsing it
		requestURL += "&audience=" + encodeURIComponent(audience)
	}
	Debugf("ID token url is %s", requestURL)

	requestToken, ok := lookupEnv(ActionsIDTokenRequestTokenEnvName)
	if !ok || requestToken == "" {
		return "", fmt.Errorf("Unable to get %s env variable", ActionsIDTokenRequestTokenEnvName) //nolint:staticcheck
	}

	status, body, err := getWithRetries(requestURL, requestToken)
	if err != nil {
		return "", idTokenError("undefined", err.Error())
	}
	token, err := idTokenFromResponse(status, body)
	if err != nil {
		return "", err
	}
	SetSecret(token)
	return token, nil
}

func idTokenError(code, message string) error {
	return fmt.Errorf("Failed to get ID Token. \n \n        Error Code : %s\n \n        Error Message: %s", code, message) //nolint:staticcheck
}

// idTokenFromResponse interprets the response like @actions/http-client getJson
// followed by the checks of @actions/core.
func idTokenFromResponse(status int, body []byte) (string, error) {
	missing := fmt.Errorf("Response json body do not have ID Token field") //nolint:staticcheck
	// not found leads to a null result
	if status == http.StatusNotFound {
		return "", missing
	}
	var parsed interface{}
	var obj map[string]interface{}
	if len(body) > 0 && json.Unmarshal(body, &parsed) == nil {
		obj, _ = parsed.(map[string]interface{})
	}
	if status > 299 {
		msg := fmt.Sprintf("Failed request: (%d)", status)
		if m, ok := obj["message"].(string); ok && m != "" {
			msg = m
		} else if len(body) > 0 {
			msg = string(body)
		}
		return "", idTokenError(fmt.Sprint(status), msg)
	}
	value, _ := obj["value"].(string)
	if value == "" {
		return "", missing
	}
	return value, nil
}

// getWithRetries performs the GET like @actions/http-client with retries enabled:
// 502, 503 and 504 are retried up to 10 times with an exponential back-off.
func getWithRetries(url, token string) (status int, body []byte, err error) {
	userAgent := "actions/oidc-client"
	if id := os.Getenv(actionsOrchestrationIDEnvName); id != "" {
		userAgent += " actions_orchestration_id/" + nonUserAgentChars.ReplaceAllString(id, "_")
	}
	url = escapeQuery(url)
	for try := 1; ; try++ {
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return 0, nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", userAgent)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return 0, nil, err
		}
		body, err = io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return 0, nil, err
		}
		retryable := resp.StatusCode == http.StatusBadGateway || resp.StatusCode == http.StatusServiceUnavailable || resp.StatusCode == http.StatusGatewayTimeout
		if !retryable || try >= oidcMaxTries {
			return resp.StatusCode, body, nil
		}
		exponent := try
		if exponent > oidcBackoffCeiling {
			exponent = oidcBackoffCeiling
		}
		time.Sleep(oidcBackoffTimeSlice << exponent)
	}
}

// escapeQuery percent-encodes the query of a URL the way the WHATWG URL parser
// used by Node does for http(s) URLs: control characters, space, ", <, >, ' and
// non-ASCII bytes. Go would send some of them verbatim.
func escapeQuery(rawURL string) string {
	start := strings.IndexByte(rawURL, '?')
	if start < 0 {
		return rawURL
	}
	end := len(rawURL)
	if i := strings.IndexByte(rawURL[start:], '#'); i >= 0 {
		end = start + i
	}
	var b strings.Builder
	b.WriteString(rawURL[:start+1])
	for i := start + 1; i < end; i++ {
		c := rawURL[i]
		if c <= 0x20 || c >= 0x7f || strings.IndexByte(`"<>'`, c) >= 0 {
			fmt.Fprintf(&b, "%%%02X", c)
		} else {
			b.WriteByte(c)
		}
	}
	b.WriteString(rawURL[end:])
	return b.String()
}

// encodeURIComponent escapes like the JavaScript function of the same name:
// everything but A-Z a-z 0-9 - _ . ! ~ * ' ( ) is percent-encoded as UTF-8.
func encodeURIComponent(s string) string {
	const unreserved = "-_.!~*'()"
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.IndexByte(unreserved, c) >= 0 {
			b.WriteByte(c)
		} else {
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}
