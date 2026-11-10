-- 001_init.sql: the double-entry ledger tables.
--
-- All money is BIGINT in USDC units (6 decimals): 1 USDC = 1000000.
-- Never use floating point types for money.
--
-- The other tables from the project plan (users, invoices, payments,
-- webhook_deliveries) are added by later migrations, with their features.

-- A ledger transaction groups entries that must balance together.
-- (Not the same thing as a database transaction or a blockchain transaction.)
CREATE TABLE ledger_transactions (
    id          BIGSERIAL PRIMARY KEY,
    description TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- One line of a ledger transaction: a debit OR a credit on one account.
CREATE TABLE ledger_entries (
    id             BIGSERIAL PRIMARY KEY,
    transaction_id BIGINT NOT NULL REFERENCES ledger_transactions (id),
    -- Same list as the Account constants in internal/ledger/ledger.go.
    account        TEXT   NOT NULL
                   CHECK (account IN ('accounts_receivable', 'revenue', 'usdc_wallet')),
    debit          BIGINT NOT NULL DEFAULT 0,
    credit         BIGINT NOT NULL DEFAULT 0,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- Exactly one side is set, and it is positive.
    CONSTRAINT ledger_entries_one_side
        CHECK ((debit > 0 AND credit = 0) OR (debit = 0 AND credit > 0))
);

CREATE INDEX ledger_entries_transaction_id_idx ON ledger_entries (transaction_id);

-- Rule 1: every ledger transaction must balance (debits = credits).
--
-- This is the database's copy of the check in internal/ledger.Validate.
-- If the Go code has a bug, the database still refuses the bad data.
CREATE FUNCTION check_ledger_transaction_balanced() RETURNS trigger AS $$
DECLARE
    debits  NUMERIC;
    credits NUMERIC;
BEGIN
    -- SUM of BIGINT gives NUMERIC in Postgres, so the totals cannot overflow.
    SELECT COALESCE(SUM(debit), 0), COALESCE(SUM(credit), 0)
      INTO debits, credits
      FROM ledger_entries
     WHERE transaction_id = NEW.transaction_id;

    IF debits <> credits THEN
        RAISE EXCEPTION 'ledger transaction % is not balanced: debits=% credits=%',
            NEW.transaction_id, debits, credits;
    END IF;

    RETURN NULL; -- the return value is ignored for AFTER triggers
END;
$$ LANGUAGE plpgsql;

-- DEFERRABLE INITIALLY DEFERRED means: run the check at COMMIT, not after each
-- row. We need that because entries are inserted one at a time, so after the
-- first insert the transaction is always unbalanced for a moment.
CREATE CONSTRAINT TRIGGER ledger_entries_balanced
    AFTER INSERT ON ledger_entries
    DEFERRABLE INITIALLY DEFERRED
    FOR EACH ROW
    EXECUTE FUNCTION check_ledger_transaction_balanced();

-- Rule 2: the ledger is append-only.
--
-- Entries are never edited or deleted. To fix a mistake, insert a new
-- transaction that reverses it (this is how "Invoice voided" works). This
-- keeps a full history, and it means the balance check above only has to
-- care about INSERT.
CREATE FUNCTION forbid_ledger_entry_change() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'ledger_entries is append-only: % is not allowed', TG_OP;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER ledger_entries_append_only
    BEFORE UPDATE OR DELETE ON ledger_entries
    FOR EACH ROW
    EXECUTE FUNCTION forbid_ledger_entry_change();
