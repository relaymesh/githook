FROM golang:1.24-alpine as builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/githook .

FROM alpine:latest
WORKDIR /app
COPY --from=builder /out/githook /usr/local/bin/githook
EXPOSE 8080
COPY config.yaml .

CMD ["/usr/local/bin/githook", "serve", "--config", "config.yaml"]

