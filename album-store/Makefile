# Makefile for album-store
# Usage: make build | make run | make deploy HOST=ec2-user@<ip> KEY=~/.ssh/key.pem

APP      := album-store
BIN      := ./bin/$(APP)
LINUX    := GOOS=linux GOARCH=amd64 CGO_ENABLED=0

.PHONY: build build-linux run tidy deploy test leaderboard help

## Local build (for your OS/arch)
build:
	go build -ldflags="-s -w" -o $(BIN) .

## Cross-compile for EC2 (Linux amd64)
build-linux:
	$(LINUX) go build -ldflags="-s -w" -o $(BIN) .

## Download / tidy dependencies
tidy:
	go mod tidy

## Run locally (requires AWS creds in env / ~/.aws)
run: build
	PORT=8080 $(BIN)

## Deploy to EC2:
##   make deploy HOST=ec2-user@<public-ip> KEY=~/.ssh/MyKey.pem
deploy: build-linux
	@test -n "$(HOST)" || (echo "ERROR: set HOST=ec2-user@<ip>"; exit 1)
	@test -n "$(KEY)"  || (echo "ERROR: set KEY=~/.ssh/key.pem"; exit 1)
	scp -i $(KEY) $(BIN)                 $(HOST):~/$(APP)
	scp -i $(KEY) album-store.service    $(HOST):~/album-store.service
	ssh -i $(KEY) $(HOST) " \
	  sudo cp ~/album-store.service /etc/systemd/system/ && \
	  sudo systemctl daemon-reload && \
	  sudo systemctl enable album-store && \
	  sudo systemctl restart album-store && \
	  sudo systemctl status album-store --no-pager \
	"

## Run smoke tests against deployed service
##   make test URL=http://<public-ip>:8080
test:
	@test -n "$(URL)" || (echo "ERROR: set URL=http://<ip>:8080"; exit 1)
	./test.sh $(URL)

## Check ChaosArena leaderboard
leaderboard:
	@curl -s https://chaosarena-alb-938452724.us-west-2.elb.amazonaws.com/leaderboard | jq

## Help menu
help:
	@echo "Album Store Makefile"
	@echo ""
	@echo "Targets:"
	@echo "  make build         - Build for local OS"
	@echo "  make build-linux   - Cross-compile for EC2 (Linux amd64)"
	@echo "  make run           - Run locally"
	@echo "  make tidy          - Update go.mod/go.sum"
	@echo "  make deploy        - Deploy to EC2 (requires HOST and KEY)"
	@echo "  make test          - Run smoke tests (requires URL)"
	@echo "  make leaderboard   - Check ChaosArena leaderboard"
	@echo ""
	@echo "Examples:"
	@echo "  make deploy HOST=ec2-user@3.80.123.45 KEY=~/.ssh/key.pem"
	@echo "  make test URL=http://3.80.123.45:8080"
	@echo ""
	@echo "Quick Start:"
	@echo "  See QUICKSTART.md for deployment instructions"