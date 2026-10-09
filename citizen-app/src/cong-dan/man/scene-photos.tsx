/**
 * ẢNH HIỆN TRƯỜNG — scene photos on "Gửi phản ánh" and on the citizen's own petition, in the COMMUNE'S OWN
 * APP only (owner, 02/10/2026: "chỉ trên app chính của xã nhé, không phải app vihat"; ADR 0047 row "Ảnh hiện
 * trường khi gửi phản ánh").
 *
 * WHAT THE OWNER DECIDED, AND WHERE EACH ONE LIVES HERE:
 *
 *   optional, at most 5, images only     the label (`XA_PA.anh_bat_buoc`), `MAX_SCENE_PHOTOS`, `sniffScenePhotoType`
 *   AFTER the petition exists            `usePhotoUploads.start` runs only from a 201 (`CommuneSendScreen.send`)
 *   a photo failure never fails the send the petition is already recorded when the first photo byte moves
 *   only while `da-tiep-nhan`            the "Thêm ảnh" block of the petition detail (`OwnScenePhotos`); the
 *                                        server enforces it (409 `petition_state`) — the block only hides
 *   only the commune app's BUTTON        the shell injects `PickScenePhotos` in `AppRieng` alone; absent = no
 *                                        button anywhere (`ranh-gioi-hai-nua.test.ts`)
 *   not kept in the draft                photos live in this screen's state; the temp paths do not survive
 *
 * The state half calls no `zmp-sdk` (`ranh-gioi-hai-nua.test.ts` §3a): the shell injects `PickScenePhotos`,
 * which asks Zalo and hands back LOCAL temp paths — nothing has left the phone when it returns.
 *
 * ⚠ NOTHING HERE LOGS, STORES OR PUTS IN A URL OR KEY a path, a photo, an upload form or a read link (rule 3;
 *   the form and the links are bearer credentials). They live in React state and refs, and die with the screen.
 */
import { useEffect, useRef, useState } from "react";

import {
  completeScenePhoto,
  listScenePhotos,
  listVerificationPhotos,
  type PhotoCallResult,
  postPhotoToStorage,
  readPickedPhoto,
  requestScenePhotoSlot,
  type StorageUploadResult,
} from "../api/goi-vigov";
import {
  MAX_SCENE_PHOTOS,
  PHOTO_ERROR,
  photoUploadBody,
  type PhotoSlot,
  type PhotoUploadForm,
  type ScenePhotoLink,
  type ScenePhotoOut,
  type ScenePhotoType,
  sniffScenePhotoType,
} from "../api/hop-dong-phan-anh";
import { taoLanGui } from "../api/lan-gui"; // vi-name-ok: existing export, not renamed (rule 12 #3)
import type { ZaloFailure } from "../api/mo-phien-vigov";

import { BieuTuong } from "./BieuTuong";
import { ConsentDialog } from "./consent-dialog";
import { SCENE_PHOTOS, VERIFICATION_PHOTOS, zaloFailureSentence, zaloSupportCode } from "./noi-dung";
import type { OnSessionLost } from "./PhanAnhAppXa";

/* ═══════════════════════════════ PICKING — the injected capability ═══════════════════════════════ */

/** "Chụp ảnh" (Zalo's camera, one photo) or "Chọn ảnh có sẵn" (the photo picker). */
export type ScenePhotoSource = "camera" | "library";

/**
 * What one pick became, one branch per thing the citizen does next:
 *
 *   `xong`        local temp paths (possibly none)
 *   `huy`         the citizen closed the picker — nothing to say
 *   `tu-choi`     the citizen said no to Zalo's question — they can still send without a photo
 *   `ngoai-zalo`  opened outside Zalo
 *   `thu-lai`     Zalo did not answer, or refused with a code (`zalo`) — the sentence names the capability
 */
export type ScenePhotoPickResult =
  | { readonly kind: "xong"; readonly paths: readonly string[] }
  | { readonly kind: "huy" }
  | { readonly kind: "tu-choi" }
  | { readonly kind: "ngoai-zalo" }
  | { readonly kind: "thu-lai"; readonly zalo?: ZaloFailure };

/** Injected by the shell (`App.tsx` `AppRieng` ONLY). `max` is how many photos the petition can still take. */
export type PickScenePhotos = (source: ScenePhotoSource, max: number) => Promise<ScenePhotoPickResult>;

export type PickFailure = {
  readonly source: ScenePhotoSource;
  readonly kind: "tu-choi" | "ngoai-zalo" | "thu-lai";
  readonly zalo?: ZaloFailure;
};

/**
 * One tap → the paths to keep, or the failure to say. PURE apart from calling `pick`. A rejection is
 * `thu-lai`; non-string or empty paths are dropped; never more than `max` (the camera takes ONE).
 */
