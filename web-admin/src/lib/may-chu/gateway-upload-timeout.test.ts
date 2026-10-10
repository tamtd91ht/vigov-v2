import { afterEach, describe, expect, it, vi } from "vitest";

vi.mock("server-only", () => ({}));

const calls: { idleTimeoutMs?: number; method: string }[] = [];
vi.mock("./goi-noi-bo", async (importOriginal) => {
  const real = await importOriginal<typeof import("./goi-noi-bo")>();
  return {
    ...real,
    // Records what the gateway asked for, then fails the call — the status is not what is tested here.
    goiNoiBo: vi.fn(async (_service: unknown, req: { idleTimeoutMs?: number; method: string }) => {
      calls.push({ idleTimeoutMs: req.idleTimeoutMs, method: req.method });
      throw new real.LoiGoiNoiBo("khong-ket-noi");
    }),
  };
});

import { chuyenTiep } from "./chuyen-tiep";
import { THOI_GIAN_CHO_MS, UPLOAD_IDLE_TIMEOUT_MS } from "./goi-noi-bo";

/**
 * ADR 0052 §Sửa đổi 09/10/2026: every upload crosses this gateway as ONE multipart POST, and the service
 * is silent while it scans the file (up to its own 180 s deadline). The 30 s idle timer must not cut it.
 */

const HOST = "tanphu.example.gov.vn";

function req(path: string, init: RequestInit & { headers?: Record<string, string> }) {
  return new Request(`http://${HOST}${path}`, { ...init, headers: { host: HOST, ...init.headers } });
}

afterEach(() => {
  calls.length = 0;
  vi.restoreAllMocks();
});

describe("chuyenTiep — the idle timer of an upload", () => {
  it("a multipart POST gets UPLOAD_IDLE_TIMEOUT_MS, longer than the service's 180 s upload deadline", async () => {
    vi.spyOn(console, "info").mockImplementation(() => {});
    const form = new FormData();
    form.append("size", "3");
    form.append("file", new Blob(["abc"]), "a.pdf");
    await chuyenTiep(req("/api/v1/tasks/NV19/attachments", { method: "POST", body: form }));

    expect(calls).toEqual([{ idleTimeoutMs: UPLOAD_IDLE_TIMEOUT_MS, method: "POST" }]);
    expect(UPLOAD_IDLE_TIMEOUT_MS).toBeGreaterThan(180_000);
  });

  it("every other call keeps the default 30 s (no override)", async () => {
    vi.spyOn(console, "info").mockImplementation(() => {});
    await chuyenTiep(
      req("/api/v1/tasks/NV19/log-entries", {
        method: "POST",
        body: JSON.stringify({ note: "x" }),
        headers: { "content-type": "application/json" },
      }),
    );
    await chuyenTiep(req("/api/v1/tasks/NV19", { method: "GET" }));

    expect(calls.map((c) => c.idleTimeoutMs)).toEqual([undefined, undefined]);
    expect(THOI_GIAN_CHO_MS).toBe(30_000);
  });
});
