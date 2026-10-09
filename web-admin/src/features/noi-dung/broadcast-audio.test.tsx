import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, describe, expect, it, vi } from "vitest";

import { AUDIO_FORM_MISSING, removeContentAudio, requestAudioUpload, suaNoiDung } from "@/lib/api/noi-dung";
import type { comms_audioOut, comms_noiDungRa } from "@/lib/api/schema.gen"; // vi-name-ok: generated contract type

import {
  AUDIO_ACCEPT,
  AUDIO_DURATION_FORMAT,
  AUDIO_DURATION_MAX_SECONDS,
  AUDIO_DURATION_RANGE,
  AUDIO_DURATION_LABEL,
  AUDIO_EMPTY,
  AUDIO_HAS_FILE,
  AUDIO_HINT,
  AUDIO_MAX_BYTES,
  AUDIO_NOT_READY,
  AUDIO_PICK_BUTTON,
  AUDIO_RETRY_BUTTON,
  AUDIO_DURATION_NEEDED_TO_SAVE,
  AUDIO_STORAGE_FAILED,
  AUDIO_TOO_LARGE,
  AUDIO_TYPE_REFUSED,
  AUDIO_WAIT_NOTE,
  afterAudioCompletion,
  audioInFlight,
  declaredAudioType,
  formatAudioSize,
  heldAudioProblem,
  parseAudioDuration,
  retryAudioCompletion,
  runAudioUpload,
  savedAudioPatch,
  type AudioUploadState,
} from "./broadcast-audio";
import { HeldAudioField } from "./broadcast-audio-field";
import { FORM_TRONG, giaTriTuHang, thanSua } from "./nhan-noi-dung";
import { FormNoiDung } from "./so-noi-dung";

/**
 * §7 `Truyền thanh` — the broadcast audio (ADR 0067 §4). The pre-check (MP3/M4A, 30 MB), the typed
 * duration in whole seconds, the completion answers BY CODE, the flow a → b → c over a fake `fetch` and a
 * fake `XMLHttpRequest`, the edit form's audio PATCH, and the two boxes as rendered.
 */

const MB = 1024 * 1024;

describe("pre-check — convenience; the server's `content-audio` policy still decides", () => {
  it("MP3 / M4A by the browser's type (aliases included) or by extension when the type is empty", () => {
    expect(declaredAudioType({ name: "a.mp3", type: "audio/mpeg", size: 1 })).toEqual({ ok: true, contentType: "audio/mpeg" });
    expect(declaredAudioType({ name: "a.mp3", type: "audio/mp3", size: 1 })).toEqual({ ok: true, contentType: "audio/mpeg" });
    expect(declaredAudioType({ name: "a.m4a", type: "audio/x-m4a", size: 1 })).toEqual({ ok: true, contentType: "audio/mp4" });
    expect(declaredAudioType({ name: "a.m4a", type: "audio/mp4", size: 1 })).toEqual({ ok: true, contentType: "audio/mp4" });
    expect(declaredAudioType({ name: "BAN-TIN.M4A", type: "", size: 1 })).toEqual({ ok: true, contentType: "audio/mp4" });
    expect(AUDIO_ACCEPT).toContain(".mp3");
    expect(AUDIO_ACCEPT).toContain(".m4a");
  });

  it("a video, a WAV, an OGG or an unknown extension is refused before any request", () => {
    for (const f of [
      { name: "a.mp4", type: "video/mp4", size: 1 },
      { name: "a.wav", type: "audio/wav", size: 1 },
      { name: "a.ogg", type: "audio/ogg", size: 1 },
      { name: "a.aac", type: "", size: 1 },
    ]) {
      expect(declaredAudioType(f), f.name).toEqual({ ok: false, message: AUDIO_TYPE_REFUSED });
    }
  });

  it("30 MB is 31 457 280 bytes (platform migration 0013): exactly that passes, one byte more does not", () => {
    expect(AUDIO_MAX_BYTES).toBe(31457280);
    expect(declaredAudioType({ name: "a.mp3", type: "audio/mpeg", size: 30 * MB }).ok).toBe(true);
    expect(declaredAudioType({ name: "a.mp3", type: "audio/mpeg", size: 30 * MB + 1 })).toEqual({
      ok: false,
      message: AUDIO_TOO_LARGE,
    });
    expect(declaredAudioType({ name: "a.mp3", type: "audio/mpeg", size: 0 })).toEqual({ ok: false, message: AUDIO_EMPTY });
  });
});

