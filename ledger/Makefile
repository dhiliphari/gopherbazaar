-include .env

.PHONY: db/up db/migrate db/reset run/wallet load test test/short

db/up:
	docker compose up -d --wait

db/migrate:
	migrate -path migrations -database "$(LEDGER_DB_DSN)" up

db/reset:
	migrate -path migrations -database "$(LEDGER_DB_DSN)" drop -f
	migrate -path migrations -database "$(LEDGER_DB_DSN)" up

run/wallet:
	go run ./cmd/wallet -db-dsn="$(LEDGER_DB_DSN)"

load:
	go run ./cmd/loadgen $(ARGS)

test:
	go test -race -count=1 ./...

test/short:
	go test -short -race -count=1 ./...