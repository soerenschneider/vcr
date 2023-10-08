package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
	"vcr/internal"
	"vcr/internal/ports"

	"github.com/rs/zerolog/log"
	"go.uber.org/multierr"
)

type Webhook struct {
	address string

	vcr *internal.Vcr

	certFile string
	keyFile  string
}

type WebhookOpts func(*Webhook) error

func New(address string, vcr *internal.Vcr, opts ...WebhookOpts) (*Webhook, error) {
	if len(address) == 0 {
		return nil, errors.New("empty address provided")
	}

	if vcr == nil {
		return nil, errors.New("no vcr provided")
	}

	w := &Webhook{
		address: address,
		vcr:     vcr,
	}

	var errs error
	for _, opt := range opts {
		if err := opt(w); err != nil {
			errs = multierr.Append(errs, err)
		}
	}

	return w, errs
}

func (w *Webhook) IsTLSConfigured() bool {
	return len(w.certFile) > 0 && len(w.keyFile) > 0
}

func (s *Webhook) add(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	data, err := io.ReadAll(r.Body)
	if err != nil {
		log.Error().Err(err).Msg("can not read from body")
		http.Error(w, "Internal server error", 500)
		return
	}
	_ = r.Body.Close()

	p := ports.AddProgrammingRequest{}
	if err := json.Unmarshal(data, &p); err != nil {
		http.Error(w, "Ops", http.StatusBadRequest)
		return
	}

	if err := s.vcr.AddProgramming(p); err != nil {
		if errors.Is(err, internal.ErrValidationError) {
			http.Error(w, err.Error(), http.StatusBadRequest)
		} else {
			http.Error(w, "Ops", http.StatusInternalServerError)
		}
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (s *Webhook) delete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	data, err := io.ReadAll(r.Body)
	if err != nil {
		log.Error().Err(err).Msg("can not read from body")
		http.Error(w, "Internal server error", 500)
		return
	}
	_ = r.Body.Close()

	p := ports.DeleteProgrammingRequest{}
	if err := json.Unmarshal(data, &p); err != nil {
		http.Error(w, "Can not marshal json", http.StatusBadRequest)
		return
	}

	if err := s.vcr.DeleteProgramming(p); err != nil {
		if errors.Is(err, internal.ErrValidationError) {
			http.Error(w, err.Error(), http.StatusBadRequest)
		} else {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Webhook) get(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	data, err := io.ReadAll(r.Body)
	if err != nil {
		log.Error().Err(err).Msg("can not read from body")
		http.Error(w, "Internal server error", 500)
		return
	}
	_ = r.Body.Close()

	p := ports.GetProgrammingRequest{}
	if err := json.Unmarshal(data, &p); err != nil {
		http.Error(w, "Ops", http.StatusBadRequest)
		return
	}

	programming, err := s.vcr.GetProgrammings(p)
	if err != nil {
		if errors.Is(err, internal.ErrValidationError) {
			http.Error(w, err.Error(), http.StatusBadRequest)
		} else {
			http.Error(w, "Ops", http.StatusInternalServerError)
		}
		return
	}

	json.NewEncoder(w).Encode(programming)
}

func (s *Webhook) list(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	p, err := s.vcr.ListProgrammings()
	if err != nil {
		log.Error().Err(err).Msg("can not list programmings")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(p)
}

func (w *Webhook) Listen(ctx context.Context, wg *sync.WaitGroup) error {
	wg.Add(1)
	defer wg.Done()

	mux := http.NewServeMux()
	mux.HandleFunc("/add", w.add)
	mux.HandleFunc("/delete", w.delete)
	mux.HandleFunc("/list", w.list)
	mux.HandleFunc("/get", w.get)

	server := http.Server{
		Addr:              w.address,
		Handler:           mux,
		ReadTimeout:       3 * time.Second,
		ReadHeaderTimeout: 3 * time.Second,
		WriteTimeout:      3 * time.Second,
		IdleTimeout:       30 * time.Second,
	}

	errChan := make(chan error)
	go func() {
		if w.IsTLSConfigured() {
			if err := server.ListenAndServeTLS(w.certFile, w.keyFile); err != nil && !errors.Is(err, http.ErrServerClosed) {
				errChan <- fmt.Errorf("can not start http server: %w", err)
			}
		} else {
			if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				errChan <- fmt.Errorf("can not start http server: %w", err)
			}
		}
	}()

	select {
	case <-ctx.Done():
		log.Info().Msg("Stopping http server")
		err := server.Shutdown(ctx)
		return err
	case err := <-errChan:
		return err
	}
}