describe("typed duration — whole seconds (prototype `Thời lượng (giây)`), 1 s .. 6 h", () => {
  it("the label is the prototype's, verbatim", () => {
    expect(AUDIO_DURATION_LABEL).toBe("Thời lượng (giây)");
  });

  it("parses a whole number of seconds", () => {
    expect(parseAudioDuration("750")).toEqual({ ok: true, seconds: 750 });
    expect(parseAudioDuration(" 1 ")).toEqual({ ok: true, seconds: 1 });
    expect(parseAudioDuration("21600")).toEqual({ ok: true, seconds: AUDIO_DURATION_MAX_SECONDS });
  });

  it("bounds: 0 and anything past 6 hours are refused with the range sentence", () => {
    expect(parseAudioDuration("0")).toEqual({ ok: false, message: AUDIO_DURATION_RANGE });
    expect(parseAudioDuration("21601")).toEqual({ ok: false, message: AUDIO_DURATION_RANGE });
  });

  it("minutes:seconds, decimals, signs, letters or empty are refused with the format sentence", () => {
    for (const raw of ["12:30", "1:05:00", "12.5", "-1", "+5", "1e3", "abc", "", "1234567"]) {
      expect(parseAudioDuration(raw), raw).toEqual({ ok: false, message: AUDIO_DURATION_FORMAT });
    }
  });
});

describe("display of a picked file", () => {
  it("size with a Vietnamese decimal comma", () => {
    expect(formatAudioSize(13107200)).toBe("12,5 MB");
    expect(formatAudioSize(undefined)).toBe("không rõ dung lượng");
  });
});

describe("completion answers → state, by the server's CODE", () => {
  const file = { id: "A", content_item_id: "I", mime_type: "audio/mpeg", size_bytes: 9, status: "ready", duration_seconds: 750 };

  it("200 ready ⇒ ready; 200 other ⇒ refused", () => {
    expect(afterAudioCompletion("A", { ok: true, data: file })).toEqual({ kind: "ready", id: "A", file });
    expect(afterAudioCompletion("A", { ok: true, data: { ...file, status: "rejected" } })).toEqual({
      kind: "refused",
      message: AUDIO_NOT_READY,
    });
  });

  it("422 audio_rejected is FINAL; 422 invalid_audio_duration is a retry (the file is still pending)", () => {
    expect(afterAudioCompletion("A", { ok: false, status: 422, code: "audio_rejected", message: "mã độc" })).toEqual({
      kind: "refused",
      message: "mã độc",
    });
    expect(
      afterAudioCompletion("A", { ok: false, status: 422, code: "invalid_audio_duration", message: "thời lượng sai" }),
    ).toEqual({ kind: "retry", id: "A", message: "thời lượng sai" });
  });

  it("503 / 409 / no answer ⇒ retry with the sentence verbatim", () => {
    for (const status of [503, 409, 0]) {
      expect(afterAudioCompletion("A", { ok: false, status, code: "x", message: "câu" })).toEqual({
        kind: "retry",
        id: "A",
        message: "câu",
      });
    }
  });

  it("only requesting / uploading / checking hold Lưu", () => {
    expect(audioInFlight({ kind: "requesting" })).toBe(true);
    expect(audioInFlight({ kind: "uploading", id: "A", percent: 1 })).toBe(true);
    expect(audioInFlight({ kind: "checking", id: "A" })).toBe(true);
    expect(audioInFlight({ kind: "idle" })).toBe(false);
    expect(audioInFlight({ kind: "retry", id: "A", message: "x" })).toBe(false);
  });
});

