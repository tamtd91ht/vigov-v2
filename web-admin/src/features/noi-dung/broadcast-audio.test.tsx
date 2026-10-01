import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, describe, expect, it, vi } from "vitest";

import { AUDIO_FORM_MISSING, removeContentAudio, requestAudioUpload, suaNoiDung } from "@/lib/api/noi-dung";
import type { comms_audioOut, comms_noiDungRa } from "@/lib/api/schema.gen";

import {
  AUDIO_ACCEPT,
  AUDIO_DURATION_FORMAT,
  AUDIO_DURATION_MAX_SECONDS,
  AUDIO_DURATION_RANGE,
  AUDIO_EMPTY,
  AUDIO_MAX_BYTES,
  AUDIO_NOT_READY,
  AUDIO_PICK_BUTTON,
  AUDIO_PREVIEW_MISSING,
  AUDIO_REMOVE_BUTTON,
  AUDIO_REMOVE_QUESTION,
  AUDIO_RETRY_BUTTON,
  AUDIO_SAVE_FIRST,
  AUDIO_STORAGE_FAILED,
  AUDIO_TOO_LARGE,
  AUDIO_TYPE_CHANGE_DETACHES,
  AUDIO_TYPE_REFUSED,
  AUDIO_WAIT_NOTE,
  afterAudioCompletion,
  audioFormatLabel,
  audioInFlight,
  audioPreviewSrc,
  declaredAudioType,
  formatAudioDuration,
  formatAudioSize,
  parseAudioDuration,
  previewExpiresInMs,
  retryAudioCompletion,
  runAudioUpload,
  savedAudioText,
  type AudioUploadState,
} from "./broadcast-audio";
import { BroadcastAudioField } from "./broadcast-audio-field";
import { FORM_TRONG, giaTriTuHang, thanSua } from "./nhan-noi-dung";
import { FormNoiDung } from "./so-noi-dung";

