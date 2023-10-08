package internal

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
	"vcr/internal/config"
	"vcr/internal/dbs"
	"vcr/internal/ports"
	"vcr/internal/runtime"

	"github.com/rs/zerolog/log"
)

var ErrValidationError = errors.New("error validating input")

type ScheduledRecording struct {
	done      chan bool
	recording *Recorder
}

type Vcr struct {
	db           dbs.Db
	programmings map[string]ScheduledRecording

	wg            *sync.WaitGroup
	runtime       runtime.ContainerRuntime
	containerConf config.ContainerConfig
}

func NewVcr(db dbs.Db, runtime runtime.ContainerRuntime, containerConf config.ContainerConfig) (*Vcr, error) {
	if db == nil {
		return nil, errors.New("no db supplied")
	}

	if runtime == nil {
		return nil, errors.New("no runtime supplied")
	}

	return &Vcr{
		db:            db,
		runtime:       runtime,
		containerConf: containerConf,

		programmings: map[string]ScheduledRecording{},
		wg:           &sync.WaitGroup{},
	}, nil
}

func (a *Vcr) ControlLoop(ctx context.Context, wg *sync.WaitGroup) {
	ticker := time.NewTicker(1 * time.Minute)

	wg.Add(1)
	defer func() {
		log.Info().Msgf("vcr: defer wg.Done()")
		wg.Done()
		ticker.Stop()
	}()

	for {
		select {
		case <-ctx.Done():
			log.Info().Msgf("app: received done, sending signals to scheduled runs")
			for _, recording := range a.programmings {
				recording.done <- true
				close(recording.done)
			}
			log.Info().Msgf("Closed")
			return
		case <-ticker.C:
			programmings, err := a.db.List()
			if err != nil {
				log.Error().Err(err).Msg("could not get a list of programmings")
				continue
			}

			for _, programming := range programmings {
				if programming.IsUpcoming() && programming.Date.Sub(time.Now()) < 5*time.Minute {
					_, ok := a.programmings[programming.Name]
					if ok {
						continue
					}
					log.Info().Msgf("Creating new recording for programming '%s'", programming.Name)
					recording, err := NewRecording(a.runtime, programming, a.containerConf, a.wg)
					if err != nil {
						log.Error().Err(err).Msg("could not create recording")
					}

					s := ScheduledRecording{
						recording: recording,
						done:      make(chan bool, 1),
					}
					a.programmings[programming.Name] = s
					go func() {
						if err := recording.Schedule(s.done); err != nil {
							log.Error().Err(err).Msg("scheduling failed")
						}
					}()
				}
			}
		}
	}
}

func (a *Vcr) AddProgramming(req ports.AddProgrammingRequest) error {
	if err := config.Validate(req); err != nil {
		return fmt.Errorf("%w: %v", ErrValidationError, err)
	}

	p, err := req.ToProgramming()
	if err != nil {
		return err
	}

	return a.db.Add(p)
}

func (a *Vcr) GetProgrammings(req ports.GetProgrammingRequest) (*config.Programming, error) {
	if err := config.Validate(req); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrValidationError, err)
	}

	return a.db.Find(req.Name)
}

func (a *Vcr) ListProgrammings() ([]config.Programming, error) {
	return a.db.List()
}

func (a *Vcr) DeleteProgramming(req ports.DeleteProgrammingRequest) error {
	if err := config.Validate(req); err != nil {
		return fmt.Errorf("%w: %v", ErrValidationError, err)
	}

	return a.db.Delete(req.Name)
}
