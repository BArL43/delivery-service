-- +goose Up
-- +goose StatementBegin
ALTER TABLE orders ADD COLUMN weight DECIMAL(10,2) NOT NULL DEFAULT 0;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE orders DROP COLUMN weight;
-- +goose StatementEnd
