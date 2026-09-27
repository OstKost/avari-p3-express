-- +goose Up
CREATE TABLE work_tasks (id TEXT PRIMARY KEY, project_id TEXT NOT NULL REFERENCES p3_projects(id), version INTEGER NOT NULL, body TEXT NOT NULL CHECK(json_valid(body)));
CREATE INDEX work_tasks_project ON work_tasks(project_id);
CREATE TABLE work_events (id TEXT PRIMARY KEY, task_id TEXT NOT NULL REFERENCES work_tasks(id), actor TEXT NOT NULL, run_id TEXT NOT NULL, result_id TEXT NOT NULL, kind TEXT NOT NULL, created_at TEXT NOT NULL, body TEXT NOT NULL);
CREATE TABLE agent_tokens (id TEXT PRIMARY KEY, name TEXT NOT NULL, projects TEXT NOT NULL, hash TEXT NOT NULL UNIQUE, revoked INTEGER NOT NULL DEFAULT 0);
-- +goose Down
DROP TABLE agent_tokens;
DROP TABLE work_events;
DROP TABLE work_tasks;
