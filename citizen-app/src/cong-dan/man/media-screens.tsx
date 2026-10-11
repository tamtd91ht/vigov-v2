/**
 * VIDEO AND TRUYỀN THANH — the prototype's two library screens (`../vigov-require/apps/miniapp/src/pages/VideoPage.tsx`,
 * `BroadcastPage.tsx`; owner 11/10/2026: "làm hết theo prototype"), on the SAME public data the news tab reads
 * (`/commune-news?type=video|truyen-thanh`, and `/commune-news/categories?type=` for the chips) — no new API.
 *
 *   Video        a two-column grid: 16:10 cover with a Play mark, the title, "x lượt xem" when the server counted.
 *                The length badge (mm:ss) only when the commune typed `video_duration_seconds` (ADR 0091 §7).
 *                A tap opens the item, whose "Xem video" opens the commune's link (`TinTucAppXa.tsx` `WatchVideo`).
 *   Truyền thanh items grouped by day ("Hôm nay", "Hôm qua", "Thứ Ba, 22/8/2026"); each row's round button
 *                PLAYS IN PLACE through one real `<audio>` and the fixed bar under the list: a seek slider, ±15 s,
 *                and the speed (0.75 / 1 / 1.25×). Nothing plays by itself and nothing is fetched until the first
 *                tap (`preload="none"`, `src` set on the tap — `broadcast-player.tsx`). An expired link is re-read
 *                once per attempt, as on the article.
 *
 * Chips: "Tất cả" + the categories of that type; a failed or empty category read shows no chips and says nothing
 * (the list works without them — the news tab's stance). NO VIEW COUNT on Truyền thanh (the prototype has none).
 *
 * ⚠ THE AUDIO LINK IS A PRESIGNED URL: handed to `<audio src>` only — never logged, stored, shared or used as a key.
 */
import { useEffect, useRef, useState } from "react";

import type { TinXaTomTat } from "../api/hop-dong-cong-khai";
import { formatDayGroup } from "../../lib/vn-format";
import { ngayVN } from "../../lib/thoi-diem";

import { BieuTuong } from "./BieuTuong";
import { createPlayback, durationWords, formatClock, type Playback, type PlayerPhase, SKIP_SECONDS } from "./broadcast-player";
import { DauManCon, KhoiTrangThai } from "./khung-xa";
import { TIN_XA, XA_TN, COMMUNE_NEWS_RATE_LIMIT } from "./noi-dung";
import { SeekBar } from "./o-nhap"; // vi-name-ok: existing module name, not renamed (rule 12 #3)
import { type CategoryRows, rereadAudio, todayVN, useNewsCategories, useTinXa } from "./TinTucAppXa";
import { errorSentence } from "./TinTucXaScreen";

/* ═══════════════════════════════ CHIPS ═══════════════════════════════ */

/**
 * One row of filter chips (`FilterChips.tsx` of the prototype): "Tất cả" + the type's ROOT categories (the server's
 * filter includes each root's children). `null` rows or none → nothing at all. PURE.
 */
export function TopicChips(props: {
  label: string;
  rows: CategoryRows | null;
  value: string | null;
  onChange: (id: string | null) => void;
}) {
  if (props.rows === null || props.rows.roots.length === 0) return null;
  return (
    <div className="xa-chips" role="group" aria-label={props.label}>
      <button
        type="button"
        className={`xa-chip${props.value === null ? " xa-chip--on" : ""}`}
        aria-pressed={props.value === null}
        onClick={() => props.onChange(null)}
      >
        {XA_TN.loc_tat_ca}
      </button>
      {props.rows.roots.map((c) => (
        <button
          key={c.id}
          type="button"
          className={`xa-chip${props.value === c.id ? " xa-chip--on" : ""}`}
          aria-pressed={props.value === c.id}
          onClick={() => props.onChange(c.id)}
        >
          {c.name}
        </button>
      ))}
    </div>
  );
}

/** Loading · failed (with "Thử lại" unless it cannot help) — the part both libraries share. `null` once loaded. */
function ListState(props: { ds: ReturnType<typeof useTinXa>["ds"]; onRetry: () => void; icon: "video" | "speaker" }) {
  const { ds } = props;
  if (!ds.da_co_trang_dau && ds.dang_tai) return <KhoiTrangThai bieu_tuong={props.icon} cau={TIN_XA.dang_tai} dang_tai />;
  if (!ds.da_co_trang_dau && ds.loi !== null) {
    return (
      <KhoiTrangThai
        bieu_tuong="alert"
        loi
        cau={errorSentence(ds.loi, ds.retryAfterSeconds ?? null, COMMUNE_NEWS_RATE_LIMIT)}
        nut={ds.loi !== "khong-hop-le" ? { nhan: TIN_XA.nut_thu_lai, onBam: props.onRetry } : undefined}
      />
    );
  }
  return null;
}

