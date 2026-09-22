// Command scorecard ranks Nansen Smart Money wallets and tokens per
// docs/DESIGN.md: seed -> wallet enrichment -> token enrichment -> score
// -> render.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/watarus/nansen/internal/nansen"
	"github.com/watarus/nansen/internal/pipeline"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "scorecard:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("scorecard", flag.ContinueOnError)
	var (
		dryRun         = fs.Bool("dry-run", false, "print the planned call list and credit estimate, then exit without any HTTP request")
		offline        = fs.Bool("offline", false, "run entirely from cache/fixtures; any cache miss is a hard error, never an HTTP call")
		yes            = fs.Bool("yes", false, "skip the confirmation prompt before spending credits")
		concurrency    = fs.Int("concurrency", 8, "bounded worker pool size for wallet/token enrichment")
		maxWallets     = fs.Int("max-wallets", 0, "cap enrichment to the N highest-value wallets in the seed (0 = all)")
		creditFloor    = fs.Int("credit-floor", 100, "refuse a call if it would drop remaining credits below this floor")
		allowExpensive = fs.Bool("allow-expensive", false, "allow calls to endpoints costing more than 1 credit")
		ratePerSec     = fs.Int("rate-per-sec", 12, "max requests per second (Free tier allows 15)")
		ratePerMin     = fs.Int("rate-per-min", 250, "max requests per minute (Free tier allows 300)")
		repoRoot       = fs.String("repo-root", ".", "repository root, used to locate data/fixtures and out/")
		creditsSeed    = fs.Int("credits-remaining", -1, "prime the known remaining-credit balance (e.g. from the last calls.jsonl line); -1 means unknown until the first live response")
	)
	if err := fs.Parse(args); err != nil {
		return err
	}

	root, err := filepath.Abs(*repoRoot)
	if err != nil {
		return err
	}
	cacheDir := filepath.Join(root, "data", "cache")
	outDir := filepath.Join(root, "out")
	callLogPath := filepath.Join(outDir, "calls.jsonl")

	cache := nansen.NewCache(cacheDir)
	if err := pipeline.ImportSeedFixture(cache, pipeline.DefaultFixturePath(root)); err != nil {
		return fmt.Errorf("importing seed fixture: %w", err)
	}

	callLog, err := nansen.NewCallLog(callLogPath)
	if err != nil {
		return fmt.Errorf("opening call log: %w", err)
	}
	defer callLog.Close()

	limiter := nansen.NewRateLimiter(*ratePerSec, *ratePerMin)
	client := nansen.NewClient(os.Getenv("NANSEN_API_KEY"), cache, limiter, callLog)
	client.CreditFloor = *creditFloor
	client.AllowExpensive = *allowExpensive
	client.Offline = *offline
	if *creditsSeed >= 0 {
		client.SeedCreditsRemaining(*creditsSeed)
	}

	pl := pipeline.New(client, *concurrency)
	pl.MaxWallets = *maxWallets

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if *dryRun {
		return runDryRun(ctx, pl)
	}

	if !*offline && !*yes {
		items, err := pl.Plan(ctx)
		if err != nil {
			return err
		}
		printPlan(items)
		if !confirm() {
			fmt.Println("aborted")
			return nil
		}
	}

	result, err := pl.Run(ctx)
	if err != nil {
		return err
	}

	if len(result.Failures) > 0 {
		fmt.Fprintf(os.Stderr, "scorecard: %d partial failures during enrichment (continuing)\n", len(result.Failures))
		for _, f := range result.Failures {
			if isOfflineMiss(f.Err) {
				fmt.Fprintf(os.Stderr, "  [cache miss] %s %s: %v\n", f.Stage, f.Key, f.Err)
			} else {
				fmt.Fprintf(os.Stderr, "  [error] %s %s: %v\n", f.Stage, f.Key, f.Err)
			}
		}
	}

	printTable(result)

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	reportPath := filepath.Join(outDir, "report.html")
	if err := writeReport(reportPath, result); err != nil {
		return fmt.Errorf("writing report: %w", err)
	}
	fmt.Printf("\nwrote %s\n", reportPath)
	return nil
}

func isOfflineMiss(err error) bool {
	return errors.Is(err, nansen.ErrOffline)
}

func runDryRun(ctx context.Context, pl *pipeline.Pipeline) error {
	items, err := pl.Plan(ctx)
	if err != nil {
		return err
	}
	printPlan(items)
	return nil
}

func confirm() bool {
	fmt.Print("Proceed with this credit spend? [y/N] ")
	var answer string
	fmt.Scanln(&answer)
	return answer == "y" || answer == "Y" || answer == "yes"
}
