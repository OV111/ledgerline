-- 002_users.sql: user accounts and login sessions.
--
-- The Go code (internal/auth) already cleans emails and wallets. The CHECKs
-- below are the database's own copy of those rules: if the Go code has a bug,
-- bad data is still refused. Same idea as the ledger balance trigger.

CREATE TABLE users (
    id             BIGSERIAL PRIMARY KEY,
    -- Stored lowercase, so "A@x.com" and "a@x.com" can't be two accounts.
    email          TEXT NOT NULL UNIQUE
                   CHECK (email = lower(email) AND length(email) BETWEEN 3 AND 254),
    -- bcrypt hashes always start with "$2" and are 60 characters long.
    -- The plain password is never stored.
    password_hash  TEXT NOT NULL
                   CHECK (password_hash LIKE '$2%' AND length(password_hash) = 60),
    -- Public address where the freelancer is paid (EIP-55 checksum form).
    -- We never store a private key.
    wallet_address TEXT NOT NULL
                   CHECK (wallet_address ~ '^0x[0-9a-fA-F]{40}$'),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- One wallet belongs to one user only.
-- Why: the listener finds an invoice by (wallet, exact amount). If two users
-- shared a wallet, both could have an open invoice of 100.000417 and one
-- payment would match two invoices. lower() makes the check ignore letter case.
CREATE UNIQUE INDEX users_wallet_address_key ON users (lower(wallet_address));

-- A session is one logged-in browser.
CREATE TABLE sessions (
    -- SHA-256 of the cookie token (32 bytes), never the token itself.
    -- If this table leaks, the hashes can't be used as cookies.
    token_hash BYTEA PRIMARY KEY CHECK (length(token_hash) = 32),
    -- ON DELETE CASCADE: deleting a user also deletes their sessions.
    user_id    BIGINT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL,
    CHECK (expires_at > created_at)
);

-- Used for "log out all other sessions" after a wallet or password change.
CREATE INDEX sessions_user_id_idx ON sessions (user_id);
