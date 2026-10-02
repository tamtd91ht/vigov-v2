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

import {
  MAX_SCENE_PHOTOS,
  PHOTO_UPLOAD_FIELDS,
  photoCompletionAddress,
  photosAddress,
  photoUploadBody,
  readPhotoSlot,
  readScenePhotoList,
  sniffScenePhotoType,
  STORAGE_FILE_FIELD,
} from "../api/hop-dong-phan-anh";
import type { PhieuCuaToi } from "../api/hop-dong-phan-anh";

import { SCENE_PHOTOS, XA_PA, ZALO_FAILURE } from "./noi-dung";
import { CommuneSendScreen, PetitionBody, SendDone } from "./PhanAnhAppXa";
import {
  attachScenePhoto,
  completionOutcome,
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
  slotOutcome,
  storageOutcome,
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
const STORE = "https://store.example.vn/vigov-temp";
const FUTURE = "2999-01-01T00:00:00Z";

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

const slotReply = (id = "f-1") => ({
  photo: { id, content_type: "", size_bytes: 0, status: "pending", created_at: "2026-10-02T01:00:00Z" },
  upload: { url: STORE, fields: { key: "t_x/tmp/1", policy: "p", "x-amz-signature": "s" }, expires_at: FUTURE },
});
const storedReply = (id = "f-1") => ({ id, content_type: "image/jpeg", size_bytes: 10, status: "stored", created_at: "x" });
const headers = (c: Call) => (c.init?.headers ?? {}) as Record<string, string>;

const FILE: PreparedPhoto = { blob: new Blob([JPEG], { type: "image/jpeg" }), type: "image/jpeg" };
const FORM = { url: STORE, fields: { key: "k" }, expires_at: FUTURE };

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

  it("the slot body is EXACTLY content_type and size — no file name, nothing of the citizen", () => {
    const body = JSON.parse(photoUploadBody("image/png", 12345)) as Record<string, unknown>;
    expect(Object.keys(body).sort()).toEqual([...PHOTO_UPLOAD_FIELDS].sort());
    expect(body).toEqual({ content_type: "image/png", size: 12345 });
  });

  it("addresses: under the lookup code, encoded once; empty host = empty address (no call)", () => {
    expect(photosAddress(CODE)).toBe(`https://petitions.example.vn/api/v1/my-citizen-reports/${CODE}/photos`);
    expect(photoCompletionAddress(CODE, "f/1")).toBe(
      `https://petitions.example.vn/api/v1/my-citizen-reports/${CODE}/photos/f%2F1/completion`,
    );
    state.host = "";
    expect(photosAddress(CODE)).toBe("");
  });

  it("a slot reply without a form (an idempotent replay), with an http store, or naming `file` is malformed", () => {
    expect(readPhotoSlot(slotReply())).not.toBeNull();
    expect(readPhotoSlot({ code: "f-1", replayed: true })).toBeNull();
    expect(readPhotoSlot({ ...slotReply(), upload: { ...slotReply().upload, url: "http://store.example.vn" } })).toBeNull();
    expect(
      readPhotoSlot({ ...slotReply(), upload: { ...slotReply().upload, fields: { [STORAGE_FILE_FIELD]: "x" } } }),
    ).toBeNull();
  });

  it("the list: https links only; one bad row is a bad page", () => {
    const ok = { id: "a", content_type: "image/jpeg", size_bytes: 1, created_at: "x", url: "https://s/a", url_expires_at: FUTURE };
    expect(readScenePhotoList({ items: [ok] })).toEqual([{ id: "a", url: "https://s/a", url_expires_at: FUTURE }]);
    expect(readScenePhotoList({ items: [ok, { ...ok, url: "http://s/a" }] })).toBeNull();
    expect(readScenePhotoList({ items: [] })).toEqual([]);
  });
});

/* ─────────────────────────────── one photo, end to end ─────────────────────────────── */

