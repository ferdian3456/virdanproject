FROM golang:1.26.5 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/loadtester .

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/loadtester /loadtester
ENTRYPOINT ["/loadtester"]
