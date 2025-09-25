package util

// Config holds runtime settings and flags shared across the program.
type Config struct {
    SeedText     string
    DSN          string
    TextDensity  string // concise|standard|rich
    UseAI        bool
    DebugLAD     bool // enabled via ZEROPOINT_DEBUG_LAD env or runtime toggle (F6)
    RulesVersion string
}
