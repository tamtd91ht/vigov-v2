import { Eye, EyeOff, Star } from "lucide-react";

import { Card, CardContent, CardHeader } from "@/components/ui/card";
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
  SCENE_COORDINATES_NOTE,
  SCENE_NO_ADDRESS,
  SCENE_NO_COORDINATES,
  sceneCoordinates,
} from "./nhan-phieu";
import { buttonClass, Glyph, SectionTitle } from "./petition-ui";

/**
 * Three read blocks shared by the register drawer (`so-phan-anh.tsx`) and the lookup view
 * (`tra-cuu-phieu.tsx`), so both screens say the same thing about the same petition.
 *
 * NO HOOKS HERE, on purpose: `chon-can-bo.test.tsx` calls `ChiTietPhieu` as a plain function with a
 * call-order `useState` mock, and a hook in a child it renders inline would shift that order.
 */

/**
 * The `<dd>` of the `Vị trí hiện trường` row: the address line, then the coordinates as TEXT (see
 * `sceneCoordinates` for why there is no map). A row, not the requirement's vanishing section —
 * absence is SAID, same choice as `CitizenRatingBlock`: an officer must tell "the citizen sent no
 * coordinates" from "the screen did not load them".
 *
 * No copy button: the codebase has no clipboard pattern, and `mat-khau-tam.tsx` records why one was
 * refused (`navigator.clipboard` is absent during server rendering).
 */
export function SceneLocation({ petition }: { petition: petitions_phieuPhanAnhRa }) {
  const coordinates = sceneCoordinates(petition);
  return (
    <>
      <p>{petition.address === "" ? SCENE_NO_ADDRESS : petition.address}</p>
      {coordinates === null ? (
        <p className="text-ink-500 italic">{SCENE_NO_COORDINATES}</p>
      ) : (
        <>
          <p className="tabular-nums">{coordinates}</p>
          <p className="ghi-chu">{SCENE_COORDINATES_NOTE}</p>
        </>
      )}
    </>
  );
}

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
    <Card as="section" aria-labelledby={headingId}>
      <CardHeader>
        <SectionTitle icon={Star} id={headingId}>
          {RATING_TITLE}
        </SectionTitle>
      </CardHeader>
      <CardContent className="flex flex-col gap-2 text-sm [&>p]:m-0">
        {view.kind === "none" ? (
          <p className="text-ink-500 italic">{NOT_RATED}</p>
        ) : (
          <>
            <p className="inline-flex flex-wrap items-center gap-x-2">
              <span role="img" aria-label={`${view.score} sao`} className="text-lg leading-none text-legal-800">
                {view.stars}
              </span>{" "}
              <strong className={view.low ? "nhan-lech" : undefined}>{view.score}</strong>
              {view.at !== null && <span className="text-ink-500"> · {view.at}</span>}
            </p>
            {view.comment !== "" && <p className="noi-dung-phan-anh">“{view.comment}”</p>}
            {view.reopened && <p className="nhan-lech">{LOW_RATING_REOPENED}</p>}
          </>
        )}
      </CardContent>
    </Card>
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
 *
 * NATIVE `<button>`s, ON PURPOSE: `so-phan-anh.test.tsx` calls this block as a function and drives the
 * `"button"` elements of its tree; a `Button` component there would hide them from that walk.
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
      {/* Prototype `FeedbackDetailDrawer.tsx:266-267`: the state in words, then its hint — no glyph. */}
      <p className="text-[12.5px] text-navy">{view.label}</p>
      {view.hint !== "" && <p className="mt-0.5 text-[11px] text-ink-muted">{view.hint}</p>}
      {writable && mayModerate && (view.canPublish || view.canHide) && (
        <div className="mt-1.5 flex flex-wrap gap-1.5">
          {view.canPublish && (
            <button
              type="button"
              className={buttonClass("secondary", "sm")}
              disabled={busy}
              onClick={() => setPublication("cong-khai")}
            >
              <Glyph icon={Eye} />
              {PUBLISH_BUTTON_LABEL}
            </button>
          )}
          {view.canHide && (
            <button
              type="button"
              className={buttonClass("secondary", "sm")}
              disabled={busy}
              onClick={() => setPublication("an")}
            >
              <Glyph icon={EyeOff} />
              {HIDE_BUTTON_LABEL}
            </button>
          )}
        </div>
      )}
      {writable && !mayModerate && <p className="ghi-chu">{PUBLICATION_DENIED}</p>}
    </>
  );
}
