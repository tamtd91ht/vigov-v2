/**
 * PULL TO REFRESH — every screen of the commune app (customer sheet 12/10/2026, row 2: "bổ sung reload ở tất cả các
 * màn hình để call lại API lấy dữ liệu mới nhất"; row 8: petition status not updating).
 *
 * WHY A GESTURE AND NOT A BUTTON: the header's right slot is Zalo's corner (`khung-xa.tsx` `HeaderShell`), and a
 * reload button inside the content would sit on every screen as one more thing to read. The gesture is the one every
 * phone already teaches. It is NOT the only way back to fresh data: every failure state keeps its "Thử lại" button,
 * and returning to the home / Phản ánh tab re-reads the petitions on its own (`TrangXa.tsx`). A screen-reader user
 * cannot pull — the "Thử lại" buttons and the tab re-entry are their path.
 *
 * WHAT A PULL RE-RUNS: exactly the loaders of the screen it is on, and nothing else. Two ways to name them:
 *   · `onRefresh` — the screen's own loaders, held ABOVE the scroller (`AppCuaXa` holds home's, a detail screen
 *     holds its read);
 *   · `useRefresher` — a loader that lives BELOW it (a news list keyed by tab and chip) registers itself, and leaves
 *     the list when it unmounts — so a tab or chip not shown is never re-read.
 * A screen with neither (a form, an "empty" screen) ignores the gesture: there is nothing to fetch.
 *
 * ONE AT A TIME: a pull while a refresh is in flight does nothing (a ref, not state, so two touches in the same tick
 * cannot both start one). A loader reports `false` when it could not refresh; the screen then KEEPS what it showed and
 * one sentence above the content says so and what to do (`PULL_REFRESH.failed`) — never an error code, never a blank
 * screen in place of data the citizen was reading. A 429 is such a failure: a refresh does not wait out
 * `Retry-After` behind a spinner, it keeps the old data and asks the citizen to pull again later.
 *
 * IDLE = NO MARKUP: the indicator exists only while pulling, refreshing or after a failure, so a screen at rest renders
 * exactly the scroller it rendered before this file existed.
 */
import { createContext, type ReactNode, type TouchEvent, useContext, useEffect, useRef, useState } from "react";

import { BieuTuong } from "./BieuTuong";
import { PULL_REFRESH } from "./noi-dung";

/** One loader's refresh: `true` = refreshed (or nothing to do), `false` = could not, the old data is still shown. */
export type Refresher = () => Promise<boolean>;

type Entry = { readonly run: Refresher };

/** The loaders registered under one scroller. An array swapped on each change: a handful of entries, read only on a pull. */
type RegistryBox = { entries: readonly Entry[] };

const Registry = createContext<RegistryBox | null>(null);

/**
 * Registers `run` with the nearest `PullToRefresh` for as long as the caller is mounted. Outside one (home's own
 * hooks in `AppCuaXa`, the shared app, a test that renders a list alone) it does nothing — the caller then passes its
 * refresh as `onRefresh` where the scroller is drawn. The LATEST `run` is called, never the first render's closure.
 */
export function useRefresher(run: Refresher): void {
  const registry = useContext(Registry);
  const latest = useRef(run);
  latest.current = run;
  useEffect(() => {
    if (registry === null) return;
    const entry: Entry = { run: () => latest.current() };
    registry.entries = [...registry.entries, entry];
    return () => {
      registry.entries = registry.entries.filter((e) => e !== entry);
    };
  }, [registry]);
}

/** Band height at which letting go refreshes. 64px of band = 128px of finger: a deliberate pull, not a scroll jitter. */
export const PULL_TRIGGER_PX = 64;
const PULL_MAX_PX = 96;

/** Finger travel → band height: half the travel (the band trails the finger, as phones do), capped. PURE. */
export function pullDistance(travel: number): number {
  return travel <= 0 ? 0 : Math.min(travel / 2, PULL_MAX_PX);
}

/**
 * Runs every loader at once and answers whether ALL refreshed. A loader that throws counts as `false`, never as an
 * unhandled rejection under the citizen's finger. PURE apart from the loaders.
 */
export async function runRefreshers(runs: readonly Refresher[]): Promise<boolean> {
  const results = await Promise.all(runs.map((r) => r().catch(() => false)));
  return results.every((ok) => ok);
}

