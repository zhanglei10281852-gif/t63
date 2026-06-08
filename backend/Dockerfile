FROM golang:1.21-alpine

WORKDIR /app

COPY go.mod ./
RUN go mod tidy || true

COPY . .

EXPOSE 8653

CMD ["sh", "-c", "go mod tidy && go run main.go"]
