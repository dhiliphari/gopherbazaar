include .env

.PHONY: db/up db/migrate db/reset run/wallet

db/up:
	docker compose up -d --wait

db/migrate:
	migrate -path migrations -database "$(LEDGER_DB_DSN)" up

db/reset:
	migrate -path migrations -database "$(LEDGER_DB_DSN)" drop -f
	migrate -path migrations -database "$(LEDGER_DB_DSN)" up

run/wallet:
	go run ./cmd/wallet