import { afterEach, describe, expect, it, vi } from "vitest";

import {
  commitDisbursementImport,
  downloadDisbursementImportTemplate,
  previewDisbursementImport,
} from "./disbursement-import";
import { FILE_TOO_LARGE_FALLBACK, FILE_TYPE_FALLBACK, IMPORT_FILE_FIELD } from "./excel-import";

function reply(status: number, body: unknown) {
  return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
}

function stubFetch(answer: () => Response) {
  const fake = vi.fn(async () => answer());
  vi.stubGlobal("fetch", fake);
  return fake;
}

function call(fake: ReturnType<typeof stubFetch>, i = 0) {
  const [path, init] = fake.mock.calls[i] as unknown as [string, RequestInit];
  return { path, method: init.method, headers: new Headers(init.headers), body: init.body };
}

const XLSX = new Blob(["PK fake"], { type: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" });

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("disbursement import — the three finance routes", () => {
  it("template: GET the finance path, the body as a blob", async () => {
    const fake = stubFetch(() => new Response(XLSX, { status: 200 }));
    const r = await downloadDisbursementImportTemplate();
    expect(r.ok).toBe(true);
    expect(call(fake).path).toBe("/api/v1/disbursements/import-template");
    expect(call(fake).method).toBe("GET");
  });

  it("template 403: the server's sentence", async () => {
    stubFetch(() => reply(403, { code: "forbidden", message: "Bạn không có quyền xem giải ngân." }));
    expect(await downloadDisbursementImportTemplate()).toEqual({
      ok: false,
      thongBao: "Bạn không có quyền xem giải ngân.",
    });
  });

  it("preview: multipart `file`, no key, row count and total in full đồng", async () => {
    const fake = stubFetch(() =>
      reply(200, { valid: true, row_count: 3, total_amount: 1234567890, errors: [] }),
    );
    const r = await previewDisbursementImport(XLSX, "a.xlsx");
    expect(r).toEqual({ ok: true, duLieu: { valid: true, rowCount: 3, totalAmount: 1234567890, errors: [] } });
    const c = call(fake);
    expect(c.path).toBe("/api/v1/disbursements/import-previews");
    expect(c.method).toBe("POST");
    expect(c.headers.has("Idempotency-Key")).toBe(false);
    expect([...(c.body as FormData).keys()]).toEqual([IMPORT_FILE_FIELD]);
  });

  it("preview 200 invalid: the row errors come through", async () => {
    stubFetch(() =>
      reply(200, {
        valid: false,
        row_count: 2,
        total_amount: 0,
        errors: [{ row: 3, column: "Mã dự án", message: "Không có dự án mã này." }],
      }),
    );
    const r = await previewDisbursementImport(XLSX, "a.xlsx");
    expect(r.ok && r.duLieu.valid).toBe(false);
    expect(r.ok && r.duLieu.errors).toEqual([{ row: 3, column: "Mã dự án", message: "Không có dự án mã này." }]);
  });

  it("preview 200 without the two numbers is NOT read as '0 chứng từ, 0 đ'", async () => {
    stubFetch(() => reply(200, { valid: true, errors: [] }));
    expect((await previewDisbursementImport(XLSX, "a.xlsx")).ok).toBe(false);
  });

  it("preview 413 / 415: the server's sentence verbatim, else the shared fallback", async () => {
    stubFetch(() => reply(413, { code: "too_large", message: "Tệp lớn hơn 2 MB." }));
    expect(await previewDisbursementImport(XLSX, "a.xlsx")).toEqual({ ok: false, thongBao: "Tệp lớn hơn 2 MB." });
    stubFetch(() => new Response("<html>413</html>", { status: 413 }));
    expect(await previewDisbursementImport(XLSX, "a.xlsx")).toEqual({ ok: false, thongBao: FILE_TOO_LARGE_FALLBACK });
    stubFetch(() => new Response("", { status: 415 }));
    expect(await previewDisbursementImport(XLSX, "a.xls")).toEqual({ ok: false, thongBao: FILE_TYPE_FALLBACK });
  });

  it("import: the caller's key, 201 → the created rows", async () => {
    const fake = stubFetch(() =>
      reply(201, {
        valid: true,
        row_count: 1,
        total_amount: 5000000,
        errors: [],
        batch: "01JBATCH",
        created: [{ row: 2, id: "01JV", project_code: "DA01" }],
      }),
    );
    const r = await commitDisbursementImport(XLSX, "a.xlsx", "k-1");
    expect(r).toEqual({ ok: true, created: [{ row: 2, id: "01JV", project_code: "DA01" }] });
    const c = call(fake);
    expect(c.path).toBe("/api/v1/disbursements/imports");
    expect(c.headers.get("Idempotency-Key")).toBe("k-1");
  });

  it("import 400 import_invalid: message and every refused row", async () => {
    stubFetch(() =>
      reply(400, {
        code: "import_invalid",
        message: "Tệp còn dòng sai.",
        trace_id: "t",
        errors: [{ row: 4, column: "Số tiền (đồng)", message: "Phải là số dương." }],
      }),
    );
    expect(await commitDisbursementImport(XLSX, "a.xlsx", "k")).toEqual({
      ok: false,
      message: "Tệp còn dòng sai.",
      errors: [{ row: 4, column: "Số tiền (đồng)", message: "Phải là số dương." }],
    });
  });
});