/* ── The flow over a fake fetch and a fake XMLHttpRequest ─────────────────────────────────────── */

const UPLOAD = {
  audio_file: { id: "01JAUDIO1", content_item_id: "01JTT1", mime_type: "", size_bytes: 0, status: "pending" },
  upload: {
    url: "https://files.example.test/vigov-stg-temp",
    fields: { key: "upload/t_01JXA/x", policy: "P", "x-amz-signature": "S", "Content-Type": "audio/mpeg" },
    expires_at: "2026-10-01T03:15:00Z",
  },
};

const json = (body: unknown, status: number) =>
  new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });

type Sent = { method: string; url: string; withCredentials: boolean; keys: string[] };

function fakeXHR(status: number) {
  const sent: Sent[] = [];
  class FakeXHR {
    status = 0;
    withCredentials = false;
    upload: { onprogress: ((e: { lengthComputable: boolean; loaded: number; total: number }) => void) | null } = {
      onprogress: null,
    };
    onload: (() => void) | null = null;
    onerror: (() => void) | null = null;
    private m = "";
    private u = "";
    open(method: string, url: string) {
      this.m = method;
      this.u = url;
    }
    send(form: FormData) {
      sent.push({ method: this.m, url: this.u, withCredentials: this.withCredentials, keys: [...form.keys()] });
      queueMicrotask(() => {
        this.upload.onprogress?.({ lengthComputable: true, loaded: 50, total: 100 });
        this.status = status;
        this.onload?.();
      });
    }
  }
  vi.stubGlobal("XMLHttpRequest", FakeXHR);
  return sent;
}

function fakeFetch(declaration: Response, completion: Response) {
  const calls: { url: string; init?: RequestInit }[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string, init?: RequestInit) => {
      calls.push({ url, init });
      if (url === "/api/v1/content-items/audio-files") return declaration.clone();
      if (url.endsWith("/completion")) return completion.clone();
      if (url.startsWith("/api/v1/content-items/")) return json({ id: "01JTT1" }, 200);
      return json({ code: "not_found", message: "?" }, 404);
    }),
  );
  return calls;
}

