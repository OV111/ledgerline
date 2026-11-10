package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"time"
)

// SessionTTL is how long a login lasts before the user must log in again.
const SessionTTL = 7 * 24 * time.Hour

// newSessionToken makes a random token for the cookie and its hash for the DB.
//
// 32 random bytes = 256 bits: impossible to guess. crypto/rand (not math/rand)
// is required here, because math/rand is predictable.
func newSessionToken() (token string, hash []byte, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", nil, err
	}
	// RawURLEncoding: only letters, digits, "-" and "_", safe in a cookie.
	token = base64.RawURLEncoding.EncodeToString(b)
	return token, hashToken(token), nil
}

// hashToken returns the SHA-256 of a token. This is what the DB stores.
//
// Why SHA-256 here but bcrypt for passwords? Passwords are short and
// guessable, so they need a slow hash. A 256-bit random token can't be
// guessed at all, so a fast hash is enough to make a leaked table useless.
func hashToken(token string) []byte {
	h := sha256.Sum256([]byte(token))
	return h[:]
}
