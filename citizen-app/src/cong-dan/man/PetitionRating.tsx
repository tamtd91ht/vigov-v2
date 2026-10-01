/**
 * THE CITIZEN RATES THEIR OWN PETITION — over the network (ADR 0050 point 2), on the petition opened in
 * "Tra cứu phiếu" (reached from the lookup box and from "Phản ánh của tôi" alike, `KenhCongDan.tsx`).
 *
 *   · Shown when the server says the petition may be rated (`isRateable`: `da-xu-ly`, `cho-dan-xac-nhan`),
 *     or when a rating already exists — then "Bạn đã đánh giá n sao." in words, stars read-only.
 *   · Already rated and still rateable (the server lets a later rating REPLACE the earlier one — after a
 *     reopen and a second resolve, most of all): the picker stays closed behind "Đánh giá lại", so a
 *     citizen who just rated is not shown an empty form as if nothing happened.
 *   · After a 200 the screen RE-RENDERS FROM THE BODY (`onRated`) — status included: the server may have
 *     reopened the petition, and the card must say so rather than what this block guessed.
 *
 * NO THRESHOLD, anywhere (ADR 0050: "Giao diện dân không nói ngưỡng"). This file never compares a star
 * count with anything; "reopened" is read off the status the server returned.
 *
 * ONE USER ACT = ONE `Idempotency-Key` (`api/lan-gui.ts`): created on "Gửi đánh giá", reused by "Gửi lại"
 * after a lost answer, dropped the moment the stars or the comment change — a different rating is a
 * different act.
 *
 * ⚠ THE COMMENT IS CITIZEN FREE TEXT (rule 3): it lives in `useState` only, goes into exactly one request
 * body, and is never logged, stored on the phone, or put in a URL.
 */
import { type ReactNode, useState } from "react";

import { type KetQuaGoi, ratePetition } from "../api/goi-vigov"; // vi-name-ok: existing export, not renamed (rule 12 #3)
import {
  isRateable,
  type PhieuCuaToi, // vi-name-ok: existing contract type, not renamed (rule 12 #3)
  RATING_COMMENT_MAX_LEN,
  RATING_MAX_STARS,
  RATING_MIN_STARS,
  ratingBody,
} from "../api/hop-dong-phan-anh";
import { type LanGui, taoLanGui } from "../api/lan-gui"; // vi-name-ok: existing export, not renamed (rule 12 #3)
import type { ReopenWithPhone } from "../api/mo-phien-vigov";

import { RATING, RATING_ERROR } from "./noi-dung";
import { ONhapDoan } from "./o-nhap"; // vi-name-ok: existing export, not renamed (rule 12 #3)
import { PhoneVerificationPanel, usePhoneVerification } from "./phone-verification";
import { StarPicker } from "./star-picker";

export type RatingErrorBranch = keyof typeof RATING_ERROR;

/** What one call's result means for this block. PURE, so each branch is tested without a DOM. */
export type RatingOutcome =
  | { readonly kind: "done"; readonly petition: PhieuCuaToi }
  | { readonly kind: "phone" }
  | { readonly kind: "error"; readonly branch: RatingErrorBranch };

export function ratingOutcome(kq: KetQuaGoi): RatingOutcome {
  switch (kq.kieu) {
    case "xong":
      return { kind: "done", petition: kq.phieu };
    case "can-xac-thuc-so":
      return { kind: "phone" };
    // No session at all cannot normally happen here (the petition was read with one); if it does, the
    // next step is the same as an expired one.
    case "chua-co-phien":
    case "het-phien":
      return { kind: "error", branch: "session-expired" };
    case "khong-thay":
      return { kind: "error", branch: "not-found" };
    case "dang-xu-ly-truoc":
      return { kind: "error", branch: "state-changed" };
    case "khong-hop-le":
      return { kind: "error", branch: "invalid" };
    case "loi-mang":
      return { kind: "error", branch: "network" };
    default:
      return { kind: "error", branch: "server-fault" };
  }
}

/**
 * The act "Gửi đánh giá" sends. PURE — the key rule in one testable place:
 *   · an act already in progress (`current`) is sent AGAIN, same key, same body — the stars/comment cannot
 *     have changed since, because every change drops it (`changed()` below);
 *   · otherwise a NEW act: body from `ratingBody`, fresh key;
 *   · stars outside 1..5 → `null`, nothing is sent (the button is disabled anyway);
 *   · no CSPRNG → `"no-key"`: refuse rather than send without a key (`lan-gui.ts`).
 */
export function attemptFor(current: LanGui | null, stars: number, comment: string): LanGui | "no-key" | null {
  if (current !== null) return current;
  if (!Number.isInteger(stars) || stars < RATING_MIN_STARS || stars > RATING_MAX_STARS) return null;
  try {
    return taoLanGui(ratingBody(stars, comment));
  } catch {
    return "no-key";
  }
}

/** Whether the block appears at all for this petition. */
export function showsRating(p: PhieuCuaToi): boolean {
  return isRateable(p.trang_thai) || p.rating !== null;
}

/** "sent" after a 200; "reopened" when that 200 also carried a different status. */
export type RatingNotice = "sent" | "reopened";

