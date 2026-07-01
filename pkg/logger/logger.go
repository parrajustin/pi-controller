package logger

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
)

type customHandler struct {
	slog.Handler
	binaryName string
	mu         *sync.Mutex
}

func (h *customHandler) Handle(ctx context.Context, r slog.Record) error {
	dateStr := r.Time.Format("2006/01/02 15:04:15")
	msg := fmt.Sprintf("%s (%s) %s: %s\n", dateStr, h.binaryName, r.Level.String(), r.Message)

	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := os.Stdout.WriteString(msg)
	return err
}

// Init configures the default slog logger with our custom format.
func Init(binaryName string) {
	h := &customHandler{
		Handler:    slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{}),
		binaryName: binaryName,
		mu:         &sync.Mutex{},
	}
	logger := slog.New(h)
	slog.SetDefault(logger)
}

// Fatalf logs an error level message using slog, then calls os.Exit(1).
func Fatalf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	slog.Error(msg)
	os.Exit(1)
}