export type PullPhase =
  | { readonly kind: "idle" }
  | { readonly kind: "pulling"; readonly px: number }
  | { readonly kind: "refreshing" }
  | { readonly kind: "failed" };

/**
 * The band above the content. Pulling: the words follow the finger (`aria-hidden` — it lasts as long as a touch).
 * Refreshing: the arrow and "Đang tải lại…" as `role="status"` — words, not a spinner alone. Failed: the sentence as
 * `role="alert"`, with an icon — never colour alone. PURE.
 */
export function PullIndicator({ phase }: { phase: PullPhase }) {
  if (phase.kind === "idle") return null;
  if (phase.kind === "failed") {
    return (
      <p className="xa-pull xa-pull--failed" role="alert">
        <BieuTuong ten="alert" co={20} />
        <span>{PULL_REFRESH.failed}</span>
      </p>
    );
  }
  if (phase.kind === "refreshing") {
    return (
      <div className="xa-pull xa-pull--busy" role="status">
        <span className="xa-pull__icon">
          <BieuTuong ten="refresh" co={22} />
        </span>
        <span>{PULL_REFRESH.refreshing}</span>
      </div>
    );
  }
  // The height follows the finger directly — a drag, not an animation, so nothing here moves on its own.
  return (
    <div className="xa-pull" aria-hidden="true" style={{ height: `${phase.px}px` }}>
      <span className="xa-pull__icon">
        <BieuTuong ten="refresh" co={22} />
      </span>
      <span>{phase.px >= PULL_TRIGGER_PX ? PULL_REFRESH.release : PULL_REFRESH.pull}</span>
    </div>
  );
}

/**
 * The scrolling body of a screen (`.xa-trang`), with the gesture on it. `className` is the scroller's own classes;
 * `onRefresh` the loaders held above it (see the header). React's touch handlers are passive, so the page's own
 * scroll is never blocked — the gesture only counts while the scroller is already at its top.
 */
export function PullToRefresh(props: { className: string; onRefresh?: Refresher; children: ReactNode }) {
  const [registry] = useState<RegistryBox>(() => ({ entries: [] }));
  const [phase, setPhase] = useState<PullPhase>({ kind: "idle" });
  const own = useRef(props.onRefresh);
  own.current = props.onRefresh;
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
    const runs = [...(own.current === undefined ? [] : [own.current]), ...registry.entries.map((e) => e.run)];
    if (runs.length === 0) return;
    busy.current = true;
    setPhase({ kind: "refreshing" });
    const ok = await runRefreshers(runs);
    busy.current = false;
    if (alive.current) setPhase(ok ? { kind: "idle" } : { kind: "failed" });
  }

  function onTouchStart(e: TouchEvent<HTMLDivElement>) {
    const nothing = own.current === undefined && registry.entries.length === 0;
    // `scrollTop > 0`: the citizen is scrolling a list, not asking for a reload.
    const off = busy.current || nothing || e.touches.length !== 1 || e.currentTarget.scrollTop > 0;
    startY.current = off ? null : e.touches[0]!.clientY;
    pulled.current = 0;
  }

  function onTouchMove(e: TouchEvent<HTMLDivElement>) {
    if (startY.current === null || e.touches.length !== 1) return;
    if (e.currentTarget.scrollTop > 0) {
      startY.current = null;
      pulled.current = 0;
      setPhase((p) => (p.kind === "pulling" ? { kind: "idle" } : p));
      return;
    }
    const px = pullDistance(e.touches[0]!.clientY - startY.current);
    pulled.current = px;
    // A failure sentence stays until the citizen pulls again; the first pixel of a new pull replaces it.
    setPhase((p) => (px > 0 ? { kind: "pulling", px } : p.kind === "pulling" ? { kind: "idle" } : p));
  }

  function onTouchEnd() {
    if (startY.current === null) return;
    startY.current = null;
    const px = pulled.current;
    pulled.current = 0;
    if (px >= PULL_TRIGGER_PX) void refresh();
    else setPhase((p) => (p.kind === "pulling" ? { kind: "idle" } : p));
  }

  return (
    <Registry.Provider value={registry}>
      <div
        className={props.className}
        aria-busy={phase.kind === "refreshing" ? true : undefined}
        onTouchStart={onTouchStart}
        onTouchMove={onTouchMove}
        onTouchEnd={onTouchEnd}
        onTouchCancel={onTouchEnd}
      >
        <PullIndicator phase={phase} />
        {props.children}
      </div>
    </Registry.Provider>
  );
}
