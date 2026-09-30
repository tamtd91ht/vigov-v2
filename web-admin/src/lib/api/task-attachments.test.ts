import { afterEach, describe, expect, it, vi } from "vitest";

import { LOI_KHONG_RO } from "./goi";
import { addTaskLogEntry } from "./nhiem-vu";
import {
  UPLOAD_FORM_MISSING,
  attachmentDownloadLink,
  completeAttachment,
  requestAttachmentUpload,
  uploadForm,
} from "./task-attachments";

/**
 * ADR 0052 §1 on the task timeline (A4, b37ec2d). Pinned: each route and body, the key, the multipart
 * ORDER (fields first, file last), and that every refusal comes back with its status and the server's
 * sentence. The upload to the store itself (XHR) is not run here: no DOM.
 */

function stub(res: Response) {
  const fake = vi.fn(async (_url: string, _init?: RequestInit) => res);
  vi.stubGlobal("fetch", fake);
  return fake;
}
const json = (body: unknown, status: number) =>
  new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });

const ATT = { id: "01JFILE", file_name: "bien-ban.pdf", mime_type: "", size_bytes: 0, status: "pending" };
const UPLOAD = {
  attachment: ATT,
  upload: {
    url: "https://files.example.test/vigov-stg-temp",
    fields: { key: "upload/x", policy: "P", "x-amz-signature": "S", "Content-Type": "application/pdf" },
    expires_at: "2026-09-29T03:15:00Z",
  },
};

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("a. declare the file", () => {
  it("POST …/attachments with exactly {file_name, content_type, size} and the caller's Idempotency-Key", async () => {
    const fake = stub(json(UPLOAD, 201));
    const r = await requestAttachmentUpload(
      "NV 19",
      { file_name: "bien-ban.pdf", content_type: "application/pdf", size: 1234, extra: 1 } as never,
      "khoa-gia",
    );
    expect(fake.mock.calls[0]?.[0]).toBe("/api/v1/tasks/NV%2019/attachments");
    const init = fake.mock.calls[0]?.[1];
    expect(init?.method).toBe("POST");
    expect((init?.headers as Record<string, string>)["Idempotency-Key"]).toBe("khoa-gia");
    expect(JSON.parse(String(init?.body))).toEqual({ file_name: "bien-ban.pdf", content_type: "application/pdf", size: 1234 });
    expect(r).toEqual({ ok: true, data: UPLOAD });
  });

  it("the limits refusal and 503 \"chưa cấu hình kho lưu tệp\" come back VERBATIM, with the status", async () => {
    const cau = "Chưa cấu hình kho lưu tệp nên chưa đính kèm được tệp. Hãy báo quản trị hệ thống.";
    stub(json({ code: "storage_not_configured", message: cau, trace_id: "t" }, 503));
    expect(await requestAttachmentUpload("NV19", { file_name: "a.pdf", content_type: "application/pdf", size: 1 }, "k")).toEqual({
      ok: false,
      status: 503,
      message: cau,
    });
  });

  it("a replayed 201 without the signed form is NOT an upload slot", async () => {
    stub(json({ code: "", replayed: true }, 201));
    const r = await requestAttachmentUpload("NV19", { file_name: "a.pdf", content_type: "application/pdf", size: 1 }, "k");
    expect(r).toEqual({ ok: false, status: 201, message: UPLOAD_FORM_MISSING });
  });
});

describe("b. the form sent to the store", () => {
  it("every policy field FIRST, in order, then the file LAST, named `file`", () => {
    const f = uploadForm(UPLOAD.upload, new Blob(["%PDF"]), "bien-ban.pdf");
    const keys = [...f.keys()];
    expect(keys).toEqual(["key", "policy", "x-amz-signature", "Content-Type", "file"]);
    expect(f.get("policy")).toBe("P");
    expect(f.get("file")).toBeInstanceOf(Blob);
  });
});

describe("c. completion — the status picks the button, the sentence is the server's", () => {
  it("200 ⇒ the stored file", async () => {
    const fake = stub(json({ ...ATT, status: "stored", mime_type: "application/pdf", size_bytes: 1234 }, 200));
    const r = await completeAttachment("NV19", "01JFILE");
    expect(fake.mock.calls[0]?.[0]).toBe("/api/v1/tasks/NV19/attachments/01JFILE/completion");
    expect(fake.mock.calls[0]?.[1]?.method).toBe("POST");
    expect(r.ok && r.data.status).toBe("stored");
  });

  for (const [status, code] of [
    [422, "attachment_rejected"],
    [503, "malware_scan_unavailable"],
    [409, "upload_changed"],
  ] as const) {
    it(`${status} ${code} ⇒ status + sentence`, async () => {
      stub(json({ code, message: `Câu giả ${code}.`, trace_id: "t" }, status));
      expect(await completeAttachment("NV19", "01JFILE")).toEqual({ ok: false, status, message: `Câu giả ${code}.` });
    });
  }

  it("no answer ⇒ status 0, the generic sentence", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => Promise.reject(new Error("x"))));
    expect(await completeAttachment("NV19", "x")).toEqual({ ok: false, status: 0, message: LOI_KHONG_RO });
  });
});

describe("d. download link", () => {
  it("GET …/download ⇒ {url, expires_at}", async () => {
    const fake = stub(json({ url: "https://files.example.test/signed", expires_at: "2026-09-29T03:15:00Z" }, 200));
    const r = await attachmentDownloadLink("NV19", "01JFILE");
    expect(fake.mock.calls[0]?.[0]).toBe("/api/v1/tasks/NV19/attachments/01JFILE/download");
    expect(fake.mock.calls[0]?.[1]?.method).toBe("GET");
    expect(r.ok && r.data.url).toBe("https://files.example.test/signed");
  });
});

describe("the entry carries the stored ids", () => {
  it("`attachments` present with ids, ABSENT without — an entry without files is the old body", async () => {
    const fake = stub(json({}, 201));
    await addTaskLogEntry("NV19", "Đã gửi.", "k", ["01JA", "01JB"]);
    expect(JSON.parse(String(fake.mock.calls[0]?.[1]?.body))).toEqual({ note: "Đã gửi.", attachments: ["01JA", "01JB"] });
    vi.unstubAllGlobals();
    const fake2 = stub(json({}, 201));
    await addTaskLogEntry("NV19", "Đã gửi.", "k");
    expect(JSON.parse(String(fake2.mock.calls[0]?.[1]?.body))).toEqual({ note: "Đã gửi." });
  });
});
