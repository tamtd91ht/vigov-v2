/**
 * WHAT A WAITING OR AN EMPTY SCREEN DRAWS, BOTH APPS — the owner's "livelier, on the existing plain CSS" decision
 * (UI-1, 02/10/2026, ADR 0047 §6). Two pieces, both pure decoration beside words that carry the meaning:
 *
 *   · `LoadingSkeleton` — flat blocks in the shape of what is coming (rows, a card, an article). The sentence that
 *     says "đang tải" stays VISIBLE beside it and keeps `role="status"`: a block says nothing to a screen reader,
 *     and a grey shape alone cannot tell "loading" from "broken". Heights are in `em`, so the blocks grow with the
 *     commune app's text-size setting (`.xa-co-chu--*`) instead of drawing rows smaller than the text that follows.
 *     They pulse by OPACITY only, and only under `prefers-reduced-motion: no-preference` (`styles.css`); no
 *     gradient, no shimmer — a sweeping gradient is a colour nobody measured (`accessibility.test.ts`).
 *   · `EmptyIllustration` — one small drawing per kind of empty list, never the only signal: the title sentence
 *     and the hint under it say what is empty and what to do next.
 *
 * JSX elements, never an SVG string (`BieuTuong.tsx` says why). Every `<svg>` is `aria-hidden` and unfocusable.
 * Colours come from CSS classes on the shapes (`.xa-illus` / `.cd-illus` in `styles.css`), so the drawing uses
 * exactly the measured tokens of the app it sits in and the shared app's look is set apart from the commune app's.
 */
import type { ReactNode } from "react";

/* ═══════════════════════════════ SKELETON ═══════════════════════════════ */

/**
 * `list`    news rows: a picture slot and two lines (the shape of `HangTin`)
 * `rows`    text-only cards: three lines each (petitions, the directory, the shared app's lists)
 * `card`    one card of lines (a petition, a lookup, the field catalogue, the commune check)
 * `article` a title, the meta line, paragraphs — and NO cover block: an article may have no picture, and a
 *           grey band would promise one
 * `none`    no blocks: the wait is not for content (opening a session, opening the app)
 */
export type SkeletonShape = "list" | "rows" | "card" | "article" | "none";

/** `prefix`: whose look — `xa` (commune app) or `cd` (shared app). `rows`: how many rows a list shape draws. */
export function LoadingSkeleton(props: { prefix: "xa" | "cd"; shape: SkeletonShape; rows?: number }) {
  if (props.shape === "none") return null;
  const count = Math.max(1, props.rows ?? 3);
  let body: ReactNode;
  if (props.shape === "list" || props.shape === "rows") {
    body = Array.from({ length: count }, (_, i) => (
      <span key={i} className="skel-row">
        {props.shape === "list" && <span className="skel skel--thumb" />}
        <span className="skel-lines">
          <span className="skel skel--line" />
          {props.shape === "rows" && <span className="skel skel--line" />}
          <span className="skel skel--line skel--short" />
        </span>
      </span>
    ));
  } else if (props.shape === "card") {
    body = (
      <span className="skel-row">
        <span className="skel-lines">
          <span className="skel skel--title" />
          <span className="skel skel--line" />
          <span className="skel skel--line" />
          <span className="skel skel--line skel--short" />
        </span>
      </span>
    );
  } else {
    body = (
      <span className="skel-lines">
        <span className="skel skel--title" />
        <span className="skel skel--line skel--short" />
        <span className="skel skel--line" />
        <span className="skel skel--line" />
        <span className="skel skel--line" />
        <span className="skel skel--line skel--short" />
      </span>
    );
  }
  return (
    <span className={`${props.prefix}-skeleton ${props.prefix}-skeleton--${props.shape}`} aria-hidden="true">
      {body}
    </span>
  );
}

/* ═══════════════════════════════ ILLUSTRATIONS ═══════════════════════════════ */

/** Four drawings, and no more: one per kind of empty list a citizen meets, and one for everything else. */
export type IllustrationKind = "news" | "petition" | "directory" | "generic";

const SHAPES: Readonly<Record<IllustrationKind, ReactNode>> = {
  // A folded newspaper: the sheet, its picture, its lines.
  news: (
    <>
      <path className="illus-fill" d="M22,20a6,6,0,0,1,6,-6h56a6,6,0,0,1,6,6v50a6,6,0,0,0,6,6H28a6,6,0,0,1,-6,-6z" />
      <path className="illus-line" d="M90,30h6a4,4,0,0,1,4,4v36a6,6,0,0,1,-12,0" />
      <rect className="illus-line" x="32" y="25" width="22" height="17" rx="3" />
      <path className="illus-line" d="M62,27h18M62,36h14M32,52h48M32,61h48M32,70h30" />
    </>
  ),
  // A speech bubble with a plus: "say something to the commune".
  petition: (
    <>
      <path
        className="illus-fill"
        d="M18,20a8,8,0,0,1,8,-8h68a8,8,0,0,1,8,8v38a8,8,0,0,1,-8,8H48l-16,13v-13h-6a8,8,0,0,1,-8,-8z"
      />
      <path className="illus-line" d="M60,26v26M47,39h26" />
    </>
  ),
  // A contact card: a person and three lines.
  directory: (
    <>
      <rect className="illus-fill" x="16" y="14" width="88" height="62" rx="8" />
      <circle className="illus-line" cx="42" cy="37" r="9" />
      <path className="illus-line" d="M28,62a14,14,0,0,1,28,0M66,32h26M66,43h20M66,54h14" />
    </>
  ),
  // An empty tray: nothing here yet.
  generic: (
    <>
      <path className="illus-fill" d="M16,48l14,-30h60l14,30v22a6,6,0,0,1,-6,6H22a6,6,0,0,1,-6,-6z" />
      <path className="illus-line" d="M16,48h26a18,18,0,0,0,36,0h26" />
    </>
  ),
};

/** `className`: `xa-illus` (commune app) or `cd-illus` (shared app) — the CSS that colours the shapes. */
export function EmptyIllustration({ kind, className }: { kind: IllustrationKind; className: string }) {
  return (
    <svg className={className} viewBox="0 0 120 90" width="120" height="90" aria-hidden="true" focusable="false">
      {SHAPES[kind]}
    </svg>
  );
}