const mp3 = () => new File([new Uint8Array([0x49, 0x44, 0x33])], "ban-tin-sang.mp3", { type: "audio/mpeg" });

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("runAudioUpload — request → upload → completion", () => {
  it("happy path: declares for the SAVED item, POSTs the form to the store (no cookie, file last), completes with the typed seconds", async () => {
    const xhr = fakeXHR(204);
    const done = { ...UPLOAD.audio_file, mime_type: "audio/mpeg", size_bytes: 3, status: "ready", duration_seconds: 750 };
    const calls = fakeFetch(json(UPLOAD, 201), json(done, 200));
    const states: AudioUploadState[] = [];

    const last = await runAudioUpload(mp3(), "01JTT1", "750", (s) => states.push(s));

    expect(calls[0]?.url).toBe("/api/v1/content-items/audio-files");
    expect(calls[0]?.init?.method).toBe("POST");
    expect(JSON.parse(String(calls[0]?.init?.body))).toEqual({
      content_item_id: "01JTT1",
      file_name: "ban-tin-sang.mp3",
      content_type: "audio/mpeg",
      size: 3,
    });
    expect(new Headers(calls[0]?.init?.headers).get("Idempotency-Key")).toMatch(/.+/);
    expect(xhr).toEqual([
      {
        method: "POST",
        url: UPLOAD.upload.url,
        withCredentials: false,
        keys: ["key", "policy", "x-amz-signature", "Content-Type", "file"],
      },
    ]);
    expect(calls[1]?.url).toBe("/api/v1/content-items/audio-files/01JAUDIO1/completion");
    expect(JSON.parse(String(calls[1]?.init?.body))).toEqual({ audio_duration_seconds: 750 });
    expect(states.map((s) => s.kind)).toEqual(["requesting", "uploading", "uploading", "checking", "ready"]);
    expect(states).toContainEqual({ kind: "uploading", id: "01JAUDIO1", percent: 50 });
    expect(last.kind).toBe("ready");
  });

  it("an invalid duration, a WAV or a 31 MB file never reaches the network", async () => {
    const xhr = fakeXHR(204);
    const calls = fakeFetch(json(UPLOAD, 201), json({}, 200));
    expect(await runAudioUpload(mp3(), "01JTT1", "12:30", () => {})).toEqual({ kind: "refused", message: AUDIO_DURATION_FORMAT });
    const wav = new File([new Uint8Array([1])], "a.wav", { type: "audio/wav" });
    expect(await runAudioUpload(wav, "01JTT1", "60", () => {})).toEqual({ kind: "refused", message: AUDIO_TYPE_REFUSED });
    const big = { name: "a.mp3", type: "audio/mpeg", size: 31 * MB } as unknown as File;
    expect(await runAudioUpload(big, "01JTT1", "60", () => {})).toEqual({ kind: "refused", message: AUDIO_TOO_LARGE });
    expect(calls).toHaveLength(0);
    expect(xhr).toHaveLength(0);
  });

  it("409 audio_limit / 422 audio_only_for_truyen_thanh at the declaration: the server's sentence, no upload", async () => {
    for (const [status, code, message] of [
      [409, "audio_limit", "Mục truyền thanh này đã có tệp âm thanh."],
      [422, "audio_only_for_truyen_thanh", "Chỉ mục truyền thanh mới có tệp âm thanh."],
      [400, "invalid_request", "Cần `content_item_id` của mục truyền thanh đã lưu."],
      [503, "upload_limits_unavailable", "Chưa đọc được giới hạn tải tệp."],
    ] as const) {
      const xhr = fakeXHR(204);
      fakeFetch(json({ code, message }, status), json({}, 200));
      expect(await runAudioUpload(mp3(), "01JTT1", "60", () => {})).toEqual({ kind: "refused", message });
      expect(xhr).toHaveLength(0);
      vi.unstubAllGlobals();
    }
  });

  it("the declaration's error carries the server's code", async () => {
    fakeFetch(json({ code: "audio_limit", message: "đã có tệp" }, 409), json({}, 200));
    expect(
      await requestAudioUpload({ content_item_id: "I", file_name: "a.mp3", content_type: "audio/mpeg", size: 1 }, "k"),
    ).toEqual({ ok: false, status: 409, code: "audio_limit", message: "đã có tệp" });
  });

  it("a 201 without the form is refused, not uploaded", async () => {
    const xhr = fakeXHR(204);
    fakeFetch(json({ audio_file: UPLOAD.audio_file }, 201), json({}, 200));
    expect(await runAudioUpload(mp3(), "01JTT1", "60", () => {})).toEqual({
      kind: "refused",
      message: AUDIO_FORM_MISSING,
    });
    expect(xhr).toHaveLength(0);
  });

  it("422 audio_rejected at completion: refused verbatim", async () => {
    fakeXHR(204);
    const cau = "Tệp bị từ chối: tệp không phải âm thanh MP3/M4A hợp lệ (ví dụ tệp video đổi đuôi).";
    fakeFetch(json(UPLOAD, 201), json({ code: "audio_rejected", message: cau }, 422));
    expect(await runAudioUpload(mp3(), "01JTT1", "60", () => {})).toEqual({ kind: "refused", message: cau });
  });

  it("503 at completion: `Hoàn tất lại` completes AGAIN — no second declaration, no second upload", async () => {
    const xhr = fakeXHR(204);
    const calls = fakeFetch(json(UPLOAD, 201), json({ code: "malware_scan_unavailable", message: "Chưa quét được." }, 503));
    expect(await runAudioUpload(mp3(), "01JTT1", "60", () => {})).toEqual({
      kind: "retry",
      id: "01JAUDIO1",
      message: "Chưa quét được.",
    });
    await retryAudioCompletion("01JAUDIO1", "120", () => {});
    expect(calls.filter((c) => c.url === "/api/v1/content-items/audio-files")).toHaveLength(1);
    const completions = calls.filter((c) => c.url.endsWith("/completion"));
    expect(completions).toHaveLength(2);
    // The corrected duration is the one sent the second time.
    expect(JSON.parse(String(completions[1]?.init?.body))).toEqual({ audio_duration_seconds: 120 });
    expect(xhr).toHaveLength(1);
  });

  it("retry with a duration that does not parse: stays retry, nothing sent", async () => {
    const calls = fakeFetch(json(UPLOAD, 201), json({}, 200));
    expect(await retryAudioCompletion("01JAUDIO1", "abc", () => {})).toEqual({
      kind: "retry",
      id: "01JAUDIO1",
      message: AUDIO_DURATION_FORMAT,
    });
    expect(calls).toHaveLength(0);
  });

  it("the store refuses the form: one sentence of our own, no completion asked", async () => {
    fakeXHR(403);
    const calls = fakeFetch(json(UPLOAD, 201), json({}, 200));
    expect(await runAudioUpload(mp3(), "01JTT1", "60", () => {})).toEqual({ kind: "refused", message: AUDIO_STORAGE_FAILED });
    expect(calls.some((c) => c.url.endsWith("/completion"))).toBe(false);
  });
});

