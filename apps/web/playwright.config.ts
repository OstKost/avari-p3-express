import { defineConfig } from "@playwright/test";
import { mkdtempSync, mkdirSync, writeFileSync } from "node:fs";
import { createServer } from "node:net";
import { tmpdir } from "node:os";
import path from "node:path";

async function freePort(): Promise<string> {
  return await new Promise((resolve, reject) => {
    const server = createServer();
    server.on("error", reject);
    server.listen(0, "127.0.0.1", () => {
      const address = server.address();
      if (!address || typeof address === "string") return reject(new Error("No test port"));
      server.close((error) => error ? reject(error) : resolve(String(address.port)));
    });
  });
}
const temp = process.env.AVARI_TEST_TEMP || mkdtempSync(path.join(tmpdir(), "avari-browser-"));
if (!process.env.AVARI_TEST_TEMP) process.env.AVARI_TEST_OWNS_TEMP = "1";
process.env.AVARI_TEST_TEMP = temp;
const apiPort = process.env.AVARI_TEST_API_PORT || await freePort();
const webPort = process.env.AVARI_TEST_WEB_PORT || await freePort();
process.env.AVARI_TEST_API_PORT = apiPort;
process.env.AVARI_TEST_WEB_PORT = webPort;
const origin = `http://127.0.0.1:${webPort}`;
process.env.AVARI_TEST_WEB_ORIGIN = origin;
const evidence = process.env.AVARI_TEST_EVIDENCE || path.resolve("../../.harness/runs/browser-" + path.basename(temp));
process.env.AVARI_TEST_EVIDENCE = evidence;
mkdirSync(evidence, { recursive: true });
writeFileSync(path.join(evidence, "runtime.json"), JSON.stringify({ apiURL: `http://127.0.0.1:${apiPort}`, webURL: origin, fixtureDate: "2026-09-27", fixtureVersion: 1, profile: "demo", retries: 0 }, null, 2));
process.env.AVARI_TEST_KEY_PATH = path.join(temp, "manager.key");
export default defineConfig({
  testDir: "./tests",
  globalTeardown: "./tests/cleanup.ts",
  workers: 1,
  retries: 0,
  timeout: 90000,
  reporter: [["list"], ["html", { outputFolder: path.join(evidence, "report"), open: "never" }]],
  outputDir: path.join(evidence, "artifacts"),
  use: { baseURL: origin, trace: "off", screenshot: "only-on-failure" },
  webServer: [{
    command: `pnpm dev --host 127.0.0.1 --port ${webPort} --strictPort`,
    url: origin,
    reuseExistingServer: false,
    env: { AVARI_PROXY_TARGET: `http://127.0.0.1:${apiPort}` },
  }],
});