/** "Xem thêm" and a later page's failure, under a loaded list. */
function MoreButton(props: { ds: ReturnType<typeof useTinXa>["ds"]; onMore: () => void }) {
  const { ds } = props;
  return (
    <>
      {ds.loi !== null && (
        <KhoiTrangThai
          bieu_tuong="alert"
          loi
          cau={errorSentence(ds.loi, ds.retryAfterSeconds ?? null, COMMUNE_NEWS_RATE_LIMIT)}
          nut={ds.loi !== "khong-hop-le" ? { nhan: TIN_XA.nut_thu_lai, onBam: props.onMore } : undefined}
        />
      )}
      {ds.loi === null && ds.con_nua && (
        <button type="button" className="xa-nut xa-nut--phu" disabled={ds.dang_tai} onClick={props.onMore}>
          {ds.dang_tai ? TIN_XA.dang_tai_them : TIN_XA.nut_xem_them}
        </button>
      )}
    </>
  );
}

/* ═══════════════════════════════ VIDEO ═══════════════════════════════ */

/**
 * One video tile (`VideoPage.tsx:47-84`): cover with a Play mark, the length badge in its corner when the commune
 * typed one (mm:ss — ADR 0091 §7; no badge otherwise, never a measured or guessed length), title on two lines, the
 * view count. The badge is inside the decorative cover; the length reaches a screen reader in WORDS through the
 * tile's name. PURE.
 */
export function VideoTile(props: { tin: TinXaTomTat; onMo: (id: string) => void }) {
  const { tin } = props;
  const [coverFailed, setCoverFailed] = useState(false);
  const length = tin.videoDurationSeconds;
  const name = length === undefined ? XA_TN.video_open(tin.tieu_de) : XA_TN.video_open_with_length(tin.tieu_de, durationWords(length));
  return (
    <button type="button" className="xa-video-tile" aria-label={name} onClick={() => props.onMo(tin.id)}>
      <span className="xa-video-tile__cover" aria-hidden="true">
        {tin.imageUrl !== undefined && !coverFailed ? (
          <img className="xa-video-tile__image" src={tin.imageUrl} alt="" loading="lazy" decoding="async" onError={() => setCoverFailed(true)} />
        ) : (
          <BieuTuong ten="video" co={36} />
        )}
        <span className="xa-video-tile__play">
          <BieuTuong ten="play-triangle" co={22} fill="currentColor" />
        </span>
        {length !== undefined && <span className="xa-video-tile__length">{formatClock(length)}</span>}
      </span>
      <span className="xa-video-tile__title xa-cat-2">{tin.tieu_de}</span>
      {tin.viewCount !== undefined && <span className="xa-phu">{TIN_XA.view_count(tin.viewCount)}</span>}
    </button>
  );
}

function VideoList(props: { ten_mien: string; category: string | null; onMo: (id: string) => void }) {
  const news = useTinXa(props.ten_mien, "video", props.category);
  const { ds } = news;
  const state = <ListState ds={ds} onRetry={news.taiTiep} icon="video" />;
  if (!ds.da_co_trang_dau) return state;
  if (ds.muc.length === 0 && ds.loi === null) {
    return <KhoiTrangThai bieu_tuong="video" cau={XA_TN.video_trong} hint={XA_TN.video_empty_hint} />;
  }
  return (
    <>
      <ul className="xa-video-grid">
        {ds.muc.map((t) => (
          <li key={t.id}>
            <VideoTile tin={t} onMo={props.onMo} />
          </li>
        ))}
      </ul>
      <MoreButton ds={ds} onMore={news.taiTiep} />
    </>
  );
}

/** The Video screen: header, "Chủ đề" chips, the grid (keyed by the chosen topic — one fresh load each). */
export function VideoScreen(props: { ten_mien: string; onBack: () => void; onMo: (id: string) => void }) {
  const rows = useNewsCategories(props.ten_mien, "video");
  const [topic, setTopic] = useState<string | null>(null);
  return (
    <>
      <DauManCon tieu_de={XA_TN.video_tieu_de} onQuayLai={props.onBack} />
      <div className="xa-trang xa-trang--con">
        <TopicChips label={XA_TN.video_topics} rows={rows} value={topic} onChange={setTopic} />
        <VideoList key={topic ?? ""} ten_mien={props.ten_mien} category={topic} onMo={props.onMo} />
      </div>
    </>
  );
}

