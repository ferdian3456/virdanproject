package main

import (
	"flag"
	"io"
	"log"
	"net/http"

	"github.com/ferdian3456/virdanproject/benchmark/go-frameworks/app/internal/device"
)

func writeJSON(w http.ResponseWriter, status int, b []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(b)
}

func main() {
	addr := flag.String("addr", ":8081", "listen address")
	flag.Parse()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/devices", func(w http.ResponseWriter, r *http.Request) {
		b, err := device.List()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, b)
	})
	mux.HandleFunc("POST /api/devices", func(w http.ResponseWriter, r *http.Request) {
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
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "OK")
	})

	log.Printf("stdlib listening on %s", *addr)
	log.Fatal(http.ListenAndServe(*addr, mux))
}