export async function pickOnce(
  pick: PickScenePhotos,
  source: ScenePhotoSource,
  max: number,
): Promise<{ paths: string[]; failure: PickFailure | null }> {
  const limit = Math.min(source === "camera" ? 1 : max, MAX_SCENE_PHOTOS);
  if (limit <= 0) return { paths: [], failure: null };
  let result: ScenePhotoPickResult;
  try {
    result = await pick(source, limit);
  } catch {
    return { paths: [], failure: { source, kind: "thu-lai" } };
  }
  switch (result.kind) {
    case "xong":
      return {
        paths: result.paths.filter((p): p is string => typeof p === "string" && p !== "").slice(0, limit),
        failure: null,
      };
    case "huy":
      return { paths: [], failure: null };
    case "thu-lai":
      return {
        paths: [],
        failure: result.zalo === undefined ? { source, kind: "thu-lai" } : { source, kind: "thu-lai", zalo: result.zalo },
      };
    default:
      return { paths: [], failure: { source, kind: result.kind } };
  }
}

/** The sentence for a failed pick — Zalo's refusal (capability, never the code) when it gave one. */
export function pickFailureText(f: PickFailure): string {
  if (f.zalo !== undefined) return SCENE_PHOTOS.pick_zalo_failed(zaloFailureSentence(f.zalo), f.zalo.transient);
  if (f.kind === "tu-choi") return f.source === "camera" ? SCENE_PHOTOS.camera_refused : SCENE_PHOTOS.library_refused;
  if (f.kind === "ngoai-zalo") return SCENE_PHOTOS.outside_zalo;
  return SCENE_PHOTOS.pick_failed;
}

/**
 * The two buttons' behaviour. THE SCREEN ASKS BEFORE ZALO DOES (Zalo policy 3.3.4, as the phone gate does): the
 * first tap on each button opens a short question (`ConsentDialog`, owner 09/10/2026 — it replaced a long
 * explanation card); "Cho phép" runs the pick, "Không" closes it and calls nothing. Once allowed, later taps on
 * THAT button go straight to Zalo for as long as this screen lives — the same scope the card had: per button,
 * per screen. `explaining` is the button whose question is open.
 */
export function usePhotoPicking(pick: PickScenePhotos | undefined, remaining: () => number, onPicked: (paths: string[]) => void) {
  const [explaining, setExplaining] = useState<ScenePhotoSource | null>(null);
  const explained = useRef(new Set<ScenePhotoSource>());
  const [picking, setPicking] = useState(false);
  const [failure, setFailure] = useState<PickFailure | null>(null);

  async function run(source: ScenePhotoSource) {
    if (pick === undefined || picking) return;
    setPicking(true);
    setFailure(null);
    const out = await pickOnce(pick, source, remaining());
    setPicking(false);
    setFailure(out.failure);
    if (out.paths.length > 0) onPicked(out.paths);
  }

  function ask(source: ScenePhotoSource) {
    if (explained.current.has(source)) void run(source);
    else setExplaining(source);
  }

  function confirm() {
    if (explaining === null) return;
    const source = explaining;
    explained.current.add(source);
    setExplaining(null);
    void run(source);
  }

  return { explaining, picking, failure, ask, confirm, cancel: () => setExplaining(null) };
}

export type PhotoPicking = ReturnType<typeof usePhotoPicking>;

/**
 * The two buttons, the open question over them (`explaining`), plus the pick's failure line (and Zalo's "Mã hỗ
 * trợ" under it). Pure apart from the callbacks — rendered by the tests without a DOM.
 */
export function ScenePhotoButtons(props: {
  count: number;
  explaining: ScenePhotoSource | null;
  picking: boolean;
  failure: PickFailure | null;
  onAsk: (source: ScenePhotoSource) => void;
  onConfirm: () => void;
  onCancel: () => void;
}) {
  const full = props.count >= MAX_SCENE_PHOTOS;
  return (
    <div className="xa-photo-buttons">
      {props.explaining !== null && (
        <ConsentDialog
          id="xa-photo-consent-question"
          question={props.explaining === "camera" ? SCENE_PHOTOS.camera_question : SCENE_PHOTOS.library_question}
          onAllow={props.onConfirm}
          onDeny={props.onCancel}
        />
      )}
      {full ? (
        <p className="xa-phu" role="status">
          {SCENE_PHOTOS.full(MAX_SCENE_PHOTOS)}
        </p>
      ) : (
        <>
          <button type="button" className="xa-nut xa-nut--phu" disabled={props.picking} onClick={() => props.onAsk("camera")}>
            <BieuTuong ten="camera-pin" co={20} />
            {SCENE_PHOTOS.take}
          </button>
          <button type="button" className="xa-nut xa-nut--phu" disabled={props.picking} onClick={() => props.onAsk("library")}>
            <BieuTuong ten="plus" co={20} />
            {SCENE_PHOTOS.pick}
          </button>
        </>
      )}
      {props.picking && (
        <p className="xa-phu" role="status">
          {SCENE_PHOTOS.picking}
        </p>
      )}
      {props.failure !== null && (
        <p className="xa-loi-o" role="status">
          {pickFailureText(props.failure)}
        </p>
      )}
      {props.failure !== null && zaloSupportCode(props.failure.zalo) !== null && (
        <p className="xa-phu">{zaloSupportCode(props.failure.zalo)}</p>
      )}
    </div>
  );
}

