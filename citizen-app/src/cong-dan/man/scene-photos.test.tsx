import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

/**
 * ẢNH HIỆN TRƯỜNG — the state half: the contract, one photo's steps against a faked network, what the screens
 * say, and that the owner's decisions hold (optional · ≤ 5 · images only · AFTER the petition · a photo failure
 * never fails the send · only while `da-tiep-nhan` · button only where the shell injects it).
 *
 * Session and host are faked at module level, as in `goi-vigov.test.tsx`: production has no parameter through
 * which a session could be handed in.
 */
const state = vi.hoisted(() => ({
  session: { token: "tok-test", ten_xa: "Xã Thử Nghiệm" } as { token: string; ten_xa: string } | null,
  host: "https://petitions.example.vn",
}));
vi.mock("../api/phien-vigov", () => ({ layPhienViGov: () => state.session }));
vi.mock("../api/dia-chi-vigov", () => ({
  diaChiViGov: (_service: string, path: string) => (state.host === "" ? "" : `${state.host}${path}`),
}));

import { SCENE_PHOTO_UPLOAD_WAIT_MS } from "../api/goi-vigov";
import {
  MAX_SCENE_PHOTOS,
  PHOTO_FILE_NAME,
  PHOTO_UPLOAD_PARTS,
  photosAddress,
  photoUploadForm,
  readPhotoUploadReply,
  readScenePhotoList,
  sniffScenePhotoType,
} from "../api/hop-dong-phan-anh";
import type { PhieuCuaToi } from "../api/hop-dong-phan-anh";

import { CONSENT_DIALOG, SCENE_PHOTOS, XA_PA, ZALO_FAILURE } from "./noi-dung";
import { CommuneSendScreen, PetitionBody, SendDone } from "./PhanAnhAppXa";
import {
  attachScenePhoto,
  msUntilRefresh,
  OwnScenePhotosView,
  type PhotoFailure,
  type PhotoPicking,
  type PhotoStep,
  pickFailureText,
  pickOnce,
  type PickScenePhotos,
  photosInFlight,
  type PreparedPhoto,
  refusalFailure,
  ScenePhotoButtons,
  ScenePhotoField,
  ScenePhotoUploads,
  sendOutcome,
  uploadSummary,
  type UploadJob,
} from "./scene-photos";

/* ─────────────────────────────── fakes ─────────────────────────────── */

const JPEG = new Uint8Array([0xff, 0xd8, 0xff, 0xe0, 0, 0x10, 0x4a, 0x46, 0x49, 0x46, 0, 1, 9, 9, 9]);
const PNG = new Uint8Array([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0, 0, 0, 0]);
const WEBP = new TextEncoder().encode("RIFF\u0000\u0000\u0000\u0000WEBPVP8 ");
// HEIC: `....ftypheic` — the type the server refuses.
const HEIC = new Uint8Array([0, 0, 0, 0x18, 0x66, 0x74, 0x79, 0x70, 0x68, 0x65, 0x69, 0x63]);

const CODE = "PA7K2QX9M4TD";
const LOCAL = "zalo-temp://photo-1.jpg";
const FUTURE = "2999-01-01T00:00:00Z";
const PETITIONS_HOST = "https://petitions.example.vn";

type Call = { url: string; init: RequestInit | undefined };
let calls: Call[] = [];

function answer(status: number, body?: unknown, blob?: Blob) {
  return {
    status,
    ok: status >= 200 && status < 300,
    json: async () => {
      if (body === undefined) throw new SyntaxError("no body");
      return body;
    },
    blob: async () => blob ?? new Blob([]),
    headers: { get: () => null },
  };
}

/** Each fetch gets the next answer; an `Error` is a dropped line. */
function fakeFetch(...answers: Array<ReturnType<typeof answer> | Error>) {
  let i = 0;
  vi.stubGlobal("fetch", (url: string, init?: RequestInit) => {
    calls.push({ url, init });
    const a = answers[Math.min(i++, answers.length - 1)]!;
    return a instanceof Error ? Promise.reject(a) : Promise.resolve(a);
  });
}

const storedReply = (id = "f-1") => ({ id, content_type: "image/jpeg", size_bytes: 10, status: "stored", created_at: "x" });
const replayReply = (id = "f-1") => ({ code: id, replayed: true });
const headers = (c: Call) => (c.init?.headers ?? {}) as Record<string, string>;
const keyOf = (c: Call) => headers(c)["Idempotency-Key"];

