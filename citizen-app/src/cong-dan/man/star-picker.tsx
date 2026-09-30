/**
 * THE FIVE STARS — one component for BOTH rating blocks: the experience app's in-memory one
 * (`PhanAnhAppXa.tsx` `KhoiDanhGia`) and the live one that goes over the network (`PetitionRating.tsx`).
 * Taken from the customer prototype (`../vigov-require/apps/miniapp/src/components/feedback/RatingBlock.tsx`).
 *
 * ONE COPY ON PURPOSE: the star row is where an elderly citizen mis-taps, and two copies would be two
 * places to fix it. Each star is its OWN touch target (`.xa-sao__nut`, ≥ tap-min + 4px, pinned by
 * `accessibility.test.ts`), the chosen level is said in WORDS under the row, never by colour alone.
 *
 * PURE: no network, no storage, no state — the caller owns the number. The experience app imports this file,
 * and `trai-nghiem.test.ts` requires that nothing it imports reaches `goi-vigov`.
 *
 * NO THRESHOLD HERE, in words or in styling: no star is marked as "the one that reopens" (ADR 0050).
 */
import { BieuTuong } from "./BieuTuong";
import { XA_PA } from "./noi-dung";

/** The five levels, in the prototype's words. Neutral form — no "bà con" / "bạn" — so both apps share it. */
export const STAR_LABELS = ["Rất không hài lòng", "Không hài lòng", "Bình thường", "Hài lòng", "Rất hài lòng"] as const;

/**
 * `onPick` absent = READ-ONLY: the row is an image with one label ("Đã chấm n trên 5 sao"), the buttons are
 * disabled. Present = a radiogroup, one radio per star.
 */
export function StarPicker({ stars, onPick }: { stars: number; onPick?: (stars: number) => void }) {
  const readOnly = onPick === undefined;
  return (
    <>
      <div
        className="xa-sao"
        role={readOnly ? "img" : "radiogroup"}
        aria-label={readOnly ? XA_PA.da_cham(stars) : XA_PA.cham_diem}
      >
        {[1, 2, 3, 4, 5].map((n) => (
          <button
            key={n}
            type="button"
            disabled={readOnly}
            role={readOnly ? undefined : "radio"}
            aria-checked={readOnly ? undefined : stars === n}
            aria-label={`${n} sao — ${STAR_LABELS[n - 1]}`}
            className={`xa-sao__nut${n <= stars ? " xa-sao__nut--on" : ""}`}
            onClick={() => onPick?.(n)}
          >
            <BieuTuong ten="star" co={32} />
          </button>
        ))}
      </div>
      <p className="xa-giua">{stars > 0 ? STAR_LABELS[stars - 1] : XA_PA.cham_vao_sao}</p>
    </>
  );
}