/** A photo picked on this screen and not yet sent. `key` is a local counter — never the path. */
export type PickedPhoto = { readonly key: string; readonly path: string };

/**
 * The send form's photo field: label, why, the picked photos (each with "Bỏ ảnh thứ n"), the count and the
 * buttons. A thumbnail the webview cannot show becomes words — the photo is still there and still goes.
 */
export function ScenePhotoField(props: {
  photos: readonly PickedPhoto[];
  onRemove: (key: string) => void;
  picking: PhotoPicking;
  /** This screen keeps a draft: say that photos are not in it. */
  hasDraft: boolean;
}) {
  const [noPreview, setNoPreview] = useState<ReadonlySet<string>>(() => new Set());
  const { photos, picking } = props;
  return (
    <section className="xa-photos" aria-labelledby="xa-photos-label">
      <p className="xa-nhan-o" id="xa-photos-label">
        {SCENE_PHOTOS.label}
      </p>
      <p className="xa-phu">{SCENE_PHOTOS.why}</p>
      {props.hasDraft && <p className="xa-phu">{SCENE_PHOTOS.not_in_draft}</p>}
      {photos.length > 0 && (
        <ul className="xa-photos__list">
          {photos.map((p, i) => (
            <li key={p.key} className="xa-photos__item">
              {noPreview.has(p.key) ? (
                <p className="xa-photos__no-preview">{SCENE_PHOTOS.no_preview(i + 1)}</p>
              ) : (
                <img
                  className="xa-photos__thumb"
                  src={p.path}
                  alt={SCENE_PHOTOS.photo_alt(i + 1)}
                  onError={() => setNoPreview((s) => new Set(s).add(p.key))}
                />
              )}
              <button type="button" className="xa-nut xa-nut--phu" onClick={() => props.onRemove(p.key)}>
                <BieuTuong ten="trash" co={20} />
                {SCENE_PHOTOS.remove(i + 1)}
              </button>
            </li>
          ))}
        </ul>
      )}
      <p className="xa-phu" role="status">
        {SCENE_PHOTOS.count(photos.length, MAX_SCENE_PHOTOS)}
      </p>
      <ScenePhotoButtons
        count={photos.length}
        explaining={picking.explaining}
        picking={picking.picking}
        failure={picking.failure}
        onAsk={picking.ask}
        onConfirm={picking.confirm}
        onCancel={picking.cancel}
      />
    </section>
  );
}

/* ═══════════════════════════════ UPLOADING — one photo, step by step ═══════════════════════════════ */

/** The bytes, and the type the BYTES say (`sniffScenePhotoType`) — the type declared to the server. */
export type PreparedPhoto = { readonly blob: Blob; readonly type: ScenePhotoType };

/**
 * Where one photo stands. A failure keeps the step to RESUME from, so "Tải lại" never repeats what succeeded:
 * a granted slot is not asked for again (it counts against the five for 15 minutes, `petition_photo.go`), and
 * a received upload is not posted again.
 */
export type PhotoStep =
  | { readonly stage: "read"; readonly path: string }
  | { readonly stage: "slot"; readonly file: PreparedPhoto }
  | { readonly stage: "upload"; readonly file: PreparedPhoto; readonly id: string; readonly form: PhotoUploadForm }
  | { readonly stage: "complete"; readonly file: PreparedPhoto; readonly id: string; readonly form: PhotoUploadForm };

/** Why a photo did not go — each is ONE sentence in `SCENE_PHOTOS.failures` saying what to do next. */
export type PhotoFailure = keyof typeof SCENE_PHOTOS.failures;

export type StepOutcome =
  | { readonly kind: "next"; readonly step: PhotoStep }
  | { readonly kind: "stored" }
  /** `retry` null: pressing again changes nothing (choose another photo, or the petition moved on). */
  | { readonly kind: "failed"; readonly failure: PhotoFailure; readonly retry: PhotoStep | null };

