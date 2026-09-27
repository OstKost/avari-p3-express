import { rmSync } from "node:fs";
export default function cleanup() {
  if (process.env.AVARI_TEST_OWNS_TEMP === "1" && process.env.AVARI_TEST_TEMP) {
    rmSync(process.env.AVARI_TEST_TEMP, { recursive: true, force: true });
  }
}
