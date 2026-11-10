package auth

import (
	"sync"

	"golang.org/x/crypto/bcrypt"
)

// bcryptCost sets how slow hashing is. Each +1 doubles the time.
// 12 takes about 0.25 s per hash: unnoticeable for one login, but it makes
// guessing millions of passwords from a stolen database very expensive.
const bcryptCost = 12

// HashPassword checks the password rules and returns a bcrypt hash to store.
//
// The hash includes a random salt, so the same password gives a different
// hash every time. That means two users with the same password do not have
// the same hash, and precomputed "rainbow tables" are useless.
func HashPassword(password string) (string, error) {
	if err := ValidatePassword(password); err != nil {
		return "", err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// CheckPassword reports whether password matches the stored hash.
//
// Pass hash = "" when the user does not exist. We then compare against a
// dummy hash anyway and return false. Why: without this, "unknown email"
// would answer instantly while "wrong password" takes 0.25 s, and an
// attacker could use that time difference to find out which emails have
// accounts.
func CheckPassword(hash, password string) bool {
	if hash == "" {
		_ = bcrypt.CompareHashAndPassword(dummyHash(), []byte(password))
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// dummyHash is created once, the first time it is needed, with the same cost
// as real hashes so that the fake check takes the same time as a real one.
var dummyHash = sync.OnceValue(func() []byte {
	h, err := bcrypt.GenerateFromPassword([]byte("dummy-password-never-matches"), bcryptCost)
	if err != nil {
		// Only fails if the cost is out of range, which is a programming bug.
		panic(err)
	}
	return h
})
