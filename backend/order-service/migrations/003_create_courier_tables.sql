-- +goose Up
-- +goose StatementBegin
CREATE TABLE couriers (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL UNIQUE,
    email           TEXT NOT NULL UNIQUE,
    full_name       TEXT NOT NULL,
    phone           TEXT NOT NULL UNIQUE,
    transport_type  TEXT NOT NULL DEFAULT 'bicycle',
    is_online       BOOLEAN NOT NULL DEFAULT FALSE,
    active_order_id UUID REFERENCES orders(id),
    current_lat     DECIMAL(10,8),
    current_lon     DECIMAL(11,8),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE courier_locations (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    courier_id  UUID NOT NULL REFERENCES couriers(id) ON DELETE CASCADE,
    lat         DECIMAL(10,8) NOT NULL,
    lon         DECIMAL(11,8) NOT NULL,
    accuracy    DECIMAL(8,2),
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE courier_shifts (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    courier_id  UUID NOT NULL REFERENCES couriers(id) ON DELETE CASCADE,
    started_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ended_at    TIMESTAMPTZ,
    is_active   BOOLEAN NOT NULL DEFAULT TRUE
);

CREATE TABLE assignments (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id        UUID NOT NULL REFERENCES orders(id),
    courier_id      UUID NOT NULL REFERENCES couriers(id),
    status          TEXT NOT NULL DEFAULT 'assigned',
    eta_to_pickup   INTERVAL,
    picked_up_at    TIMESTAMPTZ,
    delivered_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_assignments_order_active
    ON assignments (order_id)
    WHERE status IN ('assigned', 'accepted', 'at_pickup', 'in_progress');

CREATE INDEX idx_couriers_location
    ON couriers (current_lat, current_lon)
    WHERE current_lat IS NOT NULL;

CREATE INDEX idx_assignments_courier_active
    ON assignments (courier_id)
    WHERE status IN ('assigned', 'accepted', 'at_pickup', 'in_progress');
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS assignments;
DROP TABLE IF EXISTS courier_shifts;
DROP TABLE IF EXISTS courier_locations;
DROP TABLE IF EXISTS couriers;
-- +goose StatementEnd
