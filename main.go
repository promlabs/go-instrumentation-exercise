package main

import (
	"context"
	"flag"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/metric"
	sdk_metric "go.opentelemetry.io/otel/sdk/metric"
)

type demoAPI struct {
	requestDurations metric.Float64Histogram
}

func newDemoAPI(meter metric.Meter) *demoAPI {
	requestDurations, err := meter.Float64Histogram(
		"http.server.request.duration",
		metric.WithDescription("A histogram of HTTP request durations."),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1),
	)
	if err != nil {
		log.Fatalf("Failed to create histogram: %v", err)
	}

	return &demoAPI{
		requestDurations: requestDurations,
	}
}

func (a demoAPI) register(mux *http.ServeMux) {
	instr := func(fn http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			fn(w, r)

			a.requestDurations.Record(
				context.Background(),
				time.Since(start).Seconds(),
				metric.WithAttributes(
					attribute.String("http.route", r.URL.Path),
				),
			)
		}
	}

	mux.HandleFunc("/api/foo", instr(a.foo))
	mux.HandleFunc("/api/bar", instr(a.bar))
}

func (a demoAPI) foo(w http.ResponseWriter, r *http.Request) {
	log.Println("Handling foo...")

	// Simulate a random duration that the "foo" operation needs to be completed.
	time.Sleep(25*time.Millisecond + time.Duration(rand.Float64()*150)*time.Millisecond)

	w.Write([]byte("Handled foo"))
}

func (a demoAPI) bar(w http.ResponseWriter, r *http.Request) {
	log.Println("Handling bar...")
	// Simulate a random duration that the "bar" operation needs to be completed.
	time.Sleep(50*time.Millisecond + time.Duration(rand.Float64()*200)*time.Millisecond)

	w.Write([]byte("Handled bar"))
}

func periodicBackgroundTask(ctx context.Context, meter metric.Meter) {
	totalCount, err := meter.Int64Counter("background_task.runs", metric.WithDescription("The total number of background task runs."))
	if err != nil {
		log.Fatalf("Failed to create counter: %v", err)
	}
	failureCount, err := meter.Int64Counter("background_task.failures", metric.WithDescription("The total number of background task failures."))
	if err != nil {
		log.Fatalf("Failed to create counter: %v", err)
	}
	lastRun, err := meter.Float64Gauge("background_task.last_run_timestamp", metric.WithDescription("The Unix timestamp in seconds of the last background task run."), metric.WithUnit("s"))
	if err != nil {
		log.Fatalf("Failed to create gauge: %v", err)
	}
	lastSuccess, err := meter.Float64Gauge("background_task.last_success_timestamp", metric.WithDescription("The Unix timestamp in seconds of the last successful background task run."), metric.WithUnit("s"))
	if err != nil {
		log.Fatalf("Failed to create gauge: %v", err)
	}

	log.Println("Starting background task loop...")
	bgTicker := time.NewTicker(5 * time.Second)
	for {
		log.Println("Performing background task...")
		// Simulate a random duration that the background task needs to be completed.
		time.Sleep(1*time.Second + time.Duration(rand.Float64()*500)*time.Millisecond)

		// In case the batch job succeeds, we want to ensure that both lastRun and lastSuccess
		// have the exact same timestamp (for example, to enable equality comparisons in PromQL
		// to check whether the last run was successful).
		lastRunTimestamp := float64(time.Now().UnixNano()) / 1e9

		// Simulate the background task either succeeding or failing (with a 30% probability).
		if rand.Float64() > 0.3 {
			log.Println("Background task completed successfully.")
			lastSuccess.Record(ctx, lastRunTimestamp)
		} else {
			failureCount.Add(ctx, 1)
			log.Println("Background task failed.")
		}
		totalCount.Add(ctx, 1)
		lastRun.Record(ctx, lastRunTimestamp)

		select {
		case <-bgTicker.C:
		case <-ctx.Done():
			return
		}
	}
}

func setupOtel(ctx context.Context) func(context.Context) error {
	// Create an OTLP metric exporter that sends all metrics to the local Prometheus server.
	otlpMetricExporter, err := otlpmetrichttp.New(ctx, otlpmetrichttp.WithEndpointURL("http://localhost:9090/api/v1/otlp/v1/metrics"))
	if err != nil {
		log.Fatalf("Failed to create OTLP metric exporter: %v", err)
	}

	// Create a new MeterProvider with a reader that sends metrics to the OTLP exporter every 5 seconds.
	meterProvider := sdk_metric.NewMeterProvider(
		sdk_metric.WithReader(sdk_metric.NewPeriodicReader(otlpMetricExporter, sdk_metric.WithInterval(5*time.Second))),
	)

	// Set the global MeterProvider to the newly created MeterProvider.
	// This enables calls like otel.Meter() anywhere in the application rather than having to pass the MeterProvider around.
	otel.SetMeterProvider(meterProvider)

	return meterProvider.Shutdown
}

func main() {
	listenAddr := flag.String("web.listen-addr", ":8080", "The address to listen on for web requests.")
	flag.Parse()

	// Handle SIGINT (CTRL+C) gracefully.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)

	shutdownOtel := setupOtel(ctx)
	// Ensure that all metris are flushed properly when terminating the program.
	defer func() {
		log.Println("Shutting down OpenTelemetry...")
		if err := shutdownOtel(context.Background()); err != nil {
			log.Fatalln("Error shutting down OpenTelemetry:", err)
		}
	}()

	// Create a new Meter.
	meter := otel.Meter("otel-instrumentation-exercise")

	go periodicBackgroundTask(ctx, meter)

	api := newDemoAPI(meter)
	api.register(http.DefaultServeMux)

	// TODO: Shut down the HTTP server properly by context as well.
	go func() {
		log.Fatal(http.ListenAndServe(*listenAddr, nil))
	}()

	// Wait for interruption / first CTRL+C.
	<-ctx.Done()
	log.Println("Shutting down...")
	// Stop receiving further signal notifications as soon as possible.
	stop()
}
