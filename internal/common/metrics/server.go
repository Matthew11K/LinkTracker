package metrics

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

//nolint:revive // Имя MetricsServer используется для ясности
type MetricsServer struct {
	server *http.Server
	logger *slog.Logger
	port   int
}

func NewMetricsServer(port int, logger *slog.Logger) *MetricsServer {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
	}

	return &MetricsServer{
		server: server,
		logger: logger,
		port:   port,
	}
}

func (s *MetricsServer) Start(ctx context.Context) error {
	s.logger.Info("Запуск сервера метрик",
		"port", s.port,
		"endpoint", "/metrics",
	)

	go func() {
		<-ctx.Done()

		shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		if err := s.server.Shutdown(shutdownCtx); err != nil {
			s.logger.Error("Ошибка при остановке сервера метрик", "error", err)
		} else {
			s.logger.Info("Сервер метрик успешно остановлен")
		}
	}()

	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("ошибка запуска сервера метрик: %w", err)
	}

	return nil
}

func (s *MetricsServer) Stop(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}
