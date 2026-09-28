import type { petitions_phieuPhanAnhRa } from "@/lib/api/schema.gen";
import type { PublicationTarget } from "@/lib/api/phieu-phan-anh";

import {
  HIDE_BUTTON_LABEL,
  NOT_RATED,
  LOW_RATING_REOPENED,
  PUBLICATION_DENIED,
  PUBLISH_BUTTON_LABEL,
  publicationView,
  RATING_MAX_STARS,
  RATING_TITLE,
  ratingView,
} from "./nhan-phieu";

/**
 * Two read blocks shared by the register drawer (`so-phan-anh.tsx`) and the lookup view
 * (`tra-cuu-phieu.tsx`), so both screens say the same thing about the same petition.
 *
 * NO HOOKS HERE, on purpose: `chon-can-bo.test.tsx` calls `ChiTietPhieu` as a plain function with a
 * call-order `useState` mock, and a hook in a child it renders inline would shift that order.
 */

/**
 * `Đánh giá của người dân` — requirement `FeedbackDetailDrawer.tsx:441-482`.
 *
 * THE COMMENT IS THE CITIZEN'S OWN WORDS and follows `content` exactly (server, `phieu_phan_anh.go`,
 * `RatingComment`): not masked, rendered as a React text node — never as HTML (rule 13) — and never
 * written anywhere else (rule 3).
 *
 * Unlike the requirement, "no rating" is SAID rather than the block vanishing: an officer checking
 * whether the citizen has answered must be able to tell "not rated" from "not loaded".
 */
export function CitizenRatingBlock({
  petition,
  headingId,
}: {
  petition: petitions_phieuPhanAnhRa;
  headingId: string;
}) {
  const view = ratingView(petition);
  return (
    <section aria-labelledby={headingId}>
      <h4 id={headingId}>{RATING_TITLE}</h4>
      {view.kind === "none" ? (
        <p className="trang-thai-rong">{NOT_RATED}</p>
      ) : (
        <>
          <p>
            <span role="img" aria-label={`${view.score} sao`}>
              {view.stars}
            </span>{" "}
            <strong className={view.low ? "nhan-lech" : undefined}>{view.score}</strong>
            {view.at !== null && <> · {view.at}</>}
          </p>
          {view.comment !== "" && <p className="noi-dung-phan-anh">“{view.comment}”</p>}
          {view.reopened && <p className="nhan-lech">{LOW_RATING_REOPENED}</p>}
        </>
      )}
    </section>
  );
}

/** Screen-reader text for the stars on a card (§7 corner). */
export function starsLabel(rating: number): string {
  return `${rating}/${RATING_MAX_STARS} sao`;
}

/**
 * `Hiển thị với người dân` (§8.3) — the state label, its hint and, for holders of `feedback.assign`,
 * the two moderation buttons.
 *
 * THE BUTTONS ARE UX, NOT SECURITY (rule 5, forbidden #1): the server checks `feedback.assign`, and
 * refuses a `can-bo` petition with 409 `never_public`; both sentences reach the screen verbatim through
 * the drawer's error line. `setPublication` absent (the lookup view, which has no write surface) means
 * read-only, whatever the permissions.
 */
export function PublicationBox({
  petition,
  mayModerate,
  busy = false,
  setPublication,
}: {
  petition: petitions_phieuPhanAnhRa;
  mayModerate: boolean;
  busy?: boolean;
  setPublication?: (target: PublicationTarget) => void;
}) {
  const view = publicationView(petition);
  const writable = setPublication !== undefined;
  return (
    <>
      <p>{view.label}</p>
      {view.hint !== "" && <p className="ghi-chu">{view.hint}</p>}
      {writable && mayModerate && (view.canPublish || view.canHide) && (
        <div className="cum-nut">
          {view.canPublish && (
            <button
              type="button"
              className="nut-phu"
              disabled={busy}
              onClick={() => setPublication("cong-khai")}
            >
              {PUBLISH_BUTTON_LABEL}
            </button>
          )}
          {view.canHide && (
            <button
              type="button"
              className="nut-phu"
              disabled={busy}
              onClick={() => setPublication("an")}
            >
              {HIDE_BUTTON_LABEL}
            </button>
          )}
        </div>
      )}
      {writable && !mayModerate && <p className="ghi-chu">{PUBLICATION_DENIED}</p>}
    </>
  );
}