/* ═══════════════════════════════ TRUYỀN THANH ═══════════════════════════════ */

/** The three speeds of the prototype's player (`BroadcastPage.tsx:25`), in the order a tap cycles them. */
export const SPEEDS = [0.75, 1, 1.25] as const;
export type Speed = (typeof SPEEDS)[number];

/** The next speed after `s`, wrapping. PURE. */
export function nextSpeed(s: Speed): Speed {
  return SPEEDS[(SPEEDS.indexOf(s) + 1) % SPEEDS.length] ?? 1;
}

/** "0,75" · "1" · "1,25" — the Vietnamese decimal comma. PURE. */
export function speedLabel(s: Speed): string {
  return String(s).replace(".", ",");
}

/**
 * Items grouped by their publication DAY, newest group first, in the server's order within a day. PURE.
 * `today` is a `YYYY-MM-DD` in Vietnam (`todayVN`); an unreadable day goes to its own group under its raw words.
 */
export function groupByDay(items: readonly TinXaTomTat[], today: string): Array<{ day: string; items: TinXaTomTat[] }> {
  const groups: Array<{ day: string; items: TinXaTomTat[] }> = [];
  for (const t of items) {
    const day = formatDayGroup(t.ngay_dang, today) ?? t.ngay_dang;
    const last = groups[groups.length - 1];
    if (last !== undefined && last.day === day) last.items.push(t);
    else groups.push({ day, items: [t] });
  }
  return groups;
}

/** "25/9/2026" for a row's spoken name — a broadcast is named by its title AND its day. */
function dayWords(day: string): string {
  const d = ngayVN(day);
  if (d === null) return day;
  const [dd, mm, yyyy] = d.split("/") as [string, string, string];
  return `${Number(dd)}/${Number(mm)}/${yyyy}`;
}

/**
 * One broadcast row (`BroadcastPage.tsx:185-236`): the round button (≥ 56px) that plays it HERE, the title,
 * "chuyên mục — thời lượng", and two lines of its summary. An item with no playable file has no round button: the
 * row opens the item instead. The playing row is marked by its button's words and icon AND a tint — not the tint alone.
 */
export function BroadcastRow(props: {
  tin: TinXaTomTat;
  active: boolean;
  playing: boolean;
  onPlay: () => void;
  onOpen: () => void;
}) {
  const { tin } = props;
  const length = tin.audio?.durationSeconds;
  const meta = [tin.chuyen_muc, length !== undefined ? durationWords(length) : ""].filter((p) => p !== "").join(" — ");
  const text = (
    <>
      <strong className="xa-broadcast-row__title">{tin.tieu_de}</strong>
      {meta !== "" && <span className="xa-phu">{meta}</span>}
      {tin.tom_tat.trim() !== "" && <span className="xa-broadcast-row__summary xa-cat-2">{tin.tom_tat}</span>}
    </>
  );
  return (
    <div className={`xa-the xa-broadcast-row${props.active ? " xa-broadcast-row--active" : ""}`}>
      {tin.audio !== undefined ? (
        <>
          <button
            type="button"
            className="xa-broadcast-row__play"
            aria-label={(props.playing ? XA_TN.broadcast_row_pause : XA_TN.broadcast_row_play)(tin.tieu_de, dayWords(tin.ngay_dang))}
            onClick={props.onPlay}
          >
            <BieuTuong ten={props.playing ? "pause" : "play-triangle"} co={26} fill="currentColor" />
          </button>
          <span className="xa-broadcast-row__text">{text}</span>
        </>
      ) : (
        <button type="button" className="xa-broadcast-row__open" aria-label={XA_TN.broadcast_open_item(tin.tieu_de)} onClick={props.onOpen}>
          <span className="xa-broadcast-row__text">{text}</span>
          <BieuTuong ten="chevron-right" co={22} />
        </button>
      )}
    </div>
  );
}

/**
 * The fixed bar under the list (`BroadcastPage.tsx:238-329`): what is playing, the time, the seek slider, ±15 s
 * (each with the words "15 giây" under its icon), the big play/pause, and the speed. Every control ≥ 44px; every
 * one named in words. PURE: the state is the caller's.
 */
