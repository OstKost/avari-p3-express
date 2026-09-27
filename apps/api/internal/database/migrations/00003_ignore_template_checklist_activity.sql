-- +goose Up
ALTER TABLE p3_checklist ADD COLUMN is_template INTEGER NOT NULL DEFAULT 0;
DROP TRIGGER p3_checklist_activity_insert;
-- +goose StatementBegin
CREATE TRIGGER p3_checklist_activity_insert AFTER INSERT ON p3_checklist
WHEN NEW.is_template=0
BEGIN
  INSERT INTO p3_activity(id,project_id,step_id,kind,description,created_at)
  SELECT lower(hex(randomblob(16))),c.project_id,NEW.step_id,'checklist_created',NEW.text,strftime('%Y-%m-%dT%H:%M:%fZ','now')
  FROM p3_steps s JOIN p3_cycles c ON c.id=s.cycle_id WHERE s.id=NEW.step_id;
END;
-- +goose StatementEnd
-- +goose Down
DROP TRIGGER p3_checklist_activity_insert;
-- +goose StatementBegin
CREATE TRIGGER p3_checklist_activity_insert AFTER INSERT ON p3_checklist
BEGIN
  INSERT INTO p3_activity(id,project_id,step_id,kind,description,created_at)
  SELECT lower(hex(randomblob(16))),c.project_id,NEW.step_id,'checklist_created',NEW.text,strftime('%Y-%m-%dT%H:%M:%fZ','now')
  FROM p3_steps s JOIN p3_cycles c ON c.id=s.cycle_id WHERE s.id=NEW.step_id;
END;
-- +goose StatementEnd
ALTER TABLE p3_checklist DROP COLUMN is_template;
