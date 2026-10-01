/**
 * THE TRUYỀN THANH PLAYER — a broadcast's audio on its article, in the commune's own app (ADR 0067 §4; ADR 0047 G7).
 *
 * A NATIVE `<audio>` WITH NO `controls` ATTRIBUTE, DRIVEN BY OUR OWN BUTTONS. The webview's built-in bar is ~30px
 * tall, its icons are unlabelled and it differs per phone; the citizens who most need the broadcast (older people
 * who would otherwise hear it from the village loudspeaker) are the ones it fails. So: a play/pause button with
 * WORDS, two "15 seconds" buttons to move back and forward (the seek — see `BroadcastPlayerView`), a progress bar
 * whose value is read out in words, and the elapsed / total time — every target ≥ 44px (`styles.css` `.xa-player__*`).
 *
 * NEVER PLAYS BY ITSELF: no `autoplay`, `preload="none"`, and no `src` until the citizen's first tap. A broadcast
 * starting on its own in a public place is a surprise nobody asked for, and fetching up to 30 MB on mobile data
 * because a page was opened costs the citizen money.
 *
 * THE LINK EXPIRES (≤ 15 minutes, `tin_xa_cong_khai.go` `tinXaRa`). One re-read of the item per attempt, never a
 * loop: before playing when the clock says the link has expired, or when the media fails (a 403 from storage is
 * reported by the element as a plain media error — it never exposes the status, so every media error is treated
 * as "maybe expired"). The re-read link failing too, or the item not re-readable, ends in ONE sentence saying what to
 * do next (`XA_TN.broadcast_failed`). The count resets once audio actually plays, so a link that expires during a
 * long pause gets its own re-read.
 *
 * STOPS WHEN THE CITIZEN LEAVES: on unmount, and when the page is hidden (`visibilitychange` / `pagehide` — Zalo's
 * webview fires these when the Mini App goes to the background). This half may not import `zmp-sdk`
 * (`ranh-gioi-hai-nua.test.ts` §3a) and the app subscribes to no zmp lifecycle event, so the web events are the
 * ones used.
 *
 * NOTHING HERE LOGS, STORES OR SENDS THE LINK. It is a presigned URL: whoever holds it can play the file until it
 * expires.
 */
import { useEffect, useRef, useState } from "react";

import type { BroadcastAudio } from "../api/hop-dong-cong-khai";

import { BieuTuong } from "./BieuTuong";
import { XA_TN } from "./noi-dung"; // vi-name-ok: existing strings module

/** How far the two skip buttons move, in seconds. */
export const SKIP_SECONDS = 15;

/** Seconds → "m:ss", or "h:mm:ss" from one hour. Negative, fractional or non-finite input is floored / 0. PURE. */
export function formatClock(seconds: number): string {
  const t = Number.isFinite(seconds) && seconds > 0 ? Math.floor(seconds) : 0;
  const h = Math.floor(t / 3600);
  const m = Math.floor((t % 3600) / 60);
  const s = t % 60;
  const ss = String(s).padStart(2, "0");
  return h > 0 ? `${h}:${String(m).padStart(2, "0")}:${ss}` : `${m}:${ss}`;
}

/** Seconds → "3 phút 20 giây" (what a screen reader should say instead of "3:20"). PURE. */
export function durationWords(seconds: number): string {
  const t = Number.isFinite(seconds) && seconds > 0 ? Math.floor(seconds) : 0;
  return XA_TN.duration_words(Math.floor(t / 3600), Math.floor((t % 3600) / 60), t % 60);
}

/** `true` when `now` has reached the link's expiry. No readable expiry → `false` (the media error says so). PURE. */
export function audioLinkExpired(expiresAt: string | undefined, now: number): boolean {
  if (expiresAt === undefined) return false;
  const at = new Date(expiresAt).getTime();
  return !Number.isNaN(at) && now >= at;
}

/** The part of `HTMLAudioElement` the controller drives — a fake in tests, the element in the app. */
export type MediaLike = {
  src: string;
  currentTime: number;
  play(): Promise<void>;
  pause(): void;
  load(): void;
};

export type PlayerPhase = "idle" | "loading" | "playing" | "paused" | "failed";

export type PlaybackDeps = {
  media: () => MediaLike | null;
  initial: BroadcastAudio;
  /** Re-reads the item; its fresh audio, or `null` (not readable, no audio any more). Absent → no re-read. */
  refresh?: () => Promise<BroadcastAudio | null>;
  now: () => number;
  onPhase: (phase: PlayerPhase) => void;
};

