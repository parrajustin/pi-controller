package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// InitTelemetry bootstraps the OpenTelemetry pipeline.
// It returns a shutdown function to be called before exit.
func InitTelemetry(ctx context.Context) (func(context.Context) error, error) {
	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") == "" {
		// Telemetry endpoint not configured; fallback to simple stdout logger
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, nil)))
		return func(context.Context) error { return nil }, nil
	}

	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			"",
			semconv.ServiceName("display_server"),
		),
	)
	if err != nil {
		return nil, err
	}

	// 1. Traces
	traceExp, err := otlptracehttp.New(ctx)
	if err != nil {
		return nil, err
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExp),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// 2. Metrics
	metricExp, err := otlpmetrichttp.New(ctx)
	if err != nil {
		return nil, err
	}
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExp, sdkmetric.WithInterval(10*time.Second))),
		sdkmetric.WithResource(res),
	)
	otel.SetMeterProvider(mp)

	// 3. Logs
	logExp, err := otlploghttp.New(ctx)
	if err != nil {
		return nil, err
	}
	lp := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(logExp)),
		sdklog.WithResource(res),
	)
	global.SetLoggerProvider(lp)

	// Configure default slog to use OTel bridge
	otelHandler := otelslog.NewHandler("display_server")
	stdoutHandler := slog.NewTextHandler(os.Stdout, nil)
	logger := slog.New(&TeeHandler{h1: stdoutHandler, h2: otelHandler})
	slog.SetDefault(logger)

	shutdown := func(c context.Context) error {
		var err error
		if e := tp.Shutdown(c); e != nil {
			err = e
		}
		if e := mp.Shutdown(c); e != nil {
			err = e
		}
		if e := lp.Shutdown(c); e != nil {
			err = e
		}
		return err
	}

	return shutdown, nil
}

type TeeHandler struct {
	h1 slog.Handler
	h2 slog.Handler
}

func (t *TeeHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return t.h1.Enabled(ctx, level) || t.h2.Enabled(ctx, level)
}

func (t *TeeHandler) Handle(ctx context.Context, r slog.Record) error {
	var err1, err2 error
	if t.h1.Enabled(ctx, r.Level) {
		err1 = t.h1.Handle(ctx, r)
	}
	if t.h2.Enabled(ctx, r.Level) {
		err2 = t.h2.Handle(ctx, r)
	}
	if err1 != nil {
		return err1
	}
	return err2
}

func (t *TeeHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &TeeHandler{h1: t.h1.WithAttrs(attrs), h2: t.h2.WithAttrs(attrs)}
}

func (t *TeeHandler) WithGroup(name string) slog.Handler {
	return &TeeHandler{h1: t.h1.WithGroup(name), h2: t.h2.WithGroup(name)}
}
