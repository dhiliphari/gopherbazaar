-include .env

.PHONY: db/up db/migrate db/reset build run/accounts run/transfers run/transfers/proxied run/gateway load test test/short proto proto/lint proto/breaking chaos/setup chaos/slow chaos/blackhole chaos/clear

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
	go run ./cmd/accounts -db-dsn $(ACCOUNTS_DB_DSN)

run/transfers:
	go run ./cmd/transfers -db-dsn $(TRANSFERS_DB_DSN)

run/transfers/proxied:
	go run ./cmd/transfers -accounts-addr localhost:19001 -db-dsn $(TRANSFERS_DB_DSN)

run/gateway:
	go run ./cmd/gateway

load:
	go run ./cmd/loadgen $(ARGS)

test:
	go test -race -count=1 ./...

test/short:
	go test -short -race -count=1 ./...

proto:
	buf lint
	buf generate

proto/lint:
	buf lint

# Compare the protos against the last commit on main.
proto/breaking:
	buf breaking --against '.git#branch=main'

# --- network fault injection with toxiproxy (chapter 1.6) ---
TOXI := http://localhost:8474

chaos/setup:
	curl -s -X POST $(TOXI)/proxies \
		-d '{"name":"accounts","listen":"0.0.0.0:19001","upstream":"host.docker.internal:9001"}'

chaos/slow:
	curl -s -X POST $(TOXI)/proxies/accounts/toxics \
		-d '{"name":"slow","type":"latency","attributes":{"latency":2000}}'

chaos/blackhole:
	curl -s -X POST $(TOXI)/proxies/accounts/toxics \
		-d '{"name":"blackhole","type":"timeout","attributes":{"timeout":0}}'

chaos/clear:
	-curl -s -X DELETE $(TOXI)/proxies/accounts/toxics/slow
	-curl -s -X DELETE $(TOXI)/proxies/accounts/toxics/blackhole