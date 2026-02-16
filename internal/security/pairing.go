package security

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

const (
	maxPairAttempts = 5
	lockoutDuration = 5 * time.Minute
)

// PairingGuard manages one-time pairing and bearer token validation.
type PairingGuard struct {
	requireAuth bool

	mu          sync.RWMutex
	pairingCode string
	tokenHashes map[string]struct{}

	failedAttempts int
	lockedUntil    time.Time
}

func NewPairingGuard(requireAuth bool, existingToken string) *PairingGuard {
	g := &PairingGuard{
		requireAuth: requireAuth,
		tokenHashes: map[string]struct{}{},
	}
	if !requireAuth {
		return g
	}
	if existingToken != "" {
		g.tokenHashes[hashToken(existingToken)] = struct{}{}
		return g
	}
	g.pairingCode = generatePairingCode()
	return g
}

func (g *PairingGuard) RequireAuth() bool { return g.requireAuth }

func (g *PairingGuard) IsPaired() bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.tokenHashes) > 0
}

func (g *PairingGuard) PairingCode() string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.pairingCode
}

// TryPair validates one-time pairing code and returns a newly issued bearer token.
func (g *PairingGuard) TryPair(code string) (token string, ok bool, retryAfterSeconds int) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if !g.requireAuth {
		return "", true, 0
	}

	now := time.Now()
	if !g.lockedUntil.IsZero() && now.Before(g.lockedUntil) {
		return "", false, int(time.Until(g.lockedUntil).Seconds())
	}

	if g.pairingCode == "" {
		return "", false, 0
	}

	if subtle.ConstantTimeCompare([]byte(code), []byte(g.pairingCode)) == 1 {
		g.failedAttempts = 0
		g.lockedUntil = time.Time{}
		token = fmt.Sprintf("hc_%d", now.UnixNano())
		g.tokenHashes[hashToken(token)] = struct{}{}
		g.pairingCode = ""
		return token, true, 0
	}

	g.failedAttempts++
	if g.failedAttempts >= maxPairAttempts {
		g.lockedUntil = now.Add(lockoutDuration)
		return "", false, int(lockoutDuration.Seconds())
	}
	return "", false, 0
}

func (g *PairingGuard) IsAuthenticated(token string) bool {
	if !g.requireAuth {
		return true
	}
	if token == "" {
		return false
	}
	g.mu.RLock()
	defer g.mu.RUnlock()
	_, ok := g.tokenHashes[hashToken(token)]
	return ok
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func generatePairingCode() string {
	// Keep it simple and deterministic-length.
	// Uses nanoseconds + monotonic randomness from time for boot-time one-shot pairing.
	n := time.Now().UnixNano() % 1_000_000
	if n < 0 {
		n = -n
	}
	return fmt.Sprintf("%06d", n)
}
