package zalo

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// accountIDTimeout is shorter than defaultTimeout: this call stands in front of EVERY sign-in,
// including the silent one when the app opens. A 6 s spinner then an error the citizen retries
// beats waiting 8. Same figure as the reference implementation.
const accountIDTimeout = 6 * time.Second

// integerDigits — an id sent as a JSON NUMBER must be an integer written in digits. It never
// goes through float64: Zalo ids are longer than 15 digits, float64 rounds them, and two
// different citizens silently become one id.
var integerDigits = regexp.MustCompile(`^[0-9]+$`)

// accountIDResponse is the flat /v2.0/me body. ID is RawMessage so both a string and a number
// are read without rounding (UNKNOWN #7 in wire.go). "message" is not decoded (see
// infoEnvelope).
type accountIDResponse struct {
	ID    json.RawMessage `json:"id"`
	Error int             `json:"error"`
}

// AccountID verifies accessToken with Zalo and returns the Zalo account id (PER APP: one
// person in two apps has two ids) of the token's holder.
//
// It sends NO secret — the call does not need one — and therefore it does NOT tell which app
// the token belongs to. App verification is Phone's job (package comment).
//
// Neither the argument nor the result is ever logged; errors never carry the token, the id or
// Zalo's body.
func (c *Client) AccountID(ctx context.Context, accessToken string) (string, error) {
	if accessToken == "" {
		return "", fmt.Errorf("zalo: missing accessToken: %w", ErrTokenInvalid)
	}

	ctx, cancel := context.WithTimeout(ctx, accountIDTimeout)
	defer cancel()

	q := url.Values{fieldsParam: {accountIDField}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+AccountIDPath+"?"+q.Encode(), nil)
	if err != nil {
		return "", fmt.Errorf("zalo: build request: %w", ErrZaloUnreachable)
	}
	req.Header.Set(HeaderAccessToken, accessToken)

	body, err := c.do(req)
	if err != nil {
		return "", err
	}

	var resp accountIDResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		// The body is deliberately NOT attached: it may hold the account id.
		return "", fmt.Errorf("zalo: response is not the expected JSON: %w", ErrZaloUnreachable)
	}
	if resp.Error != 0 {
		return "", fmt.Errorf("zalo: error code %d: %w", resp.Error, ErrTokenInvalid)
	}

	id, ok := readAccountID(resp.ID)
	if !ok {
		// error == 0 without an id: we misread the response, or Zalo changed its shape.
		// Infrastructure side — never tell the citizen to sign in again.
		return "", fmt.Errorf("zalo: no readable account id in response: %w", ErrZaloUnreachable)
	}
	return id, nil
}

// readAccountID accepts a non-empty JSON STRING, or a JSON integer kept as its exact digit
// text. Everything else — absent, null, empty, fractional, object — is unreadable, not guessed.
func readAccountID(raw json.RawMessage) (string, bool) {
	s := strings.TrimSpace(string(raw))
	if s == "" || s == "null" {
		return "", false
	}
	if strings.HasPrefix(s, `"`) {
		var str string
		if err := json.Unmarshal(raw, &str); err != nil {
			return "", false
		}
		str = strings.TrimSpace(str)
		return str, str != ""
	}
	if integerDigits.MatchString(s) {
		return s, true
	}
	return "", false
}