describe("the edit form's audio PATCH — `savedAudioPatch`", () => {
  const f = () => new File([new Uint8Array([1])], "ban-tin.mp3", { type: "audio/mpeg" });

  it("no file attached: nothing (a picked file is uploaded after the save)", () => {
    expect(savedAudioPatch(null, null, "")).toEqual({});
    expect(savedAudioPatch(null, f(), "750")).toEqual({});
  });

  it("a new file on a broadcast with its file: REMOVE first — the item holds one live file", () => {
    expect(savedAudioPatch("3900", f(), "750")).toEqual({ audio_file_id: "" });
  });

  it("only the duration edited: `audio_duration_seconds`; unchanged or invalid: nothing", () => {
    expect(savedAudioPatch("3900", null, "4000")).toEqual({ audio_duration_seconds: 4000 });
    expect(savedAudioPatch("3900", null, " 3900 ")).toEqual({});
    expect(savedAudioPatch("3900", null, "abc")).toEqual({});
  });

  it("an edited duration must be valid before Lưu, even with no new file", () => {
    expect(heldAudioProblem(null, "", true)).toBe(AUDIO_DURATION_NEEDED_TO_SAVE);
    expect(heldAudioProblem(null, "0", true)).toBe(AUDIO_DURATION_RANGE);
    expect(heldAudioProblem(null, "4000", true)).toBeNull();
  });

  it("suaNoiDung sends `{audio_file_id: \"\"}` exactly as given (the replace's removal)", async () => {
    const calls = fakeFetch(json({}, 201), json({}, 200));
    const kq = await suaNoiDung("01JTT1", { audio_file_id: "" });
    expect(kq.ok).toBe(true);
    expect(calls[0]?.url).toBe("/api/v1/content-items/01JTT1");
    expect(calls[0]?.init?.method).toBe("PATCH");
    expect(JSON.parse(String(calls[0]?.init?.body))).toEqual({ audio_file_id: "" });
  });

  it("thanSua never carries an audio field — the audio half comes from `savedAudioPatch` only", () => {
    const row = { id: "01JTT1", type: "truyen-thanh", title: "Bản tin", audio_file_id: "01JAUDIO1", audio_duration_seconds: 750 } as comms_noiDungRa; // vi-name-ok: generated contract type
    const before = giaTriTuHang({ ...row, category_id: "", summary: "", status: "an" } as comms_noiDungRa); // vi-name-ok: generated contract type
    const body = thanSua(before, { ...before, title: "Bản tin sáng" });
    expect(Object.keys(body)).toEqual(["title"]);
  });

  it("removeContentAudio sends exactly {audio_file_id: \"\"} to the item", async () => {
    const calls = fakeFetch(json({}, 201), json({}, 200));
    expect((await removeContentAudio("01JTT1")).ok).toBe(true);
    expect(calls).toHaveLength(1);
    expect(calls[0]?.init?.method).toBe("PATCH");
    expect(JSON.parse(String(calls[0]?.init?.body))).toEqual({ audio_file_id: "" });
  });

  it("suaNoiDung passes audio fields only when given", async () => {
    const calls = fakeFetch(json({}, 201), json({}, 200));
    await suaNoiDung("01JTT1", { title: "x" });
    expect(JSON.parse(String(calls[0]?.init?.body))).toEqual({ title: "x" });
  });
});