/** Failures for which "Tải lại" would meet the same answer. */
const FINAL: ReadonlySet<PhotoFailure> = new Set<PhotoFailure>([
  "not-readable",
  "not-a-photo",
  "not-accepted",
  "not-found",
  "petition-moved",
  "limit",
  "rejected",
]);

/** A refusal `code` → its failure. An unknown code is a server fault, never "fix your photo". */
export function refusalFailure(status: number, code: string | null): PhotoFailure | "not-received" {
  switch (code) {
    case PHOTO_ERROR.invalid:
      return "not-accepted";
    case PHOTO_ERROR.notFound:
      return "not-found";
    case PHOTO_ERROR.petitionState:
      return "petition-moved";
    case PHOTO_ERROR.limit:
      return "limit";
    case PHOTO_ERROR.photoState:
    case PHOTO_ERROR.expired:
      return "expired";
    case PHOTO_ERROR.notReceived:
      return "not-received";
    case PHOTO_ERROR.rejected:
      return "rejected";
    case PHOTO_ERROR.storageNotConfigured:
      return "not-configured";
    case PHOTO_ERROR.scanUnavailable:
    case PHOTO_ERROR.limitsUnavailable:
      return "busy";
    default:
      return status === 404 ? "not-found" : "server";
  }
}

/** Every non-`xong` branch of a photo route → its failure. PURE. */
export function callFailure(kq: Exclude<PhotoCallResult<unknown>, { kieu: "xong" }>): PhotoFailure | "not-received" {
  switch (kq.kieu) {
    case "chua-co-phien":
    case "het-phien":
    case "can-xac-thuc-so":
      return "session";
    case "chua-cau-hinh":
      return "closed";
    case "khong-thay":
      return "not-found";
    case "rate-limited":
      return "rate-limited";
    case "loi-mang":
      return "network";
    case "refused":
      return refusalFailure(kq.status, kq.code);
    default:
      return "server";
  }
}

/** A failure at `step`, with the step to resume from (`null` when retrying cannot help). */
function failedAt(failure: PhotoFailure, step: PhotoStep): StepOutcome {
  return { kind: "failed", failure, retry: FINAL.has(failure) ? null : step };
}

/** The slot request's answer → the next step. A failed slot is retried as a NEW slot (new key). PURE. */
export function slotOutcome(kq: PhotoCallResult<PhotoSlot>, file: PreparedPhoto): StepOutcome {
  if (kq.kieu === "xong") {
    return { kind: "next", step: { stage: "upload", file, id: kq.gia_tri.photo.id, form: kq.gia_tri.upload } };
  }
  const f = callFailure(kq);
  return failedAt(f === "not-received" ? "server" : f, { stage: "slot", file });
}

/**
 * The store's answer → the next step. A refusal (4xx) means the form is spent or the store refused it: a
 * NEW slot. A dropped line or a 5xx: post again with the same form while it lives. PURE.
 */
export function storageOutcome(res: StorageUploadResult, step: Extract<PhotoStep, { stage: "upload" }>): StepOutcome {
  switch (res.kieu) {
    case "xong":
      return { kind: "next", step: { stage: "complete", file: step.file, id: step.id, form: step.form } };
    case "tu-choi":
      return { kind: "failed", failure: "storage-refused", retry: { stage: "slot", file: step.file } };
    case "loi-mang":
      return { kind: "failed", failure: "network", retry: step };
    default:
      return { kind: "failed", failure: "server", retry: step };
  }
}

/**
 * The completion's answer → stored, or where to resume. `expired` (the form died, or the photo is no longer
 * pending) resumes at a NEW slot; `not-received` posts the bytes again; the rest retry the completion. PURE.
 */
export function completionOutcome(
  kq: PhotoCallResult<ScenePhotoOut>,
  step: Extract<PhotoStep, { stage: "complete" }>,
): StepOutcome {
  if (kq.kieu === "xong") {
    return kq.gia_tri.status === "stored" ? { kind: "stored" } : { kind: "failed", failure: "server", retry: step };
  }
  const f = callFailure(kq);
  if (f === "not-received") {
    return { kind: "failed", failure: "server", retry: { stage: "upload", file: step.file, id: step.id, form: step.form } };
  }
  if (f === "expired") return { kind: "failed", failure: "expired", retry: { stage: "slot", file: step.file } };
  return failedAt(f, step);
}

/** Whether the form is still alive at `now` (a margin, so a post does not start in its last seconds). */
export function formAlive(form: PhotoUploadForm, now: number): boolean {
  const ends = Date.parse(form.expires_at);
  return Number.isFinite(ends) && ends - 10_000 > now;
}

