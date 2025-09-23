package util

// Config holds runtime settings and flags.
type Config struct {
	Seed        int64
	DSN         string
	TextDensity string // concise|standard|rich
	UseAI       bool
}
