FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go mod tidy

RUN CGO_ENABLED=0 GOOS=linux go build -o /inventory-server ./cmd/server/main.go

FROM alpine:3.18

WORKDIR /root/

COPY --from=builder /inventory-server .

EXPOSE 6000

CMD ["./inventory-server"]