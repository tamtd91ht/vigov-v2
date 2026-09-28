import { afterEach, describe, expect, it, vi } from "vitest";

import { LOI_KHONG_RO } from "./goi";
import {
  TASK_IMPORT_FILE_FIELD,
  TASK_IMPORT_TOO_LARGE,
  downloadTaskImportTemplate,
  submitTaskImport,
} from "./task-import";

/**
 * `POST /api/v1/tasks/imports` and `GET /api/v1/tasks/import-template` (816ef81). Pinned: the route,
 * the mode (`dry_run=true` only when asked), the multipart part name, the key, and how each answer —
 * 200 check, 200 refused rows, 201, replayed 201, refusals — comes back.
 */

function stub(res: Response) {
  const fake = vi.fn(async (_url: string, _init?: RequestInit) => res);
  vi.stubGlobal("fetch", fake);
  return fake;
}
const json = (body: unknown, status: number, headers: Record<string, string> = {}) =>
  new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json", ...headers } });
const FILE = new Blob(["PK"], { type: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" });
const REPORT = { total_rows: 2, created: 0, committed: false, errors: [], codes: [] };

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("POST /api/v1/tasks/imports", () => {
  it("real import: POST, no `dry_run`, multipart part `file`, the caller's Idempotency-Key, no hand-set Content-Type", async () => {
    const fake = stub(json({ ...REPORT, created: 2, committed: true, codes: ["NV31", "NV32"] }, 201));
    const r = await submitTaskImport(FILE, "nhiem-vu.xlsx", "khoa-gia", false);
    expect(fake.mock.calls[0]?.[0]).toBe("/api/v1/tasks/imports");
    const init = fake.mock.calls[0]?.[1];
    expect(init?.method).toBe("POST");
    expect(init?.credentials).toBe("same-origin");
    const headers = init?.headers as Record<string, string>;
    expect(headers["Idempotency-Key"]).toBe("khoa-gia");
    expect(headers["Content-Type"]).toBeUndefined();
    const body = init?.body as FormData;
    expect(body.get(TASK_IMPORT_FILE_FIELD)).toBeInstanceOf(Blob);
    expect(r).toEqual({
      ok: true,
      duLieu: { kind: "imported", report: { ...REPORT, created: 2, committed: true, codes: ["NV31", "NV32"] } },
    });
  });

  it("check: `?dry_run=true` — 200 with the row report is an ANSWER, not a failure", async () => {
    const errors = [{ row: 3, column: "Hạn hoàn thành (ngày giờ)", message: "hạn hoàn thành phải có cả ngày và giờ" }];
    const fake = stub(json({ ...REPORT, errors }, 200));
    const r = await submitTaskImport(FILE, "a.xlsx", "k", true);
    expect(fake.mock.calls[0]?.[0]).toBe("/api/v1/tasks/imports?dry_run=true");
    expect(r).toEqual({ ok: true, duLieu: { kind: "checked", report: { ...REPORT, errors } } });
  });

  it("replayed 201 (`Idempotent-Replay: true`): the first issued code, nothing new", async () => {
    stub(json({ code: "NV31", replayed: true }, 201, { "Idempotent-Replay": "true" }));
    expect(await submitTaskImport(FILE, "a.xlsx", "k", false)).toEqual({
      ok: true,
      duLieu: { kind: "replayed", firstCode: "NV31" },
    });
  });

  it("missing arrays in a report become empty — a render never meets `undefined`", async () => {
    stub(json({ total_rows: 1, committed: false }, 200));
    const r = await submitTaskImport(FILE, "a.xlsx", "k", true);
    expect(r.ok && r.duLieu.kind === "checked" && r.duLieu.report.errors).toEqual([]);
  });

  for (const status of [400, 403, 422, 503]) {
    it(`${status}: the server's sentence VERBATIM`, async () => {
      const cau = `Câu giả ${status}.`;
      stub(json({ code: "x", message: cau, trace_id: "t" }, status));
      expect(await submitTaskImport(FILE, "a.xlsx", "k", false)).toEqual({ ok: false, thongBao: cau });
    });
  }

  it("413 from the INGRESS (HTML, no sentence): the size sentence, not \"cannot connect\"", async () => {
    stub(new Response("<html>413</html>", { status: 413, headers: { "Content-Type": "text/html" } }));
    expect(await submitTaskImport(FILE, "a.xlsx", "k", false)).toEqual({ ok: false, thongBao: TASK_IMPORT_TOO_LARGE });
  });

  it("network failure: the generic sentence", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => Promise.reject(new Error("x"))));
    expect(await submitTaskImport(FILE, "a.xlsx", "k", false)).toEqual({ ok: false, thongBao: LOI_KHONG_RO });
  });
});

describe("GET /api/v1/tasks/import-template", () => {
  it("200 ⇒ the bytes; 403 ⇒ the sentence", async () => {
    const fake = stub(new Response(new Uint8Array([80, 75]), { status: 200 }));
    const r = await downloadTaskImportTemplate();
    expect(fake.mock.calls[0]?.[0]).toBe("/api/v1/tasks/import-template");
    expect(r.ok && r.duLieu.size).toBe(2);
    vi.unstubAllGlobals();
    stub(json({ code: "forbidden", message: "Không có quyền.", trace_id: "t" }, 403));
    expect(await downloadTaskImportTemplate()).toEqual({ ok: false, thongBao: "Không có quyền." });
  });
});
