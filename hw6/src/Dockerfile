FROM golang:1.25-alpine AS builder

WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o product-search .

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/product-search .
EXPOSE 8080
CMD ["./product-search"]