-- +goose Up
CREATE TABLE p3_projects (id TEXT PRIMARY KEY, name TEXT NOT NULL, description TEXT NOT NULL DEFAULT '', due_date TEXT, archived INTEGER NOT NULL DEFAULT 0, rag_status TEXT NOT NULL DEFAULT 'green', progress INTEGER NOT NULL DEFAULT 0, current_phase TEXT NOT NULL DEFAULT 'A', created_at TEXT NOT NULL);
CREATE TABLE p3_cycles (id TEXT PRIMARY KEY, project_id TEXT NOT NULL REFERENCES p3_projects(id) ON DELETE CASCADE, phase_code TEXT NOT NULL, number INTEGER NOT NULL, created_at TEXT NOT NULL, UNIQUE(project_id, phase_code, number));
CREATE TABLE p3_steps (id TEXT PRIMARY KEY, cycle_id TEXT NOT NULL REFERENCES p3_cycles(id) ON DELETE CASCADE, code TEXT NOT NULL, name TEXT NOT NULL, description TEXT NOT NULL DEFAULT '', status TEXT NOT NULL DEFAULT 'todo', due_date TEXT, UNIQUE(cycle_id, code));
CREATE TABLE p3_checklist (id TEXT PRIMARY KEY, step_id TEXT NOT NULL REFERENCES p3_steps(id) ON DELETE CASCADE, text TEXT NOT NULL, done INTEGER NOT NULL DEFAULT 0);
CREATE TABLE p3_links (id TEXT PRIMARY KEY, step_id TEXT NOT NULL REFERENCES p3_steps(id) ON DELETE CASCADE, title TEXT NOT NULL, url TEXT NOT NULL, description TEXT NOT NULL DEFAULT '');
CREATE TABLE p3_comments (id TEXT PRIMARY KEY, step_id TEXT NOT NULL REFERENCES p3_steps(id) ON DELETE CASCADE, text TEXT NOT NULL, created_at TEXT NOT NULL);
CREATE TABLE p3_sdlc_stages (id TEXT PRIMARY KEY, project_id TEXT NOT NULL REFERENCES p3_projects(id) ON DELETE CASCADE, code TEXT NOT NULL, name TEXT NOT NULL, status TEXT NOT NULL DEFAULT 'not_started', progress INTEGER NOT NULL DEFAULT 0, start_date TEXT, due_date TEXT, UNIQUE(project_id, code));
CREATE TABLE p3_blockers (id TEXT PRIMARY KEY, project_id TEXT NOT NULL REFERENCES p3_projects(id) ON DELETE CASCADE, step_id TEXT REFERENCES p3_steps(id) ON DELETE SET NULL, sdlc_stage_id TEXT REFERENCES p3_sdlc_stages(id) ON DELETE SET NULL, title TEXT NOT NULL, description TEXT NOT NULL DEFAULT '', resolved INTEGER NOT NULL DEFAULT 0);
CREATE TABLE p3_actions (id TEXT PRIMARY KEY, project_id TEXT NOT NULL REFERENCES p3_projects(id) ON DELETE CASCADE, step_id TEXT REFERENCES p3_steps(id) ON DELETE SET NULL, title TEXT NOT NULL, due_date TEXT, priority TEXT NOT NULL DEFAULT 'medium', done INTEGER NOT NULL DEFAULT 0);
CREATE TABLE p3_articles (id TEXT PRIMARY KEY, title TEXT NOT NULL, category TEXT NOT NULL, summary TEXT NOT NULL DEFAULT '', body TEXT NOT NULL);
CREATE TABLE p3_activity (id TEXT PRIMARY KEY, project_id TEXT NOT NULL REFERENCES p3_projects(id) ON DELETE CASCADE, step_id TEXT, kind TEXT NOT NULL, description TEXT NOT NULL, created_at TEXT NOT NULL);
CREATE INDEX p3_cycles_project ON p3_cycles(project_id, phase_code, number);
CREATE INDEX p3_steps_cycle ON p3_steps(cycle_id);
CREATE INDEX p3_activity_project ON p3_activity(project_id, created_at);
-- +goose Down
DROP TABLE p3_activity;
DROP TABLE p3_articles;
DROP TABLE p3_actions;
DROP TABLE p3_blockers;
DROP TABLE p3_sdlc_stages;
DROP TABLE p3_comments;
DROP TABLE p3_links;
DROP TABLE p3_checklist;
DROP TABLE p3_steps;
DROP TABLE p3_cycles;
DROP TABLE p3_projects;
