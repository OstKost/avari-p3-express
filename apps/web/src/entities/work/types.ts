export interface WorkSpec {
  requires_independent_review: boolean;
  goal: string;
  expected_result: string;
  criteria: string[];
  priority: string;
  due_date: string;
  assignee: string;
  step_ids: string[];
  stage_id: string;
  dependencies: string[];
}
export interface Check {
  criterion: number;
  outcome: string;
  method: string;
  source: string;
  evidence: string;
}
export interface TaskResult {
  id: string;
  task_id: string;
  run_id: string;
  description: string;
  kind: string;
  urls: string[];
  artifact_version: string;
  created_at: string;
}
export interface Review {
  decision: string;
  comment: string;
  actor: string;
  created_at: string;
}
export interface Run {
  submission_digest: string;
  verification_reports: VerificationReport[];
  actor_name: string;
  id: string;
  task_id: string;
  actor: string;
  state: string;
  started_at: string;
  ended_at: string;
  last_message_at: string;
  snapshot: WorkSpec;
  report: string;
  results: TaskResult[];
  checks: Check[];
  review: Review | null;
}
export interface WorkEvent {
  actor_name: string;
  id: string;
  task_id: string;
  run_id: string;
  result_id: string;
  actor: string;
  kind: string;
  text: string;
  created_at: string;
}
export interface WorkTask extends WorkSpec {
  id: string;
  project_id: string;
  status: string;
  version: number;
  blocker: string;
  runs: Run[];
  events: WorkEvent[];
}
export interface Command {
  version: number;
  run_id?: string;
  idempotency_key?: string;
  text?: string;
  spec?: WorkSpec;
  result?: {
    description: string;
    kind: string;
    urls: string[];
    artifact_version: string;
  };
  checks?: Check[];
  decision?: string;
}
export interface Token {
  role: "executor" | "reviewer";
  id: string;
  name: string;
  projects: string[];
  revoked: boolean;
}

export interface VerificationReport {
  id: string;
  task_id: string;
  run_id: string;
  reviewer_id: string;
  reviewer_name: string;
  submission_digest: string;
  kind: "qa" | "security";
  summary: string;
  checks: Check[];
  created_at: string;
}
