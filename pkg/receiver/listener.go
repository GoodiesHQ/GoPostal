package receiver

import (
	"context"
	"net"

	"github.com/emersion/go-smtp"
	"github.com/goodieshq/gopostal/pkg/config"
	"github.com/goodieshq/gopostal/pkg/errs"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

type Listener struct {
	ctx            context.Context
	configListener *config.ListenerConfig
	configSender   *config.SendConfig
	configGlobal   *config.RecvGlobalConfig
}

// Create a new listener from the provided listener and receiver global configuration.
func NewListener(ctx context.Context, configListener *config.ListenerConfig, configSender *config.SendConfig, configGlobal *config.RecvGlobalConfig) *Listener {
	return &Listener{
		ctx:            ctx,
		configListener: configListener,
		configSender:   configSender,
		configGlobal:   configGlobal,
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
		ctx:            l.ctx,
		log:            sessionLogger,
		id:             id,
		configListener: l.configListener,
		configSender:   l.configSender,
		configGlobal:   l.configGlobal,
		remote:         raddr,
		authenticated:  false,
	}, nil
}
