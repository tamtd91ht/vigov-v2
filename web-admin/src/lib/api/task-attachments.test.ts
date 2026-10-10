import { afterEach, describe, expect, it, vi } from "vitest";

import { addTaskLogEntry } from "./nhiem-vu";
import { attachmentDownloadLink, uploadTaskAttachment } from "./task-attachments";
import { installFakeUploadXHR, partNames, partValue } from "./upload-test-support";

/**
 * ADR 0052 §Sửa đổi 09/10/2026 on the task timeline: ONE multipart POST to the task's own route, on this
 * origin — no declaration, no store host, no `/completion`. Pinned: the route, the parts in the contract's
 * order with `size` before `file`, the key, and the refusal coming back with its status and sentence.
 */

function stub(res: Response) {
  const fake = vi.fn(async (_url: string, _init?: RequestInit) => res);
  vi.stubGlobal("fetch", fake);
  return fake;
}
const json = (body: unknown, status: number) =>
  new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });

const STORED = { id: "01JFILE", file_name: "bien-ban.pdf", mime_type: "application/pdf", size_bytes: 4, status: "stored" };
const pdf = () => new File([new Uint8Array([0x25, 0x50, 0x44, 0x46])], "bien-ban.pdf", { type: "application/pdf" });

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("uploadTaskAttachment — one request", () => {
  it("POST /api/v1/tasks/{ma}/attachments: size, content_type, file_name, then the file; the caller's key", async () => {
    const sent = installFakeUploadXHR(() => ({ status: 201, body: STORED }));
    const fetchSpy = stub(json({}, 500));
    const progress: number[] = [];

    const r = await uploadTaskAttachment("NV 19", pdf(), "application/pdf", "khoa-gia", { onProgress: (p) => progress.push(p) });

    expect(r).toEqual({ ok: true, data: STORED });
    expect(sent).toHaveLength(1);
    expect(sent[0]?.method).toBe("POST");
    expect(sent[0]?.url).toBe("/api/v1/tasks/NV%2019/attachments");
    expect(sent[0]?.headers["Idempotency-Key"]).toBe("khoa-gia");
    expect(partNames(sent[0])).toEqual(["size", "content_type", "file_name", "file"]);
    expect(partValue(sent[0], "size")).toBe("4");
    expect(partValue(sent[0], "content_type")).toBe("application/pdf");
    expect(partValue(sent[0], "file_name")).toBe("bien-ban.pdf");
    expect(progress).toEqual([50, 100]);
    // No second call of any kind: no declaration, no completion.
    expect(fetchSpy).not.toHaveBeenCalled();
  });

  it("the limits refusal and 503 \"chưa cấu hình kho lưu tệp\" come back VERBATIM, with status and code", async () => {
    const cau = "Chưa cấu hình kho lưu tệp nên chưa đính kèm được tệp. Hãy báo quản trị hệ thống.";
    installFakeUploadXHR(() => ({ status: 503, body: { code: "storage_not_configured", message: cau, trace_id: "t" } }));
    expect(await uploadTaskAttachment("NV19", pdf(), "application/pdf", "k")).toEqual({
      ok: false,
      status: 503,
      code: "storage_not_configured",
      message: cau,
    });
  });

  for (const [status, code] of [
    [422, "attachment_rejected"],
    [413, "file_too_large"],
    [409, "attachment_limit"],
  ] as const) {
    it(`${status} ${code} ⇒ status + sentence`, async () => {
      installFakeUploadXHR(() => ({ status, body: { code, message: `Câu giả ${code}.`, trace_id: "t" } }));
      expect(await uploadTaskAttachment("NV19", pdf(), "application/pdf", "k")).toEqual({
        ok: false,
        status,
        code,
        message: `Câu giả ${code}.`,
      });
    });
  }
});

describe("download link", () => {
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
