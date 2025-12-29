FROM golang:1.23 AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/githooks .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/worker ./example/github/worker/main.go 

FROM alpine:3.19 AS githooks
WORKDIR /app
COPY --from=builder /out/githooks /usr/local/bin/githooks
EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/githooks"]


FROM alpine:3.19 AS worker
WORKDIR /app
COPY --from=builder /out/worker /usr/local/bin/worker

ENTRYPOINT ["/usr/local/bin/worker"]
