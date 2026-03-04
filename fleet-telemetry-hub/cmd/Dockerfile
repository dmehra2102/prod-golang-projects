# ------- Stage 1: builder -----------------------------
FROM golang:1.25-alpine AS builder

ARG SERVICE
ENV CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

WORKDIR /app

RUN apk add --no-cache git ca-certificates tzdata

COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .

RUN go build \
    -ldflags="-s -w -extldflags '-static'" \
    -trimpath \
    -o /bin/service \
    ./cmd/${SERVICE}


# ------- Stage 2: final ---------------------------------
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /bin/service /service
COPY --from=builder /app/migrations /migrations

USER nonroot:nonroot

ENTRYPOINT [ "/service" ]