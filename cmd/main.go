package main

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/emersion/go-smtp"
	"github.com/goodieshq/gopostal/pkg/config"
	"github.com/goodieshq/gopostal/pkg/receiver"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})

	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load configuration")
	}
	if cfg.Recv.Limits.Timeout == time.Second*30 {
		log.Info().Msg("Good!")
	}

	servers := make([]*smtp.Server, len(cfg.Recv.Listeners))

	var wg sync.WaitGroup

	// listen for ctrl+c to gracefully shutdown servers
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt)
		<-c
		log.Info().Msg("Shutting down servers...")
		for _, server := range servers {
			if server != nil {
				if err := server.Close(); err != nil {
					log.Error().Err(err).Msg("Failed to close SMTP server")
				}
			}
		}
	}()

	for _, lcfg := range cfg.Recv.Listeners {
		listener := receiver.NewListener(&lcfg, &cfg.Recv.RecvGlobalConfig)
		server := smtp.NewServer(listener)
		server.Addr = fmt.Sprintf(":%d", lcfg.Port)
		server.Domain = cfg.Recv.Domain
		server.AllowInsecureAuth = true
		servers = append(servers, server)
		wg.Add(1)
		go func() {
			defer wg.Done()
			log.Info().Msgf("Starting SMTP server at %s", server.Addr)
			if err := server.ListenAndServe(); err != nil {
				log.Error().Err(err).Msg("Failed to run SMTP server")
			}
		}()
	}
	wg.Wait()
	log.Info().Msg("All servers have been shut down. Exiting.")
}
