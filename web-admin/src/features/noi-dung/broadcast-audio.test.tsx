import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, describe, expect, it, vi } from "vitest";

import { removeContentAudio, suaNoiDung } from "@/lib/api/noi-dung";
import { installFakeUploadXHR, partNames, partValue } from "@/lib/api/upload-test-support";
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
  AUDIO_TOO_LARGE,
  AUDIO_TYPE_REFUSED,
  AUDIO_WAIT_NOTE,
  afterAudioUpload,
  audioInFlight,
  declaredAudioType,
  formatAudioSize,
  heldAudioProblem,
  parseAudioDuration,
  runAudioUpload,
  savedAudioPatch,
  type AudioUploadState,
} from "./broadcast-audio";
import { HeldAudioField } from "./broadcast-audio-field";
import { FORM_TRONG, giaTriTuHang, thanSua } from "./nhan-noi-dung";
import { FormNoiDung } from "./so-noi-dung";

/**
 * §7 `Truyền thanh` — the broadcast audio (ADR 0067 §4). The pre-check (MP3/M4A, 30 MB), the typed
 * duration in whole seconds, the upload answers BY CODE, the ONE-request flow over the shared fake
 * `XMLHttpRequest`, the edit form's audio PATCH, and the two boxes as rendered.
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

const mp3 = () => new File([new Uint8Array([0x49, 0x44, 0x33])], "ban-tin-sang.mp3", { type: "audio/mpeg" });

describe("upload answers → state, by the server's CODE", () => {
  const file = { id: "A", content_item_id: "I", mime_type: "audio/mpeg", size_bytes: 9, status: "ready", duration_seconds: 750 };
  const again = { file: mp3(), contentItemId: "I" };

  it("201 ready ⇒ ready; 201 other ⇒ refused", () => {
    expect(afterAudioUpload(again, { ok: true, data: file })).toEqual({ kind: "ready", id: "A", file });
    expect(afterAudioUpload(again, { ok: true, data: { ...file, status: "rejected" } })).toEqual({
      kind: "refused",
      message: AUDIO_NOT_READY,
    });
  });

  it("422 audio_rejected is FINAL; 422 invalid_audio_duration is a retry (fix the number, send again)", () => {
    expect(afterAudioUpload(again, { ok: false, status: 422, code: "audio_rejected", message: "mã độc" })).toEqual({
      kind: "refused",
      message: "mã độc",
    });
    expect(
      afterAudioUpload(again, { ok: false, status: 422, code: "invalid_audio_duration", message: "thời lượng sai" }),
    ).toEqual({ kind: "retry", again, message: "thời lượng sai" });
  });

  it("503 / 408 / no answer ⇒ retry with the sentence verbatim; 409 audio_limit is final", () => {
    for (const status of [503, 408, 0]) {
      expect(afterAudioUpload(again, { ok: false, status, code: "x", message: "câu" })).toEqual({ kind: "retry", again, message: "câu" });
    }
    expect(afterAudioUpload(again, { ok: false, status: 409, code: "audio_limit", message: "câu" }).kind).toBe("refused");
  });

  it("only uploading / checking hold Lưu", () => {
    expect(audioInFlight({ kind: "uploading", percent: 1 })).toBe(true);
    expect(audioInFlight({ kind: "checking" })).toBe(true);
    expect(audioInFlight({ kind: "idle" })).toBe(false);
    expect(audioInFlight({ kind: "retry", again, message: "x" })).toBe(false);
  });
});

/* ── The flow over the shared fake XMLHttpRequest ──────────────────────────────────────────────── */

const json = (body: unknown, status: number) =>
  new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });

/** The PATCH tests below: every `fetch` recorded; the item routes answer. (Uploads never use fetch.) */
function fakeFetch() {
  const calls: { url: string; init?: RequestInit }[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string, init?: RequestInit) => {
      calls.push({ url, init });
      if (url.startsWith("/api/v1/content-items/")) return json({ id: "01JTT1" }, 200);
      return json({ code: "not_found", message: "?" }, 404);
    }),
  );
  return calls;
}