const FILE: PreparedPhoto = { blob: new Blob([JPEG], { type: "image/jpeg" }), type: "image/jpeg" };
const SEND: PhotoStep = { stage: "send", file: FILE, key: null };

beforeEach(() => {
  calls = [];
  state.session = { token: "tok-test", ten_xa: "Xã Thử Nghiệm" };
  state.host = "https://petitions.example.vn";
});
afterEach(() => vi.unstubAllGlobals());

/* ─────────────────────────────── contract ─────────────────────────────── */

describe("contract — what the photo routes carry", () => {
  it("the type comes from the bytes: JPEG, PNG, WebP; HEIC and junk are refused before any call", () => {
    expect(sniffScenePhotoType(JPEG)).toBe("image/jpeg");
    expect(sniffScenePhotoType(PNG)).toBe("image/png");
    expect(sniffScenePhotoType(WEBP)).toBe("image/webp");
    expect(sniffScenePhotoType(HEIC)).toBeNull();
    expect(sniffScenePhotoType(new Uint8Array([]))).toBeNull();
  });

  it("the multipart body is size, content_type, then the file — in that order, nothing else, a fixed file name", async () => {
    const form = photoUploadForm(new Blob([PNG]), "image/png");
    expect([...form.keys()]).toEqual([...PHOTO_UPLOAD_PARTS]);
    expect(PHOTO_UPLOAD_PARTS).toEqual(["size", "content_type", "file"]);
    expect(form.get("size")).toBe(String(PNG.length));
    expect(form.get("content_type")).toBe("image/png");
    const file = form.get("file") as File;
    expect(file.type).toBe("image/png");
    // The phone's own file name never travels (rule 3): the part carries a fixed word.
    expect(file.name).toBe(PHOTO_FILE_NAME);
    expect(PHOTO_FILE_NAME).toBe("photo");
    expect(new Uint8Array(await file.arrayBuffer())).toEqual(PNG);
  });

  it("address: under the lookup code on the petitions host, encoded once; empty host = empty address (no call)", () => {
    expect(photosAddress(CODE)).toBe(`${PETITIONS_HOST}/api/v1/my-citizen-reports/${CODE}/photos`);
    expect(photosAddress("a/b")).toBe(`${PETITIONS_HOST}/api/v1/my-citizen-reports/a%2Fb/photos`);
    state.host = "";
    expect(photosAddress(CODE)).toBe("");
  });

  it("the 201: the stored photo, or the idempotent replay's file id; anything else is malformed", () => {
    expect(readPhotoUploadReply(storedReply())).toEqual({ kind: "stored", photo: { id: "f-1", status: "stored" } });
    expect(readPhotoUploadReply(replayReply("f-9"))).toEqual({ kind: "replayed", id: "f-9" });
    expect(readPhotoUploadReply({ code: "", replayed: true })).toBeNull();
    expect(readPhotoUploadReply({ code: "f-1", replayed: false })).toBeNull();
    expect(readPhotoUploadReply({ id: "", status: "stored" })).toBeNull();
    expect(readPhotoUploadReply(null)).toBeNull();
  });

  it("the list: https links only; one bad row is a bad page", () => {
    const ok = { id: "a", content_type: "image/jpeg", size_bytes: 1, created_at: "x", url: "https://s/a", url_expires_at: FUTURE };
    expect(readScenePhotoList({ items: [ok] })).toEqual([{ id: "a", url: "https://s/a", url_expires_at: FUTURE }]);
    expect(readScenePhotoList({ items: [ok, { ...ok, url: "http://s/a" }] })).toBeNull();
    expect(readScenePhotoList({ items: [] })).toEqual([]);
  });

  it("the upload waits at least as long as the server's own 180 s bound before calling it a dropped line", () => {
    expect(SCENE_PHOTO_UPLOAD_WAIT_MS).toBeGreaterThanOrEqual(180_000);
  });
});

/* ─────────────────────────────── one photo, end to end ─────────────────────────────── */