describe("one photo — read · slot · upload · complete", () => {
  it("success: four calls in order; bearer + key on ViGov, NOTHING of the session on the store", async () => {
    fakeFetch(
      answer(200, undefined, new Blob([JPEG])),
      answer(201, slotReply()),
      answer(204),
      answer(200, storedReply()),
    );
    const out = await attachScenePhoto(CODE, { stage: "read", path: LOCAL });
    expect(out).toEqual({ kind: "stored" });
    expect(calls.map((c) => c.url)).toEqual([LOCAL, photosAddress(CODE), STORE, photoCompletionAddress(CODE, "f-1")]);

    const [, slot, store, done] = calls as [Call, Call, Call, Call];
    expect(slot.init?.method).toBe("POST");
    expect(headers(slot)["Authorization"]).toBe("Bearer tok-test");
    expect(headers(slot)["Idempotency-Key"]).toMatch(/^[0-9a-f]{32}$/);
    expect(JSON.parse(String(slot.init?.body))).toEqual({ content_type: "image/jpeg", size: JPEG.length });

    // The store: the form is the credential — no bearer, no cookie.
    expect(store.init?.method).toBe("POST");
    expect(store.init?.headers).toBeUndefined();
    expect(store.init?.credentials).toBe("omit");
    const form = store.init?.body as FormData;
    const names = [...form.keys()];
    expect(names).toEqual(["key", "policy", "x-amz-signature", STORAGE_FILE_FIELD]);
    const file = form.get(STORAGE_FILE_FIELD) as Blob;
    expect(file.type).toBe("image/jpeg");
    expect((file as File).name).toBe("photo");

    expect(headers(done)["Authorization"]).toBe("Bearer tok-test");
    expect(done.init?.body).toBeUndefined();
    // Nothing personal in any address: only the lookup code and the server's photo id.
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

  it("a retried slot asks with a NEW key (a replay brings no form)", async () => {
    fakeFetch(new Error("dropped"), answer(201, slotReply()), answer(204), answer(200, storedReply()));
    const first = await attachScenePhoto(CODE, { stage: "slot", file: FILE });
    expect(first.kind).toBe("failed");
    if (first.kind !== "failed" || first.retry === null) throw new Error("expected a retry step");
    expect(first.failure).toBe("network");
    expect(first.retry.stage).toBe("slot");
    expect(await attachScenePhoto(CODE, first.retry)).toEqual({ kind: "stored" });
    expect(headers(calls[0]!)["Idempotency-Key"]).not.toBe(headers(calls[1]!)["Idempotency-Key"]);
  });

  it("a dropped upload resumes AT THE UPLOAD with the same form — no second slot", async () => {
    fakeFetch(new Error("dropped"));
    const step: PhotoStep = { stage: "upload", file: FILE, id: "f-1", form: FORM };
    const out = await attachScenePhoto(CODE, step);
    expect(out).toEqual({ kind: "failed", failure: "network", retry: step });
  });

  it("an expired form is never posted: a fresh slot is asked first", async () => {
    fakeFetch(answer(201, slotReply("f-2")), answer(204), answer(200, storedReply("f-2")));
    const dead = { ...FORM, expires_at: "2000-01-01T00:00:00Z" };
    expect(await attachScenePhoto(CODE, { stage: "upload", file: FILE, id: "f-1", form: dead })).toEqual({ kind: "stored" });
    expect(calls[0]!.url).toBe(photosAddress(CODE));
  });

  it("no session: nothing goes out, and the photo waits for the gate", async () => {
    state.session = null;
    fakeFetch(answer(201, slotReply()));
    const out = await attachScenePhoto(CODE, { stage: "slot", file: FILE });
    expect(out).toMatchObject({ kind: "failed", failure: "session" });
    expect(calls).toHaveLength(0);
  });
});

/* ─────────────────────────────── refusals → next step ─────────────────────────────── */

describe("each refusal code → one failure, and where 'Tải lại' resumes", () => {
  const complete: Extract<PhotoStep, { stage: "complete" }> = { stage: "complete", file: FILE, id: "f-1", form: FORM };
  const refused = (status: number, code: string | null) => ({ kieu: "refused" as const, status, code });

  it("the server's codes", () => {
    expect(refusalFailure(400, "invalid_request")).toBe("not-accepted");
    expect(refusalFailure(404, "not_found")).toBe("not-found");
    expect(refusalFailure(409, "petition_state")).toBe("petition-moved");
    expect(refusalFailure(409, "photo_limit")).toBe("limit");
    expect(refusalFailure(409, "photo_state")).toBe("expired");
    expect(refusalFailure(409, "upload_expired")).toBe("expired");
    expect(refusalFailure(409, "upload_not_received")).toBe("not-received");
    expect(refusalFailure(422, "photo_rejected")).toBe("rejected");
    expect(refusalFailure(503, "storage_not_configured")).toBe("not-configured");
    expect(refusalFailure(503, "malware_scan_unavailable")).toBe("busy");
    expect(refusalFailure(503, "upload_limits_unavailable")).toBe("busy");
    // Unknown: a server fault, never "fix your photo".
    expect(refusalFailure(500, "internal")).toBe("server");
    expect(refusalFailure(409, "something_new")).toBe("server");
    expect(refusalFailure(404, null)).toBe("not-found");
  });

  it("final answers offer no retry; transient ones resume at the right step", () => {
    expect(completionOutcome(refused(422, "photo_rejected"), complete)).toEqual({ kind: "failed", failure: "rejected", retry: null });
    expect(completionOutcome(refused(409, "petition_state"), complete)).toEqual({
      kind: "failed",
      failure: "petition-moved",
      retry: null,
    });
    expect(completionOutcome(refused(503, "malware_scan_unavailable"), complete)).toEqual({
      kind: "failed",
      failure: "busy",
      retry: complete,
    });
    expect(completionOutcome(refused(409, "upload_expired"), complete)).toEqual({
      kind: "failed",
      failure: "expired",
      retry: { stage: "slot", file: FILE },
    });
    expect(completionOutcome(refused(409, "upload_not_received"), complete)).toEqual({
      kind: "failed",
      failure: "server",
      retry: { stage: "upload", file: FILE, id: "f-1", form: FORM },
    });
    expect(completionOutcome({ kieu: "rate-limited", retryAfterSeconds: 60 }, complete)).toMatchObject({
      failure: "rate-limited",
      retry: complete,
    });
    expect(slotOutcome(refused(409, "photo_limit"), FILE)).toEqual({ kind: "failed", failure: "limit", retry: null });
    expect(slotOutcome(refused(400, "invalid_request"), FILE)).toEqual({ kind: "failed", failure: "not-accepted", retry: null });
    expect(storageOutcome({ kieu: "tu-choi", status: 403 }, { stage: "upload", file: FILE, id: "f-1", form: FORM })).toEqual({
      kind: "failed",
      failure: "storage-refused",
      retry: { stage: "slot", file: FILE },
    });
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

  it("BEFORE Zalo asks, the card says why and that Zalo will ask — for each button", () => {
    const camera = renderToStaticMarkup(createElement(ScenePhotoButtons, { ...buttonProps(), explaining: "camera" }));
    expect(camera).toContain(SCENE_PHOTOS.camera_why);
    expect(camera).toContain(SCENE_PHOTOS.continue);
    expect(camera).toContain(SCENE_PHOTOS.later);
    expect(camera).not.toContain(SCENE_PHOTOS.pick);
    expect(camera).not.toContain(`${SCENE_PHOTOS.take}</button>`);
    expect(SCENE_PHOTOS.camera_why).toMatch(/Zalo sẽ hỏi/);
    const library = renderToStaticMarkup(createElement(ScenePhotoButtons, { ...buttonProps(), explaining: "library" }));
    expect(library).toContain(SCENE_PHOTOS.library_why);
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
