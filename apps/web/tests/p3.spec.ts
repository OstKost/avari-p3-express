import { test, expect, type Page, type TestInfo } from "@playwright/test";
import { execFileSync, spawn, type ChildProcess } from "node:child_process";
import { readFileSync, openSync, closeSync, rmSync, mkdtempSync } from "node:fs";
import path from "node:path";
import type { Project, ProjectSummary, Phase, Cycle } from "../src/entities/p3/types";

// These tests exercise real seeded services. Demo artifacts remain synthetic;
// screenshots and assertions below are actual local verification evidence.
async function demo(page: Page, info: TestInfo) {
  const root = path.resolve("../..");
  const temp = mkdtempSync(path.join(process.env.AVARI_TEST_TEMP!, "p3-"));
  const directory = path.join(temp, "demo");
  const script = path.join(root, "scripts/demo.py");
  const binary = path.join(temp, "server");
  let backend: ChildProcess | undefined;
  const crashes: string[] = [];
  page.on("pageerror", (error) => crashes.push(error.message));
  const fd = openSync(info.outputPath("p3-api.log"), "a");
  const apiURL = `http://127.0.0.1:${process.env.AVARI_TEST_API_PORT || "14820"}`;
  const origin = process.env.AVARI_TEST_WEB_ORIGIN!;
  const keyPath = path.join(temp, "manager.key");
  async function stop() {
    if (!backend || backend.exitCode !== null || backend.signalCode !== null) return;
    const child = backend;
    await new Promise<void>((resolve) => {
      const timer = setTimeout(() => child.kill("SIGKILL"), 3000);
      child.once("exit", () => { clearTimeout(timer); resolve(); });
      child.kill("SIGTERM");
    });
  }
  async function cleanup() {
    await stop();
    closeSync(fd);
    rmSync(temp, { recursive: true, force: true });
    expect(crashes).toEqual([]);
  }
  async function start() {
    backend = spawn(binary, [], {
      cwd: temp,
      env: { ...process.env, PORT: process.env.AVARI_TEST_API_PORT || "14820", HOST: "127.0.0.1", DB_PATH: paths.db, MANAGER_KEY_PATH: keyPath, ALLOWED_ORIGINS: origin },
      stdio: ["ignore", fd, fd],
    });
    await expect.poll(async () => {
      try { return (await fetch(`${apiURL}/healthz`, { signal: AbortSignal.timeout(1000) })).status; }
      catch { return 0; }
    }, { timeout: 15000 }).toBe(200);
  }
  async function login() {
    await expect(page.getByLabel("Ключ менеджера")).toBeVisible();
    await page.getByLabel("Ключ менеджера").fill(readFileSync(keyPath, "utf8").trim());
    await page.getByRole("button", { name: "Войти", exact: true }).click();
    await expect(page.getByLabel("Ключ менеджера")).toHaveCount(0);
  }
  async function project(id: string): Promise<Project> {
    const response = await page.request.get(`/api/v1/p3/projects/${id}`);
    expect(response.status()).toBe(200);
    return response.json();
  }
  let paths: { db: string; manifest: string };
  try {
    execFileSync("python3", [script, "seed", "--dir", directory], { timeout: 30000 });
    paths = JSON.parse(execFileSync("python3", [script, "paths", "--dir", directory], { encoding: "utf8", timeout: 10000 }));
    const manifest = JSON.parse(readFileSync(paths.manifest, "utf8")) as { projects: Record<string, string> };
    execFileSync("go", ["build", "-o", binary, "./cmd/server"], { cwd: path.join(root, "apps/api"), timeout: 30000 });
    await start();
    await page.goto("/projects");
    await login();
    await expect(page.getByRole("heading", { name: "Проекты", exact: true })).toBeVisible();
    return { manifest, paths, start, stop, login, project, cleanup };
  } catch (error) { await cleanup(); throw error; }
}

function phase(project: Project, code: string): Phase {
  const result = project.phases.find((entry) => entry.code === code);
  expect(result).toBeDefined();
  return result!;
}

function assertProgress(project: Project) {
  for (const item of project.phases) {
    for (const step of item.steps) {
      expect(step.progress).toBe(step.checklist.length ? Math.floor(step.checklist.filter((check) => check.done).length * 100 / step.checklist.length) : 0);
    }
    expect(item.progress).toBe(Math.floor(item.steps.reduce((sum, step) => sum + step.progress, 0) / item.steps.length));
  }
}

