-- +goose Up
-- Keep activity rows in the same SQLite statement transaction as each change.
-- +goose StatementBegin
CREATE TRIGGER p3_project_activity_insert AFTER INSERT ON p3_projects
BEGIN
  INSERT INTO p3_activity(id,project_id,kind,description,created_at)
  VALUES(lower(hex(randomblob(16))),NEW.id,'project_created',NEW.name,strftime('%Y-%m-%dT%H:%M:%fZ','now'));
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER p3_project_activity_update AFTER UPDATE ON p3_projects
BEGIN
  INSERT INTO p3_activity(id,project_id,kind,description,created_at)
  VALUES(lower(hex(randomblob(16))),NEW.id,'project_updated','Project details updated',strftime('%Y-%m-%dT%H:%M:%fZ','now'));
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER p3_cycle_activity_insert AFTER INSERT ON p3_cycles
BEGIN
  INSERT INTO p3_activity(id,project_id,kind,description,created_at)
  VALUES(lower(hex(randomblob(16))),NEW.project_id,'cycle_created',NEW.phase_code || ' cycle ' || NEW.number,strftime('%Y-%m-%dT%H:%M:%fZ','now'));
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER p3_step_activity_update AFTER UPDATE ON p3_steps
WHEN OLD.status IS NOT NEW.status OR OLD.due_date IS NOT NEW.due_date
BEGIN
  INSERT INTO p3_activity(id,project_id,step_id,kind,description,created_at)
  SELECT lower(hex(randomblob(16))),c.project_id,NEW.id,'step_updated',NEW.code || ' updated',strftime('%Y-%m-%dT%H:%M:%fZ','now')
  FROM p3_cycles c WHERE c.id=NEW.cycle_id;
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER p3_checklist_activity_insert AFTER INSERT ON p3_checklist
BEGIN
  INSERT INTO p3_activity(id,project_id,step_id,kind,description,created_at)
  SELECT lower(hex(randomblob(16))),c.project_id,NEW.step_id,'checklist_created',NEW.text,strftime('%Y-%m-%dT%H:%M:%fZ','now')
  FROM p3_steps s JOIN p3_cycles c ON c.id=s.cycle_id WHERE s.id=NEW.step_id;
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER p3_checklist_activity_update AFTER UPDATE ON p3_checklist
WHEN OLD.done IS NOT NEW.done OR OLD.text IS NOT NEW.text
BEGIN
  INSERT INTO p3_activity(id,project_id,step_id,kind,description,created_at)
  SELECT lower(hex(randomblob(16))),c.project_id,NEW.step_id,'checklist_updated',NEW.text,strftime('%Y-%m-%dT%H:%M:%fZ','now')
  FROM p3_steps s JOIN p3_cycles c ON c.id=s.cycle_id WHERE s.id=NEW.step_id;
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER p3_checklist_activity_delete AFTER DELETE ON p3_checklist
BEGIN
  INSERT INTO p3_activity(id,project_id,step_id,kind,description,created_at)
  SELECT lower(hex(randomblob(16))),c.project_id,OLD.step_id,'checklist_deleted',OLD.text,strftime('%Y-%m-%dT%H:%M:%fZ','now')
  FROM p3_steps s JOIN p3_cycles c ON c.id=s.cycle_id WHERE s.id=OLD.step_id;
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER p3_link_activity_insert AFTER INSERT ON p3_links
BEGIN
  INSERT INTO p3_activity(id,project_id,step_id,kind,description,created_at)
  SELECT lower(hex(randomblob(16))),c.project_id,NEW.step_id,'link_created',NEW.title,strftime('%Y-%m-%dT%H:%M:%fZ','now')
  FROM p3_steps s JOIN p3_cycles c ON c.id=s.cycle_id WHERE s.id=NEW.step_id;
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER p3_link_activity_delete AFTER DELETE ON p3_links
BEGIN
  INSERT INTO p3_activity(id,project_id,step_id,kind,description,created_at)
  SELECT lower(hex(randomblob(16))),c.project_id,OLD.step_id,'link_deleted',OLD.title,strftime('%Y-%m-%dT%H:%M:%fZ','now')
  FROM p3_steps s JOIN p3_cycles c ON c.id=s.cycle_id WHERE s.id=OLD.step_id;
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER p3_comment_activity_insert AFTER INSERT ON p3_comments
BEGIN
  INSERT INTO p3_activity(id,project_id,step_id,kind,description,created_at)
  SELECT lower(hex(randomblob(16))),c.project_id,NEW.step_id,'comment_created',NEW.text,strftime('%Y-%m-%dT%H:%M:%fZ','now')
  FROM p3_steps s JOIN p3_cycles c ON c.id=s.cycle_id WHERE s.id=NEW.step_id;
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER p3_blocker_activity_insert AFTER INSERT ON p3_blockers
BEGIN
  INSERT INTO p3_activity(id,project_id,step_id,kind,description,created_at)
  VALUES(lower(hex(randomblob(16))),NEW.project_id,NEW.step_id,'blocker_created',NEW.title,strftime('%Y-%m-%dT%H:%M:%fZ','now'));
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER p3_blocker_activity_update AFTER UPDATE ON p3_blockers
WHEN OLD.resolved IS NOT NEW.resolved OR OLD.title IS NOT NEW.title OR OLD.description IS NOT NEW.description
BEGIN
  INSERT INTO p3_activity(id,project_id,step_id,kind,description,created_at)
  VALUES(lower(hex(randomblob(16))),NEW.project_id,NEW.step_id,'blocker_updated',NEW.title,strftime('%Y-%m-%dT%H:%M:%fZ','now'));
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER p3_sdlc_activity_update AFTER UPDATE ON p3_sdlc_stages
WHEN OLD.status IS NOT NEW.status OR OLD.progress IS NOT NEW.progress OR OLD.start_date IS NOT NEW.start_date OR OLD.due_date IS NOT NEW.due_date
BEGIN
  INSERT INTO p3_activity(id,project_id,kind,description,created_at)
  VALUES(lower(hex(randomblob(16))),NEW.project_id,'sdlc_updated',NEW.name || ' updated',strftime('%Y-%m-%dT%H:%M:%fZ','now'));
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER p3_action_activity_insert AFTER INSERT ON p3_actions
BEGIN
  INSERT INTO p3_activity(id,project_id,step_id,kind,description,created_at)
  VALUES(lower(hex(randomblob(16))),NEW.project_id,NEW.step_id,'action_created',NEW.title,strftime('%Y-%m-%dT%H:%M:%fZ','now'));
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER p3_action_activity_update AFTER UPDATE ON p3_actions
WHEN OLD.title IS NOT NEW.title OR OLD.due_date IS NOT NEW.due_date OR OLD.priority IS NOT NEW.priority OR OLD.done IS NOT NEW.done
BEGIN
  INSERT INTO p3_activity(id,project_id,step_id,kind,description,created_at)
  VALUES(lower(hex(randomblob(16))),NEW.project_id,NEW.step_id,'action_updated',NEW.title,strftime('%Y-%m-%dT%H:%M:%fZ','now'));
END;
-- +goose StatementEnd
-- +goose Down
DROP TRIGGER p3_action_activity_update;
DROP TRIGGER p3_action_activity_insert;
DROP TRIGGER p3_sdlc_activity_update;
DROP TRIGGER p3_blocker_activity_update;
DROP TRIGGER p3_blocker_activity_insert;
DROP TRIGGER p3_comment_activity_insert;
DROP TRIGGER p3_link_activity_delete;
DROP TRIGGER p3_link_activity_insert;
DROP TRIGGER p3_checklist_activity_delete;
DROP TRIGGER p3_checklist_activity_update;
DROP TRIGGER p3_checklist_activity_insert;
DROP TRIGGER p3_step_activity_update;
DROP TRIGGER p3_cycle_activity_insert;
DROP TRIGGER p3_project_activity_update;
DROP TRIGGER p3_project_activity_insert;