/** The block, PURE — everything through props, so `react-dom/server` renders every state in tests. */
export function RatingPanel(props: {
  petition: PhieuCuaToi;
  open: boolean;
  stars: number;
  comment: string;
  sending: boolean;
  error: RatingErrorBranch | null;
  notice: RatingNotice | null;
  onPick: (stars: number) => void;
  onComment: (comment: string) => void;
  onSubmit: () => void;
  onRetry: () => void;
  onReload: () => void;
  onRateAgain: () => void;
  /** The phone-verification panel, when a 403 `chua_xac_thuc_so` asked for it. */
  phone?: ReactNode;
}) {
  const p = props.petition;
  if (!showsRating(p)) return null;
  const picking = isRateable(p.trang_thai) && props.open;
  const err = props.error === null ? null : RATING_ERROR[props.error];

  return (
    <section className="cd-danh-gia" aria-labelledby="cd-danh-gia-tieu-de">
      <h2 className="cd-tieu-de-phu" id="cd-danh-gia-tieu-de">
        {picking ? RATING.title : RATING.your_rating}
      </h2>

      {props.notice !== null && (
        <div role="status">
          <p className="cd-cau">{RATING.sent}</p>
          {props.notice === "reopened" && <p className="cd-cau">{RATING.reopened}</p>}
        </div>
      )}

      {p.rating !== null && !picking && <StarPicker stars={p.rating} />}
      {p.rating !== null && <p className="cd-cau">{RATING.rated(p.rating)}</p>}

      {isRateable(p.trang_thai) && !props.open && (
        <button type="button" className="cd-nut-phu" onClick={props.onRateAgain}>
          {RATING.rate_again}
        </button>
      )}

      {picking && (
        <>
          <p className="cd-ghi-chu">{RATING.why}</p>
          <StarPicker stars={props.stars} onPick={props.onPick} />
          <ONhapDoan
            id="cd-nhan-xet"
            nhan={RATING.comment_label}
            gia_tri={props.comment}
            toi_da={RATING_COMMENT_MAX_LEN}
            onDoi={props.onComment}
          />
          {err !== null && (
            <div className="cd-buoc">
              <p className="cd-loi" role="alert">
                {err.text}
              </p>
              {err.can_retry && (
                <button type="button" className="cd-nut" disabled={props.sending} onClick={props.onRetry}>
                  {RATING.retry}
                </button>
              )}
              {err.can_reload && (
                <button type="button" className="cd-nut-phu" onClick={props.onReload}>
                  {RATING.reload}
                </button>
              )}
            </div>
          )}
          {props.sending && (
            <p className="cd-cau" role="status">
              {RATING.sending}
            </p>
          )}
          {/* A retryable failure shows "Gửi lại" (same key) INSTEAD of "Gửi đánh giá": two buttons that
              look like the same act, one of which would be a new one, is the mis-tap to avoid. */}
          {!(err !== null && err.can_retry) && (
            <button
              type="button"
              className="cd-nut"
              disabled={props.stars < RATING_MIN_STARS || props.sending}
              onClick={props.onSubmit}
            >
              {RATING.submit}
            </button>
          )}
        </>
      )}

      {props.phone}
    </section>
  );
}

export function PetitionRating(props: {
  petition: PhieuCuaToi;
  /** The 200 body — the screen re-renders the whole petition from it. */
  onRated: (petition: PhieuCuaToi) => void;
  /** Read the petition again (after a 409). */
  onReload: () => void;
  reopenWithPhone?: ReopenWithPhone;
  /** Passed to the phone panel — the commune's own app names its own route (`PhoneVerificationPanel`). */
  zaloAsks?: string;
}) {
  const { petition } = props;
  const phone = usePhoneVerification(props.reopenWithPhone);
  const [open, setOpen] = useState(petition.rating === null);
  const [stars, setStars] = useState(0);
  const [comment, setComment] = useState("");
  /** The act in progress. Kept across "Gửi lại"; dropped when what is being sent changes. */
  const [attempt, setAttempt] = useState<LanGui | null>(null);
  const [sending, setSending] = useState(false);
  const [error, setError] = useState<RatingErrorBranch | null>(null);
  const [notice, setNotice] = useState<RatingNotice | null>(null);

  async function send(a: LanGui) {
    phone.reset();
    setSending(true);
    setError(null);
    setNotice(null);
    const outcome = ratingOutcome(await ratePetition(petition.ma_tra_cuu, a));
    setSending(false);
    if (outcome.kind === "phone") {
      // Re-run with the SAME act: the 403 is answered before the server records anything.
      phone.onPhoneRequired(() => void send(a));
      return;
    }
    if (outcome.kind === "error") {
      setError(outcome.branch);
      return;
    }
    setAttempt(null);
    setStars(0);
    setComment("");
    setOpen(false);
    setNotice(outcome.petition.trang_thai !== petition.trang_thai ? "reopened" : "sent");
    props.onRated(outcome.petition);
  }

  function submit() {
    if (sending) return;
    const next = attemptFor(attempt, stars, comment);
    if (next === null) return;
    if (next === "no-key") {
      setError("no-key");
      return;
    }
    if (next !== attempt) setAttempt(next);
    void send(next);
  }

  /** A change to what is being sent is a NEW act: new key on the next submit, old error cleared. */
  function changed() {
    setAttempt(null);
    setError(null);
  }

  return (
    <RatingPanel
      petition={petition}
      open={open}
      stars={stars}
      comment={comment}
      sending={sending}
      error={error}
      notice={notice}
      onPick={(n) => {
        if (sending) return;
        setStars(n);
        changed();
      }}
      onComment={(c) => {
        if (sending) return;
        setComment(c);
        changed();
      }}
      onSubmit={submit}
      onRetry={() => {
        if (!sending && attempt !== null) void send(attempt);
      }}
      onReload={props.onReload}
      onRateAgain={() => {
        setNotice(null);
        setOpen(true);
      }}
      phone={
        phone.state !== null ? (
          <PhoneVerificationPanel
            state={phone.state}
            task="rate"
            zaloAsks={props.zaloAsks}
            onAllow={() => void phone.allow()}
            onDecline={phone.decline}
          />
        ) : undefined
      }
    />
  );
}
