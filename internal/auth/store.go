package auth

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

// Store errors. The HTTP layer maps them to status codes (404, 409).
var (
	ErrNotFound    = errors.New("not found")
	ErrEmailTaken  = errors.New("email is already registered")
	ErrWalletTaken = errors.New("wallet address is already used by another account")
)

// User is one row of the users table.
type User struct {
	ID           int64
	Email        string
	PasswordHash string
	Wallet       string
	CreatedAt    time.Time
}

// Store reads and writes users and sessions in PostgreSQL.
//
// It does no validation: callers pass already-cleaned values
// (NormalizeEmail, HashPassword, NormalizeWallet). The DB CHECKs are the
// safety net if they forget.
//
// All queries use $1, $2... placeholders. The values are sent separately
// from the SQL text, so user input can never change the query (this is what
// prevents SQL injection). Never build SQL with string concatenation.
type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// userColumns is the column list used by every query that returns a User.
// It must match the order in scanUser.
const userColumns = `u.id, u.email, u.password_hash, u.wallet_address, u.created_at`

// scanner is anything with a Scan method (*sql.Row and *sql.Rows both have one).
type scanner interface {
	Scan(dest ...any) error
}

func scanUser(row scanner) (User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Wallet, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return u, err
}

// CreateUser inserts a new user. It returns ErrEmailTaken or ErrWalletTaken
// if the database's unique rules refuse it.
func (s *Store) CreateUser(ctx context.Context, email, passwordHash, wallet string) (User, error) {
	row := s.db.QueryRowContext(ctx, `
		INSERT INTO users AS u (email, password_hash, wallet_address)
		VALUES ($1, $2, $3)
		RETURNING `+userColumns,
		email, passwordHash, wallet)
	u, err := scanUser(row)
	if err != nil {
		return User{}, uniqueErr(err)
	}
	return u, nil
}

// UserByEmail finds a user by (already normalized) email.
func (s *Store) UserByEmail(ctx context.Context, email string) (User, error) {
	return scanUser(s.db.QueryRowContext(ctx,
		`SELECT `+userColumns+` FROM users u WHERE u.email = $1`, email))
}

// CreateSession logs a user in: it stores the token's hash and returns the
// token itself, which goes into the cookie. The token is never stored.
func (s *Store) CreateSession(ctx context.Context, userID int64) (string, error) {
	token, hash, err := newSessionToken()
	if err != nil {
		return "", err
	}
	// The expiry is computed by the database clock (now()), same as
	// created_at, so the two always agree.
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO sessions (token_hash, user_id, expires_at)
		VALUES ($1, $2, now() + make_interval(secs => $3))`,
		hash, userID, int64(SessionTTL.Seconds()))
	if err != nil {
		return "", err
	}
	return token, nil
}

// UserBySession returns the logged-in user for a cookie token.
// An unknown or expired token gives ErrNotFound.
func (s *Store) UserBySession(ctx context.Context, token string) (User, error) {
	return scanUser(s.db.QueryRowContext(ctx, `
		SELECT `+userColumns+`
		FROM sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = $1 AND s.expires_at > now()`,
		hashToken(token)))
}

// DeleteSession logs out one browser. Deleting an unknown token is not an
// error: the result (not logged in) is the same.
func (s *Store) DeleteSession(ctx context.Context, token string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM sessions WHERE token_hash = $1`, hashToken(token))
	return err
}

// UpdateWallet changes the payout address and logs out every other session
// of this user, keeping only the one that made the change (currentToken).
//
// Why log out the others: the wallet decides where money goes, so changing
// it is the most sensitive action. If someone else is logged in somewhere
// (a stolen cookie, a shared computer), this kicks them out. The caller must
// also re-check the password before calling this (risk S1).
//
// Both steps run in one database transaction: either both happen or neither.
func (s *Store) UpdateWallet(ctx context.Context, userID int64, wallet, currentToken string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	// Rollback after a successful Commit does nothing, so this is a safe
	// "undo everything if we return early with an error".
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx, `
		UPDATE users SET wallet_address = $1, updated_at = now()
		WHERE id = $2`, wallet, userID)
	if err != nil {
		return uniqueErr(err)
	}
	if n, err := res.RowsAffected(); err != nil {
		return err
	} else if n == 0 {
		return ErrNotFound
	}

	if _, err := tx.ExecContext(ctx, `
		DELETE FROM sessions WHERE user_id = $1 AND token_hash <> $2`,
		userID, hashToken(currentToken)); err != nil {
		return err
	}
	return tx.Commit()
}

// uniqueErr turns a PostgreSQL "unique violation" (code 23505) on our known
// constraints into a clear error. Any other error is returned unchanged.
// The constraint names come from migrations/002_users.sql.
func uniqueErr(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		switch pgErr.ConstraintName {
		case "users_email_key":
			return ErrEmailTaken
		case "users_wallet_address_key":
			return ErrWalletTaken
		}
	}
	return err
}