/**
 * The playback rules, with no React and no DOM — so the expired-link path is tested as it runs. The component wires
 * the element's events to `playing` · `paused` · `failed` · `metadataLoaded` · `timeUpdate`.
 */
export function createPlayback(deps: PlaybackDeps) {
  let link = deps.initial;
  let phase: PlayerPhase = "idle";
  let attached = false; // `src` is set on the first tap, never at render
  let renewed = false; // one re-read per attempt; cleared once audio actually plays
  let handledFailureFor = ""; // the element's `error` event and `play()`'s rejection report ONE failure twice
  let lastTime = 0;
  let resumeAt = 0;
  let generation = 0; // bumped by `stop`: a re-read that returns after the citizen left must not start playing

  const set = (p: PlayerPhase) => {
    phase = p;
    deps.onPhase(p);
  };

  async function renew(): Promise<boolean> {
    if (renewed || deps.refresh === undefined) {
      set("failed");
      return false;
    }
    renewed = true;
    const g = generation;
    const fresh = await deps.refresh().catch(() => null);
    if (g !== generation) return false;
    const m = deps.media();
    if (fresh === null || m === null) {
      set("failed");
      return false;
    }
    link = fresh;
    resumeAt = lastTime; // carry on where the citizen was, once the new source has its metadata
    m.src = fresh.url;
    attached = true;
    m.load();
    return true;
  }

  function startPlay(m: MediaLike) {
    const g = generation;
    m.play().then(undefined, (e: unknown) => {
      if (g !== generation) return;
      const name = typeof e === "object" && e !== null ? (e as { name?: unknown }).name : undefined;
      // A webview may refuse `play()` that follows a network wait (no longer "inside" the tap): not a failure —
      // the button simply offers "Nghe" again.
      if (name === "NotAllowedError") {
        if (phase !== "failed") set("paused");
        return;
      }
      // Any other rejection (NotSupportedError, AbortError…) is NOT acted on: a source that cannot load also fires
      // the element's `error` event, which is the ONE failure signal. Acting on both would count one failure twice —
      // and the late one would land after the fresh link was set, and be blamed on it.
    });
  }

  /**
   * The element (or `play()`) reported a failure. Acted on only while the citizen is LISTENING (loading or playing):
   * a failure while paused must not re-read and start playing on its own — the next tap handles it.
   */
  async function failed(): Promise<void> {
    if ((phase !== "loading" && phase !== "playing") || handledFailureFor === link.url) return;
    handledFailureFor = link.url;
    set("loading");
    const g = generation;
    if (await renew()) {
      const m = deps.media();
      if (m !== null && g === generation) startPlay(m);
    }
  }

  async function toggle(): Promise<void> {
    const m = deps.media();
    if (m === null) return;
    if (phase === "playing" || phase === "loading") {
      stop();
      return;
    }
    if (phase === "failed") {
      // A new tap is a new attempt — with its own single re-read.
      renewed = false;
      handledFailureFor = "";
    }
    set("loading");
    const g = generation;
    if (audioLinkExpired(link.expiresAt, deps.now())) {
      if (!(await renew()) || g !== generation) return;
    } else if (!attached) {
      m.src = link.url;
      attached = true;
    }
    startPlay(m);
  }

  /** Pause and cancel anything in flight: the pause button, unmount, the app going to the background. */
  function stop() {
    generation += 1;
    // The next tap is a new attempt with its own single re-read — also when this one was cut off mid-re-read.
    renewed = false;
    handledFailureFor = "";
    deps.media()?.pause();
    if (phase === "playing" || phase === "loading") set("paused");
  }

  return {
    toggle,
    failed,
    stop,
    playing() {
      renewed = false;
      handledFailureFor = "";
      set("playing");
    },
    paused() {
      if (phase === "playing") set("paused");
    },
    metadataLoaded() {
      const m = deps.media();
      if (m !== null && resumeAt > 0) m.currentTime = resumeAt;
      resumeAt = 0;
    },
    timeUpdate(t: number) {
      lastTime = t;
    },
    phase: () => phase,
  };
}

export type Playback = ReturnType<typeof createPlayback>;

/**
 * The controls, PURE (the state is the caller's) so they render and are tapped in tests without a DOM. Words on
 * every button, an icon beside them, the failure as a sentence in a box with `role="alert"` — never colour alone.
 */
