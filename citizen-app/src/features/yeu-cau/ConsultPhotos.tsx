/**
 * THE PHOTO FIELD OF "TƯ VẤN VÀ BÁO GIÁ", AND THE PHOTOS' STATUS AFTER SENDING (owner 12/10/2026).
 *
 * This is the ViHAT intro's own block — it imports nothing of `cong-dan/**` (`ranh-gioi-hai-nua.test.ts` forbids a
 * cross-half import) and names no commune. The commune half has its own photo field (`cong-dan/man/scene-photos.tsx`);
 * the shape is the same on purpose — a 3-tile grid, an "X" per photo, a short question before Zalo's dialog — but
 * the words are ViHAT's and the photos go to ViHAT's server.
 *
 * ⚠ THE QUESTION IS ASKED BEFORE ZALO'S OWN DIALOG (Mini App policy 3.3.4): the first tap on "Chụp ảnh" or "Chọn
 *   ảnh" shows the purpose and "Cho phép" / "Không"; only "Cho phép" reaches Zalo. Once allowed, later taps on THAT
 *   button go straight to Zalo for as long as the screen lives — per button, per screen opening. "Không" calls
 *   nothing and the request can still go without photos.
 *
 * ⚠ THE QUESTION IS INLINE, NOT A FIXED OVERLAY: a `position: fixed` box inside the intro's scrolling frame is painted
 *   in the scroller's stacking context in WebKit webviews — the commune half met exactly that (its consent dialog
 *   opened BEHIND a footer, customer sheet 12/10/2026 row 23) and needed a portal. In the flow, right under the tiles
 *   that asked it, there is no stacking to get wrong. The focus moves onto "Không", the answer that grants nothing.
 */
import { useEffect, useRef, useState } from "react";

import { REQUEST_PHOTO_MAX } from "../../api/hop-dong-yeu-cau";
import { LOI_MO_NGOAI } from "../tinh-nang/noi-dung"; // vi-name-ok: existing export

import {
  type ConsultPhoto,
  type PhotoPicker,
  type PhotoRejection,
  type PhotoSource,
  type PhotoUpload,
  type PickFailure,
  pickConsultPhotos,
} from "./consult-photos";
import { CONSULT_PHOTOS } from "./noi-dung";

/** The sentence for a pick that brought nothing. */
export function pickFailureText(source: PhotoSource, failure: PickFailure): string {
  if (failure === "refused") return source === "camera" ? CONSULT_PHOTOS.camera_refused : CONSULT_PHOTOS.library_refused;
  if (failure === "outside-zalo") return LOI_MO_NGOAI;
  return CONSULT_PHOTOS.pick_failed;
}

/** What the field says under the tiles after a pick: one line per thing that went wrong, deduplicated. PURE. */
export function pickMessages(source: PhotoSource, failure: PickFailure | null, rejected: readonly PhotoRejection[]): string[] {
  const lines = failure === null ? [] : [pickFailureText(source, failure)];
  for (const r of new Set(rejected)) lines.push(CONSULT_PHOTOS.rejected[r]);
  return lines;
}

/**
 * "Which button still owes its question" — per button, for one screen opening. A plain object (not React state) so
 * the rule is testable without a DOM: a first tap asks, "Cho phép" remembers THAT button only, "Không" remembers
 * nothing.
 */
export function createConsentGate() {
  const allowed = new Set<PhotoSource>();
  return {
    needsQuestion: (source: PhotoSource) => !allowed.has(source),
    allow: (source: PhotoSource) => {
      allowed.add(source);
    },
  };
}

/**
 * The picking state of one screen opening: which question is open, which buttons were already allowed, the pick in
 * flight, and what to say. `photos` / `setPhotos` are the form's.
 */
export function useConsultPhotoPicking(
  picker: PhotoPicker,
  photos: readonly ConsultPhoto[],
  setPhotos: (next: readonly ConsultPhoto[]) => void,
) {
  const [asking, setAsking] = useState<PhotoSource | null>(null);
  const gate = useRef(createConsentGate());
  const [picking, setPicking] = useState(false);
  const [messages, setMessages] = useState<readonly string[]>([]);
  const counter = useRef(0);
  // The latest list, for a pick that resolves after the person removed a photo.
  const latest = useRef(photos);
  latest.current = photos;

  async function run(source: PhotoSource) {
    if (picking) return;
    setPicking(true);
    setMessages([]);
    const out = await pickConsultPhotos(picker, source, REQUEST_PHOTO_MAX - latest.current.length, () => `anh-${counter.current++}`);
    setPicking(false);
    setMessages(pickMessages(source, out.failure, out.rejected));
    if (out.photos.length > 0) setPhotos([...latest.current, ...out.photos].slice(0, REQUEST_PHOTO_MAX));
  }

  return {
    asking,
    picking,
    messages,
    ask(source: PhotoSource) {
      if (gate.current.needsQuestion(source)) setAsking(source);
      else void run(source);
    },
    allow() {
      if (asking === null) return;
      const source = asking;
      gate.current.allow(source);
      setAsking(null);
      void run(source);
    },
    deny() {
      setAsking(null);
    },
  };
}

