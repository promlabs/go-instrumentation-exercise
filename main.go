package main

import (
	"flag"
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type demoAPI struct {
	requestDurations *prometheus.HistogramVec
}

func newDemoAPI(reg prometheus.Registerer) *demoAPI {
	requestDurations := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "demo_api_http_request_duration_seconds",
		Help:    "A histogram of the demo API request durations in seconds.",
		Buckets: prometheus.LinearBuckets(.05, .025, 10),
	}, []string{"handler"})
	reg.MustRegister(requestDurations)

	return &demoAPI{
		requestDurations: requestDurations,
	}
}

func (a demoAPI) register(mux *http.ServeMux) {
	instr := func(handler string, fn http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			timer := prometheus.NewTimer(a.requestDurations.WithLabelValues(handler))
			fn(w, r)
			timer.ObserveDuration()
		}
	}

	mux.HandleFunc("/api/foo", instr("foo", a.foo))
	mux.HandleFunc("/api/bar", instr("bar", a.bar))
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

func periodicBackgroundTask(reg prometheus.Registerer) {
	// You may or may not need / want these counter metrics in addition to the timestamp
	// metrics here, depending on your requirements.
	totalCount := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "demo_background_task_runs_total",
		Help: "The total number of background task runs.",
	})
	failureCount := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "demo_background_task_failures_total",
		Help: "The total number of background task failures.",
	})
	lastRun := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "demo_background_task_last_run_timestamp_seconds",
		Help: "The Unix timestamp in seconds of the last background task run, successful or not.",
	})
	lastSuccess := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "demo_background_task_last_success_timestamp_seconds",
		Help: "The Unix timestamp in seconds of the last successful background task run.",
	})
	reg.MustRegister(totalCount)
	reg.MustRegister(failureCount)
	reg.MustRegister(lastRun)
	reg.MustRegister(lastSuccess)

	log.Println("Starting background task loop...")
	bgTicker := time.NewTicker(5 * time.Second)
	for {
		log.Println("Performing background task...")
		// Simulate a random duration that the background task needs to be completed.
		time.Sleep(1*time.Second + time.Duration(rand.Float64()*500)*time.Millisecond)

		lastRunTimestamp := float64(time.Now().Unix())

		// Simulate the background task either succeeding or failing (with a 30% probability).
		if rand.Float64() > 0.3 {
			log.Println("Background task completed successfully.")
			lastSuccess.Set(lastRunTimestamp)
		} else {
			failureCount.Inc()
			log.Println("Background task failed.")
		}
		totalCount.Inc()
		lastRun.Set(lastRunTimestamp)

		<-bgTicker.C
	}
}

func main() {
	listenAddr := flag.String("web.listen-addr", ":8080", "The address to listen on for web requests.")
	flag.Parse()

	go periodicBackgroundTask(prometheus.DefaultRegisterer)

	api := newDemoAPI(prometheus.DefaultRegisterer)
	api.register(http.DefaultServeMux)

	http.Handle("/metrics", promhttp.Handler())

	log.Fatal(http.ListenAndServe(*listenAddr, nil))
}
