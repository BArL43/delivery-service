-- +goose Up
-- +goose StatementBegin
CREATE TABLE orders (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL,
    from_address JSONB NOT NULL,
    to_address JSONB NOT NULL,
    from_coords JSONB NOT NULL,
    to_coords JSONB NOT NULL,
    weight DECIMAL(8,2) NOT NULL,
    comment TEXT,
    pickup_time TIMESTAMP WITH TIME ZONE,
    status TEXT NOT NULL DEFAULT 'created',
    estimated_distance DECIMAL(8,2),
    estimated_duration INTEGER,
    estimated_price DECIMAL(12,2) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE order_status_history (
    id UUID PRIMARY KEY,
    order_id UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    status TEXT NOT NULL,
    changed_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    actor_id UUID,
    reason TEXT,
    INDEX (order_id, changed_at)
);

CREATE TABLE parcel_meta (
    order_id UUID PRIMARY KEY REFERENCES orders(id) ON DELETE CASCADE,
    dimensions TEXT,
    fragile BOOLEAN DEFAULT FALSE
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE parcel_meta;
DROP TABLE order_status_history;
DROP TABLE orders;
-- +goose StatementEnd
