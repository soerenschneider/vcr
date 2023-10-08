package main

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
	"vcr/internal"
	"vcr/internal/config"
	"vcr/internal/dbs"
	"vcr/internal/ports/http"
	"vcr/internal/runtime"

	"github.com/rs/zerolog/log"
)

type deps struct {
	runtime runtime.ContainerRuntime
	db      dbs.Db
}

func run(deps *deps, conf config.ContainerConfig) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*5)
	log.Info().Msg("Pulling image...")
	defer cancel()
	if err := deps.runtime.Pull(ctx, conf.Image); err != nil {
		log.Error().Err(err).Msg("could not pull image")
	}
	log.Info().Msg("Done pulling image")

	vcr, err := internal.NewVcr(deps.db, deps.runtime, conf)
	if err != nil {
		log.Fatal().Err(err).Msg("can not build vcr")
	}

	server, err := http.New(":9999", vcr)
	if err != nil {
		log.Fatal().Err(err).Msg("could not build http server")
	}

	ctx, cancel = context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}
	go server.Listen(ctx, wg)
	go vcr.ControlLoop(ctx, wg)

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	<-c
	log.Warn().Msg("Received signal")
	cancel()
	log.Warn().Msg("Waiting")
	wg.Wait()
}

func main() {
	deps := &deps{}
	var err error

	deps.runtime, err = buildRuntime()
	if err != nil {
		log.Fatal().Err(err).Msg("could not build runtime")
	}

	deps.db, err = buildDb()
	if err != nil {
		log.Fatal().Err(err).Msg("could not build db")
	}

	conf, err := config.GetContainerConfig()
	if err != nil {
		log.Fatal().Err(err).Msg("could not get config")
	}

	if err := config.Validate(conf); err != nil {
		log.Fatal().Err(err).Msg("invalid config")
	}

	run(deps, conf)
}