const DONE = { id: "01JAUDIO1", content_item_id: "01JTT1", mime_type: "audio/mpeg", size_bytes: 3, status: "ready", duration_seconds: 750 };

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("runAudioUpload — ONE request, the typed seconds BEFORE the file", () => {
  it("happy path: one multipart POST for the SAVED item, same origin, attached on 201", async () => {
    const xhr = installFakeUploadXHR(() => ({ status: 201, body: DONE }));
    const fetchSpy = vi.fn();
    vi.stubGlobal("fetch", fetchSpy);
    const states: AudioUploadState[] = [];

    const last = await runAudioUpload(mp3(), "01JTT1", "750", (s) => states.push(s));

    expect(xhr).toHaveLength(1);
    expect(xhr[0]?.method).toBe("POST");
    expect(xhr[0]?.url).toBe("/api/v1/content-items/audio-files");
    expect(xhr[0]?.headers["Idempotency-Key"]).toMatch(/.+/);
    expect(partNames(xhr[0])).toEqual(["size", "file_name", "content_type", "content_item_id", "audio_duration_seconds", "file"]);
    expect(partValue(xhr[0], "audio_duration_seconds")).toBe("750");
    expect(partValue(xhr[0], "content_item_id")).toBe("01JTT1");
    expect(partValue(xhr[0], "size")).toBe("3");
    expect(fetchSpy).not.toHaveBeenCalled();
    expect(states.map((s) => s.kind)).toEqual(["uploading", "uploading", "uploading", "checking", "ready"]);
    expect(states).toContainEqual({ kind: "uploading", percent: 50 });
    expect(last).toEqual({ kind: "ready", id: "01JAUDIO1", file: DONE });
  });

  it("a WAV or a 31 MB file never reaches the network; a bad duration is a retry on the same file", async () => {
    const xhr = installFakeUploadXHR(() => ({ status: 201, body: DONE }));
    const f = mp3();
    expect(await runAudioUpload(f, "01JTT1", "12:30", () => {})).toEqual({
      kind: "retry",
      again: { file: f, contentItemId: "01JTT1" },
      message: AUDIO_DURATION_FORMAT,
    });
    const wav = new File([new Uint8Array([1])], "a.wav", { type: "audio/wav" });
    expect(await runAudioUpload(wav, "01JTT1", "60", () => {})).toEqual({ kind: "refused", message: AUDIO_TYPE_REFUSED });
    const big = { name: "a.mp3", type: "audio/mpeg", size: 31 * MB } as unknown as File;
    expect(await runAudioUpload(big, "01JTT1", "60", () => {})).toEqual({ kind: "refused", message: AUDIO_TOO_LARGE });
    expect(xhr).toHaveLength(0);
  });

  it("409 audio_limit / 422 audio_only_for_truyen_thanh / 400 / 422 audio_rejected: the server's sentence, final", async () => {
    for (const [status, code, message] of [
      [409, "audio_limit", "Mục truyền thanh này đã có tệp âm thanh."],
      [422, "audio_only_for_truyen_thanh", "Chỉ mục truyền thanh mới có tệp âm thanh."],
      [400, "invalid_request", "Cần `content_item_id` của mục truyền thanh đã lưu."],
      [422, "audio_rejected", "Tệp bị từ chối: tệp không phải âm thanh MP3/M4A hợp lệ (ví dụ tệp video đổi đuôi)."],
    ] as const) {
      installFakeUploadXHR(() => ({ status, body: { code, message } }));
      expect(await runAudioUpload(mp3(), "01JTT1", "60", () => {}), code).toEqual({ kind: "refused", message });
      vi.unstubAllGlobals();
    }
  });

  it("503 then `Gửi lại` with a corrected duration: a SECOND request, the same file, the NEW seconds", async () => {
    let n = 0;
    const xhr = installFakeUploadXHR(() =>
      ++n === 1 ? { status: 503, body: { code: "malware_scan_unavailable", message: "Chưa quét được." } } : { status: 201, body: DONE },
    );
    const f = mp3();
    const first = await runAudioUpload(f, "01JTT1", "60", () => {});
    expect(first).toEqual({ kind: "retry", again: { file: f, contentItemId: "01JTT1" }, message: "Chưa quét được." });
    if (first.kind !== "retry") throw new Error("not retry");
    expect((await runAudioUpload(first.again.file, first.again.contentItemId, "120", () => {})).kind).toBe("ready");
    expect(xhr).toHaveLength(2);
    expect(partValue(xhr[1], "audio_duration_seconds")).toBe("120");
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
    const calls = fakeFetch();
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
    const calls = fakeFetch();
    expect((await removeContentAudio("01JTT1")).ok).toBe(true);
    expect(calls).toHaveLength(1);
    expect(calls[0]?.init?.method).toBe("PATCH");
    expect(JSON.parse(String(calls[0]?.init?.body))).toEqual({ audio_file_id: "" });
  });

  it("suaNoiDung passes audio fields only when given", async () => {
    const calls = fakeFetch();
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

  it("a carried upload refused for the moment: its sentence as an alert, and `Gửi lại`", () => {
    const none = { ...row, audio: undefined, audio_file_id: undefined, audio_duration_seconds: undefined } as comms_noiDungRa; // vi-name-ok: generated contract type
    const html = form(giaTriTuHang(none), none, {
      kind: "retry",
      again: { file: mp3(), contentItemId: "01JTT1" },
      message: "Chưa quét được mã độc.",
    });
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
