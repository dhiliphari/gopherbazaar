# Gopher Bazaar

My work through *Let's Go Distributed*: one folder per project.

| Folder | Project | Module |
|---|---|---|
| [`ledger/`](ledger) | Part 1: a wallet and money-transfer system, split into gRPC services over Postgres | `github.com/gopherbazaar/ledger` |
| `bazaar/` | Part 2: an event-driven order platform (to come) | `github.com/gopherbazaar/bazaar` |
| `deploy/` | Part 3: running it all on Kubernetes (to come) | |

`go.work` lets the Go tools and your editor see every module from the repository root.
Run `make` targets from inside each project folder, for example:

```
$ cd ledger
$ cp .example.env .env
$ make db/up db/migrate
$ make run/wallet
```