/** Read the picked file and learn its type from its bytes. */
export async function preparePhoto(path: string): Promise<PreparedPhoto | "not-readable" | "not-a-photo"> {
  const blob = await readPickedPhoto(path);
  if (blob === null) return "not-readable";
  let head: Uint8Array;
  try {
    head = new Uint8Array(await blob.slice(0, 12).arrayBuffer());
  } catch {
    return "not-readable";
  }
  const type = sniffScenePhotoType(head);
  return type === null ? "not-a-photo" : { blob, type };
}

/** ONE stage of one photo. */
export async function runStep(code: string, step: PhotoStep, now: () => number = Date.now): Promise<StepOutcome> {
  switch (step.stage) {
    case "read": {
      const file = await preparePhoto(step.path);
      if (file === "not-readable" || file === "not-a-photo") return { kind: "failed", failure: file, retry: null };
      return { kind: "next", step: { stage: "slot", file } };
    }
    case "slot": {
      // A NEW key for every slot request — see `requestScenePhotoSlot`.
      const key = taoLanGui("").khoa;
      return slotOutcome(
        await requestScenePhotoSlot(code, photoUploadBody(step.file.type, step.file.blob.size), key),
        step.file,
      );
    }
    case "upload":
      if (!formAlive(step.form, now())) return { kind: "next", step: { stage: "slot", file: step.file } };
      return storageOutcome(await postPhotoToStorage(step.form, step.file.blob, step.file.type), step);
    case "complete":
      return completionOutcome(await completeScenePhoto(code, step.id), step);
  }
}

/** The most stages one attempt may run: read · slot · upload · complete, plus one fresh slot for a dead form. */
const MAX_STAGES = 6;

/** Run one photo from `step` until it is stored or a stage fails. Never throws. */
export async function attachScenePhoto(
  code: string,
  step: PhotoStep,
  now: () => number = Date.now,
): Promise<Exclude<StepOutcome, { kind: "next" }>> {
  let current = step;
  for (let i = 0; i < MAX_STAGES; i++) {
    let out: StepOutcome;
    try {
      out = await runStep(code, current, now);
    } catch {
      return { kind: "failed", failure: "server", retry: current };
    }
    if (out.kind !== "next") return out;
    current = out.step;
  }
  return { kind: "failed", failure: "server", retry: current };
}

/** One photo in the upload list. `index` is its number as the citizen sees it ("Ảnh thứ 2"). */
export type UploadJob = {
  readonly key: string;
  readonly index: number;
  readonly status: "waiting" | "sending" | "stored" | "failed";
  readonly failure?: PhotoFailure;
  readonly canRetry?: boolean;
};

/**
 * Sends photos ONE AT A TIME to petition `code` — after the petition exists. `start` queues; `retry` resumes a
 * failed photo from where it stopped. A 401/403 hands the act to the gate (`onSessionLost`) and resumes after
 * it. `onStored` runs after each stored photo (the detail reloads its list).
 *
 * The step a job resumes from lives in `steps` (memory only — it holds the bytes and the upload form). An entry
 * is overwritten, never needed again once its job is stored or finally refused (`canRetry` false), and goes
 * with the screen.
 */
export function usePhotoUploads(onSessionLost: OnSessionLost, onStored?: () => void) {
  const [jobs, setJobs] = useState<readonly UploadJob[]>([]);
  const jobsRef = useRef<readonly UploadJob[]>([]);
  const codeRef = useRef("");
  const steps = useRef(new Map<string, PhotoStep>());
  const running = useRef(false);
  const counter = useRef(0);

  function update(next: readonly UploadJob[]) {
    jobsRef.current = next;
    setJobs(next);
  }
  function patch(key: string, change: Partial<UploadJob>) {
    update(jobsRef.current.map((j) => (j.key === key ? { ...j, ...change } : j)));
  }

  async function drain() {
    if (running.current) return;
    running.current = true;
    try {
      for (;;) {
        const job = jobsRef.current.find((j) => j.status === "waiting");
        if (job === undefined) break;
        const step = steps.current.get(job.key);
        if (step === undefined) {
          patch(job.key, { status: "failed", failure: "server", canRetry: false });
          continue;
        }
        patch(job.key, { status: "sending", failure: undefined });
        const out = await attachScenePhoto(codeRef.current, step);
        if (out.kind === "stored") {
          patch(job.key, { status: "stored", failure: undefined, canRetry: false });
          onStored?.();
          continue;
        }
        if (out.retry !== null) steps.current.set(job.key, out.retry);
        patch(job.key, { status: "failed", failure: out.failure, canRetry: out.retry !== null });
        if (out.failure === "session") {
          // The session is gone for every photo still waiting: stop, and resume them all after the gate.
          onSessionLost(() => retry(job.key));
          break;
        }
      }
    } finally {
      running.current = false;
    }
  }

  function start(code: string, paths: readonly string[]) {
    if (paths.length === 0) return;
    codeRef.current = code;
    const base = jobsRef.current.length;
    const added = paths.map((path, i): UploadJob => {
      const key = `photo-${counter.current++}`;
      steps.current.set(key, { stage: "read", path });
      return { key, index: base + i + 1, status: "waiting" };
    });
    update([...jobsRef.current, ...added]);
    void drain();
  }

  function retry(key: string) {
    const job = jobsRef.current.find((j) => j.key === key);
    if (job === undefined || job.status !== "failed" || job.canRetry !== true) return;
    patch(key, { status: "waiting", failure: undefined });
    void drain();
  }

  return { jobs, start, retry };
}