/**
 * §7 `Truyền thanh` — the broadcast audio (ADR 0067 §4). The pre-check (MP3/M4A, 30 MB), the typed
 * duration, the completion answers BY CODE, the player's `src`, the flow a → b → c over a fake `fetch`
 * and a fake `XMLHttpRequest`, the remove PATCH, and the block as rendered in each state.
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

describe("typed duration — mm:ss or h:mm:ss, 1 s .. 6 h", () => {
  it("parses minutes:seconds (minutes may pass 59) and hours:minutes:seconds", () => {
    expect(parseAudioDuration("12:30")).toEqual({ ok: true, seconds: 750 });
    expect(parseAudioDuration(" 0:01 ")).toEqual({ ok: true, seconds: 1 });
    expect(parseAudioDuration("90:00")).toEqual({ ok: true, seconds: 5400 });
    expect(parseAudioDuration("1:05:00")).toEqual({ ok: true, seconds: 3900 });
    expect(parseAudioDuration("6:00:00")).toEqual({ ok: true, seconds: AUDIO_DURATION_MAX_SECONDS });
    expect(parseAudioDuration("360:00")).toEqual({ ok: true, seconds: 21600 });
  });

  it("bounds: 0:00 and anything past 6 hours are refused with the range sentence", () => {
    expect(parseAudioDuration("0:00")).toEqual({ ok: false, message: AUDIO_DURATION_RANGE });
    expect(parseAudioDuration("6:00:01")).toEqual({ ok: false, message: AUDIO_DURATION_RANGE });
    expect(parseAudioDuration("360:01")).toEqual({ ok: false, message: AUDIO_DURATION_RANGE });
  });

  it("a bare number, seconds ≥ 60, letters, negatives or empty are refused with the format sentence", () => {
    for (const raw of ["750", "5", "12:60", "1:60:00", "abc", "-1:00", "", "12:3", "1:2:3"]) {
      expect(parseAudioDuration(raw), raw).toEqual({ ok: false, message: AUDIO_DURATION_FORMAT });
    }
  });

  it("formats back for display", () => {
    expect(formatAudioDuration(750)).toBe("12:30");
    expect(formatAudioDuration(5)).toBe("0:05");
    expect(formatAudioDuration(3900)).toBe("1:05:00");
  });
});

describe("display of the saved audio", () => {
  const ready: comms_audioOut = {
    file_id: "01JAUD1",
    status: "ready",
    mime_type: "audio/mpeg",
    size_bytes: 13107200,
    duration_seconds: 750,
    preview_url: "https://files.example.test/vigov-private/t_01JXA/a.mp3?X-Amz-Signature=S",
    preview_expires_at: "2026-10-01T03:15:00Z",
  };

  it("format, size and duration in one line", () => {
    expect(savedAudioText(ready)).toBe("Tệp hiện tại: MP3 · 12,5 MB · thời lượng 12:30");
    expect(audioFormatLabel("audio/mp4")).toBe("M4A");
    expect(formatAudioSize(undefined)).toBe("không rõ dung lượng");
    expect(savedAudioText({ ...ready, status: "" })).toContain("không đọc được");
  });

  it("player `src`: only the signed http(s) `preview_url` of a READY file", () => {
    expect(audioPreviewSrc(ready)).toBe(ready.preview_url);
    for (const preview_url of ["javascript:alert(1)", "data:audio/mpeg;base64,AAAA", "/a.mp3", "", undefined]) {
      expect(audioPreviewSrc({ ...ready, preview_url }), String(preview_url)).toBeNull();
    }
    expect(audioPreviewSrc({ ...ready, status: "pending" })).toBeNull();
    expect(audioPreviewSrc(null)).toBeNull();
  });

  it("expiry: milliseconds left from `preview_expires_at`, null when absent or unreadable", () => {
    const now = Date.parse("2026-10-01T03:00:00Z");
    expect(previewExpiresInMs(ready, now)).toBe(15 * 60 * 1000);
    expect(previewExpiresInMs({ ...ready, preview_expires_at: null }, now)).toBeNull();
    expect(previewExpiresInMs({ ...ready, preview_expires_at: "x" }, now)).toBeNull();
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

    const last = await runAudioUpload(mp3(), "01JTT1", "12:30", (s) => states.push(s));

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
    expect(await runAudioUpload(mp3(), "01JTT1", "750", () => {})).toEqual({ kind: "refused", message: AUDIO_DURATION_FORMAT });
    const wav = new File([new Uint8Array([1])], "a.wav", { type: "audio/wav" });
    expect(await runAudioUpload(wav, "01JTT1", "1:00", () => {})).toEqual({ kind: "refused", message: AUDIO_TYPE_REFUSED });
    const big = { name: "a.mp3", type: "audio/mpeg", size: 31 * MB } as unknown as File;
    expect(await runAudioUpload(big, "01JTT1", "1:00", () => {})).toEqual({ kind: "refused", message: AUDIO_TOO_LARGE });
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
      expect(await runAudioUpload(mp3(), "01JTT1", "1:00", () => {})).toEqual({ kind: "refused", message });
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
    expect(await runAudioUpload(mp3(), "01JTT1", "1:00", () => {})).toEqual({
      kind: "refused",
      message: AUDIO_FORM_MISSING,
    });
    expect(xhr).toHaveLength(0);
  });

  it("422 audio_rejected at completion: refused verbatim", async () => {
    fakeXHR(204);
    const cau = "Tệp bị từ chối: tệp không phải âm thanh MP3/M4A hợp lệ (ví dụ tệp video đổi đuôi).";
    fakeFetch(json(UPLOAD, 201), json({ code: "audio_rejected", message: cau }, 422));
    expect(await runAudioUpload(mp3(), "01JTT1", "1:00", () => {})).toEqual({ kind: "refused", message: cau });
  });

  it("503 at completion: `Hoàn tất lại` completes AGAIN — no second declaration, no second upload", async () => {
    const xhr = fakeXHR(204);
    const calls = fakeFetch(json(UPLOAD, 201), json({ code: "malware_scan_unavailable", message: "Chưa quét được." }, 503));
    expect(await runAudioUpload(mp3(), "01JTT1", "1:00", () => {})).toEqual({
      kind: "retry",
      id: "01JAUDIO1",
      message: "Chưa quét được.",
    });
    await retryAudioCompletion("01JAUDIO1", "2:00", () => {});
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
    expect(await runAudioUpload(mp3(), "01JTT1", "1:00", () => {})).toEqual({ kind: "refused", message: AUDIO_STORAGE_FAILED });
    expect(calls.some((c) => c.url.endsWith("/completion"))).toBe(false);
  });
});

describe("Gỡ âm thanh — PATCH audio_file_id \"\"", () => {
  it("sends exactly {audio_file_id: \"\"} to the item", async () => {
    const calls = fakeFetch(json({}, 201), json({}, 200));
    const kq = await removeContentAudio("01JTT1");
    expect(kq.ok).toBe(true);
    expect(calls).toHaveLength(1);
    expect(calls[0]?.url).toBe("/api/v1/content-items/01JTT1");
    expect(calls[0]?.init?.method).toBe("PATCH");
    expect(JSON.parse(String(calls[0]?.init?.body))).toEqual({ audio_file_id: "" });
  });

  it("the form's Lưu never carries an audio field — the audio is not part of the form", () => {
    const row = { id: "01JTT1", type: "truyen-thanh", title: "Bản tin", audio_file_id: "01JAUDIO1", audio_duration_seconds: 750 } as comms_noiDungRa;
    const before = giaTriTuHang({ ...row, category_id: "", summary: "", status: "an" } as comms_noiDungRa);
    const body = thanSua(before, { ...before, title: "Bản tin sáng" });
    expect(Object.keys(body)).toEqual(["title"]);
  });

  it("suaNoiDung passes audio fields only when given", async () => {
    const calls = fakeFetch(json({}, 201), json({}, 200));
    await suaNoiDung("01JTT1", { title: "x" });
    expect(JSON.parse(String(calls[0]?.init?.body))).toEqual({ title: "x" });
  });
});

/* ── The block as rendered (no DOM events: rendered to a string) ─────────────────────────────── */

