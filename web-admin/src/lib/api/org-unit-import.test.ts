import { afterEach, describe, expect, it, vi } from "vitest";

import { LOI_KHONG_RO } from "./request";
import {
  FILE_TOO_LARGE_FALLBACK,
  FILE_TYPE_FALLBACK,
  IMPORT_FILE_FIELD,
  downloadOrgUnitTemplate,
  importOrgUnits,
  previewOrgUnitImport,
} from "./org-unit-import";

/**
 * The three import routes. What a working screen never shows: the file goes up as ONE multipart
 * part named `file` with the browser's own boundary, the import carries the caller's key (never a
 * fresh one), a replayed success is still a success, and a proxy's 413 page is not "no connection".
 */

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

const UNIT = { row: 2, code: "van-phong", name: "VĂN PHÒNG", parent_id: "", parent_code: "", order: 1 };

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("downloadOrgUnitTemplate — GET import-template", () => {
  it("GET the relative path, same-origin, no-store; hands back the blob", async () => {
    const fake = stubFetch(() => new Response(XLSX, { status: 200 }));
    const r = await downloadOrgUnitTemplate();
    expect(r.ok).toBe(true);
    const [path, init] = fake.mock.calls[0] as unknown as [string, RequestInit];
    expect(path).toBe("/api/v1/org-units/import-template");
    expect(init.method).toBe("GET");
    expect(init.credentials).toBe("same-origin");
    expect(init.cache).toBe("no-store");
  });

  it("403 is the server's sentence", async () => {
    stubFetch(() => reply(403, { code: "forbidden", message: "Bạn không có quyền này." }));
    expect(await downloadOrgUnitTemplate()).toEqual({ ok: false, thongBao: "Bạn không có quyền này." });
  });
});

describe("previewOrgUnitImport — POST import-previews", () => {
  it("one multipart part named `file`, no hand-set Content-Type, no Idempotency-Key", async () => {
    const fake = stubFetch(() => reply(200, { valid: true, units: [UNIT], errors: [] }));
    const r = await previewOrgUnitImport(XLSX, "so-do.xlsx");
    expect(r).toEqual({ ok: true, duLieu: { valid: true, units: [UNIT], errors: [] } });

    const c = call(fake);
    expect(c.path).toBe("/api/v1/org-units/import-previews");
    expect(c.method).toBe("POST");
    expect(c.headers.get("Content-Type")).toBeNull();
    expect(c.headers.get("Idempotency-Key")).toBeNull();
    expect(c.body).toBeInstanceOf(FormData);
    const form = c.body as FormData;
    expect([...form.keys()]).toEqual([IMPORT_FILE_FIELD]);
    expect((form.get(IMPORT_FILE_FIELD) as File).name).toBe("so-do.xlsx");
  });

  it("200 valid:false is a RESULT (the errors are the answer), not a failure", async () => {
    const errors = [{ row: 4, column: "Bộ phận cha", message: "Không có bộ phận mang mã này." }];
    stubFetch(() => reply(200, { valid: false, units: [], errors }));
    expect(await previewOrgUnitImport(XLSX, "a.xlsx")).toEqual({ ok: true, duLieu: { valid: false, units: [], errors } });
  });

  it("413 / 415: the server's sentence when it wrote one; a friendly fallback when a proxy did", async () => {
    stubFetch(() => reply(413, { code: "file_too_large", message: "Tệp quá lớn. Tối đa 2 MB và 500 dòng mỗi lần nhập." }));
    expect(await previewOrgUnitImport(XLSX, "a.xlsx")).toEqual({
      ok: false,
      thongBao: "Tệp quá lớn. Tối đa 2 MB và 500 dòng mỗi lần nhập.",
    });

    stubFetch(() => new Response("<html>413 Request Entity Too Large</html>", { status: 413 }));
    expect(await previewOrgUnitImport(XLSX, "a.xlsx")).toEqual({ ok: false, thongBao: FILE_TOO_LARGE_FALLBACK });

    stubFetch(() => new Response("", { status: 415 }));
    expect(await previewOrgUnitImport(XLSX, "a.xls")).toEqual({ ok: false, thongBao: FILE_TYPE_FALLBACK });

    stubFetch(() => new Response("", { status: 500 }));
    expect(await previewOrgUnitImport(XLSX, "a.xlsx")).toEqual({ ok: false, thongBao: LOI_KHONG_RO });
  });
});

describe("importOrgUnits — POST imports", () => {
  it("carries the CALLER's Idempotency-Key and the same one part", async () => {
    const fake = stubFetch(() => reply(201, { created: [{ ...UNIT, id: "01JNEW" }] }));
    const r = await importOrgUnits(XLSX, "so-do.xlsx", "khoa-lan-1");
    expect(r).toEqual({ ok: true, created: [{ ...UNIT, id: "01JNEW" }] });
    const c = call(fake);
    expect(c.path).toBe("/api/v1/org-units/imports");
    expect(c.method).toBe("POST");
    expect(c.headers.get("Idempotency-Key")).toBe("khoa-lan-1");
    expect(c.headers.get("Content-Type")).toBeNull();
    expect([...(c.body as FormData).keys()]).toEqual([IMPORT_FILE_FIELD]);
  });

  it("a replayed 201 ({ replayed: true }, no `created`) is still a success", async () => {
    stubFetch(() => reply(201, { replayed: true }));
    expect(await importOrgUnits(XLSX, "a.xlsx", "k")).toEqual({ ok: true, created: null });
  });

  it("400 import_invalid carries every row error", async () => {
    const errors = [
      { row: 3, column: "Tên", message: "Thiếu tên bộ phận." },
      { row: 0, column: "", message: "Tệp thiếu cột Mã." },
    ];
    stubFetch(() =>
      reply(400, {
        code: "import_invalid",
        message: "Tệp có lỗi nên chưa bộ phận nào được tạo. Hãy sửa các dòng được liệt kê rồi nhập lại.",
        trace_id: "t",
        errors,
      }),
    );
    const r = await importOrgUnits(XLSX, "a.xlsx", "k");
    expect(r.ok).toBe(false);
    if (r.ok) return;
    expect(r.errors).toEqual(errors);
    expect(r.message).toMatch(/chưa bộ phận nào được tạo/);
  });

  it("409 org_chart_changed and 413 are sentences with no rows", async () => {
    const sentence = "Sơ đồ tổ chức của xã vừa được người khác thay đổi. Chưa bộ phận nào được tạo.";
    stubFetch(() => reply(409, { code: "org_chart_changed", message: sentence }));
    expect(await importOrgUnits(XLSX, "a.xlsx", "k")).toEqual({ ok: false, message: sentence, errors: [] });

    stubFetch(() => new Response("proxy", { status: 413 }));
    expect(await importOrgUnits(XLSX, "a.xlsx", "k")).toEqual({
      ok: false,
      message: FILE_TOO_LARGE_FALLBACK,
      errors: [],
    });
  });
});
