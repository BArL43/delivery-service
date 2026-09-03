-- +goose Up
CREATE TABLE IF NOT EXISTS orders (
    id UUID PRIMARY KEY,
    user_id TEXT NOT NULL,
    from_address JSONB NOT NULL,
    to_address JSONB NOT NULL,
    from_coords JSONB NOT NULL,
    to_coords JSONB NOT NULL,
    distance_km NUMERIC(10,3) NOT NULL DEFAULT 0 CHECK (distance_km >= 0),
    price NUMERIC(12,2) NOT NULL DEFAULT 0 CHECK (price >= 0),
    status TEXT NOT NULL DEFAULT 'created' CHECK (status IN ('created','assigned','at_pickup','in_progress','delivered','cancelled')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_orders_user_created_at ON orders(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_orders_status ON orders(status);

-- +goose Down
DROP TABLE IF EXISTS orders;
