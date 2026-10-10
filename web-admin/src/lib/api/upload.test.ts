import { afterEach, describe, expect, it, vi } from "vitest";

import type { comms_post_content_items_audio_files, petitions_post_tasks_by_ma_attachments } from "./schema.gen";
import {
  LOI_KHONG_RO,
} from "./goi";
import {
  UPLOAD_NETWORK_FAILED,
  UPLOAD_REPLAYED,
  UPLOAD_TOO_LARGE,
  UPLOAD_TOO_SLOW,
  UPLOAD_UNAVAILABLE,
  isTransientUpload,
  sendUpload,
  uploadFormData,
} from "./upload";
import { installFakeUploadXHR, partNames, partValue } from "./upload-test-support";

/**
 * ADR 0052 §Sửa đổi 09/10/2026: ONE multipart POST, same origin, parts in the contract's order (`size`
 * before `file`, `file` last), the Idempotency-Key, and every refusal → one sentence.
 */

type Task = petitions_post_tasks_by_ma_attachments;
const TASK_PARTS: Task["multipartParts"] = ["size", "content_type", "file_name", "file"];
type Audio = comms_post_content_items_audio_files;
const AUDIO_PARTS: Audio["multipartParts"] = [
  "size",
  "file_name",
  "content_type",
  "content_item_id",
  "audio_duration_seconds",
  "file",
];

const pdf = () => new File([new Uint8Array([0x25, 0x50, 0x44, 0x46, 0x2d])], "bien-ban.pdf", { type: "application/pdf" });
const STORED = { id: "01JFILE", file_name: "bien-ban.pdf", mime_type: "application/pdf", size_bytes: 5, status: "stored" };

function send(extra: { onProgress?: (p: number) => void; onSent?: () => void } = {}) {
  return sendUpload<Task, typeof STORED>({
    path: "/api/v1/tasks/NV19/attachments",
    parts: TASK_PARTS,
    fields: { content_type: "application/pdf", file_name: "bien-ban.pdf" },
    file: pdf(),
    fileName: "bien-ban.pdf",
    idempotencyKey: "khoa-1",
    ...extra,
  });
}

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("the multipart body", () => {
  it("parts in the contract's order: size FIRST (= file.size), text parts, file LAST", () => {
    const f = uploadFormData<Task>(TASK_PARTS, { content_type: "application/pdf", file_name: "a.pdf" }, pdf(), "a.pdf");
    expect([...f.keys()]).toEqual(["size", "content_type", "file_name", "file"]);
    expect(f.get("size")).toBe("5");
    expect(f.get("file")).toBeInstanceOf(Blob);
  });

  it("an absent or empty optional part is NOT sent; the order of the rest holds", () => {
    const f = uploadFormData<Audio>(
      AUDIO_PARTS,
      { file_name: "ban-tin.mp3", content_type: "audio/mpeg", content_item_id: "", audio_duration_seconds: "750" },
      new Blob(["ID3"]),
      "ban-tin.mp3",
    );
    expect([...f.keys()]).toEqual(["size", "file_name", "content_type", "audio_duration_seconds", "file"]);
    expect(f.get("audio_duration_seconds")).toBe("750");
  });
});

describe("sendUpload — one request, this origin", () => {
  it("ONE POST to the relative path, Idempotency-Key, no Content-Type of ours, no credentials flag", async () => {
    const sent = installFakeUploadXHR(() => ({ status: 201, body: STORED }));
    const fetchSpy = vi.fn();
    vi.stubGlobal("fetch", fetchSpy);

    const r = await send();

    expect(r).toEqual({ ok: true, data: STORED });
    expect(sent).toHaveLength(1);
    expect(sent[0]?.method).toBe("POST");
    expect(sent[0]?.url).toBe("/api/v1/tasks/NV19/attachments");
    expect(sent[0]?.url.startsWith("/")).toBe(true);
    expect(sent[0]?.headers).toEqual({ "Idempotency-Key": "khoa-1" });
    expect(sent[0]?.withCredentials).toBe(false);
    expect(partNames(sent[0])).toEqual(["size", "content_type", "file_name", "file"]);
    expect(partValue(sent[0], "size")).toBe("5");
    // Nothing else went out: no declaration, no store, no completion.
    expect(fetchSpy).not.toHaveBeenCalled();
    expect(sent.some((s) => s.url.endsWith("/completion"))).toBe(false);
  });

  it("progress still fires, then `onSent` (the server is checking) before the reply", async () => {
    installFakeUploadXHR(() => ({ status: 201, body: STORED }));
    const seen: string[] = [];
    await send({ onProgress: (p) => seen.push(`p${p}`), onSent: () => seen.push("sent") });
    expect(seen).toEqual(["p50", "p100", "sent"]);
  });
});

describe("refusals → one sentence", () => {
  it("the server's httpx.Error: its sentence VERBATIM (technical prefix stripped), its code, its status", async () => {
    for (const [status, code, message] of [
      [400, "invalid_upload", "Dữ liệu tải lên không hợp lệ hoặc không khớp kích thước đã khai báo. Vui lòng chọn tệp và thử lại."],
      [408, "upload_timeout", "Tải tệp lên quá thời gian cho phép nên tệp CHƯA được nhận. Vui lòng thử lại."],
      [413, "file_too_large", "Tệp lớn hơn dung lượng tối đa được phép đính kèm."],
      [415, "unsupported_media_type", "Tệp phải được gửi bằng biểu mẫu multipart/form-data."],
      [422, "attachment_rejected", "Tệp bị từ chối."],
      [503, "upload_busy", "Hệ thống đang nhận nhiều tệp cùng lúc. Vui lòng thử lại sau ít giây."],
    ] as const) {
      installFakeUploadXHR(() => ({ status, body: { code, message, trace_id: "t" } }));
      expect(await send(), code).toEqual({ ok: false, status, code, message });
    }
  });

  it("no httpx.Error body (a proxy page): a FIXED sentence per status", async () => {
    for (const [status, sentence] of [
      [413, UPLOAD_TOO_LARGE],
      [408, UPLOAD_TOO_SLOW],
      [504, UPLOAD_TOO_SLOW],
      [502, UPLOAD_UNAVAILABLE],
      [503, UPLOAD_UNAVAILABLE],
      [500, LOI_KHONG_RO],
    ] as const) {
      installFakeUploadXHR(() => ({ status, raw: "<html>413 Request Entity Too Large</html>" }));
      expect(await send(), String(status)).toEqual({ ok: false, status, code: "", message: sentence });
    }
  });

  it("no answer at all: status 0 and its own sentence", async () => {
    installFakeUploadXHR(() => "network");
    expect(await send()).toEqual({ ok: false, status: 0, code: "", message: UPLOAD_NETWORK_FAILED });
  });

  it("a replayed key (`{code, replayed}`) is NOT a stored file", async () => {
    installFakeUploadXHR(() => ({ status: 201, body: { code: "01JFILE", replayed: true } }));
    expect(await send()).toEqual({ ok: false, status: 201, code: "replayed", message: UPLOAD_REPLAYED });
  });

  it("only the moment's failures are worth re-sending the same file", () => {
    for (const status of [0, 408, 502, 503, 504]) expect(isTransientUpload({ status }), String(status)).toBe(true);
    for (const status of [400, 403, 404, 409, 413, 415, 422, 201]) expect(isTransientUpload({ status }), String(status)).toBe(false);
  });
});
