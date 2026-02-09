package worker

import (
	"context"
	"sync"
	"time"
	"transaction-processor/internal/service"

	"github.com/rs/zerolog"
)

type CancellationWorker struct {
	service  service.CancellationService
	interval time.Duration
	logger   zerolog.Logger
	stopChan chan struct{}
	wg       *sync.WaitGroup
}

func NewCancellationWorker(svc service.CancellationService, interval time.Duration, logger zerolog.Logger) *CancellationWorker {
	return &CancellationWorker{
		service:  svc,
		interval: interval,
		logger:   logger,
		stopChan: make(chan struct{}),
		wg:       &sync.WaitGroup{},
	}
}

func (w *CancellationWorker) Start(ctx context.Context) {
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		ticker := time.NewTicker(w.interval)
		defer ticker.Stop()

		w.logger.Info().Dur("interval", w.interval).Msg("Cancellation worker started")

		for {
			select {
			case <-ticker.C:
				w.logger.Debug().Msg("Running cancellation task")
				err := w.service.ProcessOddRecordCancellation(ctx)
				if err != nil {
					w.logger.Error().Err(err).Msg("Failed to run cancellation task")
				}
			case <-w.stopChan:
				w.logger.Info().Msg("Cancellation worker stopping")
				return
			case <-ctx.Done():
				w.logger.Info().Msg("Cancellation worker stopping (context done)")
				return
			}
		}
	}()
}

func (w *CancellationWorker) Stop() {
	close(w.stopChan)
	w.wg.Wait()
}
