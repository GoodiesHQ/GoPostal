package receiver

import (
	"bytes"
	"context"
	"io"
	"mime"
	"net"
	"net/mail"
	"regexp"
	"slices"
	"strings"

	"github.com/emersion/go-sasl"
	"github.com/emersion/go-smtp"
	"github.com/goodieshq/gopostal/pkg/config"
	"github.com/goodieshq/gopostal/pkg/errs"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// Session is a struct that implements the smtp.Session interface.
type Session struct {
	ctx            context.Context
	log            zerolog.Logger
	id             uuid.UUID
	configListener *config.ListenerConfig
	configSender   *config.SendConfig
	configGlobal   *config.RecvGlobalConfig
	remote         net.Addr
	authenticated  bool
	emailSubject   string
	emailFrom      string
	emailTo        []string
	emailBody      []byte
}

// Return the allowed authentication mechanisms for this service
func (s *Session) AuthMechanisms() []string {
	var mechanisms []string

	if !s.configListener.RequireAuth {
		// If authentication is not required, return an empty list
		return mechanisms
	}

	switch s.configGlobal.Auth.Mode {
	case config.AuthDisabled:
		// No authentication required, so no mechanisms to offer
	case config.AuthPlainAny:
		fallthrough
	case config.AuthPlain:
		mechanisms = append(mechanisms, sasl.Plain)
	case config.AuthAnonymous:
		mechanisms = append(mechanisms, sasl.Anonymous)
	default:
		s.log.Warn().Str("auth_mode", string(s.configGlobal.Auth.Mode)).Msg("Unsupported authentication mode configured")
	}

	return mechanisms
}

func (s *Session) authPlain(identity, username, password string) error {
	log := s.log.With().Str("username", username).Logger()

	if s.configGlobal.Authenticator.Check(username, password) {
		s.authenticated = true
		log.Info().Msg("User authenticated successfully")
		return nil
	}
	log.Info().Msg("Failed to authenticate user")
	return smtp.ErrAuthFailed
}

func (s *Session) authAnonymous(identity string) error {
	s.log.Info().Str("identity", identity).Msg("Authenticating anonymous user")
	s.authenticated = true
	return nil
}

func (s *Session) Auth(mech string) (sasl.Server, error) {
	// Check if the context has been cancelled
	if s.ctx.Err() != nil {
		s.log.Warn().Msg("Session context has been cancelled")
		return nil, smtp.ErrServerClosed
	}

	// Check if the requested mechanism is supported
	mechs := s.AuthMechanisms()
	if !slices.Contains(mechs, mech) {
		s.log.Debug().Msgf("Requested authentication mechanism '%s' is not supported", mech)
		return nil, smtp.ErrAuthUnsupported
	}

	switch mech {
	case sasl.Anonymous:
		return sasl.NewAnonymousServer(s.authAnonymous), nil
	case sasl.Plain:
		return sasl.NewPlainServer(s.authPlain), nil
	}

	return nil, smtp.ErrAuthUnsupported
}

// Mail handles the MAIL command from the SMTP client.
func (s *Session) Mail(from string, _ *smtp.MailOptions) error {
	if s.configListener.RequireAuth && !s.authenticated {
		return smtp.ErrAuthRequired
	}

	// Check if the context has been cancelled
	if s.ctx.Err() != nil {
		s.log.Warn().Msg("Session context has been cancelled")
		return smtp.ErrServerClosed
	}

	from = strings.Trim(from, "<>")
	if len(from) == 0 {
		s.log.Warn().Msg("Mail from address is empty")
		return errs.ErrInvalidEmail
	}

	if len(s.configGlobal.ValidFrom.Addresses) > 0 || len(s.configGlobal.ValidFrom.Domains) > 0 {
		valid := false
		for _, addr := range s.configGlobal.ValidFrom.Addresses {
			if strings.EqualFold(from, addr) {
				valid = true
				break
			}
		}
		for _, dom := range s.configGlobal.ValidFrom.Domains {
			if strings.HasSuffix(strings.ToLower(from), "@"+strings.ToLower(dom)) {
				valid = true
				break
			}
		}
		if !valid {
			s.log.Warn().Str("from", from).Msg("Sender address is not allowed by configuration")
			return errs.ErrFromDisallowed
		}
	}
	s.emailFrom = from
	s.log.Info().Str("from", from).Msg("Mail from")
	return nil
}

// Rcpt handles the RCPT command from the SMTP client.
func (s *Session) Rcpt(to string, _ *smtp.RcptOptions) error {
	if s.configListener.RequireAuth && !s.authenticated {
		return smtp.ErrAuthRequired
	}

	// Check if the context has been cancelled
	if s.ctx.Err() != nil {
		s.log.Warn().Msg("Session context has been cancelled")
		return smtp.ErrServerClosed
	}

	// Trim angle brackets from the email address if present
	to = strings.Trim(to, "<>")
	if len(to) == 0 {
		s.log.Warn().Msg("Mail to address is empty")
		return errs.ErrInvalidEmail
	}

	// If there are any address/domain restrictions, enforce them
	if len(s.configGlobal.ValidTo.Addresses) > 0 || len(s.configGlobal.ValidTo.Domains) > 0 {
		valid := false
		for _, addr := range s.configGlobal.ValidTo.Addresses {
			if strings.EqualFold(to, addr) {
				valid = true
				break
			}
		}
		for _, dom := range s.configGlobal.ValidTo.Domains {
			if strings.HasSuffix(strings.ToLower(to), "@"+strings.ToLower(dom)) {
				valid = true
				break
			}
		}
		if !valid {
			s.log.Warn().Str("to", to).Msg("Recipient address is not allowed by configuration")
			return errs.ErrToDisallowed
		}
	}

	// Enforce maximum recipients limit
	if len(s.emailTo) >= s.configGlobal.Limits.MaxRecipients {
		s.log.Warn().Int("max_recipients", s.configGlobal.Limits.MaxRecipients).Msg("Too many recipients")
		return errs.ErrTooManyRecipients
	}

	// Add the recipient to the list
	s.emailTo = append(s.emailTo, to)
	s.log.Info().Strs("to", s.emailTo).Msg("Added recipient successfully")
	return nil
}

// Data handles the DATA command from the SMTP client.
func (s *Session) Data(r io.Reader) error {
	if s.configListener.RequireAuth && !s.authenticated {
		return smtp.ErrAuthRequired
	}

	// Check if the context has been cancelled
	if s.ctx.Err() != nil {
		s.log.Warn().Msg("Session context has been cancelled")
		return smtp.ErrServerClosed
	}

	// Read the email data with an enforced size limit
	reader := io.LimitReader(r, int64(s.configGlobal.Limits.MaxSize)+1) // prevent reading more than max size + 1 byte
	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}

	// Enforce maximum email size limit
	if len(data) > s.configGlobal.Limits.MaxSize {
		s.log.Warn().Int("max_size", s.configGlobal.Limits.MaxSize).Int("data_size", len(data)).Msg("Email data exceeds maximum allowed size")
		return smtp.ErrDataTooLarge
	}

	// Parse email message as RFC5322 to extract a clean body
	if msg, err := mail.ReadMessage(bytes.NewReader(data)); err != nil {
		s.log.Debug().Err(err).Msg("Failed to parse email message as RFC5322")
		re := regexp.MustCompile(`(?mi)^Subject:\s*(.+)$`)
		if m := re.FindSubmatch(data); m != nil {
			subject := string(bytes.TrimSpace(m[1]))
			if decoded, err := (&mime.WordDecoder{}).DecodeHeader(subject); err == nil {
				subject = decoded
			}
			s.emailSubject = subject
			data = re.ReplaceAll(data, []byte{})
		} else {
			s.emailSubject = "(no subject)"
		}
		s.emailBody = data
	} else {
		s.log.Debug().Msg("Parsed email message as RFC5322 successfully")
		subject := msg.Header.Get("Subject")
		if subject != "" {
			if decoded, err := (&mime.WordDecoder{}).DecodeHeader(subject); err == nil {
				subject = decoded
			}
		} else {
			subject = "(no subject)"
		}

		bodyBytes, err := io.ReadAll(msg.Body)
		if err != nil {
			s.log.Warn().Err(err).Msg("Failed to read email body, using raw data instead")
			s.emailBody = data
		} else {
			s.emailBody = bodyBytes
		}

		s.emailSubject = subject
	}

	s.log.Info().
		Str("subject", s.emailSubject).
		Str("from", s.emailFrom).
		Strs("to", s.emailTo).
		Msg("Sending email using configured sender")

	err = s.configSender.Sender.SendEmail(
		s.ctx,
		s.emailFrom,
		s.emailTo,
		s.emailSubject,
		s.emailBody,
	)
	if err != nil {
		s.log.Error().Err(err).Msg("Failed to send email")
		return err
	}

	return nil
}

// Reset resets the session state for a new email transaction.
func (s *Session) Reset() {
	s.emailFrom = ""
	s.emailTo = []string{}
	s.emailBody = nil
}

// Logout handles the logout of the SMTP session.
func (s *Session) Logout() error {
	return nil
}
