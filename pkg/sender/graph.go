package sender

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/goodieshq/gopostal/pkg/utils"
	"github.com/rs/zerolog/log"
)

type Sender interface {
	SendEmail(ctx context.Context, from string, to []string, subject string, body []byte) error
	Authenticate(ctx context.Context) error
}

type GraphSender struct {
	mu           sync.Mutex
	token        *AuthToken
	mailbox      string
	tenantID     string
	clientID     string
	clientSecret string
	httpClient   *http.Client
	retries      int
	backoff      time.Duration
}

func NewGraphSender(tenantID, clientID, clientSecret string, timeout time.Duration, retries int, backoff time.Duration) *GraphSender {
	return &GraphSender{
		tenantID:     tenantID,
		clientID:     clientID,
		clientSecret: clientSecret,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		retries: retries,
		backoff: backoff,
	}
}

func (gs *GraphSender) getAuthTokenWithTimeout(ctx context.Context, timeout time.Duration) (*AuthToken, error) {
	// Create a context with a timeout for the token request
	ctxWithTimeout, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return gs.getAuthToken(ctxWithTimeout)
}

func (gs *GraphSender) getAuthToken(ctx context.Context) (*AuthToken, error) {
	apiUrl := "https://login.microsoftonline.com/" + gs.tenantID + "/oauth2/v2.0/token"

	// Create the form data for the token request
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("scope", "https://graph.microsoft.com/.default")
	form.Set("client_id", gs.clientID)
	form.Set("client_secret", gs.clientSecret)

	// Create a new HTTP request with the form data
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiUrl, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}

	// Set the appropriate headers for the request
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Send the request to get the access token
	resp, err := gs.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Check if the response status code indicates success
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get access token: %s", resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // limit to 1MB
	if err != nil {
		return nil, err
	}

	// Decode the response body to extract the access token
	var tokenResp AuthTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, err
	}

	return &AuthToken{
		Token:     tokenResp.AccessToken,
		ExpiresAt: time.Now().UTC().Add(time.Duration(tokenResp.ExpiresIn) * time.Second),
	}, nil
}

func (gs *GraphSender) Authenticate(ctx context.Context) error {
	if gs.token != nil && time.Until(gs.token.ExpiresAt) > 1*time.Minute {
		// Token is still valid, no need to re-authenticate
		log.Debug().Msg("Existing token is still valid")
		return nil
	}
	log.Debug().Msg("Fetching a new access token for Microsoft Graph API")

	tok, err := gs.getAuthTokenWithTimeout(ctx, 10*time.Second)
	if err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	gs.mu.Lock()
	defer gs.mu.Unlock()
	gs.token = tok
	log.Debug().Time("expires_at", gs.token.ExpiresAt).Msg("Successfully obtained access token for Microsoft Graph API")
	return nil
}

func makeEmailRequest(from string, to []string, subject string, body []byte) *SendEmailRequest {
	var emailReq SendEmailRequest

	// Set the email request fields
	emailReq.Message.Subject = subject

	// Set the body
	emailReq.Message.Body.ContentType = "HTML"
	emailReq.Message.Body.Content = string(body)

	// Set the from and to addresses
	emailReq.Message.From.EmailAddress.Address = from
	emailReq.Message.ToRecipients = make([]EmailAddress, len(to))
	for i, addr := range to {
		emailReq.Message.ToRecipients[i] = EmailAddress{
			EmailAddress: Address{
				Address: addr,
			},
		}
	}

	return &emailReq
}

func (gs *GraphSender) sendEmailOnce(ctx context.Context, from string, to []string, subject string, body []byte) error {
	// Ensure the authentication token is valid before sending the email
	if err := gs.Authenticate(ctx); err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	// If a mailbox is configured, use it as the sender address instead of the provided 'from' parameter
	if gs.mailbox != "" {
		from = gs.mailbox
		log.Debug().
			Str("original", from).
			Str("mailbox", gs.mailbox).
			Msg("Using configured mailbox as sender address")
	}

	apiUrl := "https://graph.microsoft.com/v1.0/users/" + url.PathEscape(from) + "/sendMail"

	// Build the email request payload
	emailReq := makeEmailRequest(from, to, subject, body)
	emailReqData, err := json.Marshal(emailReq)
	if err != nil {
		return fmt.Errorf("failed to marshal email request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiUrl, bytes.NewReader(emailReqData))
	if err != nil {
		return err
	}

	// Set the request authorization and content type headers
	req.Header.Set("Authorization", "Bearer "+gs.token.Token)
	req.Header.Set("Content-Type", "application/json")

	// Send the email request
	resp, err := gs.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}
	defer resp.Body.Close()

	// Read the response body
	respData, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20)) // limit to 10MB
	if err != nil {
		return fmt.Errorf("failed to read email send response: %w", err)
	}

	// Check if the response status code indicates success
	if resp.StatusCode != http.StatusAccepted {
		var errorResp SendEmailErrorResponse
		if err := json.Unmarshal(respData, &errorResp); err != nil {
			log.Debug().Int("status_code", resp.StatusCode).Str("response", string(respData)).Msg("Invalid error response from email send")
			return fmt.Errorf("failed to send email: %s", resp.Status)
		}
		return fmt.Errorf("failed to send email (%s): %s", errorResp.Error.Code, errorResp.Error.Message)
	}

	return nil
}

func (gs *GraphSender) SendEmail(ctx context.Context, from string, to []string, subject string, body []byte) error {
	return utils.DoWithBackoff(ctx, func() error {
		return gs.sendEmailOnce(ctx, from, to, subject, body)
	}, gs.retries, gs.backoff)
}
