import { useState } from "react";
import {
  Link,
  useNavigate,
  useParams,
  useSearchParams,
} from "react-router-dom";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useQuery } from "@tanstack/react-query";
import { useProject } from "@/entities/p3/queries";
import type { Project } from "@/entities/p3/types";
import {
  useWorkMutation,
  useWorkTask,
  useWorkTasks,
  workApi,
} from "@/entities/work/api";
import type {
  Command,
  Run,
  TaskResult,
  WorkSpec,
  WorkTask,
} from "@/entities/work/types";
import { Card } from "@/shared/components/Card";
import { Button } from "@/shared/components/Button";
import {
  Empty,
  Field,
  fieldClass,
  PageTitle,
  QueryState,
  Section,
} from "@/features/p3/ui";
import { ProjectHeader } from "@/features/p3/ProjectPages";

const labels: Record<string, string> = {
  draft: "Черновик",
  ready: "Готова к работе",
  in_progress: "В работе",
  in_review: "Требует решения",
  done: "Принята",
  cancelled: "Отменена",
  active: "Активна",
  submitted: "Сдана",
  stopped: "Остановлена",
  accept: "Принято менеджером",
  return: "Возвращено",
  pending: "Ожидает приёмки",
};
function WorkStatus({ value }: { value: string }) {
  return (
    <span className="rounded-lg bg-[var(--av-surface-raised)] px-2 py-1 text-sm">
      {labels[value] || value}
    </span>
  );
}
function ResultView({ result }: { result: TaskResult }) {
  return (
    <div className="my-3 border-l-2 border-[var(--av-gold)] pl-3 break-words">
      <p>{result.description}</p>
      <p className="text-sm avari-muted">
        {result.kind} · Версия: {result.artifact_version || "не указана"}
      </p>
      {result.urls?.map((u) => (
        <p key={u}>
          <a
            className="avari-gold underline break-all"
            href={u}
            target="_blank"
            rel="noopener noreferrer"
          >
            {u}
          </a>
        </p>
      ))}
    </div>
  );
}
const specSchema = z.object({
  requires_independent_review: z.boolean(),
  goal: z.string(),
  expected_result: z.string(),
  criteriaText: z.string(),
  priority: z.enum(["low", "medium", "high"]),
  due_date: z.string(),
  assignee: z.string(),
  step_ids: z.array(z.string()),
  stage_id: z.string(),
  dependencies: z.array(z.string()),
});
type SpecFields = z.infer<typeof specSchema>;
function SpecForm({
  project,
  tasks,
  task,
  onSave,
  pending,
}: {
  project: Project;
  tasks: WorkTask[];
  task?: WorkTask;
  onSave: (spec: WorkSpec) => void;
  pending: boolean;
}) {
  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<SpecFields>({
    resolver: zodResolver(specSchema),
    defaultValues: {
      requires_independent_review: task?.requires_independent_review || false,
      goal: task?.goal || "",
      expected_result: task?.expected_result || "",
      criteriaText: task?.criteria.join("\n") || "",
      priority: (task?.priority || "medium") as SpecFields["priority"],
      due_date: task?.due_date || "",
      assignee: task?.assignee || "",
      step_ids: task?.step_ids || [],
      stage_id: task?.stage_id || "",
      dependencies: task?.dependencies || [],
    },
  });
  return (
    <form
      className="grid gap-4 sm:grid-cols-2"
      onSubmit={handleSubmit(({ criteriaText, ...v }) =>
        onSave({
          ...v,
          criteria: criteriaText
            .split("\n")
            .map((c) => c.trim())
            .filter(Boolean),
        }),
      )}
    >
      <Field label="Цель">
        <textarea className={fieldClass} {...register("goal")} />
      </Field>
      <Field label="Ожидаемый результат">
        <textarea className={fieldClass} {...register("expected_result")} />
      </Field>
      <Field label="Критерии приёмки — по одному на строку">
        <textarea
          className={fieldClass}
          rows={4}
          {...register("criteriaText")}
        />
      </Field>
      <label className="flex items-center gap-3">
        <input type="checkbox" {...register("requires_independent_review")} />
        <span>Требовать независимый QA перед приёмкой</span>
      </label>
      <Field label="Исполнитель">
        <input className={fieldClass} {...register("assignee")} />
      </Field>
      <Field label="Приоритет">
        <select className={fieldClass} {...register("priority")}>
          <option value="low">Низкий</option>
          <option value="medium">Средний</option>
          <option value="high">Высокий</option>
        </select>
      </Field>
      <Field label="Срок">
        <input type="date" className={fieldClass} {...register("due_date")} />
      </Field>
      <fieldset className="rounded-xl border avari-divider p-3">
        <legend>Шаги P3.express (конкретный цикл)</legend>
        <div className="max-h-52 overflow-auto space-y-2">
          {project.phases
            .flatMap((p) => p.steps)
            .map((s) => (
              <label className="flex gap-2" key={s.id}>
                <input type="checkbox" value={s.id} {...register("step_ids")} />
                <span>
                  {s.code} · {s.name}
                </span>
              </label>
            ))}
        </div>
        {task?.step_ids
          .filter(
            (id) =>
              !project.phases.some((p) => p.steps.some((s) => s.id === id)),
          )
          .map((id) => (
            <label className="flex gap-2" key={id}>
              <input type="checkbox" value={id} {...register("step_ids")} />
              <span>Прошлый цикл · {id}</span>
            </label>
          ))}
      </fieldset>
      <div className="space-y-4">
        <Field label="Стадия SDLC">
          <select className={fieldClass} {...register("stage_id")}>
            <option value="">Без стадии</option>
            {project.sdlc_stages.map((s) => (
              <option key={s.id} value={s.id}>
                {s.name}
              </option>
            ))}
          </select>
        </Field>
        <fieldset className="rounded-xl border avari-divider p-3">
          <legend>Зависимости</legend>
          {tasks
            .filter((t) => t.id !== task?.id)
            .map((t) => (
              <label className="flex gap-2" key={t.id}>
                <input
                  type="checkbox"
                  value={t.id}
                  {...register("dependencies")}
                />
                <span>
                  {t.goal || "Черновик"} · {labels[t.status]}
                </span>
              </label>
            ))}
        </fieldset>
      </div>
      {Object.keys(errors).length > 0 && (
        <p role="alert">Проверьте поля задания.</p>
      )}
      <div className="sm:col-span-2">
        <Button type="submit" isLoading={pending}>
          {task ? "Сохранить задание" : "Создать черновик"}
        </Button>
      </div>
    </form>
  );
}
function TaskRow({ task }: { task: WorkTask }) {
  return (
    <Link
      to={`/projects/${task.project_id}/tasks/${task.id}`}
      className="block mb-3"
    >
      <Card className="hover:border-[var(--av-gold)]">
        <div className="flex flex-wrap justify-between gap-3">
          <h3>{task.goal || "Черновик без цели"}</h3>
          <WorkStatus value={task.status} />
        </div>
        <p className="text-sm avari-secondary mt-2">{task.expected_result}</p>
        {task.blocker && (
          <p className="text-[var(--av-danger)] mt-2">Блокер: {task.blocker}</p>
        )}
        <p className="text-sm avari-muted">
          {task.assignee || "Исполнитель не назначен"} ·{" "}
          {task.due_date || "Без срока"}
        </p>
      </Card>
    </Link>
  );
}
export function WorkTasksPage() {
  const { projectId = "" } = useParams();
  const project = useProject(projectId);
  const tasks = useWorkTasks(projectId);
  const [params, setParams] = useSearchParams();
  const [show, setShow] = useState(false);
  const navigate = useNavigate();
  const create = useWorkMutation((spec: WorkSpec) =>
    workApi.create(projectId, spec),
  );
  const status = params.get("status") || "";
  const filtered = (tasks.data || []).filter(
    (t) =>
      (!status || t.status === status) &&
      (!params.get("blocker") || !!t.blocker),
  );
  return (
    <>
      <QueryState
        loading={project.isLoading || tasks.isLoading}
        error={(project.error || tasks.error) as Error | null}
        retry={() => {
          project.refetch();
          tasks.refetch();
        }}
      />
      {project.data && (
        <>
          <ProjectHeader project={project.data} section="Задачи" />
          <PageTitle
            title="Производственные задачи"
            action={
              <Button onClick={() => setShow(!show)}>
                {show ? "Закрыть форму" : "Новая задача"}
              </Button>
            }
          >
            Результаты и проверки проходят отдельную приёмку менеджером.
          </PageTitle>
          {show && (
            <Card className="mb-6">
              <SpecForm
                project={project.data}
                tasks={tasks.data || []}
                pending={create.isPending}
                onSave={(spec) =>
                  create.mutate(spec, {
                    onSuccess: (t) =>
                      navigate(`/projects/${projectId}/tasks/${t.id}`),
                  })
                }
              />
            </Card>
          )}
          <Field label="Статус задачи">
            <select
              className={`${fieldClass} mb-4`}
              value={status}
              onChange={(e) =>
                setParams(e.target.value ? { status: e.target.value } : {})
              }
            >
              <option value="">Все статусы</option>
              {[
                "draft",
                "ready",
                "in_progress",
                "in_review",
                "done",
                "cancelled",
              ].map((s) => (
                <option key={s} value={s}>
                  {labels[s]}
                </option>
              ))}
            </select>
          </Field>
          {params.get("blocker") && (
            <p className="mb-3">
              Показаны задачи с блокерами.{" "}
              <Button variant="ghost" onClick={() => setParams({})}>
                Сбросить
              </Button>
            </p>
          )}
          {filtered.length
            ? filtered.map((t) => <TaskRow key={t.id} task={t} />)
            : !tasks.isLoading && (
                <Empty text="Задач по этому фильтру пока нет." />
              )}
          <AgentAccess projectId={projectId} />
        </>
      )}
    </>
  );
}
function AgentAccess({ projectId }: { projectId: string }) {
  const [name, setName] = useState("");
  const [secret, setSecret] = useState("");
  const [role, setRole] = useState<"executor" | "reviewer">("executor");
  const tokens = useQuery({
    queryKey: ["work", "tokens"],
    queryFn: workApi.tokens,
  });
  const create = useWorkMutation((name: string) =>
    workApi.createToken(name, [projectId], role),
  );
  const revoke = useWorkMutation(workApi.revokeToken);
  return (
    <details className="mt-8">
      <summary className="cursor-pointer avari-gold">
        Доступ внешних агентов
      </summary>
      <Card className="mt-3">
        <p className="mb-3 text-sm avari-secondary">
          Токен даёт доступ только к этому проекту. Скопируйте его в
          конфигурацию внешнего агента.
        </p>
        <form
          className="flex flex-wrap gap-3"
          onSubmit={(e) => {
            e.preventDefault();
            create.mutate(name, {
              onSuccess: (v) => {
                setSecret(v.secret);
                setName("");
              },
            });
          }}
        >
          <Field label="Название агента">
            <input
              className={fieldClass}
              required
              value={name}
              onChange={(e) => setName(e.target.value)}
            />
          </Field>
          <Field label="Роль агента">
            <select
              className={fieldClass}
              value={role}
              onChange={(e) =>
                setRole(e.target.value as "executor" | "reviewer")
              }
            >
              <option value="executor">Исполнитель</option>
              <option value="reviewer">Независимый проверяющий</option>
            </select>
          </Field>
          <Button type="submit" isLoading={create.isPending}>
            Выдать токен
          </Button>
        </form>
        {secret && (
          <div className="mt-3">
            <Field label="Новый токен (показывается сейчас)">
              <input
                readOnly
                className={fieldClass}
                value={secret}
                onFocus={(e) => e.target.select()}
              />
            </Field>
            <Button variant="ghost" onClick={() => setSecret("")}>
              Скрыть токен
            </Button>
          </div>
        )}
        <QueryState
          loading={tokens.isLoading}
          error={tokens.error as Error | null}
          retry={() => tokens.refetch()}
        />
        {tokens.data
          ?.filter((t) => t.projects.includes(projectId))
          .map((t) => (
            <div className="flex flex-wrap gap-3 items-center mt-3" key={t.id}>
              <span>
                {t.name} ·{" "}
                {t.role === "reviewer" ? "Проверяющий" : "Исполнитель"} ·{" "}
                {t.revoked ? "Отозван" : "Активен"}
              </span>
              {!t.revoked && (
                <Button
                  variant="outline"
                  size="sm"
                  isLoading={revoke.isPending}
                  onClick={() => revoke.mutate(t.id)}
                >
                  Отозвать
                </Button>
              )}
            </div>
          ))}
      </Card>
    </details>
  );
}
function Delivery({
  task,
  run,
  onCommand,
  pending,
}: {
  task: WorkTask;
  run: Run;
  onCommand: (op: string, c: Omit<Command, "version">) => void;
  pending: boolean;
}) {
  const [description, setDescription] = useState("");
  const [kind, setKind] = useState("document");
  const [urls, setUrls] = useState("");
  const [artifactVersion, setArtifactVersion] = useState("");
  const [report, setReport] = useState("");
  const [reviewComment, setReviewComment] = useState("");
  const result = useWorkMutation((v: Command) =>
    workApi.command(task.id, "result", v),
  );
  const submit = useWorkMutation((v: Command) =>
    workApi.command(task.id, "submit", v),
  );
  const reports = run.verification_reports || [];
  const fullQA = reports.some(
    (r) =>
      r.kind === "qa" &&
      r.submission_digest === run.submission_digest &&
      r.checks.length === run.snapshot.criteria.length,
  );
  const qaMissing = run.snapshot.requires_independent_review && !fullQA;
  const issues = reports.some((r) =>
    r.checks.some((c) => c.outcome !== "pass"),
  );
  // Retain keys on transport failure so retry cannot duplicate an artifact or submission.
  const [resultKey, setResultKey] = useState(() => crypto.randomUUID());
  const [submitKey] = useState(() => crypto.randomUUID());
  return (
    <Card className="mb-4">
      <div className="flex flex-wrap gap-3 justify-between">
        <h3>Попытка · {run.actor_name || run.actor}</h3>
        <WorkStatus value={run.state} />
      </div>
      <p className="text-sm avari-muted">
        Начало: {new Date(run.started_at).toLocaleString("ru-RU")} · Последнее
        сообщение: {new Date(run.last_message_at).toLocaleString("ru-RU")}
      </p>
      <details className="my-3">
        <summary className="cursor-pointer">Принятое задание</summary>
        <p>{run.snapshot.goal}</p>
        <p>{run.snapshot.expected_result}</p>
        <ol className="list-decimal pl-5">
          {run.snapshot.criteria.map((c, i) => (
            <li key={i}>{c}</li>
          ))}
        </ol>
      </details>
      {run.results.map((r) => (
        <ResultView key={r.id} result={r} />
      ))}
      {run.report && <p className="whitespace-pre-wrap">Отчёт: {run.report}</p>}
      {run.checks.length > 0 && (
        <>
          <h4 className="mt-4">Проверки исполнителя — сообщает исполнитель</h4>
          {run.checks.map((c) => (
            <div key={c.criterion} className="my-3 border-t avari-divider pt-2">
              <strong>{run.snapshot.criteria[c.criterion]}</strong>
              <p>
                {c.outcome} · {c.method}
              </p>
              <p>Источник: {c.source}</p>
              <p className="whitespace-pre-wrap">
                Доказательства: {c.evidence}
              </p>
            </div>
          ))}
        </>
      )}
      <div className="mt-4">
        <h4>Независимый QA</h4>
        <p className="text-sm avari-secondary">
          {run.snapshot.requires_independent_review
            ? "Обязателен для этой попытки"
            : "Не обязателен для этой попытки"}
        </p>
        {!reports.length && (
          <p className="avari-muted">Независимых заключений пока нет.</p>
        )}
        {reports.map((r) => (
          <div
            className="my-3 border-t avari-divider pt-3 break-words"
            key={r.id}
          >
            <p>
              {r.kind === "qa" ? "QA" : "Безопасность"} ·{" "}
              {r.reviewer_name || r.reviewer_id} ·{" "}
              {new Date(r.created_at).toLocaleString("ru-RU")}
            </p>
            <p className="whitespace-pre-wrap">{r.summary}</p>
            {r.checks.map((c) => (
              <div key={c.criterion} className="mt-2">
                <strong>{run.snapshot.criteria[c.criterion]}</strong>
                <p>
                  {c.outcome === "pass"
                    ? "Пройдено"
                    : c.outcome === "fail"
                      ? "Не пройдено"
                      : "Не проверено"}{" "}
                  · {c.method}
                </p>
                <p>Источник: {c.source}</p>
                <p className="whitespace-pre-wrap">
                  Доказательства: {c.evidence}
                </p>
              </div>
            ))}
          </div>
        ))}
        {run.submission_digest && (
          <details className="mt-2">
            <summary className="cursor-pointer text-sm avari-secondary">
              Идентификатор проверяемой сдачи
            </summary>
            <p className="break-all text-xs avari-muted">
              {run.submission_digest}
            </p>
          </details>
        )}
      </div>
      {run.review && (
        <p className="mt-4 rounded-xl bg-[var(--av-surface-raised)] p-3">
          Решение менеджера: {labels[run.review.decision]} · {run.review.actor}{" "}
          · {new Date(run.review.created_at).toLocaleString("ru-RU")}
          <br />
          {run.review.comment}
        </p>
      )}
      {run.state === "active" && (
        <>
          <details className="mt-4">
            <summary className="avari-gold cursor-pointer">
              Прикрепить результат вручную
            </summary>
            <form
              className="mt-3 grid gap-3 sm:grid-cols-2"
              onSubmit={(e) => {
                e.preventDefault();
                result.mutate(
                  {
                    version: task.version,
                    run_id: run.id,
                    idempotency_key: resultKey,
                    result: {
                      description,
                      kind,
                      urls: urls
                        .split("\n")
                        .map((v) => v.trim())
                        .filter(Boolean),
                      artifact_version: artifactVersion,
                    },
                  },
                  {
                    onSuccess: () => {
                      setDescription("");
                      setUrls("");
                      setArtifactVersion("");
                      setResultKey(crypto.randomUUID());
                    },
                  },
                );
              }}
            >
              <Field label="Что появилось">
                <textarea
                  required
                  className={fieldClass}
                  value={description}
                  onChange={(e) => setDescription(e.target.value)}
                />
              </Field>
              <Field label="Тип результата">
                <select
                  className={fieldClass}
                  value={kind}
                  onChange={(e) => setKind(e.target.value)}
                >
                  {["document", "code", "release", "other"].map((s) => (
                    <option key={s}>{s}</option>
                  ))}
                </select>
              </Field>
              <Field label="Ссылки HTTP(S), по одной на строку">
                <textarea
                  className={fieldClass}
                  value={urls}
                  onChange={(e) => setUrls(e.target.value)}
                />
              </Field>
              <Field label="Commit / версия документа или релиза">
                <input
                  className={fieldClass}
                  value={artifactVersion}
                  onChange={(e) => setArtifactVersion(e.target.value)}
                />
              </Field>
              <Button type="submit" isLoading={result.isPending}>
                Добавить результат
              </Button>
            </form>
          </details>
          <details className="mt-4">
            <summary className="avari-gold cursor-pointer">
              Сдать на приёмку
            </summary>
            <form
              className="mt-3 space-y-4"
              onSubmit={(e) => {
                e.preventDefault();
                const data = new FormData(e.currentTarget);
                submit.mutate({
                  version: task.version,
                  run_id: run.id,
                  idempotency_key: submitKey,
                  text: report,
                  checks: run.snapshot.criteria.map((_, i) => ({
                    criterion: i,
                    outcome: String(data.get(`outcome${i}`)),
                    method: String(data.get(`method${i}`)),
                    source: String(data.get(`source${i}`)),
                    evidence: String(data.get(`evidence${i}`)),
                  })),
                });
              }}
            >
              <Field label="Отчёт, вопросы и ограничения">
                <textarea
                  required
                  className={fieldClass}
                  value={report}
                  onChange={(e) => setReport(e.target.value)}
                />
              </Field>
              {run.snapshot.criteria.map((c, i) => (
                <fieldset
                  key={i}
                  className="grid gap-3 sm:grid-cols-2 border avari-divider rounded-xl p-3"
                >
                  <legend>{c}</legend>
                  <Field label="Сообщаемый результат">
                    <select className={fieldClass} name={`outcome${i}`}>
                      <option value="pass">Успех</option>
                      <option value="fail">Не пройдено</option>
                      <option value="unknown">Не проверено</option>
                    </select>
                  </Field>
                  <Field label="Способ проверки">
                    <input
                      required
                      className={fieldClass}
                      name={`method${i}`}
                    />
                  </Field>
                  <Field label="Источник">
                    <input
                      required
                      className={fieldClass}
                      name={`source${i}`}
                    />
                  </Field>
                  <Field label="Доказательства">
                    <textarea
                      required
                      className={fieldClass}
                      name={`evidence${i}`}
                    />
                  </Field>
                </fieldset>
              ))}
              <Button
                type="submit"
                disabled={!run.results.length}
                isLoading={submit.isPending}
              >
                Сдать результат
              </Button>
              {!run.results.length && <p>Сначала прикрепите результат.</p>}
            </form>
          </details>
          <form
            className="mt-4 flex flex-wrap gap-3"
            onSubmit={(e) => {
              e.preventDefault();
              const data = new FormData(e.currentTarget);
              onCommand(String(data.get("operation")), {
                run_id: run.id,
                text: String(data.get("text")),
              });
            }}
          >
            <Field label="Сообщение или причина остановки">
              <input name="text" className={fieldClass} />
            </Field>
            <Field label="Действие">
              <select name="operation" className={fieldClass}>
                <option value="progress">Сообщить прогресс</option>
                <option value="comment">Комментарий</option>
                <option value="blocker">Установить / очистить блокер</option>
                <option value="finish">Остановить попытку</option>
              </select>
            </Field>
            <Button type="submit" isLoading={pending}>
              Отправить
            </Button>
          </form>
        </>
      )}
      {task.status === "in_review" &&
        run.state === "submitted" &&
        !run.review && (
          <div className="mt-4">
            <h4>Решение менеджера</h4>
            {qaMissing && (
              <p role="status" className="text-[var(--av-danger)]">
                До приёмки нужно независимое QA-заключение по этой сдаче.
              </p>
            )}
            {issues && (
              <p className="avari-secondary">
                В независимых проверках есть замечания или непроверенные пункты.
                Для принятия укажите причину.
              </p>
            )}
            <Field label="Замечание менеджера (обязательно при возврате)">
              <textarea
                className={fieldClass}
                value={reviewComment}
                onChange={(e) => setReviewComment(e.target.value)}
              />
            </Field>
            <div className="flex flex-wrap gap-3 mt-3">
              <Button
                disabled={qaMissing || (issues && !reviewComment.trim())}
                isLoading={pending}
                onClick={() =>
                  onCommand("review", {
                    run_id: run.id,
                    decision: "accept",
                    text: reviewComment,
                  })
                }
              >
                Принять
              </Button>
              <Button
                variant="outline"
                disabled={!reviewComment.trim()}
                isLoading={pending}
                onClick={() =>
                  onCommand("review", {
                    run_id: run.id,
                    decision: "return",
                    text: reviewComment,
                  })
                }
              >
                Вернуть
              </Button>
            </div>
          </div>
        )}
    </Card>
  );
}
export function WorkTaskPage() {
  const { projectId = "", taskId = "" } = useParams();
  const query = useWorkTask(taskId);
  const tasks = useWorkTasks(projectId);
  const t = query.data?.task;
  const p = query.data?.project;
  const [editing, setEditing] = useState(false);
  const [startKey, setStartKey] = useState(() => crypto.randomUUID());
  const cmd = useWorkMutation((v: { op: string; body: Command }) =>
    workApi.command(taskId, v.op, v.body),
  );
  function command(op: string, body: Omit<Command, "version"> = {}) {
    if (t)
      cmd.mutate(
        { op, body: { version: t.version, ...body } },
        {
          onSuccess: () => {
            if (op === "edit") setEditing(false);
            if (op === "start") setStartKey(crypto.randomUUID());
          },
        },
      );
  }
  return (
    <>
      <QueryState
        loading={query.isLoading}
        error={query.error as Error | null}
        retry={() => query.refetch()}
      />
      {t && p && (
        <>
          <ProjectHeader project={p} section="Задача" />
          <PageTitle title={t.goal || "Черновик"}>
            <WorkStatus value={t.status} />
          </PageTitle>
          <Card className="mb-6">
            <h2 className="text-lg">Ожидаемый результат</h2>
            <p className="whitespace-pre-wrap mt-2">
              {t.expected_result || "Не указан"}
            </p>
            <h3 className="mt-4">Критерии приёмки</h3>
            <ol className="list-decimal pl-5">
              {t.criteria.map((c, i) => (
                <li key={i}>{c}</li>
              ))}
            </ol>
            <p className="mt-3 avari-secondary">
              {t.assignee || "Исполнитель не назначен"} · {t.priority} ·{" "}
              {t.due_date || "Без срока"}
            </p>
            {t.blocker && (
              <p className="mt-3 text-[var(--av-danger)]">
                Блокер: {t.blocker}
              </p>
            )}
            <p className="mt-3">
              Стадия:{" "}
              {p.sdlc_stages.find((s) => s.id === t.stage_id)?.name ||
                "Не связана"}
            </p>
            {t.step_ids.map((id) => {
              const phase = p.phases.find((p) =>
                p.steps.some((s) => s.id === id),
              );
              const step = phase?.steps.find((s) => s.id === id);
              return (
                <p key={id}>
                  {step ? (
                    <Link
                      className="avari-gold"
                      to={`/projects/${p.id}/p3/${phase!.code}/${step.code}`}
                    >
                      {step.code} · {step.name}
                    </Link>
                  ) : (
                    `Шаг ${query.data?.linked_steps.find((s) => s.id === id)?.code || id} · цикл ${query.data?.linked_steps.find((s) => s.id === id)?.cycle_number || "—"}`
                  )}
                </p>
              );
            })}
            {t.dependencies.map((id) => (
              <p key={id}>
                Зависит от{" "}
                <Link
                  className="avari-gold"
                  to={`/projects/${p.id}/tasks/${id}`}
                >
                  {tasks.data?.find((t) => t.id === id)?.goal || id}
                </Link>
              </p>
            ))}
            <div className="mt-4 flex flex-wrap gap-3">
              {["draft", "ready"].includes(t.status) && (
                <Button variant="outline" onClick={() => setEditing(!editing)}>
                  Редактировать задание
                </Button>
              )}
              {t.status === "draft" && (
                <Button
                  isLoading={cmd.isPending}
                  onClick={() => command("ready")}
                >
                  Поставить в работу
                </Button>
              )}
              {t.status === "ready" && (
                <Button
                  isLoading={cmd.isPending}
                  onClick={() =>
                    command("start", { idempotency_key: startKey })
                  }
                >
                  Начать вручную
                </Button>
              )}
              {!["done", "cancelled"].includes(t.status) && (
                <Button
                  variant="outline"
                  isLoading={cmd.isPending}
                  onClick={() => command("cancel")}
                >
                  Отменить задачу
                </Button>
              )}
            </div>
          </Card>
          {editing && (
            <Card className="mb-6">
              <SpecForm
                key={`${t.id}:${t.version}`}
                project={p}
                tasks={tasks.data || []}
                task={t}
                pending={cmd.isPending}
                onSave={(spec) => command("edit", { spec })}
              />
            </Card>
          )}
          <Section title="Попытки и результаты">
            {t.runs.length ? (
              t.runs
                .slice()
                .reverse()
                .map((run) => (
                  <Delivery
                    key={run.id}
                    task={t}
                    run={run}
                    onCommand={command}
                    pending={cmd.isPending}
                  />
                ))
            ) : (
              <Empty text="Попыток пока нет." />
            )}
          </Section>
          <Section title="История">
            <Card>
              {t.events
                .slice()
                .reverse()
                .map((ev) => (
                  <p
                    className="py-2 border-b avari-divider break-words"
                    key={ev.id}
                  >
                    {ev.kind} · {ev.actor_name || ev.actor} ·{" "}
                    {new Date(ev.created_at).toLocaleString("ru-RU")}
                    <br />
                    {ev.text}
                    {ev.run_id && (
                      <small className="block avari-muted">
                        Попытка {ev.run_id}
                        {ev.result_id && ` · Результат ${ev.result_id}`}
                      </small>
                    )}
                  </p>
                ))}
            </Card>
          </Section>
        </>
      )}
    </>
  );
}
export function WorkResultsPage() {
  const { projectId = "" } = useParams();
  const project = useProject(projectId);
  const tasks = useWorkTasks(projectId);
  const query = useQuery({
    queryKey: ["work", "results", projectId],
    queryFn: () => workApi.results(projectId),
    refetchInterval: 5000,
  });
  const [task, setTask] = useState("");
  const [stage, setStage] = useState("");
  const [step, setStep] = useState("");
  const [actor, setActor] = useState("");
  const [state, setState] = useState("");
  const filtered = query.data?.filter((row) => {
    return (
      (!task || row.result.task_id === task) &&
      (!stage || row.stage_id === stage) &&
      (!step || row.step_ids.includes(step)) &&
      (!actor || row.actor === actor) &&
      (!state || row.state === state)
    );
  });
  return (
    <>
      <QueryState
        loading={project.isLoading || query.isLoading || tasks.isLoading}
        error={(project.error || query.error || tasks.error) as Error | null}
        retry={() => {
          project.refetch();
          query.refetch();
          tasks.refetch();
        }}
      />
      {project.data && (
        <>
          <ProjectHeader project={project.data} section="Результаты" />
          <PageTitle title="Результаты проекта">
            Сообщённые проверки и приёмка менеджером показаны отдельно.
          </PageTitle>
          <div className="grid gap-3 sm:grid-cols-3 mb-6">
            <Field label="Задача">
              <select
                className={fieldClass}
                value={task}
                onChange={(e) => setTask(e.target.value)}
              >
                <option value="">Все задачи</option>
                {tasks.data?.map((t) => (
                  <option key={t.id} value={t.id}>
                    {t.goal}
                  </option>
                ))}
              </select>
            </Field>
            <Field label="Стадия">
              <select
                className={fieldClass}
                value={stage}
                onChange={(e) => setStage(e.target.value)}
              >
                <option value="">Все стадии</option>
                {project.data.sdlc_stages.map((s) => (
                  <option key={s.id} value={s.id}>
                    {s.name}
                  </option>
                ))}
              </select>
            </Field>
            <Field label="Шаг">
              <select
                className={fieldClass}
                value={step}
                onChange={(e) => setStep(e.target.value)}
              >
                <option value="">Все шаги</option>
                {project.data.phases
                  .flatMap((p) => p.steps)
                  .map((s) => (
                    <option key={s.id} value={s.id}>
                      {s.code} · {s.name}
                    </option>
                  ))}
                {[
                  ...new Set([
                    ...(tasks.data?.flatMap((t) => t.step_ids) || []),
                    ...(query.data?.flatMap((row) => row.step_ids) || []),
                  ]),
                ]
                  .filter(
                    (id) =>
                      !project.data!.phases.some((p) =>
                        p.steps.some((s) => s.id === id),
                      ),
                  )
                  .map((id) => (
                    <option key={id} value={id}>
                      Прошлый цикл · {id}
                    </option>
                  ))}
              </select>
            </Field>
            <Field label="Исполнитель попытки">
              <select
                className={fieldClass}
                value={actor}
                onChange={(e) => setActor(e.target.value)}
              >
                <option value="">Все исполнители</option>
                {[...new Set(query.data?.map((r) => r.actor))].map((a) => (
                  <option key={a} value={a}>
                    {query.data?.find((row) => row.actor === a)?.actor_name ||
                      a}
                  </option>
                ))}
              </select>
            </Field>
            <Field label="Приёмка">
              <select
                className={fieldClass}
                value={state}
                onChange={(e) => setState(e.target.value)}
              >
                <option value="">Все сдачи</option>
                {["pending", "accept", "return", "cancelled"].map((s) => (
                  <option key={s} value={s}>
                    {labels[s]}
                  </option>
                ))}
              </select>
            </Field>
          </div>
          {filtered?.length
            ? filtered.map((row) => (
                <Card className="mb-4" key={row.result.id}>
                  <Link
                    className="avari-gold"
                    to={`/projects/${projectId}/tasks/${row.result.task_id}`}
                  >
                    {tasks.data?.find((t) => t.id === row.result.task_id)
                      ?.goal || "Открыть задачу"}
                  </Link>
                  <p className="mt-2">
                    <WorkStatus value={row.state} /> ·{" "}
                    {row.actor_name || row.actor}
                  </p>
                  <ResultView result={row.result} />
                  {(row.verification_reports || []).map((report) => (
                    <p className="mt-2 text-sm" key={report.id}>
                      Независимый{" "}
                      {report.kind === "qa" ? "QA" : "security review"} ·{" "}
                      {report.reviewer_name || report.reviewer_id}:{" "}
                      {report.summary}
                    </p>
                  ))}
                </Card>
              ))
            : !query.isLoading && (
                <Empty text="Сданных результатов по этому фильтру пока нет." />
              )}
        </>
      )}
    </>
  );
}
export function WorkOverview({ projectId }: { projectId: string }) {
  const query = useWorkTasks(projectId);
  const ts = query.data || [];
  return (
    <Section title="Работа и результаты">
      <QueryState
        loading={query.isLoading}
        error={query.error as Error | null}
        retry={() => query.refetch()}
      />
      <div className="grid gap-3 sm:grid-cols-4">
        {[
          {
            name: "Требует моего решения",
            n: ts.filter((t) => t.status === "in_review").length,
            path: "tasks?status=in_review",
          },
          {
            name: "В работе",
            n: ts.filter((t) => t.status === "in_progress").length,
            path: "tasks?status=in_progress",
          },
          {
            name: "Новые результаты",
            n: ts.flatMap((t) =>
              t.runs
                .filter(
                  (r) =>
                    t.status !== "cancelled" &&
                    r.state === "submitted" &&
                    r.review?.decision !== "return",
                )
                .flatMap((r) => r.results),
            ).length,
            path: "results",
          },
          {
            name: "Блокеры",
            n: ts.filter((t) => !!t.blocker).length,
            path: "tasks?blocker=1",
          },
        ].map((x) => (
          <Link key={x.name} to={`/projects/${projectId}/${x.path}`}>
            <Card>
              <p>{x.name}</p>
              <strong className="text-2xl">{x.n}</strong>
            </Card>
          </Link>
        ))}
      </div>
    </Section>
  );
}
export function LinkedWork({
  projectId,
  stepId,
  stageId,
}: {
  projectId: string;
  stepId?: string;
  stageId?: string;
}) {
  const query = useWorkTasks(projectId);
  const tasks = query.data?.filter(
    (t) =>
      (!stepId || t.step_ids.includes(stepId)) &&
      (!stageId || t.stage_id === stageId),
  );
  return (
    <div className="mt-4">
      <h3>Связанные задачи и принятые результаты</h3>
      <QueryState
        loading={query.isLoading}
        error={query.error as Error | null}
        retry={() => query.refetch()}
      />
      {tasks?.length
        ? tasks.map((t) => (
            <div key={t.id} className="mt-3">
              <Link
                className="avari-gold"
                to={`/projects/${projectId}/tasks/${t.id}`}
              >
                {t.goal || "Черновик"}
              </Link>{" "}
              · <WorkStatus value={t.status} />
              {t.runs
                .filter((r) => r.review?.decision === "accept")
                .flatMap((r) => r.results)
                .map((r) => (
                  <ResultView key={r.id} result={r} />
                ))}
            </div>
          ))
        : !query.isLoading && (
            <p className="avari-muted text-sm">Связанных задач пока нет.</p>
          )}
    </div>
  );
}