describe("one photo — read · send (ONE multipart request to petitions)", () => {
  it("success: the local read, then ONE POST to the petitions host — bearer + key, multipart, no storage host", async () => {
    fakeFetch(answer(200, undefined, new Blob([JPEG])), answer(201, storedReply()));
    const out = await attachScenePhoto(CODE, { stage: "read", path: LOCAL });
    expect(out).toEqual({ kind: "stored" });
    // Exactly two fetches: the phone's own temp file, and petitions. Nothing to any object store.
    expect(calls.map((c) => c.url)).toEqual([LOCAL, photosAddress(CODE)]);
    expect(calls.filter((c) => !c.url.startsWith("zalo-temp://")).every((c) => c.url.startsWith(`${PETITIONS_HOST}/`))).toBe(
      true,
    );

    const send = calls[1]!;
    expect(send.init?.method).toBe("POST");
    expect(headers(send)["Authorization"]).toBe("Bearer tok-test");
    expect(keyOf(send)).toMatch(/^[0-9a-f]{32}$/);
    // The boundary is the runtime's to write: a hand-set Content-Type would have none.
    expect(headers(send)["Content-Type"]).toBeUndefined();
    const form = send.init?.body as FormData;
    expect(form).toBeInstanceOf(FormData);
    expect([...form.keys()]).toEqual(["size", "content_type", "file"]);
    expect(form.get("size")).toBe(String(JPEG.length));
    expect(form.get("content_type")).toBe("image/jpeg");
    const file = form.get("file") as File;
    expect(file.type).toBe("image/jpeg");
    expect(file.name).toBe("photo");
    expect(file.name).not.toContain("photo-1");
    // Nothing personal in any address: only the lookup code.
    for (const c of calls) expect(c.url).not.toMatch(/0900000000|Nguyễn/);
  });

  it("an unreadable local file and a non-image never reach ViGov — and are not retried", async () => {
    fakeFetch(new Error("no such file"));
    expect(await attachScenePhoto(CODE, { stage: "read", path: LOCAL })).toEqual({
      kind: "failed",
      failure: "not-readable",
      retry: null,
    });
    calls = [];
    fakeFetch(answer(200, undefined, new Blob([HEIC])));
    expect(await attachScenePhoto(CODE, { stage: "read", path: LOCAL })).toEqual({
      kind: "failed",
      failure: "not-a-photo",
      retry: null,
    });
    expect(calls).toHaveLength(1);
  });

  it("a DROPPED LINE retries with the SAME Idempotency-Key — and the server's replay counts as stored", async () => {
    fakeFetch(new Error("dropped"), answer(201, replayReply()));
    const first = await attachScenePhoto(CODE, SEND);
    if (first.kind !== "failed" || first.retry === null) throw new Error("expected a retry step");
    expect(first.failure).toBe("network");
    expect(await attachScenePhoto(CODE, first.retry)).toEqual({ kind: "stored" });
    expect(calls).toHaveLength(2);
    expect(keyOf(calls[0]!)).toMatch(/^[0-9a-f]{32}$/);
    expect(keyOf(calls[1]!)).toBe(keyOf(calls[0]!));
  });

  it("a 201 whose body cannot be read retries with the SAME key too — the photo may already be stored", async () => {
    fakeFetch(answer(201, { nonsense: true }), answer(201, replayReply()));
    const first = await attachScenePhoto(CODE, SEND);
    if (first.kind !== "failed" || first.retry === null) throw new Error("expected a retry step");
    expect(first.failure).toBe("server");
    expect(await attachScenePhoto(CODE, first.retry)).toEqual({ kind: "stored" });
    expect(keyOf(calls[1]!)).toBe(keyOf(calls[0]!));
  });

  it("the same key still in flight (409 request_in_progress) keeps the key", async () => {
    fakeFetch(answer(409, { code: "request_in_progress" }), answer(201, replayReply()));
    const first = await attachScenePhoto(CODE, SEND);
    if (first.kind !== "failed" || first.retry === null) throw new Error("expected a retry step");
    expect(first.failure).toBe("in-progress");
    await attachScenePhoto(CODE, first.retry);
    expect(keyOf(calls[1]!)).toBe(keyOf(calls[0]!));
  });

  it("a DEFINITE refusal that can be retried asks again with a NEW key", async () => {
    fakeFetch(answer(503, { code: "upload_busy" }), answer(201, storedReply()));
    const first = await attachScenePhoto(CODE, SEND);
    if (first.kind !== "failed" || first.retry === null) throw new Error("expected a retry step");
    expect(first.failure).toBe("busy");
    expect(await attachScenePhoto(CODE, first.retry)).toEqual({ kind: "stored" });
    expect(keyOf(calls[1]!)).toMatch(/^[0-9a-f]{32}$/);
    expect(keyOf(calls[1]!)).not.toBe(keyOf(calls[0]!));
  });

  it("a 201 that is not `stored` is not reported as sent", async () => {
    fakeFetch(answer(201, { ...storedReply(), status: "pending" }));
    expect(await attachScenePhoto(CODE, SEND)).toMatchObject({ kind: "failed", failure: "server" });
  });

  it("no session: nothing goes out, and the photo waits for the gate", async () => {
    state.session = null;
    fakeFetch(answer(201, storedReply()));
    const out = await attachScenePhoto(CODE, SEND);
    expect(out).toMatchObject({ kind: "failed", failure: "session" });
    expect(calls).toHaveLength(0);
  });
});

