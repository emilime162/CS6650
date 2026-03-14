FROM golang:1.22 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# GOARCH=amd64 ensures correct arch for ECS Fargate regardless of build machine (e.g. Apple Silicon)
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o app .

FROM gcr.io/distroless/base-debian12
WORKDIR /app
COPY --from=build /src/app /app/app
EXPOSE 8080
# SERVICE_MODE=receiver  → runs HTTP server (:8080)
# SERVICE_MODE=processor → runs SQS worker pool
CMD ["/app/app"]