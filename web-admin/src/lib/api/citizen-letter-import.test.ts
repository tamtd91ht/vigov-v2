import { afterEach, describe, expect, it, vi } from "vitest";

import {
  LETTER_IMPORT_ROUTES,
  LETTER_ROWS_FIELD,
  downloadLetterReport,
  letterReportExportPath,
} from "./citizen-letter-import";
import { commitImport, previewImport } from "./excel-import";
import { LOI_KHONG_RO } from "./goi";

/**
 * The citizen-letter import (three routes, the shared `excel-import.ts` client) and the year's report
 * export. Pinned: the paths, the row-list name, the key, `year` always sent, and how a refusal returns.
 */

function stub(res: Response) {
  const fake = vi.fn(async (_url: string, _init?: RequestInit) => res);
  vi.stubGlobal("fetch", fake);
  return fake;
}
const json = (body: unknown, status: number) =>
  new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
const FILE = new Blob(["PK"]);

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("citizen-letter import routes", () => {
  it("the three paths of service-documents, and the preview's `letters` list", () => {
    expect(LETTER_IMPORT_ROUTES).toEqual({
      template: "/api/v1/citizen-letters/import-template",
      previews: "/api/v1/citizen-letters/import-previews",
      imports: "/api/v1/citizen-letters/imports",
    });
    expect(LETTER_ROWS_FIELD).toBe("letters");
  });

  it("preview: 200 `valid:false` with row errors is an ANSWER; no Idempotency-Key on a check", async () => {
    const errors = [{ row: 4, column: "Loại đơn", message: "Loại đơn không hợp lệ." }];
    const fake = stub(json({ valid: false, letters: [], errors }, 200));
    const r = await previewImport(LETTER_IMPORT_ROUTES, LETTER_ROWS_FIELD, FILE, "so.xlsx");
    expect(fake.mock.calls[0]?.[0]).toBe("/api/v1/citizen-letters/import-previews");
    expect((fake.mock.calls[0]?.[1]?.headers as Record<string, string> | undefined)?.["Idempotency-Key"]).toBeUndefined();
    expect(r).toEqual({ ok: true, duLieu: { valid: false, rows: [], errors } });
  });

  it("import: the caller's key; 400 import_invalid returns every refused row and the server's sentence", async () => {
    const errors = [{ row: 2, column: "", message: "Dòng trống." }];
    const fake = stub(json({ code: "import_invalid", message: "Tệp có lỗi nên chưa đơn nào được vào sổ.", trace_id: "", errors }, 400));
    const r = await commitImport(LETTER_IMPORT_ROUTES, FILE, "so.xlsx", "khoa-1");
    expect(fake.mock.calls[0]?.[0]).toBe("/api/v1/citizen-letters/imports");
    expect((fake.mock.calls[0]?.[1]?.headers as Record<string, string>)["Idempotency-Key"]).toBe("khoa-1");
    expect(r).toEqual({ ok: false, message: "Tệp có lỗi nên chưa đơn nào được vào sổ.", errors });
  });
});

describe("GET /api/v1/citizen-letter-report/exports?year=", () => {
  it("always names the year — the route requires it (contract gap: the generated route has no query)", () => {
    expect(letterReportExportPath(2026)).toBe("/api/v1/citizen-letter-report/exports?year=2026");
  });

  it("200: the bytes and the server's file name from Content-Disposition", async () => {
    const fake = stub(
      new Response(new Blob(["PK"]), {
        status: 200,
        headers: { "Content-Disposition": 'attachment; filename="bao-cao-don-thu-nam-2026.xlsx"' },
      }),
    );
    const r = await downloadLetterReport(2026);
    expect(fake.mock.calls[0]?.[0]).toBe("/api/v1/citizen-letter-report/exports?year=2026");
    expect(fake.mock.calls[0]?.[1]?.method).toBe("GET");
    expect(fake.mock.calls[0]?.[1]?.credentials).toBe("same-origin");
    expect(r.ok && r.duLieu.fileName).toBe("bao-cao-don-thu-nam-2026.xlsx");
  });

  it("no Content-Disposition: this file's own name for the year — never the task register's", async () => {
    stub(new Response(new Blob(["PK"]), { status: 200 }));
    const r = await downloadLetterReport(2025);
    expect(r.ok && r.duLieu.fileName).toBe("bao-cao-don-thu-nam-2025.xlsx");
  });

  it("403: the server's sentence, verbatim; a network failure is the generic one", async () => {
    stub(json({ code: "forbidden", message: "Bạn không có quyền xuất báo cáo.", trace_id: "" }, 403));
    expect(await downloadLetterReport(2026)).toEqual({ ok: false, thongBao: "Bạn không có quyền xuất báo cáo." });
    vi.stubGlobal("fetch", vi.fn(async () => Promise.reject(new TypeError("offline"))));
    expect(await downloadLetterReport(2026)).toEqual({ ok: false, thongBao: LOI_KHONG_RO });
  });
});