/* ─────────────────────────────── refusals → next step ─────────────────────────────── */

describe("each refusal code → one failure, its sentence, and whether 'Tải lại' is offered", () => {
  const refused = (status: number, code: string | null) => ({ kieu: "refused" as const, status, code });
  const step = { stage: "send" as const, file: FILE, key: "k".repeat(32) };

  it("the server's codes", () => {
    expect(refusalFailure(400, "invalid_request")).toBe("not-accepted");
    expect(refusalFailure(400, "invalid_upload")).toBe("not-accepted");
    expect(refusalFailure(415, "unsupported_media_type")).toBe("not-accepted");
    expect(refusalFailure(413, "file_too_large")).toBe("too-large");
    expect(refusalFailure(408, "upload_timeout")).toBe("slow");
    expect(refusalFailure(404, "not_found")).toBe("not-found");
    expect(refusalFailure(409, "petition_state")).toBe("petition-moved");
    expect(refusalFailure(409, "photo_limit")).toBe("limit");
    expect(refusalFailure(409, "upload_changed")).toBe("not-stored");
    expect(refusalFailure(409, "photo_state")).toBe("not-stored");
    expect(refusalFailure(409, "request_in_progress")).toBe("in-progress");
    expect(refusalFailure(422, "photo_rejected")).toBe("rejected");
    expect(refusalFailure(503, "upload_busy")).toBe("busy");
    expect(refusalFailure(503, "storage_not_configured")).toBe("not-configured");
    expect(refusalFailure(503, "malware_scan_unavailable")).toBe("busy");
    expect(refusalFailure(503, "upload_limits_unavailable")).toBe("busy");
    // A proxy in front of petitions answers by status alone, with no ViGov body.
    expect(refusalFailure(413, null)).toBe("too-large");
    expect(refusalFailure(415, null)).toBe("not-accepted");
    expect(refusalFailure(408, null)).toBe("slow");
    // Unknown: a server fault, never "fix your photo".
    expect(refusalFailure(500, "internal")).toBe("server");
    expect(refusalFailure(409, "something_new")).toBe("server");
    expect(refusalFailure(404, null)).toBe("not-found");
  });

  it("each new code reaches the citizen as the plain sentence the owner's table names", async () => {
    const cases: Array<[number, string, RegExp]> = [
      [413, "file_too_large", /lớn hơn dung lượng/],
      [415, "unsupported_media_type", /Không gửi được ảnh này/],
      [400, "invalid_upload", /Không gửi được ảnh này/],
      [503, "upload_busy", /đang bận.*ít phút/],
      [408, "upload_timeout", /Mạng chậm/],
    ];
    for (const [status, code, sentence] of cases) {
      calls = [];
      fakeFetch(answer(status, { code, message: "câu của máy chủ", trace_id: "t" }));
      const out = await attachScenePhoto(CODE, SEND);
      if (out.kind !== "failed") throw new Error(`${code}: expected a failure`);
      const text = SCENE_PHOTOS.failures[out.failure];
      expect(text, code).toMatch(sentence);
      // The server's own sentence, a status or a code never reaches the screen.
      expect(text, code).not.toContain("câu của máy chủ");
      expect(text, code).not.toContain(String(status));
      expect(text, code).not.toContain(code);
    }
  });

  it("final answers offer no retry; transient ones do", () => {
    for (const [status, code] of [
      [422, "photo_rejected"],
      [409, "petition_state"],
      [409, "photo_limit"],
      [413, "file_too_large"],
      [415, "unsupported_media_type"],
      [400, "invalid_upload"],
      [404, "not_found"],
    ] as const) {
      expect(sendOutcome(refused(status, code), step), code).toMatchObject({ kind: "failed", retry: null });
    }
    // Definite but transient refusals: a NEW key (`key: null`).
    for (const [status, code] of [
      [503, "upload_busy"],
      [503, "malware_scan_unavailable"],
      [408, "upload_timeout"],
      [409, "upload_changed"],
      [500, "internal"],
    ] as const) {
      expect(sendOutcome(refused(status, code), step), code).toMatchObject({
        kind: "failed",
        retry: { stage: "send", file: FILE, key: null },
      });
    }
    expect(sendOutcome({ kieu: "rate-limited", retryAfterSeconds: 60 }, step)).toMatchObject({
      failure: "rate-limited",
      retry: { stage: "send", key: null },
    });
    // Not a refusal: the same key, so a photo already stored is replayed, never stored twice.
    expect(sendOutcome({ kieu: "loi-mang" }, step)).toEqual({ kind: "failed", failure: "network", retry: step });
    expect(sendOutcome({ kieu: "loi-may-chu" }, step)).toEqual({ kind: "failed", failure: "server", retry: step });
    expect(sendOutcome({ kieu: "het-phien" }, step)).toEqual({ kind: "failed", failure: "session", retry: step });
    expect(sendOutcome({ kieu: "xong", gia_tri: { kind: "replayed", id: "f-1" } }, step)).toEqual({ kind: "stored" });
  });
});

