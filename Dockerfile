FROM golang:1.24 AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/githook .

FROM alpine:3.19 
WORKDIR /app
COPY --from=builder /out/githook /usr/local/bin/githook
EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/githook"]