export function BroadcastPlayerView(props: {
  phase: PlayerPhase;
  elapsed: number;
  /** Seconds, or `null` when no length is known — then no progress bar and no total. */
  total: number | null;
  onToggle: () => void;
  onSkip: (deltaSeconds: number) => void;
}) {
  const { phase, total } = props;
  const active = phase !== "idle" && phase !== "failed";
  const elapsed = total === null ? props.elapsed : Math.min(props.elapsed, total);
  const busy = phase === "loading";
  const label = phase === "playing" ? XA_TN.broadcast_pause : busy ? XA_TN.broadcast_loading : XA_TN.broadcast_play;
  return (
    <section className="xa-player" aria-label={XA_TN.broadcast_player}>
      <button type="button" className="xa-nut xa-player__nut" onClick={props.onToggle} aria-busy={busy || undefined}>
        <BieuTuong ten={phase === "playing" ? "pause" : "play"} co={24} />
        {label}
      </button>
      <p className="xa-player__time">
        {total === null ? formatClock(elapsed) : `${formatClock(elapsed)} / ${formatClock(total)}`}
      </p>
      {/* SEEKING IS THE TWO ±15 s BUTTONS, NOT A DRAGGED SLIDER — on purpose. An `<input type="range">` is an input
          control, and `phase1-collects-nothing.test.ts` keeps every input in two named files (what Zalo reviewed is
          an app whose fields are known); a hand-built ARIA slider would be the same control dodging that check.
          Buttons are also the easier seek for an older hand. The bar below shows where the citizen is. */}
      {total !== null && (
        <progress
          className="xa-player__bar"
          max={total}
          value={Math.floor(elapsed)}
          aria-label={XA_TN.broadcast_position}
          aria-valuetext={XA_TN.broadcast_position_text(durationWords(elapsed), durationWords(total))}
        />
      )}
      <div className="xa-player__skip">
        <button type="button" className="xa-nut xa-nut--quiet xa-player__nut" disabled={!active} onClick={() => props.onSkip(-SKIP_SECONDS)}>
          {XA_TN.broadcast_back}
        </button>
        <button type="button" className="xa-nut xa-nut--quiet xa-player__nut" disabled={!active} onClick={() => props.onSkip(SKIP_SECONDS)}>
          {XA_TN.broadcast_forward}
        </button>
      </div>
      {phase === "failed" && (
        <p className="xa-error-box" role="alert">
          {XA_TN.broadcast_failed}
        </p>
      )}
    </section>
  );
}

/**
 * The player on a broadcast's article. `refresh` re-reads the item (the caller's network call); absent, an expired
 * link ends in the failure sentence.
 */
export function BroadcastPlayer(props: { audio: BroadcastAudio; refresh?: () => Promise<BroadcastAudio | null> }) {
  const ref = useRef<HTMLAudioElement>(null);
  const [phase, setPhase] = useState<PlayerPhase>("idle");
  const [elapsed, setElapsed] = useState(0);
  const [mediaLength, setMediaLength] = useState<number | null>(null);
  const control = useRef<Playback | null>(null);
  if (control.current === null) {
    control.current = createPlayback({
      media: () => ref.current,
      initial: props.audio,
      refresh: props.refresh,
      now: () => Date.now(),
      onPhase: setPhase,
    });
  }
  const c = control.current;

  useEffect(() => {
    const leave = () => {
      if (document.visibilityState === "hidden") c.stop();
    };
    const gone = () => c.stop();
    document.addEventListener("visibilitychange", leave);
    window.addEventListener("pagehide", gone);
    return () => {
      document.removeEventListener("visibilitychange", leave);
      window.removeEventListener("pagehide", gone);
      c.stop();
    };
  }, [c]);

  // The officer's typed length is the total (ADR 0067 §4.1); the file's own only when none was typed.
  const total = props.audio.durationSeconds ?? mediaLength;
  const seekTo = (s: number) => {
    const m = ref.current;
    if (m === null) return;
    const max = total ?? Number.POSITIVE_INFINITY;
    m.currentTime = Math.max(0, Math.min(s, max));
    setElapsed(m.currentTime);
  };

  return (
    <>
      <audio
        ref={ref}
        preload="none"
        onPlaying={() => c.playing()}
        onPause={() => c.paused()}
        onEnded={() => c.paused()}
        onError={() => void c.failed()}
        onLoadedMetadata={(e) => {
          const d = e.currentTarget.duration;
          if (Number.isFinite(d) && d > 0) setMediaLength(Math.round(d));
          c.metadataLoaded();
        }}
        onTimeUpdate={(e) => {
          c.timeUpdate(e.currentTarget.currentTime);
          setElapsed(e.currentTarget.currentTime);
        }}
      />
      <BroadcastPlayerView
        phase={phase}
        elapsed={elapsed}
        total={total}
        onToggle={() => void c.toggle()}
        onSkip={(d) => seekTo((ref.current?.currentTime ?? 0) + d)}
      />
    </>
  );
}
