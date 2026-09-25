FROM golang:1.26.5 AS build
WORKDIR /src
COPY go.mod ./
COPY main.go ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/vmvar .

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/vmvar /vmvar
ENTRYPOINT ["/vmvar"]
