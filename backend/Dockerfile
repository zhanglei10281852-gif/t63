FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /sanitation-api .

FROM alpine:3.20

WORKDIR /app
COPY --from=builder /sanitation-api /usr/local/bin/sanitation-api

EXPOSE 8653

CMD ["sanitation-api"]