/* ── The two boxes as rendered (no DOM events: rendered to a string) ─────────────────────────── */

const SAVED: comms_audioOut = {
  file_id: "01JAUDIO1",
  status: "ready",
  mime_type: "audio/mp4",
  size_bytes: 5 * MB,
  duration_seconds: 3900,
  preview_url: "https://files.example.test/vigov-private/t_01JXA/a.m4a?X-Amz-Signature=S",
  preview_expires_at: "2099-01-01T00:00:00Z",
};

describe("HeldAudioField — the prototype's two boxes", () => {
  const box = (props: Partial<Parameters<typeof HeldAudioField>[0]> = {}) =>
    renderToStaticMarkup(
      <HeldAudioField file={null} duration="" problem={null} disabled={false} onFile={() => {}} onDuration={() => {}} {...props} />,
    );

  it("empty: `Thời lượng (giây)` as a whole-seconds number box (min 1), `Chọn tệp từ máy`, the 30MB hint", () => {
    const html = box();
    expect(html).toContain("Thời lượng (giây)");
    expect(html).toMatch(/<input[^>]*id="thoi-luong-am-thanh"[^>]*type="number"[^>]*min="1"[^>]*step="1"/);
    expect(AUDIO_PICK_BUTTON).toBe("Chọn tệp từ máy");
    expect(html).toContain(AUDIO_PICK_BUTTON);
    expect(AUDIO_HINT).toBe("MP3 hoặc M4A — tối đa 30MB");
    expect(html).toContain(AUDIO_HINT);
    expect(html).not.toContain("Mỗi mục truyền thanh có một tệp");
    expect(html).toContain('accept=".mp3,.m4a,audio/mpeg,audio/mp4,audio/x-m4a"');
  });

  it("a saved file: `Đã có tệp — chọn tệp mới để thay`", () => {
    expect(AUDIO_HAS_FILE).toBe("Đã có tệp — chọn tệp mới để thay");
    const html = box({ hasExisting: true, duration: "3900" });
    expect(html).toContain(AUDIO_HAS_FILE);
    expect(html).not.toContain(AUDIO_PICK_BUTTON);
    expect(html).toContain('value="3900"');
  });

  it("a chosen file: its name, and its size in MB in leaf", () => {
    const file = new File([new Uint8Array(3 * MB)], "ban-tin-sang.mp3", { type: "audio/mpeg" });
    const html = box({ file, hasExisting: true });
    expect(html).toContain("ban-tin-sang.mp3");
    expect(html).not.toContain(AUDIO_HAS_FILE);
    expect(html).toMatch(/text-leaf[^>]*>3,0 MB</);
  });
});

