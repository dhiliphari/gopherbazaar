package main

import (
	"flag"
	"fmt"
	"math/rand/v2"
	"net/http"
	"os"
	"slices"
	"sync"
	"time"
)

type config struct {
	baseURL   string
	accounts  int
	deposit   int64
	workers   int
	transfers int
	maxAmount int64
}

func main() {
	var cfg config
	flag.StringVar(&cfg.baseURL, "base-url", "http://localhost:4000", "wallet base URL")
	flag.IntVar(&cfg.accounts, "accounts", 10, "number of accounts to open")
	flag.Int64Var(&cfg.deposit, "deposit", 10_000, "opening deposit per account (cents)")
	flag.IntVar(&cfg.workers, "workers", 50, "concurrent workers")
	flag.IntVar(&cfg.transfers, "transfers", 5_000, "total transfers to send")
	flag.Int64Var(&cfg.maxAmount, "max-amount", 500, "maximum transfer amount (cents)")
	flag.Parse()

	if err := run(cfg); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(2)
	}
}

func run(cfg config) error {
	c := &client{base: cfg.baseURL, http: &http.Client{Timeout: 10 * time.Second}}

	ids := make([]int64, cfg.accounts)
	for i := range cfg.accounts {
		id, err := c.createAccount(fmt.Sprintf("load-%d", i), cfg.deposit)
		if err != nil {
			return err
		}
		ids[i] = id
	}

	before, err := c.total()
	if err != nil {
		return err
	}

	start := time.Now()
	statuses := sendTransfers(c, cfg, ids)
	elapsed := time.Since(start)

	after, err := c.total()
	if err != nil {
		return err
	}

	negatives := 0
	for _, id := range ids {
		bal, err := c.balance(id)
		if err != nil {
			return err
		}
		if bal < 0 {
			negatives++
		}
	}

	fmt.Printf("accounts=%d workers=%d transfers=%d elapsed=%s (%.0f req/s)\n",
		cfg.accounts, cfg.workers, cfg.transfers, elapsed.Round(time.Millisecond),
		float64(cfg.transfers)/elapsed.Seconds())
	fmt.Printf("responses: %s\n", formatStatuses(statuses))
	fmt.Printf("total before=%d after=%d drift=%d\n", before, after, after-before)
	fmt.Printf("negative balances: %d\n", negatives)

	if after != before || negatives > 0 {
		fmt.Println("INVARIANT VIOLATED")
		os.Exit(1)
	}
	fmt.Println("invariants hold")
	return nil
}

// fires cfg.transfers random transfers between ids using
// cfg.workers goroutines, and counts the responses by HTTP status code.
func sendTransfers(c *client, cfg config, ids []int64) map[int]int {
	mu := sync.Mutex{}
	statuses := make(map[int]int, cfg.transfers)
	wg := sync.WaitGroup{}
	jobs := make(chan struct{})

	for range cfg.workers {
		wg.Go(func() {
			for range jobs {
				from := ids[rand.IntN(len(ids))]
				to := ids[rand.IntN(len(ids))]
				for to == from {
					to = ids[rand.IntN(len(ids))]
				}
				amount := rand.Int64N(cfg.maxAmount) + 1

				status := c.transfer(from, to, amount)

				mu.Lock()
				statuses[status]++
				mu.Unlock()
			}
		})
	}

	for range cfg.transfers {
		jobs <- struct{}{}
	}

	close(jobs)
	wg.Wait()
	return statuses
}

func formatStatuses(m map[int]int) string {
	codes := make([]int, 0, len(m))
	for code := range m {
		codes = append(codes, code)
	}
	slices.Sort(codes)

	s := ""
	for _, code := range codes {
		s += fmt.Sprintf("%d=%d ", code, m[code])
	}
	return s
}
