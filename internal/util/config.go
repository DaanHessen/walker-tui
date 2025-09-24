package util

// Config holds runtime settings and flags.
type Config struct {
	Seed        int64
	DSN         string
	TextDensity string // concise|standard|rich
	UseAI       bool
	DebugLAD    bool // enabled via ZEROPOINT_DEBUG_LAD env or runtime toggle (F6)
}
