package main

import (
	"flag"
	"log"
	"math/rand"
	"net/http"
	"time"
)

type demoAPI struct{}

func (a demoAPI) register(mux *http.ServeMux) {
	mux.HandleFunc("/api/foo", a.foo)
	mux.HandleFunc("/api/bar", a.bar)
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

func periodicBackgroundTask() {
	log.Println("Starting background task loop...")
	bgTicker := time.NewTicker(5 * time.Second)
	for {
		log.Println("Performing background task...")
		// Simulate a random duration that the background task needs to be completed.
		time.Sleep(1*time.Second + time.Duration(rand.Float64()*500)*time.Millisecond)

		// Simulate the background task either succeeding or failing (with a 30% probability).
		if rand.Float64() > 0.3 {
			log.Println("Background task completed successfully.")
		} else {
			log.Println("Background task failed.")
		}

		<-bgTicker.C
	}
}

func main() {
	listenAddr := flag.String("web.listen-addr", ":8080", "The address to listen on for web requests.")
	flag.Parse()

	go periodicBackgroundTask()

	api := &demoAPI{}
	api.register(http.DefaultServeMux)

	log.Fatal(http.ListenAndServe(*listenAddr, nil))
}