/**
 * Photos neither stored nor finally refused — they still hold one of the five places, so the picker offers
 * fewer. PURE.
 */
export function photosInFlight(jobs: readonly UploadJob[]): number {
  return jobs.filter((j) => j.status === "waiting" || j.status === "sending" || (j.status === "failed" && j.canRetry === true))
    .length;
}

/** The headline of the upload list. PURE. */
export function uploadSummary(jobs: readonly UploadJob[]): string {
  const total = jobs.length;
  const stored = jobs.filter((j) => j.status === "stored").length;
  const sending = jobs.find((j) => j.status === "sending") ?? jobs.find((j) => j.status === "waiting");
  if (sending !== undefined) return SCENE_PHOTOS.sending(sending.index, total);
  return stored === total ? SCENE_PHOTOS.all_sent(total) : SCENE_PHOTOS.some_failed(stored, total);
}

/** One row's words: the photo's number, and its state in words — never by the icon or colour alone. */
export function uploadRowText(job: UploadJob): string {
  switch (job.status) {
    case "waiting":
      return SCENE_PHOTOS.row_waiting(job.index);
    case "sending":
      return SCENE_PHOTOS.row_sending(job.index);
    case "stored":
      return SCENE_PHOTOS.row_sent(job.index);
    case "failed":
      return SCENE_PHOTOS.row_failed(job.index, SCENE_PHOTOS.failures[job.failure ?? "server"]);
  }
}

/** The upload list: the headline, then one row per photo, with "Tải lại ảnh thứ n" where it can help. */
export function ScenePhotoUploads(props: {
  jobs: readonly UploadJob[];
  onRetry: (key: string) => void;
  /** Right after "Gửi phản ánh": also say the petition itself is recorded whatever happens to the photos. */
  afterSend?: boolean;
}) {
  if (props.jobs.length === 0) return null;
  return (
    <section className="xa-the xa-the--dem xa-khoi xa-photo-uploads" aria-labelledby="xa-photo-uploads-title">
      <h3 className="xa-dau-khoi__tieu-de" id="xa-photo-uploads-title">
        {SCENE_PHOTOS.uploads_title}
      </h3>
      <p role="status">{uploadSummary(props.jobs)}</p>
      {props.afterSend && <p className="xa-phu">{SCENE_PHOTOS.petition_kept}</p>}
      <ul className="xa-photo-uploads__list">
        {props.jobs.map((j) => (
          <li key={j.key} className={`xa-photo-uploads__row xa-photo-uploads__row--${j.status}`}>
            <BieuTuong ten={j.status === "stored" ? "check-circle" : j.status === "failed" ? "alert" : "clock"} co={20} />
            <span>{uploadRowText(j)}</span>
            {j.status === "failed" && j.canRetry === true && (
              <button type="button" className="xa-nut xa-nut--phu" onClick={() => props.onRetry(j.key)}>
                <BieuTuong ten="refresh" co={20} />
                {SCENE_PHOTOS.retry(j.index)}
              </button>
            )}
          </li>
        ))}
      </ul>
    </section>
  );
}

/* ═══════════════════════════════ THE CITIZEN'S OWN PHOTOS — petition detail ═══════════════════════════════ */

export type OwnPhotoList =
  | { readonly kind: "loading" }
  | { readonly kind: "failed" }
  | { readonly kind: "ready"; readonly items: readonly ScenePhotoLink[] };

/**
 * The list call → what the block shows. A session problem is shown as "failed" with "Thử lại" rather than
 * opening the phone gate a second time: the petition above was just read with this same session. PURE.
 */
export function ownPhotoListOutcome(kq: PhotoCallResult<readonly ScenePhotoLink[]>): OwnPhotoList {
  return kq.kieu === "xong" ? { kind: "ready", items: kq.gia_tri } : { kind: "failed" };
}

