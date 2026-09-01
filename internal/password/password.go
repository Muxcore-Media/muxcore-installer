// Package password generates install-time credentials shared by the TUI and
// non-interactive pipeline.
package password

import "crypto/rand"

const chars = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789"

// Generate returns an n-character password using the same alphabet as the TUI
// admin phase (no ambiguous 0/O/1/l).
func Generate(n int) string {
	if n <= 0 {
		n = 16
	}
	b := make([]byte, n)
	rand.Read(b)
	out := make([]byte, n)
	for i, v := range b {
		out[i] = chars[int(v)%len(chars)]
	}
	return string(out)
}