/* ─────────────────────────────── picking ─────────────────────────────── */

describe("one pick (`pickOnce`)", () => {
  const paths = (n: number) => Array.from({ length: n }, (_, i) => `zalo-temp://p${i}.jpg`);

  it("the camera asks for ONE; the library for the slots left; never more than five", async () => {
    const pick = vi.fn<PickScenePhotos>(async () => ({ kind: "xong", paths: paths(9) }));
    expect((await pickOnce(pick, "camera", 5)).paths).toHaveLength(1);
    expect(pick).toHaveBeenLastCalledWith("camera", 1);
    expect((await pickOnce(pick, "library", 3)).paths).toHaveLength(3);
    expect(pick).toHaveBeenLastCalledWith("library", 3);
    expect((await pickOnce(pick, "library", 9)).paths).toHaveLength(MAX_SCENE_PHOTOS);
    // Full: Zalo is not asked at all.
    pick.mockClear();
    expect(await pickOnce(pick, "library", 0)).toEqual({ paths: [], failure: null });
    expect(pick).not.toHaveBeenCalled();
  });

  it("a cancel says nothing; a refusal, outside Zalo and a rejection each have their sentence", async () => {
    expect(await pickOnce(async () => ({ kind: "huy" }), "library", 5)).toEqual({ paths: [], failure: null });
    const refused = await pickOnce(async () => ({ kind: "tu-choi" }), "camera", 5);
    expect(pickFailureText(refused.failure!)).toBe(SCENE_PHOTOS.camera_refused);
    const outside = await pickOnce(async () => ({ kind: "ngoai-zalo" }), "library", 5);
    expect(pickFailureText(outside.failure!)).toBe(SCENE_PHOTOS.outside_zalo);
    const thrown = await pickOnce(() => Promise.reject(new Error("x")), "library", 5);
    expect(pickFailureText(thrown.failure!)).toBe(SCENE_PHOTOS.pick_failed);
  });

  it("Zalo's refusal names the capability, never the code in the sentence", async () => {
    const out = await pickOnce(
      async () => ({ kind: "thu-lai", zalo: { capability: "photos", code: -1403, transient: false } }),
      "library",
      5,
    );
    const text = pickFailureText(out.failure!);
    expect(text).toContain(ZALO_FAILURE.what.photos);
    expect(text).not.toContain("1403");
    const html = renderToStaticMarkup(createElement(ScenePhotoButtons, { ...buttonProps(), failure: out.failure }));
    expect(html).toContain(ZALO_FAILURE.support_code(-1403));
  });
});

/* ─────────────────────────────── what the citizen sees ─────────────────────────────── */

