import type { ReactElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, describe, expect, it, vi } from "vitest";

import {
  type KetQuaGui, // vi-name-ok: existing type
  type PhotoUploadResult,
  readPickedFile,
  readRequestPhotoImage,
  listRequestPhotos,
  uploadRequestPhoto,
} from "../../api/goi-may-chu";
import {
  docDanhSach, // vi-name-ok: existing export
  photoFailureFrom,
  REQUEST_PHOTO_ERROR,
  REQUEST_PHOTO_MAX,
  REQUEST_PHOTO_MAX_BYTES,
  REQUEST_PHOTO_PART,
  type RequestPhotoFailure,
  RETRYABLE_PHOTO_FAILURES,
  type YeuCauMoi, // vi-name-ok: existing type
} from "../../api/hop-dong-yeu-cau";
import { CONSULT_PHOTO_UPLOAD_FIELDS, DUONG_ROI_KHOI_MAY } from "../../content/ket-xuat-ho-so";
import { NHAN_NUT_CHAT } from "../company-intro/NutChatOA";
import { NhaCungCapPhien } from "../dang-nhap/kho-phien"; // vi-name-ok: existing export
import { KHAI_BAO_LOI_GOI, type PhotoPick } from "../tinh-nang/zalo-api";

import {
  type ConsultPhoto,
  type ConsultSteps,
  markForRetry,
  pickConsultPhotos,
  type PhotoPicker,
  type PhotoUpload,
  preparePickedPhoto,
  queueUploads,
  sendConsultation,
  uploadInOrder,
} from "./consult-photos";
import { ConsultPhotoField, ConsultPhotoStatus, createConsentGate, pickMessages, uploadRowText } from "./ConsultPhotos";
import { CONSULT_PHOTOS, YEU_CAU_CUA_TOI } from "./noi-dung";
import { DaGuiXong, TuVanBaoGiaScreen } from "./TuVanBaoGiaScreen";
import { DongYeuCau } from "./YeuCauCuaToiScreen";

/**
 * PHOTOS ON "TƯ VẤN VÀ BÁO GIÁ" (owner 12/10/2026). What must hold, one block each:
 *
 *   the order        the request is created FIRST, then each photo goes up one at a time to ITS id
 *   a photo failure  never fails the request: the code stays on screen, the failed photo says why and offers
 *                    "Tải lại" only when a second press can help
 *   every code       maps to a Vietnamese sentence that says what to do — never the code itself
 *   the question     before Zalo, per button; "Không" reaches nothing
 *   at most 3        on the form, in the queue, in the picker's count
 *   photoCount       "n ảnh" on "Yêu cầu của tôi"
 *
 * No Zalo and no server: the steps are injected, `fetch` is stubbed where the network file is under test.
 */

const ve = (node: ReactElement) => renderToStaticMarkup(node);
const textOf = (markup: string) => markup.replace(/<[^>]+>/g, " ").replace(/&amp;/g, "&").replace(/\s+/g, " ");

const JPEG = new Uint8Array([0xff, 0xd8, 0xff, 0xe0, 0, 0x10, 0x4a, 0x46, 0x49, 0x46, 0, 1]);
const jpeg = (size = JPEG.length) => {
  const bytes = new Uint8Array(size);
  bytes.set(JPEG.slice(0, Math.min(size, JPEG.length)));
  return new Blob([bytes], { type: "image/jpeg" });
};
const photo = (n: number): ConsultPhoto => ({ key: `k${n}`, path: `zalo-temp://${n}.jpg`, file: jpeg() });

const REQUEST: YeuCauMoi = { loai: "consult", quan_tam: ["messaging"], quy_mo: "", ghi_chu: "", nguon: "" };
const STORED = (position: number): PhotoUploadResult => ({
  kind: "stored",
  photo: { id: `p${position}`, request_id: "req-1", position, content_type: "image/jpeg", size: 12, created_at: "" },
});

afterEach(() => {
  vi.unstubAllGlobals();
});

/* ═══════════════════════════════ THE ORDER: create, then upload one by one ═══════════════════════════════ */

