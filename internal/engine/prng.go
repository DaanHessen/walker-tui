package engine

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
)

// SeedFromString returns a 64-bit seed from an arbitrary string using SHA256.
func SeedFromString(s string) uint64 {
	h := sha256.Sum256([]byte(s))
	return binary.LittleEndian.Uint64(h[:8])
}

// Derive returns a deterministic child seed based on a base seed and a label using HMAC-SHA256.
// Labels should be stable strings such as "origin@rules:1.0.0" or "day:3:turn:1:event".
func Derive(base uint64, label string) uint64 {
	key := make([]byte, 8)
	binary.LittleEndian.PutUint64(key, base)
	m := hmac.New(sha256.New, key)
	_, _ = m.Write([]byte(label))
	sum := m.Sum(nil)
	return binary.LittleEndian.Uint64(sum[:8])
}

// DeriveRunRoot optionally mixes runID and rules into the base root for per-run uniqueness.
// If runID or rulesVersion are empty, it falls back to base.
func DeriveRunRoot(base uint64, runID string, rulesVersion string) uint64 {
	if runID == "" && rulesVersion == "" {
		return base
	}
	key := make([]byte, 8)
	binary.LittleEndian.PutUint64(key, base)
	m := hmac.New(sha256.New, key)
	_, _ = m.Write([]byte(runID))
	_, _ = m.Write([]byte("|"))
	_, _ = m.Write([]byte(rulesVersion))
	sum := m.Sum(nil)
	return binary.LittleEndian.Uint64(sum[:8])
}

// RunSeed encapsulates the canonical seed string for a run and exposes deterministic streams.
type RunSeed struct {
	Text string
	root uint64
}

// NewRunSeed creates a deterministic RunSeed from a textual seed. Empty text is rejected.
func NewRunSeed(seedText string) (RunSeed, error) {
	if seedText == "" {
		return RunSeed{}, fmt.Errorf("seed text must not be empty")
	}
	return RunSeed{Text: seedText, root: SeedFromString(seedText)}, nil
}

// WithRunContext returns a new RunSeed whose root is mixed with runID and rulesVersion.
func (r RunSeed) WithRunContext(runID string, rulesVersion string) RunSeed {
	mixed := DeriveRunRoot(r.root, runID, rulesVersion)
	return RunSeed{Text: r.Text, root: mixed}
}

// Stream returns a new deterministic RNG stream derived from the run's root seed.
func (r RunSeed) Stream(label string) *Stream {
	return newStream(Derive(r.root, label))
}

// SplitMix64 PRNG implementation for deterministic streams.
type SplitMix64 struct{ state uint64 }

func newSplitMix64(seed uint64) *SplitMix64 { return &SplitMix64{state: seed} }

func (s *SplitMix64) next() uint64 {
	s.state += 0x9E3779B97F4A7C15
	z := s.state
	z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
	z = (z ^ (z >> 27)) * 0x94D049BB133111EB
	return z ^ (z >> 31)
}

func (s *SplitMix64) intn(n int) int {
	if n <= 0 {
		return 0
	}
	return int(s.next() % uint64(n))
}

func (s *SplitMix64) float64() float64 {
	return float64(s.next()>>11) / (1 << 53)
}

// Stream provides deterministic random numbers with support for labelled child streams.
type Stream struct {
	base uint64
	sm   *SplitMix64
}

func newStream(seed uint64) *Stream {
	return &Stream{base: seed, sm: newSplitMix64(seed)}
}

// Intn mirrors math/rand.Intn but is deterministic per stream.
func (s *Stream) Intn(n int) int { return s.sm.intn(n) }

// Float64 returns a float in [0,1).
func (s *Stream) Float64() float64 { return s.sm.float64() }

// Uint64 exposes the underlying 64-bit stream when coarse-grained randomness is needed.
func (s *Stream) Uint64() uint64 { return s.sm.next() }

// Child creates a stable sub-stream derived from this stream's base seed and label.
func (s *Stream) Child(label string) *Stream { return newStream(Derive(s.base, label)) }
