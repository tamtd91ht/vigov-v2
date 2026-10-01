import { fileURLToPath } from "node:url";

import { defineConfig } from "vitest/config";

/**
 * Mirrors web-admin's vitest config: Node environment, no jsdom. What this shell must guarantee
 * — the gateway fails closed, the headers are present, the proxy denies — is a property of an
 * HTTP exchange and of a config object, not of a rendered DOM. A presentational component is
 * rendered to a string with `react-dom/server` when a test needs to ask "is the sentence there".
 *
 * TZ is pinned to UTC for the reason web-admin's config records: a pin equal to the business time
 * zone (+07) would let a format call that forgot `timeZone` go green on every workstation here.
 */
export default defineConfig({
  resolve: {
    alias: { "@": fileURLToPath(new URL("./src", import.meta.url)) },
  },
  test: {
    environment: "node",
    env: { TZ: "UTC" },
    include: ["src/**/*.test.ts", "src/**/*.test.tsx"],
  },
});