describe("send: the request first, then each photo, one at a time, to the id the request got", () => {
  it("create → onSent → photo 1 → photo 2 → photo 3, never two uploads in flight", async () => {
    const log: string[] = [];
    let inFlight = 0;
    let maxInFlight = 0;
    const steps: ConsultSteps = {
      send: async () => {
        log.push("send");
        return { kieu: "xong", ma_yeu_cau: "req-1" };
      },
      upload: async (bearer, requestId, file) => {
        inFlight += 1;
        maxInFlight = Math.max(maxInFlight, inFlight);
        log.push(`upload:${requestId}:${bearer}:${file.size}`);
        await Promise.resolve();
        inFlight -= 1;
        return STORED(log.filter((l) => l.startsWith("upload")).length);
      },
    };
    const seen: (readonly PhotoUpload[])[] = [];
    const out = await sendConsultation(steps, "phieu-thu", REQUEST, [photo(1), photo(2), photo(3)], () => log.push("onSent"), (j) => seen.push(j));
    expect(log).toEqual(["send", "onSent", "upload:req-1:phieu-thu:12", "upload:req-1:phieu-thu:12", "upload:req-1:phieu-thu:12"]);
    expect(maxInFlight, "two photos went up at once").toBe(1);
    expect(out.photos.map((j) => j.status)).toEqual(["stored", "stored", "stored"]);
    // Every photo is reported going before it is reported stored — the screen says "Đang gửi ảnh n/3".
    expect(seen.some((jobs) => jobs[0]!.status === "sending")).toBe(true);
  });

  it("no photo goes with a callback, nor with a request that was not created", async () => {
    const upload = vi.fn(async () => STORED(1));
    for (const [request, sent] of [
      [{ ...REQUEST, loai: "callback" }, { kieu: "xong", ma_yeu_cau: "req-2" }],
      [REQUEST, { kieu: "tu-choi", cau: "" }],
      [REQUEST, { kieu: "chua-dang-nhap" }],
      [REQUEST, { kieu: "khong-goi-duoc" }],
    ] as const) {
      const out = await sendConsultation({ send: async () => sent as KetQuaGui, upload }, "t", request, [photo(1)], () => {}, () => {});
      expect(out.photos).toEqual([]);
    }
    expect(upload).not.toHaveBeenCalled();
  });

  it("no photo picked: the request alone, no upload, nothing said about photos", async () => {
    const upload = vi.fn(async () => STORED(1));
    const out = await sendConsultation({ send: async () => ({ kieu: "xong", ma_yeu_cau: "r" }), upload }, "t", REQUEST, [], () => {}, () => {});
    expect(out.result.kieu).toBe("xong");
    expect(upload).not.toHaveBeenCalled();
    expect(ve(<ConsultPhotoStatus jobs={[]} onRetry={() => {}} />)).toBe("");
  });
});

/* ═══════════════════════════════ A PHOTO FAILURE KEEPS THE REQUEST ═══════════════════════════════ */

