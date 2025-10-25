package receiver

import (
	"io"
	"net"
	"slices"
	"strings"

	"github.com/emersion/go-sasl"
	"github.com/emersion/go-smtp"
	"github.com/goodieshq/gopostal/pkg/config"
	"github.com/goodieshq/gopostal/pkg/errs"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type Listener struct {
	config       *config.ListenerConfig
	configGlobal *config.RecvGlobalConfig
}

// Create a new listener from the provided listener and receiver global configuration.
func NewListener(config *config.ListenerConfig, configGlobal *config.RecvGlobalConfig) *Listener {
	return &Listener{
		config:       config,
		configGlobal: configGlobal,
	}
}

// Create a new SMTP session for each incoming connection. This method checks if the remote address is allowed based on the configuration and returns a new Session object if it is, or an error if it is not.
func (l *Listener) NewSession(c *smtp.Conn) (smtp.Session, error) {
	raddr := c.Conn().RemoteAddr()
	allowed := false
	ta, ok := raddr.(*net.TCPAddr)
	if !ok {
		log.Warn().Str("remote", raddr.String()).Msg("Remote address is not a TCP address, cannot check against allowed networks")
		return nil, errs.ErrSourceIPInvalid
	}
	if len(l.configGlobal.AllowedNets) > 0 {
		for _, a := range l.configGlobal.AllowedNets {
			if a.Contains(ta.IP) {
				allowed = true
				break
			}
		}
	} else {
		allowed = true
	}

	if !allowed {
		log.Warn().Str("remote", raddr.String()).Msg("Remote address is not allowed by configuration")
		return nil, errs.ErrSourceIPDisallowed
	}

	id, err := uuid.NewRandom()
	if err != nil {
		log.Error().Err(err).Msg("Failed to generate session ID")
		return nil, err
	}

	sessionLogger := log.With().
		Str("session_id", id.String()).
		Str("remote_addr", raddr.String()).
		Logger()

	return &Session{
		log:           sessionLogger,
		id:            id,
		config:        l.config,
		configGlobal:  l.configGlobal,
		remote:        raddr,
		authenticated: false,
	}, nil
}

// Session is a struct that implements the smtp.Session interface.
type Session struct {
	log           zerolog.Logger
	id            uuid.UUID
	config        *config.ListenerConfig
	configGlobal  *config.RecvGlobalConfig
	remote        net.Addr
	authenticated bool
	emailFrom     string
	emailTo       []string
	emailBody     []byte
}

func (s *Session) AuthMechanisms() []string {
	var mechanisms []string

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

func (s *Session) Auth(mech string) (sasl.Server, error) {
	mechs := s.AuthMechanisms()
	if !slices.Contains(mechs, mech) {
		s.log.Debug().Msgf("Requested authentication mechanism '%s' is not supported", mech)
		return nil, smtp.ErrAuthUnsupported
	}

	switch mech {
	case sasl.Plain:
		return sasl.NewPlainServer(func(identity, username, password string) error {
			log := s.log.With().Str("username", username).Logger()

			if s.configGlobal.Authenticator.Check(username, password) {
				s.authenticated = true
				log.Info().Msg("User authenticated successfully")
				return nil
			}
			log.Info().Msg("Failed to authenticate user")
			return smtp.ErrAuthFailed
		}), nil
	default:
		return sasl.NewAnonymousServer(func(identity string) error {
			s.log.Info().Str("identity", identity).Msg("Authenticating anonymous user")
			s.authenticated = true
			return nil
		}), nil
	}
}

func (s *Session) Mail(from string, _ *smtp.MailOptions) error {
	if s.config.RequireAuth && !s.authenticated {
		return smtp.ErrAuthRequired
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

func (s *Session) Rcpt(to string, _ *smtp.RcptOptions) error {
	if s.config.RequireAuth && !s.authenticated {
		return smtp.ErrAuthRequired
	}

	to = strings.Trim(to, "<>")
	if len(to) == 0 {
		s.log.Warn().Msg("Mail to address is empty")
		return errs.ErrInvalidEmail
	}

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

	if len(s.emailTo) >= s.configGlobal.Limits.MaxRecipients {
		s.log.Warn().Int("max_recipients", s.configGlobal.Limits.MaxRecipients).Msg("Too many recipients")
		return errs.ErrTooManyRecipients
	}

	s.emailTo = append(s.emailTo, to)
	s.log.Info().Strs("to", s.emailTo).Msg("Mail to")
	return nil
}

func (s *Session) Data(r io.Reader) error {
	if s.config.RequireAuth && !s.authenticated {
		return smtp.ErrAuthRequired
	}

	reader := io.LimitReader(r, int64(s.configGlobal.Limits.MaxSize)+1) // prevent reading more than max size + 1 byte
	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	if len(data) > s.configGlobal.Limits.MaxSize {
		s.log.Warn().Int("max_size", s.configGlobal.Limits.MaxSize).Int("data_size", len(data)).Msg("Email data exceeds maximum allowed size")
		return smtp.ErrDataTooLarge
	}

	s.emailBody = data
	s.log.Info().Int("bytes", len(s.emailBody)).Msg("Received email data")

	return nil
}

func (s *Session) Reset() {
	s.emailFrom = ""
	s.emailTo = []string{}
	s.emailBody = nil
}

func (s *Session) Logout() error {
	return nil
}
