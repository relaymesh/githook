FROM golang:1.23 AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/worker ./boilerplate/worker

FROM alpine:3.19
WORKDIR /app
COPY --from=builder /out/worker /usr/local/bin/worker
ENTRYPOINT ["/usr/local/bin/worker"]
