package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/DaanHessen/walker-tui/internal/store"
	"github.com/DaanHessen/walker-tui/internal/text"
	"github.com/DaanHessen/walker-tui/internal/ui"
	"github.com/DaanHessen/walker-tui/internal/util"
)

var (
	version = "0.1.0"
)

func main() {
	// Global flags
	seed := flag.Int64("seed", time.Now().UnixNano(), "RNG seed")
	dsn := flag.String("dsn", os.Getenv("DATABASE_URL"), "PostgreSQL DSN")
	density := flag.String("density", "standard", "Text density: concise|standard|rich")
	noAI := flag.Bool("no-ai", false, "Disable DeepSeek narration (force minimal fallback)")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "walker-tui [--seed N] [--dsn DSN] [--density=concise|standard|rich] [--no-ai] | migrate up|down | version\n")
	}
	flag.Parse()

	if *dsn == "" {
		*dsn = "postgres://dev:dev@localhost:5432/zeropoint?sslmode=disable"
	}

	args := flag.Args()
	if len(args) > 0 {
		switch args[0] {
		case "version":
			fmt.Println("walker-tui", version)
			return
		case "migrate":
			if len(args) < 2 {
				log.Fatal("migrate requires 'up' or 'down'")
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			migrator, err := store.NewMigrator(*dsn)
			if err != nil {
				log.Fatal(err)
			}
			switch args[1] {
			case "up":
				if err := migrator.Up(ctx); err != nil && err != store.ErrNoChange {
					log.Fatal(err)
				}
				fmt.Println("Migrations applied")
			case "down":
				if err := migrator.Down(ctx); err != nil && err != store.ErrNoChange {
					log.Fatal(err)
				}
				fmt.Println("Migrations rolled back")
			default:
				log.Fatal("unknown migrate action; use up|down")
			}
			return
		}
	}

	cfg := util.Config{
		Seed:        *seed,
		DSN:         *dsn,
		TextDensity: *density,
		UseAI:       !*noAI && os.Getenv("DEEPSEEK_API_KEY") != "",
		DebugLAD:    os.Getenv("ZEROPOINT_DEBUG_LAD") == "1",
	}

	ctx := context.Background()

	db, err := store.Open(ctx, cfg)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	var narrator text.Narrator
	fallback := text.NewMinimalFallbackNarrator()
	if cfg.UseAI {
		ds, err := text.NewDeepSeekNarrator(os.Getenv("DEEPSEEK_API_KEY"))
		if err != nil {
			log.Printf("DeepSeek disabled: %v", err)
			narrator = fallback
		} else {
			narrator = text.WithFallback(ds, fallback)
		}
	} else {
		narrator = fallback
	}

	if err := ui.Run(ctx, db, narrator, cfg, version); err != nil {
		log.Fatal(err)
	}
}
