package main

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/DaanHessen/walker-tui/internal/store"
	"github.com/DaanHessen/walker-tui/internal/text"
	"github.com/DaanHessen/walker-tui/internal/ui"
	"github.com/DaanHessen/walker-tui/internal/util"
	"github.com/joho/godotenv"
)

var (
	version      = "0.1.0-alpha"
	rulesVersion = version
	seedAlphabet = base32.NewEncoding("abcdefghijklmnopqrstuvwxyz234567").WithPadding(base32.NoPadding)
)

func main() {
	// Load .env file if it exists (ignore error if file doesn't exist)
	_ = godotenv.Load()

	seedFlag := flag.String("seed", "", "Run seed string (optional; random if omitted)")
	dsn := flag.String("dsn", os.Getenv("DATABASE_URL"), "PostgreSQL DSN")
	density := flag.String("density", "standard", "Text density: concise|standard|rich")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "walker-tui [--seed seedstring] [--dsn DSN] [--density=concise|standard|rich] | migrate up|down | version\n")
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

	seedText := strings.TrimSpace(*seedFlag)
	if seedText == "" {
		generated, err := generateSeed()
		if err != nil {
			log.Fatalf("failed to generate seed: %v", err)
		}
		seedText = generated
		fmt.Printf("New run seed: %s\n", seedText)
	}

	cfg := util.Config{
		SeedText:     seedText,
		DSN:          *dsn,
		TextDensity:  *density,
		DebugLAD:     os.Getenv("ZEROPOINT_DEBUG_LAD") == "1",
		RulesVersion: rulesVersion,
	}

	ctx := context.Background()

	// Ensure migrations are present and applied before opening UI
	mig, err := store.NewMigrator(cfg.DSN)
	if err != nil {
		log.Fatalf("migrations init failed: %v", err)
	}
	migCtx, cancelMig := context.WithTimeout(ctx, 30*time.Second)
	defer cancelMig()
	if err := mig.Up(migCtx); err != nil && err != store.ErrNoChange {
		log.Fatalf("migrations failed: %v", err)
	}

	db, err := store.Open(ctx, cfg)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()
	ds, err := text.NewDeepSeek(os.Getenv("DEEPSEEK_API_KEY"))
	if err != nil {
		log.Printf("DeepSeek unavailable: %v", err)
	}

	if err := ui.Run(ctx, db, ds, ds, cfg, version, err); err != nil {
		log.Fatal(err)
	}
}

func generateSeed() (string, error) {
	buf := make([]byte, 15) // 24 characters base32
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return strings.ToLower(seedAlphabet.EncodeToString(buf)), nil
}
