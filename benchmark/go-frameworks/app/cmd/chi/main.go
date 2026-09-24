package main

import (
	"flag"
	"io"
	"log"
	"net/http"

	"github.com/ferdian3456/virdanproject/benchmark/go-frameworks/app/internal/device"
	"github.com/go-chi/chi/v5"
)

func writeJSON(w http.ResponseWriter, status int, b []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(b)
}

func main() {
	addr := flag.String("addr", ":8082", "listen address")
	flag.Parse()

	r := chi.NewRouter()
	r.Get("/api/devices", func(w http.ResponseWriter, r *http.Request) {
		b, err := device.List()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, b)
	})
	r.Post("/api/devices", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		b, err := device.Create(body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, http.StatusCreated, b)
	})
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "OK")
	})

	log.Printf("chi listening on %s", *addr)
	log.Fatal(http.ListenAndServe(*addr, r))
}
