-include .env

.PHONY: db/up db/migrate db/reset build run/accounts run/transfers load test test/short

db/up:
	docker compose up -d --wait

db/migrate:
	migrate -path ./migrations/accounts -database "$(ACCOUNTS_DB_DSN)" up
	migrate -path ./migrations/transfers -database "$(TRANSFERS_DB_DSN)" up

db/reset:
	migrate -path ./migrations/accounts -database "$(ACCOUNTS_DB_DSN)" drop -f
	migrate -path ./migrations/transfers -database "$(TRANSFERS_DB_DSN)" drop -f
	$(MAKE) db/migrate

build:
	go build -o ./bin/ ./cmd/...

run/accounts:
	go run ./cmd/accounts

run/transfers:
	go run ./cmd/transfers

load:
	go run ./cmd/loadgen $(ARGS)

test:
	go test -race -count=1 ./...

test/short:
	go test -short -race -count=1 ./...