describe("a photo that fails never turns the request into a failure", () => {
  const steps = (answers: PhotoUploadResult[]): ConsultSteps => ({
    send: async () => ({ kieu: "xong", ma_yeu_cau: "req-9" }),
    upload: async () => answers.shift() ?? STORED(1),
  });

  it("photo 2 fails: the request is sent, 1 and 3 still go, 2 says why and offers 'Tải lại'", async () => {
    let shown: KetQuaGui | null = null;
    const out = await sendConsultation(
      steps([STORED(1), { kind: "failed", failure: "server" }, STORED(2)]),
      "t",
      REQUEST,
      [photo(1), photo(2), photo(3)],
      (r) => (shown = r),
      () => {},
    );
    expect(shown).toEqual({ kieu: "xong", ma_yeu_cau: "req-9" });
    expect(out.photos.map((j) => [j.status, j.canRetry ?? null])).toEqual([
      ["stored", null],
      ["failed", true],
      ["stored", null],
    ]);

    // The success screen: the code, how many arrived, the failed photo's reason, and its retry button.
    const screen = textOf(
      ve(
        <DaGuiXong
          ma_yeu_cau="req-9"
          onXemDanhSach={() => {}}
          onGuiTiep={() => {}}
          photoStatus={<ConsultPhotoStatus jobs={out.photos} onRetry={() => {}} />}
        />,
      ),
    );
    expect(screen).toContain("req-9");
    expect(screen).toContain(CONSULT_PHOTOS.some_arrived(2, 3));
    expect(screen).toContain(CONSULT_PHOTOS.row_failed(2, CONSULT_PHOTOS.failures.server));
    expect(screen).toContain(CONSULT_PHOTOS.retry(2));
  });

  it("an upload that THROWS is a failed photo, never a rejected send", async () => {
    const out = await sendConsultation(
      { send: async () => ({ kieu: "xong", ma_yeu_cau: "r" }), upload: async () => Promise.reject(new Error("x")) },
      "t",
      REQUEST,
      [photo(1)],
      () => {},
      () => {},
    );
    expect(out.result.kieu).toBe("xong");
    expect(out.photos[0]).toMatchObject({ status: "failed", failure: "server", canRetry: true });
  });

  it("a final refusal offers no 'Tải lại' — pressing again would meet the same answer", async () => {
    const out = await uploadInOrder(queueUploads([photo(1)]), "t", "r", async () => ({ kind: "failed", failure: "limit" }), () => {});
    expect(out[0]).toMatchObject({ status: "failed", canRetry: false });
    const markup = textOf(ve(<ConsultPhotoStatus jobs={out} onRetry={() => {}} />));
    expect(markup).toContain(CONSULT_PHOTOS.failures.limit);
    expect(markup).not.toContain(CONSULT_PHOTOS.retry(1));
    // And `markForRetry` refuses it too: the button is not the only guard.
    expect(markForRetry(out, out[0]!.key)[0]!.status).toBe("failed");
  });

  it("'Tải lại' resends ONLY the failed photo, to the same request", async () => {
    const answers: PhotoUploadResult[] = [STORED(1), { kind: "failed", failure: "network" }];
    const first = await uploadInOrder(queueUploads([photo(1), photo(2)]), "t", "req-3", async () => answers.shift()!, () => {});
    expect(first.map((j) => [j.status, j.canRetry ?? null])).toEqual([
      ["stored", null],
      ["failed", true],
    ]);
    const calls: string[] = [];
    const again = await uploadInOrder(markForRetry(first, first[1]!.key), "t2", "req-3", async (bearer, requestId) => {
      calls.push(`${requestId}:${bearer}`);
      return STORED(2);
    }, () => {});
    expect(calls, "the stored photo was sent twice, or the retry went elsewhere").toEqual(["req-3:t2"]);
    expect(again.map((j) => j.status)).toEqual(["stored", "stored"]);
    expect(textOf(ve(<ConsultPhotoStatus jobs={again} onRetry={() => {}} />))).toContain(CONSULT_PHOTOS.all_arrived(2));
  });

  it("while a photo is still going, the screen says so and offers no retry yet", () => {
    const jobs = queueUploads([photo(1), photo(2)]).map((j, i) => (i === 0 ? { ...j, status: "sending" as const } : j));
    expect(textOf(ve(<ConsultPhotoStatus jobs={jobs} onRetry={() => {}} />))).toContain(CONSULT_PHOTOS.sending(1, 2));
  });
});

/* ═══════════════════════════════ EVERY ERROR CODE → A SENTENCE ═══════════════════════════════ */

