// Package legaltext embeds the acceptable-use agreement so the compiled
// muxcore-setup binary is self-contained. legal/TOS.txt at the repo root is
// a symlink into this package — edit the text there.
package legaltext

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
)

//go:embed TOS.txt
var text string

// Text returns the acceptable-use agreement body.
func Text() string { return text }

// SHA256 returns a hex digest of the agreement, recorded alongside acceptance
// so a later dispute can prove which wording the user agreed to.
func SHA256() string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}