const SAVED: comms_audioOut = {
  file_id: "01JAUDIO1",
  status: "ready",
  mime_type: "audio/mp4",
  size_bytes: 5 * MB,
  duration_seconds: 3900,
  preview_url: "https://files.example.test/vigov-private/t_01JXA/a.m4a?X-Amz-Signature=S",
  preview_expires_at: "2099-01-01T00:00:00Z",
};

function field(props: Partial<Parameters<typeof BroadcastAudioField>[0]> = {}): string {
  return renderToStaticMarkup(
    <BroadcastAudioField
      itemId="01JTT1"
      initialAudio={null}
      currentType="truyen-thanh"
      disabled={false}
      onBusyChange={() => {}}
      {...props}
    />,
  );
}

describe("BroadcastAudioField", () => {
  it("no audio yet: duration box, picker (held until a duration is typed), the 30MB hint", () => {
    const html = field();
    expect(html).toContain('id="thoi-luong-am-thanh"');
    expect(html).toContain('accept=".mp3,.m4a,audio/mpeg,audio/mp4,audio/x-m4a"');
    expect(html).toMatch(/<input[^>]*id="tep-am-thanh"[^>]*disabled=""/);
    expect(html).toContain(AUDIO_PICK_BUTTON);
    expect(html).toContain("tối đa 30MB");
    expect(html).not.toContain("<audio");
  });

  it("an audio attached: format, size, duration, a player on the signed link with preload=none, Gỡ âm thanh", () => {
    const html = field({ initialAudio: SAVED });
    expect(html).toContain("M4A · 5,0 MB · thời lượng 1:05:00");
    expect(html).toContain("<audio");
    expect(html).toContain('preload="none"');
    expect(html).toContain(`src="${SAVED.preview_url?.replace(/&/g, "&amp;")}"`);
    expect(html).toContain(AUDIO_REMOVE_BUTTON);
    // Replace = remove first: no picker while a file is attached.
    expect(html).not.toContain('id="tep-am-thanh"');
  });

  it("no usable preview link: no player, the missing sentence and the refresh button", () => {
    const html = field({ initialAudio: { ...SAVED, preview_url: undefined } });
    expect(html).not.toContain("<audio");
    expect(html).toContain(AUDIO_PREVIEW_MISSING);
    expect(html).toContain("Lấy link nghe thử mới");
  });

  it("the remove confirmation asks before the PATCH", () => {
    const html = field({ initialAudio: SAVED, initialConfirmRemove: true });
    expect(html).toContain(AUDIO_REMOVE_QUESTION);
    expect(html).toContain("Xác nhận gỡ");
  });

  it("retry state: the server's sentence as an alert, and `Hoàn tất lại`", () => {
    const html = field({ initialState: { kind: "retry", id: "01JAUDIO1", message: "Chưa quét được mã độc." } });
    expect(html).toContain('role="alert"');
    expect(html).toContain("Chưa quét được mã độc.");
    expect(html).toContain(AUDIO_RETRY_BUTTON);
  });

  it("refused state: the server's sentence verbatim", () => {
    const html = field({ initialState: { kind: "refused", message: "Mục truyền thanh này đã có tệp âm thanh." } });
    expect(html).toContain("Bị từ chối: Mục truyền thanh này đã có tệp âm thanh.");
  });

  it("type box moved away: only the warning that Lưu removes the audio", () => {
    const html = field({ initialAudio: SAVED, currentType: "tin-tuc" });
    expect(html).toContain(AUDIO_TYPE_CHANGE_DETACHES);
    expect(html).not.toContain("<audio");
    expect(field({ currentType: "tin-tuc" })).toBe("");
  });
});

describe("the §7 form and the audio block", () => {
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
  } as comms_noiDungRa;

  const form = (gt = FORM_TRONG, hang?: comms_noiDungRa, initialAudioState?: AudioUploadState) =>
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

  it("create form with type Truyền thanh: SAVE FIRST, no picker", () => {
    const html = form({ ...FORM_TRONG, type: "truyen-thanh" });
    expect(html).toContain(AUDIO_SAVE_FIRST);
    expect(html).not.toContain('id="tep-am-thanh"');
  });

  it("an item saved as another type, switched to Truyền thanh: SAVE FIRST too", () => {
    const other = { ...row, type: "tin-tuc", audio: undefined, audio_file_id: undefined } as comms_noiDungRa;
    const html = form({ ...giaTriTuHang(other), type: "truyen-thanh" }, other);
    expect(html).toContain("bấm Lưu trước");
    expect(html).not.toContain('id="tep-am-thanh"');
  });

  it("an item saved as Truyền thanh: the audio block with the attached file", () => {
    const html = form(giaTriTuHang(row), row);
    expect(html).toContain("thời lượng 1:05:00");
    expect(html).not.toContain("bấm Lưu trước");
  });

  it("other types never show the audio block", () => {
    const html = form(FORM_TRONG);
    expect(html).not.toContain("Tệp âm thanh");
    expect(html).not.toContain(AUDIO_WAIT_NOTE);
  });
});
