package sender

import "time"

type SendEmailRequest struct {
	Message EmailMessage `json:"message"`
}

type SendEmailErrorResponse struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

type EmailMessage struct {
	Subject      string         `json:"subject"`
	Body         EmailBody      `json:"body"`
	From         EmailAddress   `json:"from"`
	ToRecipients []EmailAddress `json:"toRecipients"`
}

type EmailBody struct {
	ContentType string `json:"contentType"`
	Content     string `json:"content"`
}

type EmailAddress struct {
	EmailAddress Address `json:"emailAddress"`
}

type Address struct {
	Address string `json:"address"`
}

type AuthTokenRequest struct {
	GrantType    string `json:"grant_type"`
	Scope        string `json:"scope"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

func NewAuthTokenRequest(clientID, clientSecret string) *AuthTokenRequest {
	return &AuthTokenRequest{
		GrantType:    "client_credentials",
		Scope:        "https://graph.microsoft.com/.default",
		ClientID:     clientID,
		ClientSecret: clientSecret,
	}
}

type AuthTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

type AuthToken struct {
	Token     string
	ExpiresAt time.Time
}
