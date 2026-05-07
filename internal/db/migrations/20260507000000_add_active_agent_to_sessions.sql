-- +goose Up
-- +goose StatementBegin
ALTER TABLE sessions ADD COLUMN active_agent TEXT NOT NULL DEFAULT 'coder';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE sessions DROP COLUMN active_agent;
-- +goose StatementEnd
