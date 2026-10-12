/**
 * PULL TO REFRESH FOR THE VIHAT INTRO — "Yêu cầu của tôi" (owner 12/10/2026).
 *
 * WHY ITS OWN FILE AND NOT THE COMMUNE ONE: the commune app's gesture lives in `cong-dan/man/pull-to-refresh.tsx`, and
 * the intro may not import the commune half (`ranh-gioi-hai-nua.test.ts`: two apps, two halves). This is the smaller
 * shape the intro needs — one screen, one loader — written in the same rules: transform-free (the band's height
 * follows the finger, nothing moves on its own), words not a spinner, a failure that keeps the old data and says
 * what to do next.
 *
 * WHERE THE SCROLL IS: the intro's scroller is the shell's `.app-main` (`App.tsx` `KhungApp`), an ANCESTOR of this
 * wrapper. A pull counts only while that scroller is at its top — `scrollTop > 0` means the user is reading a long
 * list, not asking for a reload. React's touch handlers are passive, so the page's own scroll is never blocked.
 *
 * NOT THE ONLY WAY TO FRESH DATA: a screen-reader user cannot pull. The screen keeps its "Tải lại" button and every
 * failure keeps its "Thử lại".
 */
import { type ReactNode, type TouchEvent, useEffect, useRef, useState } from "react";

import { AlertGlyph, RefreshGlyph } from "../company-intro/icons";

import { YEU_CAU_CUA_TOI } from "./noi-dung";

/** Band height at which letting go refreshes: 64px of band = 128px of finger, a deliberate pull. */
export const PULL_TRIGGER_PX = 64;
const PULL_MAX_PX = 96;

/** Finger travel → band height: half the travel, never negative, capped. PURE. */
export function pullBand(travel: number): number {
  return travel <= 0 ? 0 : Math.min(travel / 2, PULL_MAX_PX);
}

export type PullState =
  | { readonly kind: "idle" }
  | { readonly kind: "pulling"; readonly px: number }
  | { readonly kind: "refreshing" }
  | { readonly kind: "failed" };

/** The band above the content, by state. At rest it renders NOTHING. PURE. */
export function PullBand({ state }: { state: PullState }) {
  if (state.kind === "idle") return null;
  if (state.kind === "failed") {
    return (
      <p className="vh-pull vh-pull--failed" role="alert">
        <AlertGlyph className="vh-pull__glyph" />
        <span>{YEU_CAU_CUA_TOI.tai_lai_hong}</span>
      </p>
    );
  }
  if (state.kind === "refreshing") {
    return (
      <div className="vh-pull vh-pull--busy" role="status">
        <RefreshGlyph className="vh-pull__glyph" />
        <span>{YEU_CAU_CUA_TOI.dang_tai_lai}</span>
      </div>
    );
  }
  // The finger's own band: decoration for a screen reader (a touch lasts a moment), so `aria-hidden`.
  return (
    <div className="vh-pull" aria-hidden="true" style={{ height: `${state.px}px` }}>
      <RefreshGlyph className="vh-pull__glyph" />
      <span>{state.px >= PULL_TRIGGER_PX ? YEU_CAU_CUA_TOI.tha_tai_lai : YEU_CAU_CUA_TOI.keo_tai_lai}</span>
    </div>
  );
}

/** The scroll position of the shell's scroller above `element`, or 0 when there is none (a test). */
function scrollerTop(element: HTMLElement): number {
  return element.closest(".app-main")?.scrollTop ?? 0;
}

/**
 * Wraps a screen's body. `onRefresh` answers `true` when it refreshed, `false` when it could not — the screen then
 * keeps what it showed and the band says so. One refresh at a time (a ref, so two touches in one tick start one).
 */
export function IntroPullToRefresh(props: { onRefresh: () => Promise<boolean>; children: ReactNode }) {
  const [state, setState] = useState<PullState>({ kind: "idle" });
  const latest = useRef(props.onRefresh);
  latest.current = props.onRefresh;
  const busy = useRef(false);
  const startY = useRef<number | null>(null);
  const pulled = useRef(0);
  const alive = useRef(true);
  useEffect(() => {
    alive.current = true;
    return () => {
      alive.current = false;
    };
  }, []);

  async function refresh() {
    if (busy.current) return;
    busy.current = true;
    setState({ kind: "refreshing" });
    const ok = await latest.current().catch(() => false);
    busy.current = false;
    if (alive.current) setState(ok ? { kind: "idle" } : { kind: "failed" });
  }

  function onTouchStart(e: TouchEvent<HTMLDivElement>) {
    const off = busy.current || e.touches.length !== 1 || scrollerTop(e.currentTarget) > 0;
    startY.current = off ? null : e.touches[0]!.clientY;
    pulled.current = 0;
  }

  function onTouchMove(e: TouchEvent<HTMLDivElement>) {
    if (startY.current === null || e.touches.length !== 1) return;
    if (scrollerTop(e.currentTarget) > 0) {
      startY.current = null;
      pulled.current = 0;
      setState((s) => (s.kind === "pulling" ? { kind: "idle" } : s));
      return;
    }
    const px = pullBand(e.touches[0]!.clientY - startY.current);
    pulled.current = px;
    // A failure sentence stays until the next pull; the first pixel of a new pull replaces it.
    setState((s) => (px > 0 ? { kind: "pulling", px } : s.kind === "pulling" ? { kind: "idle" } : s));
  }

  function onTouchEnd() {
    if (startY.current === null) return;
    startY.current = null;
    const px = pulled.current;
    pulled.current = 0;
    if (px >= PULL_TRIGGER_PX) void refresh();
    else setState((s) => (s.kind === "pulling" ? { kind: "idle" } : s));
  }

  return (
    <div
      aria-busy={state.kind === "refreshing" ? true : undefined}
      onTouchStart={onTouchStart}
      onTouchMove={onTouchMove}
      onTouchEnd={onTouchEnd}
      onTouchCancel={onTouchEnd}
    >
      <PullBand state={state} />
      {props.children}
    </div>
  );
}
