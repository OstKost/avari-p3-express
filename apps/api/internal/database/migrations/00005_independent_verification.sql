-- +goose Up
ALTER TABLE agent_tokens ADD COLUMN role TEXT NOT NULL DEFAULT 'executor' CHECK(role IN ('executor','reviewer'));
-- Existing aggregates get a stable submission digest from their original payload
-- by the transactional Go metadata backfill after this schema change.
-- +goose Down
ALTER TABLE agent_tokens DROP COLUMN role;
