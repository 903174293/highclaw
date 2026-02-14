FROM golang:1.23-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git make

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN make build

# --- Runtime stage ---
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /app/dist/highclaw /usr/local/bin/highclaw

RUN adduser -D -h /home/highclaw highclaw
USER highclaw
WORKDIR /home/highclaw

EXPOSE 18789

ENTRYPOINT ["highclaw"]
CMD ["gateway"]