export function BroadcastBar(props: {
  tin: TinXaTomTat;
  phase: PlayerPhase;
  elapsed: number;
  total: number | null;
  speed: Speed;
  onToggle: () => void;
  onSeek: (seconds: number) => void;
  onSpeed: () => void;
}) {
  const { phase, total } = props;
  const active = phase !== "idle" && phase !== "failed";
  const elapsed = total === null ? props.elapsed : Math.min(props.elapsed, total);
  const playing = phase === "playing";
  const busy = phase === "loading";
  return (
    <section className="xa-broadcast-bar" aria-label={XA_TN.broadcast_player}>
      <p className="xa-broadcast-bar__title">
        {props.tin.tieu_de} {dayWords(props.tin.ngay_dang)}
      </p>
      <div className="xa-broadcast-bar__seek">
        <span className="xa-broadcast-bar__time">{formatClock(elapsed)}</span>
        <SeekBar
          label={XA_TN.broadcast_seek}
          valueText={XA_TN.broadcast_position_text(durationWords(elapsed), durationWords(total ?? 0))}
          max={total ?? 0}
          value={elapsed}
          disabled={total === null || !active}
          onSeek={props.onSeek}
        />
        <span className="xa-broadcast-bar__time">{total === null ? "" : formatClock(total)}</span>
      </div>
      <div className="xa-broadcast-bar__controls">
        <button
          type="button"
          className="xa-broadcast-bar__skip"
          aria-label={XA_TN.broadcast_skip_back(SKIP_SECONDS)}
          disabled={!active}
          onClick={() => props.onSeek(elapsed - SKIP_SECONDS)}
        >
          <BieuTuong ten="rotate-ccw" co={24} />
          <span aria-hidden="true">{XA_TN.broadcast_skip_label(SKIP_SECONDS)}</span>
        </button>
        <button
          type="button"
          className="xa-broadcast-bar__toggle"
          aria-label={playing ? XA_TN.broadcast_pause : busy ? XA_TN.broadcast_loading : XA_TN.broadcast_bar_play}
          aria-busy={busy || undefined}
          onClick={props.onToggle}
        >
          <BieuTuong ten={playing ? "pause" : "play-triangle"} co={30} fill="currentColor" />
        </button>
        <button
          type="button"
          className="xa-broadcast-bar__skip"
          aria-label={XA_TN.broadcast_skip_forward(SKIP_SECONDS)}
          disabled={!active}
          onClick={() => props.onSeek(elapsed + SKIP_SECONDS)}
        >
          <BieuTuong ten="rotate-cw" co={24} />
          <span aria-hidden="true">{XA_TN.broadcast_skip_label(SKIP_SECONDS)}</span>
        </button>
        <button
          type="button"
          className="xa-broadcast-bar__speed"
          aria-label={XA_TN.broadcast_speed(speedLabel(props.speed))}
          onClick={props.onSpeed}
        >
          {speedLabel(props.speed)}x
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

function BroadcastList(props: {
  ten_mien: string;
  category: { id: string; name: string } | null;
  currentId: string | null;
  playing: boolean;
  onPlay: (t: TinXaTomTat) => void;
  onMo: (id: string) => void;
}) {
  const news = useTinXa(props.ten_mien, "truyen-thanh", props.category?.id ?? null);
  const { ds } = news;
  const state = <ListState ds={ds} onRetry={news.taiTiep} icon="speaker" />;
  if (!ds.da_co_trang_dau) return state;
  if (ds.muc.length === 0 && ds.loi === null) {
    return props.category === null ? (
      <KhoiTrangThai bieu_tuong="speaker" cau={XA_TN.broadcast_empty} hint={XA_TN.broadcast_empty_hint} />
    ) : (
      <KhoiTrangThai
        bieu_tuong="speaker"
        cau={XA_TN.broadcast_category_empty(props.category.name)}
        hint={XA_TN.news_category_empty_hint}
      />
    );
  }
  return (
    <>
      {groupByDay(ds.muc, todayVN()).map((g) => (
        <section key={g.day} className="xa-broadcast-day">
          <h2 className="xa-broadcast-day__title">{g.day}</h2>
          <ul className="xa-ds">
            {g.items.map((t) => (
              <li key={t.id}>
                <BroadcastRow
                  tin={t}
                  active={props.currentId === t.id}
                  playing={props.playing && props.currentId === t.id}
                  onPlay={() => props.onPlay(t)}
                  onOpen={() => props.onMo(t.id)}
                />
              </li>
            ))}
          </ul>
        </section>
      ))}
      <MoreButton ds={ds} onMore={news.taiTiep} />
    </>
  );
}

/**
 * The Truyền thanh screen: header "Truyền thanh xã", "Chuyên mục" chips, the day-grouped list, and — once something
 * was tapped — the bar. ONE `<audio>` for the screen; a tap on another row stops the current one first. The
 * playback rules (one re-read of an expired link per attempt, stop when the app goes to the background or the
 * screen closes) are `createPlayback`'s, shared with the article's player.
 */
export function BroadcastScreen(props: { ten_mien: string; onBack: () => void; onMo: (id: string) => void }) {
  const rows = useNewsCategories(props.ten_mien, "truyen-thanh");
  const [categoryId, setCategoryId] = useState<string | null>(null);
  const category = categoryId === null ? null : (rows?.roots.find((c) => c.id === categoryId) ?? null);

  const ref = useRef<HTMLAudioElement>(null);
  const control = useRef<Playback | null>(null);
  const [current, setCurrent] = useState<TinXaTomTat | null>(null);
  const [phase, setPhase] = useState<PlayerPhase>("idle");
  const [elapsed, setElapsed] = useState(0);
  const [mediaLength, setMediaLength] = useState<number | null>(null);
  const [speed, setSpeed] = useState<Speed>(1);

  useEffect(() => {
    const leave = () => {
      if (document.visibilityState === "hidden") control.current?.stop();
    };
    const gone = () => control.current?.stop();
    document.addEventListener("visibilitychange", leave);
    window.addEventListener("pagehide", gone);
    return () => {
      document.removeEventListener("visibilitychange", leave);
      window.removeEventListener("pagehide", gone);
      control.current?.stop();
    };
  }, []);

  const applySpeed = (s: Speed) => {
    const m = ref.current;
    if (m === null) return;
    m.defaultPlaybackRate = s;
    m.playbackRate = s;
  };

  /** A row's button: the same broadcast toggles; another one stops this one and starts that one — inside the tap. */
  function play(t: TinXaTomTat) {
    if (t.audio === undefined) return;
    if (current?.id === t.id && control.current !== null) {
      void control.current.toggle();
      return;
    }
    control.current?.stop();
    const c = createPlayback({
      media: () => ref.current,
      initial: t.audio,
      refresh: () => rereadAudio(props.ten_mien, t.id),
      now: () => Date.now(),
      onPhase: setPhase,
    });
    control.current = c;
    setCurrent(t);
    setElapsed(0);
    setMediaLength(null);
    void c.toggle();
    applySpeed(speed);
  }

  const total = current?.audio?.durationSeconds ?? mediaLength;
  const seek = (s: number) => {
    const m = ref.current;
    if (m === null) return;
    const max = total ?? Number.POSITIVE_INFINITY;
    m.currentTime = Math.max(0, Math.min(s, max));
    setElapsed(m.currentTime);
  };

  return (
    <>
      <DauManCon tieu_de={XA_TN.truyen_thanh_tieu_de} onQuayLai={props.onBack} />
      <div className="xa-trang xa-trang--con">
        <TopicChips label={XA_TN.broadcast_categories} rows={rows} value={categoryId} onChange={setCategoryId} />
        <BroadcastList
          key={categoryId ?? ""}
          ten_mien={props.ten_mien}
          category={category === null ? null : { id: category.id, name: category.name }}
          currentId={current?.id ?? null}
          playing={phase === "playing" || phase === "loading"}
          onPlay={play}
          onMo={props.onMo}
        />
      </div>
      <audio
        ref={ref}
        preload="none"
        onPlaying={() => {
          applySpeed(speed);
          control.current?.playing();
        }}
        onPause={() => control.current?.paused()}
        onEnded={() => control.current?.paused()}
        onError={() => void control.current?.failed()}
        onLoadedMetadata={(e) => {
          const d = e.currentTarget.duration;
          if (Number.isFinite(d) && d > 0) setMediaLength(Math.round(d));
          control.current?.metadataLoaded();
        }}
        onTimeUpdate={(e) => {
          control.current?.timeUpdate(e.currentTarget.currentTime);
          setElapsed(e.currentTarget.currentTime);
        }}
      />
      {current !== null && (
        <BroadcastBar
          tin={current}
          phase={phase}
          elapsed={elapsed}
          total={total}
          speed={speed}
          onToggle={() => void control.current?.toggle()}
          onSeek={seek}
          onSpeed={() => {
            const s = nextSpeed(speed);
            setSpeed(s);
            applySpeed(s);
          }}
        />
      )}
    </>
  );
}
