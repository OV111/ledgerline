package main

import (
	"math"
	"net"
	"net/http"
	"strconv"
)

// limitPasswords refuses a request with 429 when its IP address made too
// many attempts in the last minute.
//
// Every attempt counts, not only failed ones. That is simpler, and it also
// limits how much bcrypt work (0.25 s of CPU each) one IP can cause.
func (s *server) limitPasswords(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ok, wait := s.passwordLimiter.Allow(clientIP(r))
		if !ok {
			// Retry-After tells the client how many seconds to wait.
			// Round up, so we never say "0".
			secs := int(math.Ceil(wait.Seconds()))
			w.Header().Set("Retry-After", strconv.Itoa(secs))
			writeError(w, http.StatusTooManyRequests, "too many attempts, try again later")
			return
		}
		next(w, r)
	}
}

// clientIP returns the IP address of the connection.
//
// We do NOT read the X-Forwarded-For header: any client can write anything
// there, so trusting it would let an attacker pick a new "IP" for every
// request and skip the limit. Stage 6 (deployment behind a proxy) must
// revisit this, because then RemoteAddr is the proxy's address.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