describe("every answer of the photo route becomes a sentence saying what to do — never a code", () => {
  const CASES: readonly [number, string | null, RequestPhotoFailure][] = [
    [400, REQUEST_PHOTO_ERROR.invalidUpload, "invalid-upload"],
    [404, REQUEST_PHOTO_ERROR.notFound, "not-found"],
    [409, REQUEST_PHOTO_ERROR.limit, "limit"],
    [409, REQUEST_PHOTO_ERROR.notAllowed, "not-allowed"],
    [413, REQUEST_PHOTO_ERROR.tooLarge, "too-large"],
    [415, REQUEST_PHOTO_ERROR.unsupported, "unsupported"],
    [422, REQUEST_PHOTO_ERROR.unreadable, "unreadable"],
    [500, REQUEST_PHOTO_ERROR.internal, "server"],
    [503, REQUEST_PHOTO_ERROR.unavailable, "unavailable"],
    // 401 and 429 carry no code — by status only.
    [401, null, "session"],
    [429, null, "rate-limited"],
    // A proxy in front answering by itself, no JSON body: the status decides.
    [413, null, "too-large"],
    [415, null, "unsupported"],
    [503, null, "unavailable"],
    // A code this build does not know is a server fault, never "fix your photo".
    [409, "a_new_code", "server"],
    [418, null, "server"],
  ];

  it("each status/code → its failure", () => {
    for (const [status, code, failure] of CASES) {
      expect(photoFailureFrom(status, code), `${status} ${code}`).toBe(failure);
    }
    // All nine published codes are covered above.
    expect(new Set(CASES.map(([, c]) => c).filter((c) => c !== null && c !== "a_new_code"))).toEqual(
      new Set(Object.values(REQUEST_PHOTO_ERROR)),
    );
  });

  it("each failure has a Vietnamese sentence with a next step, and no code or status in it", () => {
    const failures = Object.keys(CONSULT_PHOTOS.failures) as RequestPhotoFailure[];
    expect(failures.length).toBe(13);
    for (const f of failures) {
      const sentence = CONSULT_PHOTOS.failures[f];
      expect(sentence.length, f).toBeGreaterThan(30);
      expect(sentence, f).not.toMatch(/\b[1-5]\d\d\b|_|invalid|photo_|error/i);
      // A next step: retry, a place to look, or the chat button that every intro screen carries.
      expect(sentence, f).toMatch(/Tải lại|Chat Zalo|Yêu cầu của tôi|không được gửi/);
      // "Tải lại" is promised only where the button is offered.
      if (sentence.includes("“Tải lại”")) expect(RETRYABLE_PHOTO_FAILURES.has(f), f).toBe(true);
    }
    // The chat button the sentences name is the real one.
    expect(CONSULT_PHOTOS.failures.session).toContain(`“${NHAN_NUT_CHAT}”`);
    expect(uploadRowText({ key: "k", index: 2, file: jpeg(), status: "failed", failure: "network" })).toBe(
      CONSULT_PHOTOS.row_failed(2, CONSULT_PHOTOS.failures.network),
    );
  });
});

/* ═══════════════════════════════ THE NETWORK FILE ═══════════════════════════════ */