const noop = () => {};
function buttonProps() {
  return { count: 0, explaining: null, picking: false, failure: null, onAsk: noop, onConfirm: noop, onCancel: noop };
}
const idlePicking: PhotoPicking = { explaining: null, picking: false, failure: null, ask: noop, confirm: noop, cancel: noop };

describe("the words — the owner's decisions, in what the citizen reads", () => {
  it("the label says OPTIONAL and the ceiling; the required list no longer names photos; nothing 'sắp có'", () => {
    expect(XA_PA.anh_bat_buoc).toContain("không bắt buộc");
    expect(XA_PA.anh_bat_buoc).toContain(`tối đa ${MAX_SCENE_PHOTOS} ảnh`);
    expect(XA_PA.anh_bat_buoc).not.toMatch(/video|\(bắt buộc/);
    expect(XA_PA.bat_buoc).not.toMatch(/ảnh|video/);
    expect(XA_PA.bat_buoc_an_danh).not.toMatch(/ảnh|video/);
    expect(XA_PA.anh_sap_co).not.toContain("sắp có");
    expect(SCENE_PHOTOS.add_why).toContain(`tối đa ${MAX_SCENE_PHOTOS} ảnh`);
  });

  it("every failure is a sentence with a next step, no code, and never says the petition failed", () => {
    const all = Object.entries(SCENE_PHOTOS.failures) as Array<[PhotoFailure, string]>;
    expect(all.length).toBeGreaterThan(10);
    for (const [k, s] of all) {
      expect(s, k).not.toMatch(/\b[1-5]\d\d\b|_|rate_limited|photo_/);
      expect(s, k).toMatch(/Bà con|bà con|Phản ánh|phản ánh|Ứng dụng/);
      expect(s, k).not.toMatch(/phản ánh (chưa|không) (được )?gửi/i);
    }
  });
});

describe("the send form's photo field", () => {
  it("label, why, count and the two buttons; the draft note only when there is a draft", () => {
    const html = renderToStaticMarkup(
      createElement(ScenePhotoField, { photos: [], onRemove: noop, picking: idlePicking, hasDraft: true }),
    );
    expect(html).toContain(XA_PA.anh_bat_buoc);
    expect(html).toContain(SCENE_PHOTOS.why);
    expect(html).toContain(SCENE_PHOTOS.not_in_draft);
    expect(html).toContain(SCENE_PHOTOS.count(0, MAX_SCENE_PHOTOS));
    expect(html).toContain(SCENE_PHOTOS.take);
    expect(html).toContain(SCENE_PHOTOS.pick);
    expect(html.match(/class="xa-nut xa-nut--phu"/g)).toHaveLength(2);
    const noDraft = renderToStaticMarkup(
      createElement(ScenePhotoField, { photos: [], onRemove: noop, picking: idlePicking, hasDraft: false }),
    );
    expect(noDraft).not.toContain(SCENE_PHOTOS.not_in_draft);
  });

  it("each picked photo: a thumbnail with words, and its own 'Bỏ ảnh thứ n' button", () => {
    const photos = [
      { key: "a", path: "zalo-temp://a.jpg" },
      { key: "b", path: "zalo-temp://b.jpg" },
    ];
    const html = renderToStaticMarkup(createElement(ScenePhotoField, { photos, onRemove: noop, picking: idlePicking, hasDraft: false }));
    expect(html).toContain(`alt="${SCENE_PHOTOS.photo_alt(2)}"`);
    expect(html).toContain(SCENE_PHOTOS.remove(1));
    expect(html).toContain(SCENE_PHOTOS.remove(2));
    expect(html).toContain(SCENE_PHOTOS.count(2, MAX_SCENE_PHOTOS));
  });

  it("at five: no more buttons, and a sentence saying how to change one", () => {
    const html = renderToStaticMarkup(createElement(ScenePhotoButtons, { ...buttonProps(), count: MAX_SCENE_PHOTOS }));
    expect(html).not.toContain(SCENE_PHOTOS.take);
    expect(html).toContain(SCENE_PHOTOS.full(MAX_SCENE_PHOTOS));
  });

  it("BEFORE Zalo asks: a SHORT modal question per button, 'Cho phép' / 'Không' (owner, 09/10/2026)", () => {
    expect(SCENE_PHOTOS.camera_question).toBe("Cho phép ứng dụng dùng camera để chụp ảnh hiện trường?");
    expect(SCENE_PHOTOS.library_question).toBe("Cho phép ứng dụng truy cập ảnh trong máy để chọn ảnh hiện trường?");
    expect(CONSENT_DIALOG.allow).toBe("Cho phép");
    expect(CONSENT_DIALOG.deny).toBe("Không");

    const camera = renderToStaticMarkup(createElement(ScenePhotoButtons, { ...buttonProps(), explaining: "camera" }));
    // A modal dialog a screen reader reads as a question and stays in — not an inline card.
    expect(camera).toMatch(/role="alertdialog" aria-modal="true" aria-labelledby="xa-photo-consent-question"/);
    expect(camera).toContain(SCENE_PHOTOS.camera_question);
    expect(camera).not.toContain(SCENE_PHOTOS.library_question);
    expect(camera).toContain(`>${CONSENT_DIALOG.allow}</button>`);
    expect(camera).toContain(`>${CONSENT_DIALOG.deny}</button>`);
    // The long card is gone: no "Tiếp tục" / "Để sau", no paragraph about what Zalo will do.
    expect(camera).not.toContain("Tiếp tục");
    expect(camera).not.toContain("Để sau");
    expect(camera).not.toMatch(/Zalo sẽ hỏi/);
    expect("camera_why" in SCENE_PHOTOS || "library_why" in SCENE_PHOTOS).toBe(false);

    const library = renderToStaticMarkup(createElement(ScenePhotoButtons, { ...buttonProps(), explaining: "library" }));
    expect(library).toContain(SCENE_PHOTOS.library_question);
    expect(library).not.toContain(SCENE_PHOTOS.camera_question);

    // No question open: no dialog at all.
    expect(renderToStaticMarkup(createElement(ScenePhotoButtons, buttonProps()))).not.toContain("alertdialog");
  });

  it("the send screen renders without asking Zalo for anything", () => {
    const pick = vi.fn<PickScenePhotos>();
    const html = renderToStaticMarkup(
      createElement(CommuneSendScreen, {
        ten_xa: "Xã Thử Nghiệm",
        ho_ten: null,
        onBack: noop,
        onSessionLost: noop,
        onSent: noop,
        onOpenPetition: noop,
        pickScenePhotos: pick,
      }),
    );
    expect(html.length).toBeGreaterThan(0);
    expect(pick).not.toHaveBeenCalled();
  });
});

describe("after 'Gửi phản ánh' — the photos go, the petition stands", () => {
  const job = (index: number, status: UploadJob["status"], extra: Partial<UploadJob> = {}): UploadJob => ({
    key: `k${index}`,
    index,
    status,
    ...extra,
  });

  it("the headline counts, in words", () => {
    expect(uploadSummary([job(1, "stored"), job(2, "sending"), job(3, "waiting")])).toBe(SCENE_PHOTOS.sending(2, 3));
    expect(uploadSummary([job(1, "stored"), job(2, "stored")])).toBe(SCENE_PHOTOS.all_sent(2));
    expect(uploadSummary([job(1, "stored"), job(2, "failed", { failure: "network", canRetry: true })])).toBe(
      SCENE_PHOTOS.some_failed(1, 2),
    );
  });

  it("a failed photo shows its sentence and 'Tải lại' only when it can help; the petition is said to be recorded", () => {
    const html = renderToStaticMarkup(
      createElement(ScenePhotoUploads, {
        jobs: [
          job(1, "stored"),
          job(2, "failed", { failure: "network", canRetry: true }),
          job(3, "failed", { failure: "rejected", canRetry: false }),
        ],
        onRetry: noop,
        afterSend: true,
      }),
    );
    expect(html).toContain(SCENE_PHOTOS.petition_kept);
    expect(html).toContain(SCENE_PHOTOS.row_sent(1));
    expect(html).toContain(SCENE_PHOTOS.failures.network);
    expect(html).toContain(SCENE_PHOTOS.retry(2));
    expect(html).toContain(SCENE_PHOTOS.failures.rejected);
    expect(html).not.toContain(SCENE_PHOTOS.retry(3));
  });

  it("photos in flight still hold a place among the five; final refusals do not", () => {
    expect(
      photosInFlight([
        job(1, "stored"),
        job(2, "sending"),
        job(3, "failed", { canRetry: true }),
        job(4, "failed", { canRetry: false }),
      ]),
    ).toBe(2);
  });

  it("the lookup code stays on screen above the photo list", () => {
    const petition = PETITION;
    const html = renderToStaticMarkup(
      createElement(SendDone, { petition, onFollow: noop, onHome: noop }, createElement("p", null, "PHOTOS-HERE")),
    );
    expect(html).toContain(`#${CODE}`);
    expect(html.indexOf(`#${CODE}`)).toBeLessThan(html.indexOf("PHOTOS-HERE"));
  });
});

const PETITION: PhieuCuaToi = {
  ma_tra_cuu: CODE,
  trang_thai: "da-tiep-nhan",
  linh_vuc: "",
  nhan_linh_vuc: "",
  noi_dung: "Ổ gà",
  dia_chi: "",
  ho_ten_da_che: "",
  dien_thoai_da_che: "",
  an_danh: true,
  goc_dem_han: "2026-10-02T01:00:00Z",
  han_tiep_nhan: null,
  han_xu_ly_xong: null,
  ket_qua: "",
  ly_do: "",
  co_quan_nhan: "",
  rating: null,
  rated_at: null,
};

describe("the petition detail — own photos only when there are some; 'Thêm ảnh' only while Đã tiếp nhận", () => {
  const view = (over: Partial<Parameters<typeof OwnScenePhotosView>[0]>) =>
    renderToStaticMarkup(
      createElement(OwnScenePhotosView, {
        list: { kind: "ready", items: [] },
        canAdd: false,
        count: 0,
        picking: idlePicking,
        jobs: [],
        onRetryUpload: noop,
        onRetryList: noop,
        onImageError: noop,
        ...over,
      }),
    );
  const link = (id: string) => ({ id, url: `https://store.example.vn/${id}?sig=x`, url_expires_at: FUTURE });

  it("no photos and no way to add: no block at all (decision 12 — no empty frame)", () => {
    expect(view({})).toBe("");
    expect(view({ list: { kind: "loading" } })).toBe("");
  });

  it("photos: the title and one described image each", () => {
    const html = view({ list: { kind: "ready", items: [link("a"), link("b")] } });
    expect(html).toContain(SCENE_PHOTOS.own_title);
    expect(html.match(/<img /g)).toHaveLength(2);
    expect(html).toContain(`alt="${SCENE_PHOTOS.photo_alt(2)}"`);
    expect(html).not.toContain(SCENE_PHOTOS.take);
  });

  it("while Đã tiếp nhận with the picker: the add block with its why; a failed list says so with Thử lại", () => {
    const add = view({ canAdd: true });
    expect(add).toContain(SCENE_PHOTOS.add_title);
    expect(add).toContain(SCENE_PHOTOS.add_why);
    expect(add).toContain(SCENE_PHOTOS.take);
    const failed = view({ list: { kind: "failed" } });
    expect(failed).toContain(SCENE_PHOTOS.own_failed);
    expect(failed).toContain(SCENE_PHOTOS.own_retry);
  });

  it("the body never lists photos on its own render, and a moved-on petition has no add block", () => {
    const pick = vi.fn<PickScenePhotos>();
    fakeFetch(answer(200, { items: [] }));
    const moved = { ...PETITION, trang_thai: "dang-xu-ly" };
    const html = renderToStaticMarkup(createElement(PetitionBody, { petition: moved, photos: { pick, onSessionLost: noop } }));
    expect(html).not.toContain(SCENE_PHOTOS.add_title);
    expect(calls).toHaveLength(0); // effects do not run in a server render: no call on its own
    expect(pick).not.toHaveBeenCalled();
  });

  it("links are fetched again before the first one expires — never kept past it", () => {
    const now = Date.parse("2026-10-02T01:00:00Z");
    const soon = { id: "a", url: "https://s/a", url_expires_at: "2026-10-02T01:10:00Z" };
    expect(msUntilRefresh([soon], now)).toBe(10 * 60_000 - 30_000);
    expect(msUntilRefresh([{ ...soon, url_expires_at: "garbage" }], now)).toBe(5_000);
    expect(msUntilRefresh([{ ...soon, url_expires_at: FUTURE }], now)).toBe(15 * 60_000);
  });
});
