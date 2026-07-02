FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod ./
COPY *.go ./
RUN go build -o http-monitor .

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/http-monitor .
COPY config.json .
EXPOSE 8080
CMD ["./http-monitor"]
