package main

import (
	"flag"
	"log"

	"github.com/ferdian3456/virdanproject/benchmark/go-frameworks/app/internal/device"
	"github.com/valyala/fasthttp"
)

func writeJSON(ctx *fasthttp.RequestCtx, status int, b []byte) {
	ctx.SetContentType("application/json")
	ctx.SetStatusCode(status)
	ctx.SetBody(b)
}

func handler(ctx *fasthttp.RequestCtx) {
	path := string(ctx.Path())
	switch {
	case path == "/api/devices" && ctx.IsGet():
		b, err := device.List()
		if err != nil {
			ctx.Error(err.Error(), fasthttp.StatusInternalServerError)
			return
		}
		writeJSON(ctx, fasthttp.StatusOK, b)
	case path == "/api/devices" && ctx.IsPost():
		b, err := device.Create(ctx.PostBody())
		if err != nil {
			ctx.Error(err.Error(), fasthttp.StatusBadRequest)
			return
		}
		writeJSON(ctx, fasthttp.StatusCreated, b)
	case path == "/healthz" && ctx.IsGet():
		ctx.SetBodyString("OK")
	default:
		ctx.Error("not found", fasthttp.StatusNotFound)
	}
}

func main() {
	addr := flag.String("addr", ":8086", "listen address")
	flag.Parse()

	log.Printf("fasthttp listening on %s", *addr)
	log.Fatal(fasthttp.ListenAndServe(*addr, handler))
}
