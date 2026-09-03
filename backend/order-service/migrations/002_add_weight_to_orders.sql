-- +goose Up
ALTER TABLE orders ADD COLUMN IF NOT EXISTS weight NUMERIC(10,2) NOT NULL DEFAULT 0 CHECK (weight >= 0);

-- +goose Down
ALTER TABLE orders DROP COLUMN IF EXISTS weight;