describe("api/goi-may-chu.ts — the photo calls, against a stubbed fetch", () => {
  const ADDRESS = "https://vi-du.test/api/v1/requests/req-1/photos";

  function stubFetch(status: number, body: unknown, bytes?: Blob) {
    const calls: { url: string; init: RequestInit | undefined }[] = [];
    vi.stubGlobal("fetch", (url: string, init?: RequestInit) => {
      calls.push({ url, init });
      return Promise.resolve({
        status,
        ok: status >= 200 && status < 300,
        json: () => (body === undefined ? Promise.reject(new Error("no json")) : Promise.resolve(body)),
        blob: () => Promise.resolve(bytes ?? new Blob([])),
      });
    });
    return calls;
  }

  it("upload: POST, the bearer, EXACTLY one multipart part named 'photo', no hand-set Content-Type", async () => {
    const calls = stubFetch(201, { photoId: "p1", requestId: "req-1", position: 1, contentType: "image/jpeg", size: 12, createdAt: "x" });
    const out = await uploadRequestPhoto("phieu-thu", "req-1", jpeg(), ADDRESS);
    expect(out).toEqual({ kind: "stored", photo: { id: "p1", request_id: "req-1", position: 1, content_type: "image/jpeg", size: 12, created_at: "x" } });
    expect(calls).toHaveLength(1);
    expect(calls[0]!.url).toBe(ADDRESS);
    expect(calls[0]!.init?.method).toBe("POST");
    expect(calls[0]!.init?.headers).toEqual({ Authorization: "Bearer phieu-thu" });
    const form = calls[0]!.init?.body as FormData;
    expect([...form.keys()]).toEqual([REQUEST_PHOTO_PART]);
    expect((form.get(REQUEST_PHOTO_PART) as File).name, "the phone's file name left the device").toBe("photo");
  });

  it("upload: every refusal → its failure; a dropped line → network; an unreadable 201 → not stored", async () => {
    stubFetch(409, { code: "photo_limit", message: "…" });
    expect(await uploadRequestPhoto("t", "req-1", jpeg(), ADDRESS)).toEqual({ kind: "failed", failure: "limit" });
    stubFetch(401, { message: "…" });
    expect(await uploadRequestPhoto("t", "req-1", jpeg(), ADDRESS)).toEqual({ kind: "failed", failure: "session" });
    stubFetch(413, undefined);
    expect(await uploadRequestPhoto("t", "req-1", jpeg(), ADDRESS)).toEqual({ kind: "failed", failure: "too-large" });
    stubFetch(201, { requestId: "req-1" });
    expect(await uploadRequestPhoto("t", "req-1", jpeg(), ADDRESS)).toEqual({ kind: "failed", failure: "server" });
    vi.stubGlobal("fetch", () => Promise.reject(new TypeError("offline")));
    expect(await uploadRequestPhoto("t", "req-1", jpeg(), ADDRESS)).toEqual({ kind: "failed", failure: "network" });
  });

  it("fails closed before the network: no host, no session, no request id", async () => {
    const calls = stubFetch(201, {});
    expect(await uploadRequestPhoto("t", "req-1", jpeg(), "")).toEqual({ kind: "failed", failure: "not-configured" });
    expect(await uploadRequestPhoto("", "req-1", jpeg(), ADDRESS)).toEqual({ kind: "failed", failure: "session" });
    expect(await uploadRequestPhoto("t", "", jpeg(), ADDRESS)).toEqual({ kind: "failed", failure: "not-found" });
    // (Not asserted on the default address: a machine whose `.env.local` sets the API host builds a real one.)
    expect(calls).toEqual([]);
  });

  it("list and read: GET with the bearer; the image comes back as bytes, never a stored copy", async () => {
    const calls = stubFetch(200, { items: [{ photoId: "p1", requestId: "req-1", position: 1 }, { bad: true }] });
    expect(await listRequestPhotos("t", "req-1", ADDRESS)).toMatchObject({ kind: "listed", photos: [{ id: "p1", position: 1 }] });
    expect(calls[0]!.init?.method).toBe("GET");
    const image = jpeg();
    stubFetch(200, undefined, image);
    expect(await readRequestPhotoImage("t", "req-1", "p1", `${ADDRESS}/p1`)).toEqual({ kind: "read", image });
    stubFetch(404, { code: "not_found" });
    expect(await readRequestPhotoImage("t", "req-1", "p1", `${ADDRESS}/p1`)).toEqual({ kind: "failed", failure: "not-found" });
  });

  it("reading a picked file: an unreadable temp path is null, never a throw (the device question)", async () => {
    vi.stubGlobal("fetch", () => Promise.reject(new TypeError("not allowed")));
    expect(await readPickedFile("zalo-temp://a.jpg")).toBeNull();
    stubFetch(404, undefined);
    expect(await readPickedFile("zalo-temp://a.jpg")).toBeNull();
    stubFetch(0, undefined, new Blob([]));
    expect(await readPickedFile("zalo-temp://a.jpg"), "an empty file is not a photo").toBeNull();
    const file = jpeg();
    stubFetch(0, undefined, file);
    expect(await readPickedFile("zalo-temp://a.jpg"), "a local read answers status 0").toBe(file);
    expect(await readPickedFile("")).toBeNull();
  });

  it("the dossier declares the upload with exactly the parts the form sends", () => {
    const row = DUONG_ROI_KHOI_MAY.find((d) => d.tuyen === "/api/v1/requests/{requestId}/photos");
    expect(row, "the dossier does not declare the consultation photo upload").toBeDefined();
    expect(row!.app).toBe("shared");
    expect(row!.nguoi_dung_bam).toBe(true);
    expect(row!.truong).toBe(CONSULT_PHOTO_UPLOAD_FIELDS);
    const form = new FormData();
    form.append(REQUEST_PHOTO_PART, jpeg(), "photo");
    expect(CONSULT_PHOTO_UPLOAD_FIELDS.map((f) => f.khoa)).toEqual(["requestId", ...form.keys()]);
    expect(row!.may_chu).toMatch(/bỏ thông tin vị trí chụp/);
  });
});

/* ═══════════════════════════════ PICKING: ≤ 3, checked before sending ═══════════════════════════════ */