/**
 * When to fetch the links again: a little before the FIRST one expires, never sooner than 5 s, never later
 * than 15 minutes (the server's ceiling). An unreadable instant counts as "now". PURE.
 */
export function msUntilRefresh(items: readonly ScenePhotoLink[], now: number): number {
  const ends = items.map((i) => Date.parse(i.url_expires_at)).map((t) => (Number.isFinite(t) ? t : now));
  const first = Math.min(...ends);
  return Math.min(Math.max(first - now - 30_000, 5_000), 15 * 60_000);
}

/** A list route of short-lived photo links: the scene list, or the commune's "sau xử lý" list. */
export type PhotoLinkLoader = (code: string) => Promise<PhotoCallResult<readonly ScenePhotoLink[]>>;

/**
 * THE ONE PLACE a list of read links is fetched, refetched before the first link expires (`msUntilRefresh`),
 * and refetched once when an image fails to load — shared by the citizen's own photos and the commune's "sau
 * xử lý" photos, so the two lists can never refresh by two different rules. The links live in this state only.
 */
export function usePhotoLinks(load: PhotoLinkLoader, code: string) {
  const [list, setList] = useState<OwnPhotoList>({ kind: "loading" });
  const [round, setRound] = useState(0);
  /** The round an image error already reloaded — one reload per load, never a loop on a broken link. */
  const reloadedAt = useRef(-1);
  const reload = () => setRound((n) => n + 1);

  useEffect(() => {
    let alive = true;
    void load(code).then((kq) => {
      if (alive) setList(ownPhotoListOutcome(kq));
    });
    return () => {
      alive = false;
    };
    // `load` is a module function, fixed for the block's life.
  }, [code, round]);

  useEffect(() => {
    if (list.kind !== "ready" || list.items.length === 0) return;
    const timer = setTimeout(reload, msUntilRefresh(list.items, Date.now()));
    return () => clearTimeout(timer);
  }, [list]);

  const onImageError = () => {
    if (reloadedAt.current === round) return;
    reloadedAt.current = round;
    reload();
  };

  return { list, reload, onImageError };
}

/**
 * The photos of a petition the citizen opened, "Trước khi xử lý" then "Sau khi xử lý" (ADR 0047 row "THAY
 * G8"), STACKED: the detail is one phone column wide, and each block says which it is in WORDS, never by its
 * position. The "after" block appears only when the commune's list is non-empty (ADR 0047:254 (12)); then, and
 * only then, the citizen's own block is titled "Trước khi xử lý" — without an "after" there is no "before".
 */
export function PetitionPhotos(props: {
  code: string;
  status: string;
  pick?: PickScenePhotos;
  onSessionLost: OnSessionLost;
}) {
  const after = usePhotoLinks(listVerificationPhotos, props.code);
  const hasAfter = after.list.kind === "ready" && after.list.items.length > 0;
  return (
    <>
      <OwnScenePhotos {...props} title={hasAfter ? VERIFICATION_PHOTOS.before_title : undefined} />
      <VerificationPhotosView
        list={after.list}
        status={props.status}
        onRetry={after.reload}
        onImageError={after.onImageError}
      />
    </>
  );
}

/**
 * The statuses at which the server shows the commune's "sau xử lý" photos (ADR 0047 row "THAY G8", (b)). Used
 * ONLY to decide whether a FAILED load is worth a sentence: elsewhere the list is empty by rule, and "chưa tải
 * được ảnh sau khi xử lý" on a petition nobody has handled yet would promise photos that do not exist. What is
 * SHOWN always follows the server's list, never this set.
 */
const AFTER_PHOTO_STATUSES: ReadonlySet<string> = new Set(["cho-dan-xac-nhan", "da-dong"]);

/**
 * The "Sau khi xử lý" block without its effects — what the tests render. Nothing while loading, nothing for an
 * empty list (no empty frame), the photos when there are some, and a retry line when the load failed at a
 * status where the photos can exist. A session problem is that retry line too, not a second phone gate: the
 * petition above was just read with this same session (as `ownPhotoListOutcome`).
 */
