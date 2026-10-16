import { afterEach, describe, expect, it, vi } from "vitest";

import {
  CITIZEN_CONTACT_IMPORT_ROUTES,
  CITIZEN_CONTACT_TEMPLATE_FILE_NAME,
  commitCitizenContactImport,
  downloadCitizenContactImportTemplate,
  previewCitizenContactImport,
} from "./citizen-contact-import";
import { FILE_TYPE_FALLBACK } from "./excel-import";
import { LOI_KHONG_RO } from "./goi";

/**
 * The citizen-directory import (ADR 0097 D3/D13): the three paths, the multipart part `file`, no key on a
 * preview, the caller's key on the import, `row_count` read from both answers, a replay as `null`.
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

describe("citizen-contact import", () => {
  it("the three paths of service-identity and the server's template name", () => {
    expect(CITIZEN_CONTACT_IMPORT_ROUTES).toEqual({
      template: "/api/v1/citizen-contact-import-template",
      previews: "/api/v1/citizen-contact-import-previews",
      imports: "/api/v1/citizen-contact-imports",
    });
    expect(CITIZEN_CONTACT_TEMPLATE_FILE_NAME).toBe("mau-nhap-danh-ba-nguoi-dan.xlsx");
  });

  it("template: GET the bare path, the bytes back", async () => {
    const fake = stub(new Response(new Uint8Array([80, 75]), { status: 200 }));
    expect((await downloadCitizenContactImportTemplate()).ok).toBe(true);
    expect(fake.mock.calls[0]?.[0]).toBe("/api/v1/citizen-contact-import-template");
    expect(fake.mock.calls[0]?.[1]?.method).toBe("GET");
  });

  it("preview: multipart part `file`, no Idempotency-Key, no hand-set Content-Type; 200 with the count", async () => {
    const fake = stub(json({ valid: true, row_count: 3, errors: [] }, 200));
    expect(await previewCitizenContactImport(FILE, "danh-ba.xlsx")).toEqual({ ok: true, duLieu: { valid: true, rowCount: 3, errors: [] } });
    expect(fake.mock.calls[0]?.[0]).toBe("/api/v1/citizen-contact-import-previews");
    const init = fake.mock.calls[0]?.[1];
    expect(init?.method).toBe("POST");
    expect(init?.body).toBeInstanceOf(FormData);
    expect((init?.body as FormData).get("file")).toBeInstanceOf(Blob);
    expect((init?.headers as Record<string, string> | undefined)?.["Idempotency-Key"]).toBeUndefined();
    expect((init?.headers as Record<string, string> | undefined)?.["Content-Type"]).toBeUndefined();
  });

  it("preview `valid:false` with row/column errors is an ANSWER; a 415 is the file's refusal", async () => {
    const errors = [{ row: 4, column: "Số điện thoại", message: "Số này đã có trong danh bạ." }];
    stub(json({ valid: false, row_count: 0, errors }, 200));
    expect(await previewCitizenContactImport(FILE, "x.xlsx")).toEqual({ ok: true, duLieu: { valid: false, rowCount: 0, errors } });
    stub(new Response("<html>", { status: 415 }));
    expect(await previewCitizenContactImport(FILE, "x.xlsx")).toEqual({ ok: false, thongBao: FILE_TYPE_FALLBACK });
    stub(json({ valid: true }, 200));
    expect(await previewCitizenContactImport(FILE, "x.xlsx")).toEqual({ ok: false, thongBao: LOI_KHONG_RO });
  });

  it("import: the caller's key; 201 → row_count; a replay (no count) → null", async () => {
    const fake = stub(json({ batch_id: "01JB", row_count: 3 }, 201));
    expect(await commitCitizenContactImport(FILE, "x.xlsx", "khoa-1")).toEqual({ ok: true, rowCount: 3 });
    expect(fake.mock.calls[0]?.[0]).toBe("/api/v1/citizen-contact-imports");
    expect((fake.mock.calls[0]?.[1]?.headers as Record<string, string>)["Idempotency-Key"]).toBe("khoa-1");
    expect(fake.mock.calls[0]?.[1]?.body).toBeInstanceOf(FormData);
    stub(json({ replayed: true }, 201));
    expect(await commitCitizenContactImport(FILE, "x.xlsx", "khoa-1")).toEqual({ ok: true, rowCount: null });
  });

  it("import 400 import_invalid: every refused row and the server's sentence; 409 is a sentence only", async () => {
    const errors = [{ row: 2, column: "", message: "Dòng trống." }];
    stub(json({ code: "import_invalid", message: "Tệp có lỗi nên chưa ghi dòng nào.", trace_id: "", errors }, 400));
    expect(await commitCitizenContactImport(FILE, "x.xlsx", "khoa-1")).toEqual({ ok: false, message: "Tệp có lỗi nên chưa ghi dòng nào.", errors });
    stub(json({ code: "citizen_contacts_changed", message: "Danh bạ vừa đổi, hãy kiểm lại tệp.", trace_id: "" }, 409));
    expect(await commitCitizenContactImport(FILE, "x.xlsx", "khoa-1")).toEqual({ ok: false, message: "Danh bạ vừa đổi, hãy kiểm lại tệp.", errors: [] });
  });
});
