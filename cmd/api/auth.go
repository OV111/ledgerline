package main

// HTTP handlers for accounts: signup, login, logout, me, change wallet.
// The rules themselves live in internal/auth; this file only translates
// between HTTP (JSON, cookies, status codes) and those rules.

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"mime"
	"net/http"

	"ledgerline/internal/auth"
)

const (
	sessionCookie = "session"
	// 4 KB is plenty for an email, a password and a wallet.
	maxBodyBytes = 4 << 10
)

// userResponse is what we send back about a user.
// It has no password_hash on purpose: never send it to a browser.
type userResponse struct {
	ID     int64  `json:"id"`
	Email  string `json:"email"`
	Wallet string `json:"wallet"`
}

func toUserResponse(u auth.User) userResponse {
	return userResponse{ID: u.ID, Email: u.Email, Wallet: u.Wallet}
}

// POST /signup {"email", "password", "wallet"}
func (s *server) handleSignup(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Wallet   string `json:"wallet"`
	}
	if !readJSON(w, r, &req) {
		return
	}

	email, err := auth.NormalizeEmail(req.Email)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := auth.ValidatePassword(req.Password); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	wallet, err := auth.NormalizeWallet(req.Wallet)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		serverError(w, "hash password", err)
		return
	}

	u, err := s.store.CreateUser(r.Context(), email, hash, wallet)
	if errors.Is(err, auth.ErrEmailTaken) || errors.Is(err, auth.ErrWalletTaken) {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	if err != nil {
		serverError(w, "create user", err)
		return
	}

	// Log the new user in right away.
	if !s.startSession(w, r, u.ID) {
		return
	}
	writeJSON(w, http.StatusCreated, toUserResponse(u))
}

// POST /login {"email", "password"}
func (s *server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !readJSON(w, r, &req) {
		return
	}

	// A badly formatted email can't belong to anyone. We don't return early:
	// we fall through to the same "not found" path, so the answer and the
	// time it takes are the same as for any other wrong login.
	email, _ := auth.NormalizeEmail(req.Email)

	u, err := s.store.UserByEmail(r.Context(), email)
	if err != nil && !errors.Is(err, auth.ErrNotFound) {
		serverError(w, "find user", err)
		return
	}
	// If the user was not found, u is empty and u.PasswordHash is "".
	// CheckPassword then runs a dummy bcrypt check and returns false.
	if !auth.CheckPassword(u.PasswordHash, req.Password) {
		// Same message for "no such email" and "wrong password", so an
		// attacker can't learn which emails are registered.
		writeError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}

	if !s.startSession(w, r, u.ID) {
		return
	}
	writeJSON(w, http.StatusOK, toUserResponse(u))
}

// POST /logout
// Works even when not logged in: the result (logged out) is the same.
// No JSON body, so no Content-Type check; the SameSite cookie already stops
// other websites from sending this request with our cookie.
func (s *server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		if err := s.store.DeleteSession(r.Context(), c.Value); err != nil {
			serverError(w, "delete session", err)
			return
		}
	}
	clearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

// GET /me
func (s *server) handleMe(w http.ResponseWriter, r *http.Request, u auth.User, _ string) {
	writeJSON(w, http.StatusOK, toUserResponse(u))
}

// PUT /me/wallet {"wallet", "password"}
//
// The wallet decides where the freelancer's money goes, so this is the
// most sensitive action. Being logged in is not enough: the password must be
// typed again (risk S1: a stolen session alone can't redirect payments).
func (s *server) handleChangeWallet(w http.ResponseWriter, r *http.Request, u auth.User, token string) {
	var req struct {
		Wallet   string `json:"wallet"`
		Password string `json:"password"`
	}
	if !readJSON(w, r, &req) {
		return
	}

	wallet, err := auth.NormalizeWallet(req.Wallet)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	// 403, not 401: the user IS logged in, but not allowed to do this
	// without the right password.
	if !auth.CheckPassword(u.PasswordHash, req.Password) {
		writeError(w, http.StatusForbidden, "wrong password")
		return
	}

	err = s.store.UpdateWallet(r.Context(), u.ID, wallet, token)
	if errors.Is(err, auth.ErrWalletTaken) {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	if err != nil {
		serverError(w, "update wallet", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"wallet": wallet})
}

// --- helpers -----------------------------------------------------------

// userHandler is a handler that also receives the logged-in user and their
// session token.
type userHandler func(w http.ResponseWriter, r *http.Request, u auth.User, token string)

// requireUser wraps a handler so it only runs for logged-in users.
// It reads the session cookie, finds the user, and answers 401 otherwise.
func (s *server) requireUser(next userHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(sessionCookie)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "not logged in")
			return
		}
		u, err := s.store.UserBySession(r.Context(), c.Value)
		if errors.Is(err, auth.ErrNotFound) {
			// Unknown or expired: also remove the dead cookie from the browser.
			clearSessionCookie(w)
			writeError(w, http.StatusUnauthorized, "not logged in")
			return
		}
		if err != nil {
			serverError(w, "find session", err)
			return
		}
		next(w, r, u, c.Value)
	}
}

// startSession creates a session and sets the cookie.
// It returns false if it already wrote an error response.
func (s *server) startSession(w http.ResponseWriter, r *http.Request, userID int64) bool {
	token, err := s.store.CreateSession(r.Context(), userID)
	if err != nil {
		serverError(w, "create session", err)
		return false
	}
	http.SetCookie(w, &http.Cookie{
		Name:   sessionCookie,
		Value:  token,
		Path:   "/",
		MaxAge: int(auth.SessionTTL.Seconds()),
		// HttpOnly: JavaScript can't read it, so an XSS bug can't steal it.
		HttpOnly: true,
		// Secure: only sent over HTTPS (browsers also allow it on localhost).
		Secure: true,
		// Lax: the browser doesn't send it with POST requests that come
		// from other websites. That blocks CSRF.
		SameSite: http.SameSiteLaxMode,
	})
	return true
}

// clearSessionCookie tells the browser to delete the cookie.
func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode,
	})
}

// readJSON decodes the request body into dst. On any problem it writes the
// error response itself and returns false, so callers just `return`.
func readJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	// Only accept JSON. A plain HTML form on another website can't send
	// Content-Type: application/json, so this is a second CSRF block.
	mediaType, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if mediaType != "application/json" {
		writeError(w, http.StatusUnsupportedMediaType, "Content-Type must be application/json")
		return false
	}

	// Stop reading after maxBodyBytes, so a huge body can't eat our memory.
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	// Refuse unknown fields: a typo like "wallett" becomes an error instead
	// of being silently ignored.
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeError(w, http.StatusRequestEntityTooLarge, "request body too large")
			return false
		}
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return false
	}
	// The body must be exactly one JSON object, nothing after it.
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("write response: %v", err)
	}
}

// writeError sends {"error": "..."}. Only use it with messages that are safe
// for the user to see.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// serverError logs the real error and sends the user only a generic message.
// Internal details (SQL errors, table names) help attackers, not users.
func serverError(w http.ResponseWriter, what string, err error) {
	log.Printf("%s: %v", what, err)
	writeError(w, http.StatusInternalServerError, "internal error")
}
