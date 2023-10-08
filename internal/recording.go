package internal

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"vcr/internal/config"
	"vcr/internal/runtime"

	"github.com/rs/zerolog/log"
)

type Recorder struct {
	wg               *sync.WaitGroup
	runtime          runtime.ContainerRuntime
	programming      config.Programming
	containerConf    config.ContainerConfig
	startedRecording atomic.Bool
}

type VcrOperation func() error

func NewRecording(runtime runtime.ContainerRuntime, programming config.Programming,
	containerConf config.ContainerConfig, wg *sync.WaitGroup) (*Recorder, error) {

	return &Recorder{
		runtime:       runtime,
		programming:   programming,
		containerConf: containerConf,
		wg:            wg,
	}, nil
}

func (r *Recorder) GetYtpArgs() []string {
	now := time.Now()
	datePrefix := now.Format("20060102-1504")
	dirPrefix := ""
	if r.containerConf.Mount != nil {
		dirPrefix = r.containerConf.Mount.ContainerPath
	}
	return []string{
		"-o",
		fmt.Sprintf("%s/%s-%s.%%(ext)s", dirPrefix, datePrefix, strings.ToLower(r.programming.Name)),
		r.programming.Url,
	}
}

func (r *Recorder) Schedule(done chan bool) error {
	r.wg.Add(1)
	defer r.wg.Done()

	if !r.programming.IsUpcoming() {
		return errors.New("record date is in the past")
	}

	ctx, cancel := context.WithCancel(context.Background())
	errChan := make(chan error)
	go func() {
		log.Info().Msgf("Scheduling recording for %v", r.programming.Date)
		if err := r.scheduleRun(ctx, r.programming.Date, r.record); err != nil {
			errChan <- err
		}
	}()

	if r.programming.Until != nil && !r.programming.Until.IsZero() {
		go func() {
			log.Info().Msgf("Scheduling stop for %v", r.programming.Until)
			if err := r.scheduleRun(ctx, *r.programming.Until, r.stop); err != nil {
				errChan <- err
			}
		}()
	}

	select {
	case <-done:
		log.Warn().Msgf("recording: received done")
		cancel()
	case err := <-errChan:
		if err != nil {
			log.Error().Err(err).Msgf("Error on VcrOperation")
			cancel()
			return err
		}
	}

	return nil
}

func (r *Recorder) record() error {
	log.Info().Msg("Starting recording")

	r.containerConf.Args = r.GetYtpArgs()
	log.Info().Msgf("Starting recording, creating container using image %s with args %v", r.containerConf.Image, r.containerConf.Args)
	id, err := r.runtime.Run(context.Background(), r.programming.Name, r.containerConf)
	if err != nil {
		return err
	}

	r.startedRecording.Store(true)
	log.Info().Str("id", id).Msg("Started container")
	return nil
}

func (r *Recorder) stop() error {
	log.Info().Msgf("Stopping recording %s", r.programming.Name)
	id, err := r.runtime.FindByName(r.programming.Name)
	if err != nil {
		return err
	}
	log.Info().Msgf("Found container %s", id)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()
	return r.runtime.KillContainer(ctx, id)
}

func (r *Recorder) scheduleRun(ctx context.Context, until time.Time, run VcrOperation) error {
	timer := time.NewTimer(time.Until(until))
	r.wg.Add(1)
	defer func() {
		timer.Stop()
		r.wg.Done()
	}()

	select {
	case <-timer.C:
		return run()
	case <-ctx.Done():
		if r.startedRecording.Load() {
			return run()
		}
		log.Warn().Msg("Scheduled run cancelled")
		return nil
	}
}