describe("picking: at most 3, the camera one at a time, a bad file said while there is still a choice", () => {
  const counter = () => {
    let n = 0;
    return () => `k${n++}`;
  };
  const answer = (paths: string[]): PhotoPick => ({ kieu: "xong", du_lieu: paths });

  it("the picker is asked for no more than the room left, and its extra answers are dropped", async () => {
    const pick = vi.fn<PhotoPicker>(async () => answer(["a", "b", "c", "d"]));
    const out = await pickConsultPhotos(pick, "library", 2, counter(), async () => jpeg());
    expect(pick).toHaveBeenCalledWith("library", 2);
    expect(out.photos.map((p) => p.path)).toEqual(["a", "b"]);
    // The camera takes one, whatever the room.
    await pickConsultPhotos(pick, "camera", 3, counter(), async () => jpeg());
    expect(pick).toHaveBeenLastCalledWith("camera", 1);
    // No room: Zalo is not asked at all.
    pick.mockClear();
    expect(await pickConsultPhotos(pick, "library", 0, counter(), async () => jpeg())).toEqual({ photos: [], rejected: [], failure: null });
    expect(pick).not.toHaveBeenCalled();
    expect(REQUEST_PHOTO_MAX).toBe(3);
    expect(queueUploads([photo(1), photo(2), photo(3), photo(4)])).toHaveLength(3);
  });

  it("refused · outside Zalo · Zalo failing · closed: each its own outcome, none a photo", async () => {
    for (const [kq, failure] of [
      [{ kieu: "tu-choi" }, "refused"],
      [{ kieu: "ngoai-zalo" }, "outside-zalo"],
      [{ kieu: "khong-lay-duoc" }, "failed"],
      [{ kieu: "huy" }, null],
    ] as const) {
      const out = await pickConsultPhotos(async () => kq as PhotoPick, "camera", 3, counter(), async () => jpeg());
      expect(out).toEqual({ photos: [], rejected: [], failure });
    }
    expect(pickMessages("camera", "refused", [])).toEqual([CONSULT_PHOTOS.camera_refused]);
    expect(pickMessages("library", "refused", [])).toEqual([CONSULT_PHOTOS.library_refused]);
  });

  it("an unreadable path, a file over 5 MB and a HEIC are kept OUT, each with its sentence", async () => {
    expect(await preparePickedPhoto("p", async () => null)).toEqual({ rejected: "file-unreadable" });
    expect(await preparePickedPhoto("p", async () => jpeg(REQUEST_PHOTO_MAX_BYTES + 1))).toEqual({ rejected: "too-large" });
    const heic = new Blob([new Uint8Array([0, 0, 0, 0x18, 0x66, 0x74, 0x79, 0x70, 0x68, 0x65, 0x69, 0x63])]);
    expect(await preparePickedPhoto("p", async () => heic)).toEqual({ rejected: "unsupported" });
    const png = new Blob([new Uint8Array([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a])]);
    expect(await preparePickedPhoto("p", async () => png)).toEqual({ file: png });

    const read = async (path: string) => (path === "bad" ? null : jpeg());
    const out = await pickConsultPhotos(async () => answer(["ok", "bad"]), "library", 3, counter(), read);
    expect(out.photos.map((p) => p.path)).toEqual(["ok"]);
    expect(pickMessages("library", null, out.rejected)).toEqual([CONSULT_PHOTOS.rejected["file-unreadable"]]);
  });
});

/* ═══════════════════════════════ THE QUESTION BEFORE ZALO ═══════════════════════════════ */