describe("the §7 form and the audio boxes", () => {
  const row = {
    id: "01JTT1",
    type: "truyen-thanh",
    category_id: "",
    title: "Bản tin sáng",
    summary: "",
    image_url: "",
    has_image: false,
    status: "an",
    source: "soan-tay",
    source_url: "",
    hand_edited: false,
    published_on: "2026-10-01",
    author_code: "CB-00123",
    created_at: "2026-10-01T02:00:00Z",
    updated_at: "2026-10-01T02:00:00Z",
    audio_file_id: "01JAUDIO1",
    audio_duration_seconds: 3900,
    audio: SAVED,
  } as comms_noiDungRa; // vi-name-ok: generated contract type

  const form = (gt = FORM_TRONG, hang?: comms_noiDungRa, initialAudioState?: AudioUploadState) => // vi-name-ok: generated contract type
    renderToStaticMarkup(
      <FormNoiDung
        tieuDeForm="T"
        moTa=""
        giaTriDau={gt}
        hang={hang}
        danhMuc={[]}
        dangGui={false}
        loi={null}
        huy={() => {}}
        luu={() => {}}
        initialAudioState={initialAudioState}
      />,
    );

  it("create form with type Truyền thanh: the duration and the file slot at once", () => {
    const html = form({ ...FORM_TRONG, type: "truyen-thanh" });
    expect(html).toContain('id="thoi-luong-am-thanh"');
    expect(html).toContain('id="tep-am-thanh"');
    expect(html).toContain(AUDIO_PICK_BUTTON);
  });

  it("an item saved as Truyền thanh: the SAME two boxes — duration in seconds, `Đã có tệp…` — no player, no remove", () => {
    const html = form(giaTriTuHang(row), row);
    expect(html).toContain('id="thoi-luong-am-thanh"');
    expect(html).toMatch(/<input[^>]*id="thoi-luong-am-thanh"[^>]*value="3900"/);
    expect(html).toContain(AUDIO_HAS_FILE);
    expect(html).not.toContain("<audio");
    for (const gone of [
      "Gỡ âm thanh",
      "Lấy link nghe thử mới",
      "không cần bấm Lưu cho phần âm thanh",
      "Mục chưa có tệp âm thanh.",
      "Máy chủ không tự đo",
      "Muốn thay tệp",
    ]) {
      expect(html, gone).not.toContain(gone);
    }
  });

  it("an item saved as Truyền thanh with no file: `Chọn tệp từ máy`, empty duration", () => {
    const none = { ...row, audio: undefined, audio_file_id: undefined, audio_duration_seconds: undefined } as comms_noiDungRa; // vi-name-ok: generated contract type
    const html = form(giaTriTuHang(none), none);
    expect(html).toContain(AUDIO_PICK_BUTTON);
    expect(html).toMatch(/<input[^>]*id="thoi-luong-am-thanh"[^>]*value=""/);
  });

  it("a carried upload stuck at completion: its sentence as an alert, and `Hoàn tất lại`", () => {
    const none = { ...row, audio: undefined, audio_file_id: undefined, audio_duration_seconds: undefined } as comms_noiDungRa; // vi-name-ok: generated contract type
    const html = form(giaTriTuHang(none), none, { kind: "retry", id: "01JAUDIO1", message: "Chưa quét được mã độc." });
    expect(html).toContain('role="alert"');
    expect(html).toContain("Chưa quét được mã độc.");
    expect(html).toContain(AUDIO_RETRY_BUTTON);
  });

  it("a held file needs a valid duration before Lưu; no file needs none", () => {
    const f = new File([new Uint8Array([1])], "ban-tin.mp3", { type: "audio/mpeg" });
    expect(heldAudioProblem(null, "")).toBeNull();
    expect(heldAudioProblem(f, "")).toBe(AUDIO_DURATION_NEEDED_TO_SAVE);
    expect(heldAudioProblem(f, "12:30")).toBe(AUDIO_DURATION_FORMAT);
    expect(heldAudioProblem(f, "750")).toBeNull();
  });

  it("other types never show the audio boxes", () => {
    const html = form(FORM_TRONG);
    expect(html).not.toContain("Tệp âm thanh");
    expect(html).not.toContain(AUDIO_WAIT_NOTE);
  });
});
