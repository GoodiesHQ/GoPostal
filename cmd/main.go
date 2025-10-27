package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/emersion/go-smtp"
	"github.com/goodieshq/gopostal/pkg/config"
	"github.com/goodieshq/gopostal/pkg/receiver"
	"github.com/joho/godotenv"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})
	godotenv.Load()

	// Load configuration from file
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load configuration")
	}

	// Ensure a valid token can be acquired before starting servers
	if !cfg.Send.AllowStartWithoutGraph {
		if err := cfg.Send.Sender.Authenticate(context.Background()); err != nil {
			log.Fatal().Err(err).Msg("Failed to initialize email sender")
		}
		log.Info().Msg("Email sender authenticated successfully")
	}

	// Create a list of SMTP servers based on the configuration
	servers := make([]*smtp.Server, len(cfg.Recv.Listeners))
	var wg sync.WaitGroup

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// Create a new listener for each configured listener
	for i := range cfg.Recv.Listeners {
		lcfg := cfg.Recv.Listeners[i]
		listener := receiver.NewListener(ctx, &lcfg, &cfg.Send, &cfg.Recv.RecvGlobalConfig)

		// create a new SMTP server
		server := smtp.NewServer(listener)
		server.Addr = fmt.Sprintf(":%d", lcfg.Port)
		server.Domain = cfg.Recv.Domain
		server.TLSConfig = lcfg.TLSConfig

		servers[i] = server
		wg.Add(1)

		type run func() error
		var runner run

		go func(srv *smtp.Server, lc config.ListenerConfig) {
			defer wg.Done()
			log.Info().Msgf("Starting SMTP (%s) server '%s' on %s", lc.Type, lc.Name, srv.Addr)
			switch lc.Type {
			case config.ListenerSMTP:
				log.Warn().Str("server", lc.Name).Msg("SMTP listener does not use TLS, allowing insecure authentication. This is not recommended for production environments.")
				srv.AllowInsecureAuth = true
				runner = srv.ListenAndServe
			case config.ListenerSMTPS:
				srv.AllowInsecureAuth = false
				srv.TLSConfig = lc.TLSConfig
				runner = srv.ListenAndServeTLS
			case config.ListenerSTARTTLS:
				srv.AllowInsecureAuth = false
				srv.TLSConfig = lc.TLSConfig
				runner = srv.ListenAndServe
			}
			if err := runner(); err != nil {
				log.Error().Err(err).Msgf("SMTP (%s) server '%s' at %s stopped with error", lc.Type, lc.Name, srv.Addr)
			}
		}(server, lcfg)
	}

	<-ctx.Done()
	log.Info().Msg("Shutdown signal received, closing servers...")

	for _, srv := range servers {
		if srv != nil {
			log.Info().Msgf("Shutting down server at %s", srv.Addr)
			if err := srv.Close(); err != nil {
				log.Error().Err(err).Msgf("Error shutting down server at %s", srv.Addr)
			}
		}
	}

	wg.Wait()
	log.Info().Msg("All servers have been shut down. Exiting.")
}
