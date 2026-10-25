FROM golang:1.22-alpine AS builder

WORKDIR /src
COPY go.mod ./
COPY *.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /ghreport .

FROM alpine:3.20

RUN apk --no-cache add ca-certificates && \
    adduser -D -u 10001 appuser && \
    mkdir -p /app && \
    chown -R appuser:appuser /app

WORKDIR /app
COPY --from=builder /ghreport /usr/local/bin/ghreport

USER appuser
ENTRYPOINT ["ghreport"]
