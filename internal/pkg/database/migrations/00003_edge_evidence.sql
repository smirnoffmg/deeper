-- +goose Up
ALTER TABLE trace_edges ADD COLUMN strength TEXT NOT NULL DEFAULT 'strong';
ALTER TABLE trace_edges ADD COLUMN basis TEXT NOT NULL DEFAULT '';
-- +goose Down
ALTER TABLE trace_edges DROP COLUMN basis;
ALTER TABLE trace_edges DROP COLUMN strength;
