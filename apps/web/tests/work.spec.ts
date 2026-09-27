import { createInterface } from "node:readline";
import type { WorkTask } from "../src/entities/work/types";
import { test, expect } from "@playwright/test";
import { readFileSync, openSync, closeSync, rmSync, mkdirSync } from "node:fs";
import { execFileSync, spawn, type ChildProcess } from "node:child_process";
import path from "node:path";
import { once } from "node:events";

test("task delivery, return, acceptance and projections in both themes", async ({
  page,
}, testInfo) => {
  const temp = path.join(process.env.AVARI_TEST_TEMP!, "work");
  mkdirSync(temp, { recursive: true });
  const keyPath = path.join(temp, "manager.key");
  const apiURL = `http://127.0.0.1:${process.env.AVARI_TEST_API_PORT!}`;
  const binary = path.join(temp, "server");
  execFileSync("go", ["build", "-o", binary, "./cmd/server"], {
    cwd: path.resolve("../api"),
    timeout: 120000,
  });
  const fd = openSync(testInfo.outputPath("api.log"), "a");
  let backend: ChildProcess | undefined;
  async function start() {
    backend = spawn(binary, [], {
      cwd: temp,
      env: {
        ...process.env,
        PORT: process.env.AVARI_TEST_API_PORT!,
        HOST: "127.0.0.1",
        DB_PATH: path.join(temp, "p3.db"),
        MANAGER_KEY_PATH: keyPath,
        ALLOWED_ORIGINS: process.env.AVARI_TEST_WEB_ORIGIN!,
      },
      stdio: ["ignore", fd, fd],
    });
    await expect
      .poll(
        async () => {
          try {
            return (await fetch(`${apiURL}/healthz`, { signal: AbortSignal.timeout(1000) })).status;
          } catch {
            return 0;
          }
        },
        { timeout: 15000 },
      )
      .toBe(200);
  }
  async function stop() {
    if (backend && backend.exitCode === null) {
      const ended = once(backend, "exit");
      backend.kill("SIGTERM");
      const timeout = setTimeout(() => backend?.kill("SIGKILL"), 15000);
      try { await ended; } finally { clearTimeout(timeout); }
    }
  }
  try {
    await start();
    const crashes: string[] = [];
    page.on("pageerror", (e) => crashes.push(e.message));
    await page.goto("/projects");
    await page
      .getByLabel("Ключ менеджера")
      .fill(readFileSync(keyPath, "utf8").trim());
    await page.getByRole("button", { name: "Войти", exact: true }).click();
    await expect(
      page.getByRole("heading", { name: "Проекты", exact: true }),
    ).toBeVisible();
    const response = await page.request.post("/api/v1/p3/projects", {
      data: {
        name: "Browser work project",
        description: "Isolated verification",
      },
    });
    expect(response.status()).toBe(201);
    const project = await response.json();
    await page.goto(`/projects/${project.id}/tasks`);
    await expect(
      page.getByText("Задач по этому фильтру пока нет."),
    ).toBeVisible();
    await page
      .getByRole("button", { name: "Новая задача", exact: true })
      .click();
    await page
      .getByLabel("Цель", { exact: true })
      .fill("Deliver verified patch");
    await page
      .getByLabel("Ожидаемый результат", { exact: true })
      .fill("Patch and proof");
    await page
      .getByLabel("Критерии приёмки — по одному на строку")
      .fill("Acceptance test passes");
    await page
      .getByRole("checkbox", {
        name: "Требовать независимый QA перед приёмкой",
      })
      .check();
    const linkedStep = project.phases[0].steps[0];
    await page
      .getByRole("checkbox", {
        name: `${linkedStep.code} · ${linkedStep.name}`,
        exact: true,
      })
      .check();
    await page
      .getByLabel("Стадия SDLC")
      .selectOption(project.sdlc_stages[0].id);
    await page
      .getByRole("button", { name: "Создать черновик", exact: true })
      .click();
    await expect(
      page.getByRole("heading", {
        name: "Deliver verified patch",
        exact: true,
      }),
    ).toBeVisible();
    await page
      .getByRole("button", { name: "Поставить в работу", exact: true })
      .click();
    await page
      .getByRole("button", { name: "Начать вручную", exact: true })
      .click();
    async function deliver(description: string) {
      const active = page
        .locator("section")
        .filter({
          has: page.getByRole("heading", { name: "Попытки и результаты" }),
        })
        .locator("div.avari-surface")
        .filter({ has: page.getByText("Активна", { exact: true }) });
      // The newest attempt comes first. Restrict editable fields to its card.
      const card = page
        .getByText("Активна", { exact: true })
        .locator('xpath=ancestor::div[contains(@class,"avari-surface")]')
        .first();
      const scope = (await active.count()) ? active.first() : card;
      await scope
        .getByText("Прикрепить результат вручную", { exact: true })
        .click();
      await scope.getByLabel("Что появилось").fill(description);
      await scope
        .getByLabel("Ссылки HTTP(S), по одной на строку")
        .fill("https://example.com/commit");
      await scope
        .getByLabel("Commit / версия документа или релиза")
        .fill("abc123");
      await scope
        .getByRole("button", { name: "Добавить результат", exact: true })
        .click();
      await expect(scope.getByText(description, { exact: true })).toBeVisible();
      await scope.getByText("Сдать на приёмку", { exact: true }).click();
      await scope
        .getByLabel("Отчёт, вопросы и ограничения")
        .fill("Done; checks reported by author");
      await scope.getByLabel("Способ проверки").fill("Playwright");
      await scope.getByLabel("Источник", { exact: true }).fill("Agent report");
      await scope.getByLabel("Доказательства").fill("Captured local output");
      await scope
        .getByRole("button", { name: "Сдать результат", exact: true })
        .click();
    }

    async function agentDelivery(role: "executor" | "reviewer" = "executor") {
      const id = page.url().split("/").at(-1)!;
      const tokenResponse = await page.request.post("/api/v1/agent-tokens", {
        data: { name: `Browser MCP ${role}`, projects: [project.id], role },
      });
      expect(tokenResponse.status()).toBe(201);
      const { secret } = await tokenResponse.json();
      const mcpBinary = path.join(temp, "mcp");
      execFileSync("go", ["build", "-o", mcpBinary, "./cmd/mcp"], {
        cwd: path.resolve("../api"),
      });
      const child = spawn(mcpBinary, [], {
        env: {
          ...process.env,
          AVARI_API_URL: apiURL,
          AVARI_AGENT_TOKEN: secret,
        },
        stdio: ["pipe", "pipe", fd],
      });
      const lines = createInterface({ input: child.stdout });
      let serial = 0;
      const waiting = new Map<
        number,
        {
          resolve: (v: Record<string, unknown>) => void;
          reject: (e: Error) => void;
        }
      >();
      lines.on("line", (line) => {
        const message = JSON.parse(line);
        const pending = waiting.get(message.id);
        if (pending) {
          waiting.delete(message.id);
          if (message.error) pending.reject(new Error("MCP protocol error"));
          else pending.resolve(message.result);
        }
      });
      function rpc(
        method: string,
        params: Record<string, unknown>,
      ): Promise<Record<string, unknown>> {
        const id = ++serial;
        return new Promise((resolve, reject) => {
          const timer = setTimeout(() => {
            waiting.delete(id);
            reject(new Error("MCP timeout"));
          }, 10000);
          waiting.set(id, {
            resolve: (v) => {
              clearTimeout(timer);
              resolve(v);
            },
            reject: (e) => {
              clearTimeout(timer);
              reject(e);
            },
          });
          child.stdin.write(
            JSON.stringify({ jsonrpc: "2.0", id, method, params }) + "\n",
          );
        });
      }
      try {
        await rpc("initialize", {
          protocolVersion: "2025-06-18",
          capabilities: {},
          clientInfo: { name: "browser-verifier", version: "1" },
        });
        child.stdin.write(
          JSON.stringify({
            jsonrpc: "2.0",
            method: "notifications/initialized",
          }) + "\n",
        );
        const tools = await rpc("tools/list", {});
        expect((tools.tools as unknown[]).length).toBe(12);
        const context = await (
          await page.request.get(`/api/v1/work-tasks/${id}`)
        ).json();
        let task: WorkTask = context.task;
        async function call(name: string, args: Record<string, unknown>) {
          const result = await rpc("tools/call", {
            name,
            arguments: { task_id: id, version: task.version, ...args },
          });
          expect(result.isError).not.toBe(true);
          task = result.structuredContent as WorkTask;
        }
        if (role === "reviewer") {
          const run = task.runs.at(-1)!;
          const forbidden = await rpc("tools/call", {
            name: "start_task",
            arguments: {
              task_id: id,
              version: task.version,
              idempotency_key: "reviewer-start",
            },
          });
          expect(forbidden.isError).toBe(true);
          const args = {
            run_id: run.id,
            submission_digest: run.submission_digest,
            kind: "qa",
            summary:
              "Independent review: browser behavior checked; migration not observed",
            checks: [
              {
                criterion: 0,
                outcome: "unknown",
                method: "Independent browser inspection",
                source: "Browser MCP reviewer",
                evidence:
                  "Insufficient evidence for complete acceptance criterion",
              },
            ],
            idempotency_key: "browser-independent-qa",
          };
          const first = await rpc("tools/call", {
            name: "report_verification",
            arguments: args,
          });
          expect(first.isError).not.toBe(true);
          const again = await rpc("tools/call", {
            name: "report_verification",
            arguments: args,
          });
          expect(again.isError).not.toBe(true);
          expect(
            (again.structuredContent as WorkTask).runs.at(-1)!
              .verification_reports,
          ).toHaveLength(1);
          return;
        }
        await call("start_task", { idempotency_key: "browser-agent-start" });
        const run_id = task.runs.at(-1)!.id;
        await call("report_progress", {
          run_id,
          text: "Implementing revision",
        });
        await call("add_result", {
          run_id,
          idempotency_key: "browser-agent-result",
          result: {
            description: "Revised patch",
            kind: "code",
            urls: ["https://example.com/commit"],
            artifact_version: "abc123",
          },
        });
        await call("submit_task", {
          run_id,
          idempotency_key: "browser-agent-submit",
          text: "Done; checks reported by author",
          checks: [
            {
              criterion: 0,
              outcome: "pass",
              method: "Playwright",
              source: "Agent report",
              evidence: "Captured local output",
            },
          ],
        });
      } finally {
        lines.close();
        const exit = once(child, "exit");
        child.stdin.end();
        await exit;
      }
    }
    await deliver("First patch");
    await page
      .getByLabel("Замечание менеджера (обязательно при возврате)")
      .fill("Need better evidence");
    await page.getByRole("button", { name: "Вернуть", exact: true }).click();
    await expect(
      page.getByText("Need better evidence", { exact: false }).first(),
    ).toBeVisible();
    await agentDelivery();
    await expect(page.getByText("Revised patch", { exact: true })).toBeVisible({
      timeout: 10000,
    });
    const accept = page.getByRole("button", { name: "Принять", exact: true });
    await expect(accept).toBeDisabled();
    await agentDelivery("reviewer");
    await expect(
      page.getByText(
        "Independent review: browser behavior checked; migration not observed",
        { exact: true },
      ),
    ).toBeVisible({ timeout: 10000 });
    await expect(accept).toBeDisabled();
    await page
      .getByLabel("Замечание менеджера (обязательно при возврате)")
      .fill(
        "Accept with documented limitation; separate migration evidence exists",
      );
    await expect(accept).toBeEnabled();
    await accept.click();
    await expect(
      page.getByText("Принято менеджером", { exact: false }),
    ).toBeVisible();
    await expect(
      page.getByText("Проверки исполнителя — сообщает исполнитель"),
    ).toHaveCount(2);
    const taskURL = page.url();
    await stop();
    await start();
    await page.reload();
    await expect(
      page.getByRole("heading", { name: "Вход менеджера", exact: true }),
    ).toBeVisible();
    await page
      .getByLabel("Ключ менеджера")
      .fill(readFileSync(keyPath, "utf8").trim());
    await page.getByRole("button", { name: "Войти", exact: true }).click();
    await expect(
      page.getByText("Revised patch", { exact: true }),
    ).toBeVisible();
    await expect(
      page.getByText(
        "Independent review: browser behavior checked; migration not observed",
        { exact: true },
      ),
    ).toBeVisible();
    await page.goto(`/projects/${project.id}/results`);
    await expect(
      page.getByText("Revised patch", { exact: true }),
    ).toBeVisible();
    await page
      .getByRole("combobox", { name: "Приёмка", exact: true })
      .selectOption("accept");
    await expect(page.getByText("First patch", { exact: true })).toHaveCount(0);
    await page.goto(`/projects/${project.id}`);
    await expect(
      page.getByText("Требует моего решения", { exact: true }),
    ).toBeVisible();
    await page.goto(`/projects/${project.id}/sdlc`);
    await expect(
      page.getByText("Revised patch", { exact: true }),
    ).toBeVisible();
    await page.goto(`/projects/${project.id}/p3/A/${linkedStep.code}`);
    await expect(
      page.getByText("Revised patch", { exact: true }),
    ).toBeVisible();
    for (const theme of ["light", "dark"]) {
      await page.evaluate((theme) => {
        document.documentElement.classList.toggle("dark", theme === "dark");
        document.documentElement.classList.toggle("light", theme === "light");
        localStorage.setItem("theme", theme);
      }, theme);
      await page.setViewportSize({ width: 375, height: 812 });
      await page.goto(taskURL);
      await expect(
        page.getByText(
          "Independent review: browser behavior checked; migration not observed",
          { exact: true },
        ),
      ).toBeVisible();
      expect(
        await page.evaluate(
          () => document.documentElement.scrollWidth <= window.innerWidth,
        ),
      ).toBe(true);
      await page.screenshot({
        path: testInfo.outputPath(`qa-${theme}-375.png`),
        fullPage: true,
      });
      await page.getByRole("link", { name: "Задачи", exact: true }).focus();
      await expect(
        page.getByRole("link", { name: "Задачи", exact: true }),
      ).toBeFocused();
      await page.keyboard.press("Enter");
      await expect(
        page.getByRole("heading", {
          name: "Производственные задачи",
          exact: true,
        }),
      ).toBeVisible();
      expect(
        await page.evaluate(
          () => document.documentElement.scrollWidth <= window.innerWidth,
        ),
      ).toBe(true);
      await page.screenshot({
        path: testInfo.outputPath(`tasks-${theme}-375.png`),
        fullPage: true,
      });
    }
    // API-created draft uses normalized empty arrays and stays renderable.
    const draftResponse = await page.request.post("/api/v1/work-tasks", {
      data: { project_id: project.id, goal: "Bare REST draft" },
    });
    expect(draftResponse.status()).toBe(201);
    const draft = await draftResponse.json();
    await page.goto(`/projects/${project.id}/tasks/${draft.id}`);
    await expect(
      page.getByRole("heading", { name: "Bare REST draft", exact: true }),
    ).toBeVisible();
    // Simulate an expired/restarted server session without silently replaying a write.
    await page.context().clearCookies();
    await page.goto(taskURL);
    await expect(
      page.getByRole("heading", { name: "Вход менеджера", exact: true }),
    ).toBeVisible();
    await page
      .getByLabel("Ключ менеджера")
      .fill(readFileSync(keyPath, "utf8").trim());
    await page.getByRole("button", { name: "Войти", exact: true }).click();
    await expect(
      page.getByText("Revised patch", { exact: true }),
    ).toBeVisible();
    await page.route("**/api/v1/work-tasks?**", (route) =>
      route.fulfill({
        status: 500,
        contentType: "application/json",
        body: JSON.stringify({ error: "Controlled list failure" }),
      }),
    );
    await page.goto(`/projects/${project.id}/tasks`);
    await expect(
      page.getByRole("alert").filter({ hasText: "Controlled list failure" }),
    ).toBeVisible();
    await page.unroute("**/api/v1/work-tasks?**");
    await page.getByRole("button", { name: "Повторить", exact: true }).click();
    await expect(
      page.getByRole("alert").filter({ hasText: "Controlled list failure" }),
    ).toHaveCount(0);
    await expect(
      page.getByRole("link", { name: "Deliver verified patch", exact: false }),
    ).toBeVisible();
    expect(crashes).toEqual([]);
  } finally {
    await stop();
    closeSync(fd);
    rmSync(temp, { recursive: true, force: true });
  }
});