describe("the purpose is said before Zalo's dialog, per button, per screen opening", () => {
  const field = (props: Partial<Parameters<typeof ConsultPhotoField>[0]> = {}) =>
    ve(
      <ConsultPhotoField
        photos={[]}
        asking={null}
        picking={false}
        messages={[]}
        onAsk={() => {}}
        onAllow={() => {}}
        onDeny={() => {}}
        onRemove={() => {}}
        {...props}
      />,
    );

  it("the gate: a first tap asks; 'Cho phép' frees THAT button only; 'Không' frees nothing", () => {
    const gate = createConsentGate();
    expect(gate.needsQuestion("camera")).toBe(true);
    expect(gate.needsQuestion("library")).toBe(true);
    gate.allow("camera");
    expect(gate.needsQuestion("camera")).toBe(false);
    expect(gate.needsQuestion("library"), "allowing the camera allowed the library").toBe(true);
    // A new screen opening is a new gate.
    expect(createConsentGate().needsQuestion("camera")).toBe(true);
  });

  it("no question on screen until a tap; the open question names the purpose and offers Cho phép / Không", () => {
    const idle = textOf(field());
    expect(idle).toContain(CONSULT_PHOTOS.take);
    expect(idle).toContain(CONSULT_PHOTOS.pick);
    expect(idle).not.toContain(CONSULT_PHOTOS.camera_question);
    for (const [source, question] of [
      ["camera", CONSULT_PHOTOS.camera_question],
      ["library", CONSULT_PHOTOS.library_question],
    ] as const) {
      const open = field({ asking: source });
      expect(textOf(open)).toContain(question);
      expect(textOf(open)).toContain(CONSULT_PHOTOS.question_why);
      expect(open).toMatch(new RegExp(`<button[^>]*>${CONSULT_PHOTOS.allow}</button>`));
      expect(open).toMatch(new RegExp(`<button[^>]*>${CONSULT_PHOTOS.deny}</button>`));
      // While the question is open, the tiles cannot open a second one.
      expect((open.match(/vh-photo-tile--add" disabled=""/g) ?? []).length).toBe(2);
    }
  });

  it("the platform-call table quotes the screen's questions word for word (what the reviewer looks for)", () => {
    const view = (api: string) => KHAI_BAO_LOI_GOI.find((r) => r.api === api)!.intro_app!;
    expect(view("requestCameraPermission").de_lam_gi).toContain(`“${CONSULT_PHOTOS.camera_question}”`);
    expect(view("openMediaPicker").de_lam_gi).toContain(`“${CONSULT_PHOTOS.library_question}”`);
    for (const api of ["requestCameraPermission", "openMediaPicker", "chooseImage"]) {
      expect(view(api).de_lam_gi, api).toContain(`“${CONSULT_PHOTOS.allow}”`);
      expect(view(api).man, api).toBe("Tư vấn và báo giá");
    }
  });

  it("three photos: no add tile, each photo has a 'Bỏ ảnh n' button; ≤ 3 drawn", () => {
    const full = field({ photos: [photo(1), photo(2), photo(3)] });
    expect(full).not.toContain("vh-photo-tile--add");
    expect(textOf(full)).toContain(CONSULT_PHOTOS.full);
    for (const n of [1, 2, 3]) expect(full).toContain(`aria-label="${CONSULT_PHOTOS.remove(n)}"`);
    expect((full.match(/<img /g) ?? []).length).toBe(3);
  });

  it("on the real form: the field is under the note, no input element, and the callback is told it carries none", () => {
    const form = ve(
      <NhaCungCapPhien phien_ban_dau={{ token: "phieu-thu", het_han: "2026-12-29T03:00:00Z" }}>
        <TuVanBaoGiaScreen />
      </NhaCungCapPhien>,
    );
    expect(form.indexOf("<textarea")).toBeLessThan(form.indexOf(CONSULT_PHOTOS.title));
    expect(form).not.toMatch(/<(form|input|select)\b/);
    expect(textOf(form)).not.toContain(CONSULT_PHOTOS.camera_question);
    // No photo yet → no callback note.
    expect(textOf(form)).not.toContain(CONSULT_PHOTOS.callback_no_photos);
  });
});

/* ═══════════════════════════════ "YÊU CẦU CỦA TÔI": n ảnh ═══════════════════════════════ */

describe("Yêu cầu của tôi shows 'n ảnh' from photoCount", () => {
  it("the list reads photoCount; absent or nonsense is 0", () => {
    const rows = docDanhSach({
      items: [
        { requestId: "a", kind: "consult", status: "moi", createdAt: "2026-10-12T03:00:00Z", photoCount: 2 },
        { requestId: "b", kind: "consult", status: "moi", createdAt: "2026-10-12T03:00:00Z" },
        { requestId: "c", kind: "consult", status: "moi", createdAt: "2026-10-12T03:00:00Z", photoCount: 7 },
        { requestId: "d", kind: "consult", status: "moi", createdAt: "2026-10-12T03:00:00Z", photoCount: "3" },
      ],
    });
    expect(rows.map((r) => r.photo_count)).toEqual([2, 0, 0, 0]);
  });

  it("a row with photos says 'n ảnh'; a row without says nothing about photos", () => {
    const base = { ma: "YC-1", loai: "consult" as const, trang_thai: "moi" as const, tao_luc: "2026-10-12T03:00:00Z" };
    const with2 = textOf(ve(<DongYeuCau yeu_cau={{ ...base, photo_count: 2 }} />));
    expect(with2).toContain(YEU_CAU_CUA_TOI.photos_label);
    expect(with2).toContain("2 ảnh");
    const none = textOf(ve(<DongYeuCau yeu_cau={{ ...base, photo_count: 0 }} />));
    expect(none).not.toContain(YEU_CAU_CUA_TOI.photos_label);
  });
});