test("P3 checklist keyboard change persists after API restart", async ({ page }, info) => {
  const fixture = await demo(page, info);
  try {
    const id = fixture.manifest.projects.shop;
    const before = await fixture.project(id);
    const step = phase(before, "A").steps[0];
    const item = step.checklist.find((entry) => !entry.done)!;
    expect(item).toBeDefined();
    await page.getByRole("link").filter({ has: page.getByRole("heading", { name: before.name, exact: true }) }).click();
    await expect(page.getByRole("heading", { name: before.name, exact: true })).toBeVisible();
    await page.goto(`/projects/${id}/p3/A/${step.code}`);
    await page.setViewportSize({ width: 375, height: 812 });
    const checkbox = page.getByRole("checkbox", { name: item.text, exact: true });
    await checkbox.focus();
    await expect(checkbox).toBeFocused();
    await page.keyboard.press("Space");
    await expect(checkbox).toBeChecked();
    await expect.poll(async () => phase(await fixture.project(id), "A").steps[0].checklist.find((entry) => entry.id === item.id)?.done).toBe(true);
    const after = await fixture.project(id);
    assertProgress(after);
    expect(after.progress).toBe(before.progress);
    expect(phase(after, "A").steps[0].progress).toBeGreaterThan(step.progress);
    const progressCard = page.locator("div.avari-surface").filter({ has: page.getByText("Чеклист", { exact: true }) });
    await expect(progressCard.getByText(`${phase(after, "A").steps[0].progress}%`, { exact: true })).toBeVisible();
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
    await page.screenshot({ path: info.outputPath("p3-checklist-375.png"), fullPage: true });
    // A controlled connection failure must show an actionable error and recover.
    const endpoint = `**/api/v1/p3/projects/${id}`;
    await page.route(endpoint, (route) => route.abort("connectionrefused"));
    await page.reload();
    const error = page.getByRole("alert").filter({ hasText: "Не удалось загрузить данные" });
    await expect(error).toBeVisible();
    await page.unroute(endpoint);
    await error.getByRole("button", { name: "Повторить", exact: true }).click();
    await expect(error).toHaveCount(0);
    await expect(checkbox).toBeChecked();
    await fixture.stop();
    await fixture.start();
    await page.reload();
    await fixture.login();
    await expect(checkbox).toBeChecked();
    const restored = await fixture.project(id);
    expect(restored.phases).toEqual(after.phases);
    assertProgress(restored);
  } finally { await fixture.cleanup(); }
});

test("P3 new B cycle preserves old steps and linked production tasks", async ({ page }, info) => {
  const fixture = await demo(page, info);
  try {
    const id = fixture.manifest.projects.shop;
    const before = await fixture.project(id);
    const oldPhase = phase(before, "B");
    const taskURL = `/api/v1/work-tasks?project_id=${id}`;
    const taskResponse = await page.request.get(taskURL);
    expect(taskResponse.status()).toBe(200);
    const tasks = await taskResponse.json();
    expect(tasks.data.some((task: { step_ids: string[] }) => task.step_ids.some((step) => oldPhase.steps.some((old) => old.id === step)))).toBe(true);
    const cyclesURL = `/api/v1/p3/projects/${id}/cycles`;
    const cyclesBefore = (await (await page.request.get(cyclesURL)).json()).data as Cycle[];
    await page.goto(`/projects/${id}/p3/B`);
    page.once("dialog", (dialog) => dialog.accept());
    await page.getByRole("button", { name: "Новый цикл", exact: true }).click();
    await expect.poll(async () => phase(await fixture.project(id), "B").id).not.toBe(oldPhase.id);
    const after = await fixture.project(id);
    expect(phase(after, "B").steps.every((step) => !oldPhase.steps.some((old) => old.id === step.id))).toBe(true);
    expect(phase(after, "B").progress).toBe(0);
    const cyclesAfter = (await (await page.request.get(cyclesURL)).json()).data as Cycle[];
    expect(cyclesAfter).toHaveLength(cyclesBefore.length + 1);
    for (const cycle of cyclesBefore) expect(cyclesAfter).toContainEqual(cycle);
    expect(await (await page.request.get(taskURL)).json()).toEqual(tasks);
    // The current-project endpoint exposes only the newest cycle. Read the old
    // rows directly to verify retained history, without writing to SQLite.
    const stored = JSON.parse(execFileSync("python3", ["-c", "import sqlite3,json,sys; db=sqlite3.connect('file:'+sys.argv[1]+'?mode=ro',uri=True); print(json.dumps(db.execute('select id from p3_steps where cycle_id=? order by code',(sys.argv[2],)).fetchall()))", fixture.paths.db, oldPhase.id], { encoding: "utf8", timeout: 10000 })) as string[][];
    expect(stored.map((row) => row[0])).toEqual(oldPhase.steps.map((step) => step.id));
  } finally { await fixture.cleanup(); }
});

test("P3 blocker resolution survives restart", async ({ page }, info) => {
  const fixture = await demo(page, info);
  try {
    const id = fixture.manifest.projects.ai;
    const before = await fixture.project(id);
    const countBlockers = async () => {
      const response = await page.request.get("/api/v1/p3/projects");
      expect(response.status()).toBe(200);
      const body = await response.json();
      return (body.data as ProjectSummary[]).find((entry) => entry.id === id)!.open_blockers;
    };
    const beforeCount = await countBlockers();
    const step = phase(before, "A").steps[0];
    const blocker = step.blockers.find((entry) => !entry.resolved)!;
    expect(blocker).toBeDefined();
    await page.goto(`/projects/${id}/p3/A/${step.code}`);
    const row = page.getByText(blocker.title, { exact: false }).locator("..");
    await row.getByRole("button", { name: "Закрыть", exact: true }).click();
    await expect(row.getByRole("button", { name: "Открыть", exact: true })).toBeVisible();
    await expect.poll(async () => (await fixture.project(id)).blockers.find((entry) => entry.id === blocker.id)?.resolved).toBe(true);
    const afterCount = await countBlockers();
    expect(afterCount).toBe(beforeCount! - 1);
    await fixture.stop();
    await fixture.start();
    await page.reload();
    await fixture.login();
    await expect(row.getByRole("button", { name: "Открыть", exact: true })).toBeVisible();
    expect(await countBlockers()).toBe(afterCount);
  } finally { await fixture.cleanup(); }
});
