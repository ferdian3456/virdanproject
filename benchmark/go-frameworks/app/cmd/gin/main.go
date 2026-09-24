package main

import (
	"flag"
	"io"
	"log"
	"net/http"

	"github.com/ferdian3456/virdanproject/benchmark/go-frameworks/app/internal/device"
	"github.com/gin-gonic/gin"
)

func main() {
	addr := flag.String("addr", ":8083", "listen address")
	flag.Parse()

	gin.SetMode(gin.ReleaseMode)
	r := gin.New() // no default logger/recovery middleware, same as the other frameworks

	r.GET("/api/devices", func(c *gin.Context) {
		b, err := device.List()
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
		c.Data(http.StatusOK, "application/json", b)
	})
	r.POST("/api/devices", func(c *gin.Context) {
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.String(http.StatusBadRequest, err.Error())
			return
		}
		b, err := device.Create(body)
		if err != nil {
			c.String(http.StatusBadRequest, err.Error())
			return
		}
		c.Data(http.StatusCreated, "application/json", b)
	})
	r.GET("/healthz", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	log.Printf("gin listening on %s", *addr)
	log.Fatal(r.Run(*addr))
}