export function VerificationPhotosView(props: {
  list: OwnPhotoList;
  status: string;
  onRetry: () => void;
  onImageError: () => void;
}) {
  const { list } = props;
  if (list.kind === "loading") return null;
  if (list.kind === "ready" && list.items.length === 0) return null;
  if (list.kind === "failed" && !AFTER_PHOTO_STATUSES.has(props.status)) return null;
  return (
    <section className="xa-the xa-the--dem xa-khoi xa-photos" aria-labelledby="xa-after-photos-title">
      <h2 className="xa-dau-khoi__tieu-de" id="xa-after-photos-title">
        {VERIFICATION_PHOTOS.after_title}
      </h2>
      {list.kind === "failed" ? (
        <>
          <p className="xa-loi-o" role="status">
            {VERIFICATION_PHOTOS.failed}
          </p>
          <button type="button" className="xa-nut xa-nut--phu" onClick={props.onRetry}>
            <BieuTuong ten="refresh" co={20} />
            {VERIFICATION_PHOTOS.retry}
          </button>
        </>
      ) : (
        <ul className="xa-photos__list">
          {list.items.map((p, i) => (
            <li key={p.id} className="xa-photos__item">
              <img
                className="xa-photos__thumb"
                src={p.url}
                alt={VERIFICATION_PHOTOS.photo_alt(i + 1)}
                onError={props.onImageError}
              />
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}

/**
 * The photos block of a petition the citizen opened: their own photos when there ARE some (decision 12 — no
 * empty frame), and "Thêm ảnh hiện trường" while the petition is `da-tiep-nhan` and the app has the picker.
 * The links are fetched, shown and refetched — never kept anywhere but this state.
 */
export function OwnScenePhotos(props: {
  code: string;
  status: string;
  pick?: PickScenePhotos;
  onSessionLost: OnSessionLost;
  /** "Trước khi xử lý" when the commune's "after" photos are shown under it (`PetitionPhotos`). */
  title?: string;
}) {
  const { code } = props;
  const { list, reload, onImageError } = usePhotoLinks(listScenePhotos, code);
  const uploads = usePhotoUploads(props.onSessionLost, reload);
  const canAdd = props.pick !== undefined && props.status === "da-tiep-nhan";
  const stored = list.kind === "ready" ? list.items.length : 0;
  const remaining = () => MAX_SCENE_PHOTOS - stored - photosInFlight(uploads.jobs);
  const picking = usePhotoPicking(props.pick, remaining, (paths) => uploads.start(code, paths));

  return (
    <OwnScenePhotosView
      list={list}
      title={props.title}
      canAdd={canAdd}
      count={stored + photosInFlight(uploads.jobs)}
      picking={picking}
      jobs={uploads.jobs}
      onRetryUpload={uploads.retry}
      onRetryList={reload}
      onImageError={onImageError}
    />
  );
}

/** The block without its effects — what the tests render. */
export function OwnScenePhotosView(props: {
  list: OwnPhotoList;
  /** Overrides the heading when there are photos (`OwnScenePhotos.title`). */
  title?: string;
  canAdd: boolean;
  count: number;
  picking: PhotoPicking;
  jobs: readonly UploadJob[];
  onRetryUpload: (key: string) => void;
  onRetryList: () => void;
  onImageError: () => void;
}) {
  const { list } = props;
  const items = list.kind === "ready" ? list.items : [];
  // Nothing to show and nothing to do: no block at all (decision 12 — no empty photo frame).
  if (!props.canAdd && list.kind === "loading") return null;
  if (!props.canAdd && list.kind === "ready" && items.length === 0) return null;
  return (
    <section className="xa-the xa-the--dem xa-khoi xa-photos" aria-labelledby="xa-own-photos-title">
      <h2 className="xa-dau-khoi__tieu-de" id="xa-own-photos-title">
        {items.length > 0 ? (props.title ?? SCENE_PHOTOS.own_title) : SCENE_PHOTOS.add_title}
      </h2>
      {list.kind === "loading" && (
        <p className="xa-phu" role="status">
          {SCENE_PHOTOS.own_loading}
        </p>
      )}
      {list.kind === "failed" && (
        <>
          <p className="xa-loi-o" role="status">
            {SCENE_PHOTOS.own_failed}
          </p>
          <button type="button" className="xa-nut xa-nut--phu" onClick={props.onRetryList}>
            <BieuTuong ten="refresh" co={20} />
            {SCENE_PHOTOS.own_retry}
          </button>
        </>
      )}
      {items.length > 0 && (
        <ul className="xa-photos__list">
          {items.map((p, i) => (
            <li key={p.id} className="xa-photos__item">
              <img className="xa-photos__thumb" src={p.url} alt={SCENE_PHOTOS.photo_alt(i + 1)} onError={props.onImageError} />
            </li>
          ))}
        </ul>
      )}
      {props.canAdd && list.kind === "ready" && (
        <>
          <p className="xa-phu">{SCENE_PHOTOS.add_why}</p>
          <ScenePhotoButtons
            count={props.count}
            explaining={props.picking.explaining}
            picking={props.picking.picking}
            failure={props.picking.failure}
            onAsk={props.picking.ask}
            onConfirm={props.picking.confirm}
            onCancel={props.picking.cancel}
          />
        </>
      )}
      <ScenePhotoUploads jobs={props.jobs} onRetry={props.onRetryUpload} />
    </section>
  );
}