/** The inline question: the purpose, then "Cho phép" / "Không". */
export function ConsultPhotoQuestion({
  source,
  onAllow,
  onDeny,
}: {
  source: PhotoSource;
  onAllow: () => void;
  onDeny: () => void;
}) {
  const denyRef = useRef<HTMLButtonElement>(null);
  useEffect(() => {
    denyRef.current?.focus();
  }, [source]);
  const id = "tv-anh-cau-hoi";
  return (
    <div className="vh-photo-question" role="group" aria-labelledby={id}>
      <p className="vh-photo-question__text" id={id}>
        {source === "camera" ? CONSULT_PHOTOS.camera_question : CONSULT_PHOTOS.library_question}
      </p>
      <p className="vh-photo-question__why">{CONSULT_PHOTOS.question_why}</p>
      <button type="button" className="tn__nut" onClick={onAllow}>
        {CONSULT_PHOTOS.allow}
      </button>
      <button type="button" className="tn__nut-phu" ref={denyRef} onClick={onDeny}>
        {CONSULT_PHOTOS.deny}
      </button>
    </div>
  );
}

/**
 * The field, PURE apart from the callbacks — the tests render every state without a DOM.
 *
 * A 3-column grid: "Chụp ảnh" and "Chọn ảnh" tiles while there is room, then each photo with its "X" (a 48px target).
 * A thumbnail the webview cannot show becomes words: the photo is still there and still goes.
 */
export function ConsultPhotoField(props: {
  photos: readonly ConsultPhoto[];
  asking: PhotoSource | null;
  picking: boolean;
  messages: readonly string[];
  onAsk: (source: PhotoSource) => void;
  onAllow: () => void;
  onDeny: () => void;
  onRemove: (key: string) => void;
}) {
  const [noPreview, setNoPreview] = useState<ReadonlySet<string>>(() => new Set());
  const room = props.photos.length < REQUEST_PHOTO_MAX;
  const busy = props.picking || props.asking !== null;
  return (
    <section className="vh-photos" aria-labelledby="tv-anh-tieu-de">
      <h2 className="section-title" id="tv-anh-tieu-de">
        {CONSULT_PHOTOS.title}
      </h2>
      <p className="screen-lead">{CONSULT_PHOTOS.hint}</p>
      <ul className="vh-photo-grid">
        {room && (
          <>
            <li>
              <button type="button" className="vh-photo-tile vh-photo-tile--add" disabled={busy} onClick={() => props.onAsk("camera")}>
                {CONSULT_PHOTOS.take}
              </button>
            </li>
            <li>
              <button type="button" className="vh-photo-tile vh-photo-tile--add" disabled={busy} onClick={() => props.onAsk("library")}>
                {CONSULT_PHOTOS.pick}
              </button>
            </li>
          </>
        )}
        {props.photos.map((p, i) => (
          <li key={p.key} className="vh-photo-tile vh-photo-tile--photo">
            {noPreview.has(p.key) ? (
              <p className="vh-photo-tile__words">{CONSULT_PHOTOS.no_preview(i + 1)}</p>
            ) : (
              <img
                className="vh-photo-tile__image"
                src={p.path}
                alt={CONSULT_PHOTOS.photo_alt(i + 1)}
                onError={() => setNoPreview((s) => new Set(s).add(p.key))}
              />
            )}
            <button
              type="button"
              className="vh-photo-tile__remove"
              aria-label={CONSULT_PHOTOS.remove(i + 1)}
              onClick={() => props.onRemove(p.key)}
            >
              <span className="vh-photo-tile__x" aria-hidden="true">
                ✕
              </span>
            </button>
          </li>
        ))}
      </ul>
      {!room && <p className="screen-lead">{CONSULT_PHOTOS.full}</p>}
      {props.asking !== null && <ConsultPhotoQuestion source={props.asking} onAllow={props.onAllow} onDeny={props.onDeny} />}
      <div role="status">
        {props.picking && <p className="screen-lead">{CONSULT_PHOTOS.picking}</p>}
        {props.messages.map((m) => (
          <p key={m} className="tn__loi vh-photo-message">
            {m}
          </p>
        ))}
      </div>
    </section>
  );
}

/** One failed row's words: the photo's number and what to do next — never a code. PURE. */
export function uploadRowText(job: PhotoUpload): string {
  return CONSULT_PHOTOS.row_failed(job.index, CONSULT_PHOTOS.failures[job.failure ?? "server"]);
}

/**
 * The photos' state on the success screen, PURE apart from `onRetry`:
 *
 *   no photo picked                  nothing
 *   one still waiting or going up    "Đang gửi ảnh n/total tới chuyên viên…"
 *   all stored                       how many arrived
 *   any failed                       how many arrived, that the request stands, then each failure with its reason
 *                                    and "Tải lại ảnh n" when a second press can help
 */
export function ConsultPhotoStatus(props: { jobs: readonly PhotoUpload[]; onRetry: (key: string) => void }) {
  const { jobs } = props;
  if (jobs.length === 0) return null;
  const going = jobs.find((j) => j.status === "sending") ?? jobs.find((j) => j.status === "waiting");
  if (going !== undefined) {
    return (
      <p className="card__body" role="status">
        {CONSULT_PHOTOS.sending(going.index, jobs.length)}
      </p>
    );
  }
  const stored = jobs.filter((j) => j.status === "stored").length;
  const failed = jobs.filter((j) => j.status === "failed");
  if (failed.length === 0) {
    return (
      <p className="card__body" role="status">
        {CONSULT_PHOTOS.all_arrived(stored)}
      </p>
    );
  }
  return (
    <div className="vh-photo-status" role="status">
      <p className="card__body">{CONSULT_PHOTOS.some_arrived(stored, jobs.length)}</p>
      <ul className="vh-photo-status__list">
        {failed.map((j) => (
          <li key={j.key}>
            <p className="tn__loi">{uploadRowText(j)}</p>
            {j.canRetry === true && (
              <button type="button" className="tn__nut-phu" onClick={() => props.onRetry(j.key)}>
                {CONSULT_PHOTOS.retry(j.index)}
              </button>
            )}
          </li>
        ))}
      </ul>
    </div>
  );
}
