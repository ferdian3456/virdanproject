package main

import (
	"flag"
	"io"
	"log"
	"net/http"

	"github.com/ferdian3456/virdanproject/benchmark/go-frameworks/app/internal/device"
	"github.com/labstack/echo/v5"
)

func main() {
	addr := flag.String("addr", ":8084", "listen address")
	flag.Parse()

	e := echo.New()
	e.GET("/api/devices", func(c *echo.Context) error {
		b, err := device.List()
		if err != nil {
			return c.String(http.StatusInternalServerError, err.Error())
		}
		return c.Blob(http.StatusOK, "application/json", b)
	})
	e.POST("/api/devices", func(c *echo.Context) error {
		body, err := io.ReadAll(c.Request().Body)
		if err != nil {
			return c.String(http.StatusBadRequest, err.Error())
		}
		b, err := device.Create(body)
		if err != nil {
			return c.String(http.StatusBadRequest, err.Error())
		}
		return c.Blob(http.StatusCreated, "application/json", b)
	})
	e.GET("/healthz", func(c *echo.Context) error {
		return c.String(http.StatusOK, "OK")
	})

	// Echo.Start is documented as demo-only, so serve through net/http like stdlib and chi.
	log.Printf("echo listening on %s", *addr)
	log.Fatal(http.ListenAndServe(*addr, e))
}
