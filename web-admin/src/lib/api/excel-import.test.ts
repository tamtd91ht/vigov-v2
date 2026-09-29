import { afterEach, describe, expect, it, vi } from "vitest";

import {
  FILE_TOO_LARGE_FALLBACK,
  FILE_TYPE_FALLBACK,
  IMPORT_FILE_FIELD,
  commitImport,
  downloadImportTemplate,
  previewImport,
} from "./excel-import";
import { MAP_ASSET_TYPE_IMPORT_ROUTES, MAP_ASSET_TYPE_ROWS_FIELD } from "./map-asset-type-import";
import type { MapAssetTypeImportRow } from "./map-asset-type-import";

/**
 * The shared import client, exercised on the SECOND owner (`service-comms`, map-asset types) so the
 * parts that differ per owner — paths and the row list's field name (`types`, not `units`) — are
 * proved to come from the caller. The org chart's own cases stay in `org-unit-import.test.ts`.
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
const ROW: MapAssetTypeImportRow = { row: 2, code: "ho-kinh-doanh", label: "Hộ kinh doanh", order: 1 };

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("map-asset types — the three routes", () => {
  it("template: GET the comms path", async () => {
    const fake = stubFetch(() => new Response(XLSX, { status: 200 }));
    expect((await downloadImportTemplate(MAP_ASSET_TYPE_IMPORT_ROUTES)).ok).toBe(true);
    expect(call(fake).path).toBe("/api/v1/map-asset-types/import-template");
    expect(call(fake).method).toBe("GET");
  });

  it("preview: one `file` part, no Content-Type, no key; rows read from `types`", async () => {
    const fake = stubFetch(() => reply(200, { valid: true, types: [ROW], errors: [] }));
    const r = await previewImport<MapAssetTypeImportRow>(MAP_ASSET_TYPE_IMPORT_ROUTES, MAP_ASSET_TYPE_ROWS_FIELD, XLSX, "loai.xlsx");
    expect(r).toEqual({ ok: true, duLieu: { valid: true, rows: [ROW], errors: [] } });
    const c = call(fake);
    expect(c.path).toBe("/api/v1/map-asset-types/import-previews");
    expect(c.method).toBe("POST");
    expect(c.headers.get("Content-Type")).toBeNull();
    expect(c.headers.get("Idempotency-Key")).toBeNull();
    expect([...(c.body as FormData).keys()]).toEqual([IMPORT_FILE_FIELD]);
  });

  it("preview: the WRONG field name yields no rows — the field is the caller's, never guessed", async () => {
    stubFetch(() => reply(200, { valid: true, types: [ROW], errors: [] }));
    const r = await previewImport(MAP_ASSET_TYPE_IMPORT_ROUTES, "units", XLSX, "a.xlsx");
    expect(r.ok && r.duLieu.rows).toEqual([]);
  });

  it("preview 413 / 415: server sentence if written, fallback for a proxy page", async () => {
    stubFetch(() => reply(413, { code: "file_too_large", message: "Tệp quá lớn. Tối đa 2 MB và 500 dòng mỗi lần nhập." }));
    expect(await previewImport(MAP_ASSET_TYPE_IMPORT_ROUTES, "types", XLSX, "a.xlsx")).toEqual({
      ok: false,
      thongBao: "Tệp quá lớn. Tối đa 2 MB và 500 dòng mỗi lần nhập.",
    });
    stubFetch(() => new Response("<html>413</html>", { status: 413 }));
    expect(await previewImport(MAP_ASSET_TYPE_IMPORT_ROUTES, "types", XLSX, "a.xlsx")).toEqual({
      ok: false,
      thongBao: FILE_TOO_LARGE_FALLBACK,
    });
    stubFetch(() => new Response("", { status: 415 }));
    expect(await previewImport(MAP_ASSET_TYPE_IMPORT_ROUTES, "types", XLSX, "a.xls")).toEqual({
      ok: false,
      thongBao: FILE_TYPE_FALLBACK,
    });
  });

  it("import: the CALLER's key, sent unchanged on a retry of the same attempt", async () => {
    let n = 0;
    const fake = stubFetch(() => {
      n += 1;
      // First send: the connection drops after the server may have written. Second: 201.
      if (n === 1) throw new TypeError("network");
      return reply(201, { created: [{ ...ROW, id: "01JNEW" }] });
    });
    const first = await commitImport(MAP_ASSET_TYPE_IMPORT_ROUTES, XLSX, "a.xlsx", "khoa-lan-1");
    expect(first.ok).toBe(false);
    const second = await commitImport(MAP_ASSET_TYPE_IMPORT_ROUTES, XLSX, "a.xlsx", "khoa-lan-1");
    expect(second).toEqual({ ok: true, created: [{ ...ROW, id: "01JNEW" }] });
    expect(call(fake, 0).headers.get("Idempotency-Key")).toBe("khoa-lan-1");
    expect(call(fake, 1).headers.get("Idempotency-Key")).toBe("khoa-lan-1");
    expect(call(fake, 1).path).toBe("/api/v1/map-asset-types/imports");
  });

  it("import 400 import_invalid carries every row error; a replayed 201 is still a success", async () => {
    const errors = [{ row: 3, column: "Tên hiển thị", message: "Thiếu tên hiển thị." }];
    stubFetch(() =>
      reply(400, { code: "import_invalid", message: "Tệp có lỗi nên chưa loại nào được tạo.", trace_id: "t", errors }),
    );
    expect(await commitImport(MAP_ASSET_TYPE_IMPORT_ROUTES, XLSX, "a.xlsx", "k")).toEqual({
      ok: false,
      message: "Tệp có lỗi nên chưa loại nào được tạo.",
      errors,
    });
    stubFetch(() => reply(201, { replayed: true }));
    expect(await commitImport(MAP_ASSET_TYPE_IMPORT_ROUTES, XLSX, "a.xlsx", "k")).toEqual({ ok: true, created: null });
  });
});
