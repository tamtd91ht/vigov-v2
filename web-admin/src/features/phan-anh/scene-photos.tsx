"use client";

import { useEffect, useRef, useState, type KeyboardEvent } from "react";

import { listPetitionPhotos } from "@/lib/api/phieu-phan-anh";
import type { petitions_photoLinkOut, petitions_photoListOut } from "@/lib/api/schema.gen";
import type { CallResult } from "@/lib/api/task-attachments";

import {
  nhanThoiDiem,
  photoLinkUsable,
  SCENE_PHOTO_BROKEN,
  SCENE_PHOTO_CLOSE,
  SCENE_PHOTO_GONE,
  SCENE_PHOTO_NEXT,
  SCENE_PHOTO_PREVIOUS,
  SCENE_PHOTOS_AFTER,
  SCENE_PHOTOS_AFTER_NOT_BUILT,
  SCENE_PHOTOS_BEFORE,
  SCENE_PHOTOS_EMPTY,
  SCENE_PHOTOS_LOADING,
  SCENE_PHOTOS_REFRESHING,
  SCENE_PHOTOS_RETRY,
  SCENE_PHOTOS_TITLE,
  SCENE_PHOTOS_UNAVAILABLE,
  scenePhotoAlt,
  scenePhotoOpenLabel,
  scenePhotoSrc,
} from "./nhan-phieu";

/**
 * §8.4 `Ảnh trước và sau khi xử lý` on the petition drawer — the "before" column is the photos the
 * citizen attached (`GET /api/v1/citizen-reports/{maTraCuu}/photos`, `feedback.read`).
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * EVERY `url` IS A SIGNED, SHORT-LIVED BEARER CREDENTIAL TO A CITIZEN'S PHOTOGRAPH (rule 3, rule 4
 * invariant 7). Three consequences shape this file:
 *
 *   1. ONE CALL WHEN THE SECTION IS SHOWN, never a timer. Each call signs fresh links over personal
 *      data; polling would multiply that for a screen left open over lunch.
 *   2. A LINK IS NEVER USED PAST `url_expires_at` (minus `PHOTO_LINK_MARGIN_MS`). Opening the full-size
 *      view checks the link first and re-reads the list when it is stale (`freshLinksFor`); a full-size
 *      image that still fails re-reads once more. Thumbnails already on screen are not re-requested —
 *      the browser holds their bytes — so they do not need a live link.
 *   3. The URL goes into `<img src>` and nowhere else: not logged, not in the address bar, not in an
 *      `<a href>` the officer could copy into a chat. `referrerPolicy="no-referrer"` keeps the admin
 *      page's address out of the object store's access log. No `loading="lazy"`: a lazy request can
 *      fire after the link has expired.
 *
 * A plain `<img>`, not `next/image`: the optimiser would fetch the presigned link server-side and cache
 * a bearer credential under the commune's own origin (same reason as `features/noi-dung/so-noi-dung.tsx`).
 * ─────────────────────────────────────────────────────────────────────────────────────────
 *
 * THE FULL-SIZE VIEW is the repo's in-page dialog (`role="dialog"`, `features/thu-chi/dot-thu-chi.tsx`,
 * `features/nhiem-vu/task-import-dialog.tsx`), not an overlay: focus moves to its title on open and back
 * to the thumbnail on close; `Esc` closes. No new library.
 *
 * Who may see it is decided by the SERVER (`feedback.read`, plus `feedback.restricted` for a `can-bo`
 * petition — answered with the detail's own 404). The drawer only hides the block from a session
 * without `feedback.read` (rule 5, forbidden #1: the UI hides, the server decides).
 */

type PhotoList = CallResult<petitions_photoListOut>;
type Reload = () => Promise<PhotoList>;

/** What the "before" column says. Pure — `ScenePhotosView` renders it, tests read it. */
export type ScenePhotosState =
  | { readonly kind: "loading" }
  | { readonly kind: "unavailable" }
  | { readonly kind: "error"; readonly message: string }
  | { readonly kind: "empty" }
  | { readonly kind: "list"; readonly items: readonly petitions_photoLinkOut[] };

/**
 * 503 is the ONE status read: the object store is not there, and the server's 503 sentence is the
 * citizen-upload one. Everything else is the server's message verbatim (404 = the detail's 404).
 */
export function scenePhotosState(result: PhotoList | null): ScenePhotosState {
  if (result === null) return { kind: "loading" };
  if (!result.ok) {
    return result.status === 503 ? { kind: "unavailable" } : { kind: "error", message: result.message };
  }
  const items = result.data.items ?? [];
  return items.length === 0 ? { kind: "empty" } : { kind: "list", items };
}

/** Outcome of making sure one photo has a usable link. */
export type FreshLinks =
  | { readonly kind: "fresh"; readonly items: readonly petitions_photoLinkOut[]; readonly refetched: boolean }
  | { readonly kind: "failed"; readonly result: PhotoList & { ok: false } };

/**
 * The links to use for showing `photoId` NOW: the current list when that photo's link is still usable,
 * otherwise a freshly read list. `force` re-reads regardless — the full-size image failed to load.
 *
 * The photo may be gone from a fresh list (the server is the authority); the caller checks.
 */
export async function freshLinksFor(
  items: readonly petitions_photoLinkOut[],
  photoId: string,
  now: Date,
  reload: Reload,
  force = false,
): Promise<FreshLinks> {
  const photo = items.find((p) => p.id === photoId);
  if (!force && photo !== undefined && photoLinkUsable(photo, now)) {
    return { kind: "fresh", items, refetched: false };
  }
  const result = await reload();
  if (!result.ok) return { kind: "failed", result };
  return { kind: "fresh", items: result.data.items ?? [], refetched: true };
}

export function ScenePhotos({
  lookupCode,
  load = listPetitionPhotos,
}: {
  lookupCode: string;
  /** Injected only by tests; the screen always reads the contract route. */
  load?: (lookupCode: string) => Promise<PhotoList>;
}) {
  const [retries, setRetries] = useState(0);
  const [loaded, setLoaded] = useState<{ key: string; result: PhotoList } | null>(null);
  const [openId, setOpenId] = useState<string | null>(null);
  const [refreshing, setRefreshing] = useState(false);
  const [notice, setNotice] = useState<string | null>(null);
  // A failed full-size image re-reads the list ONCE (`retriedFor`); a second failure is said
  // (`brokenFor`), not retried — otherwise a store refusing every request would loop.
  const [retriedFor, setRetriedFor] = useState<string | null>(null);
  const [brokenFor, setBrokenFor] = useState<string | null>(null);

  const key = `${lookupCode}|${retries}`;

  useEffect(() => {
    let dropped = false;
    load(lookupCode).then((result) => {
      if (!dropped) setLoaded({ key, result });
    });
    return () => {
      dropped = true;
    };
  }, [lookupCode, key, load]);

  // An answer for an EARLIER read (before "Tải lại ảnh") is not drawn as this one's.
  const current = loaded !== null && loaded.key === key ? loaded.result : null;
  const state = scenePhotosState(current);
  const items = state.kind === "list" ? state.items : [];

  async function show(photoId: string, force = false): Promise<void> {
    setNotice(null);
    const fresh = await freshLinksFor(
      items,
      photoId,
      new Date(),
      () => {
        setRefreshing(true);
        return load(lookupCode).finally(() => setRefreshing(false));
      },
      force,
    );
    if (fresh.kind === "failed") {
      // The re-read answered a refusal (e.g. 503, or 404 because the petition became restricted):
      // that answer replaces the list, and no stale link stays on screen.
      setOpenId(null);
      setLoaded({ key, result: fresh.result });
      return;
    }
    if (fresh.refetched) setLoaded({ key, result: { ok: true, data: { items: [...fresh.items] } } });
    if (fresh.items.some((p) => p.id === photoId)) {
      setOpenId(photoId);
    } else {
      setOpenId(null);
      setNotice(SCENE_PHOTO_GONE);
    }
  }

  function onFullImageError(photoId: string): void {
    if (retriedFor === photoId) {
      setBrokenFor(photoId);
      return;
    }
    setRetriedFor(photoId);
    void show(photoId, true);
  }

  function retry(): void {
    setOpenId(null);
    setNotice(null);
    setRetriedFor(null);
    setBrokenFor(null);
    setRetries((n) => n + 1);
  }

  const openIndex = openId === null ? -1 : items.findIndex((p) => p.id === openId);

  return (
    <ScenePhotosView
      lookupCode={lookupCode}
      state={state}
      openIndex={openIndex}
      refreshing={refreshing}
      notice={notice}
      fullImageBroken={openId !== null && brokenFor === openId}
      open={(id) => {
        setRetriedFor(null);
        setBrokenFor(null);
        void show(id);
      }}
      close={() => setOpenId(null)}
      retry={retry}
      onFullImageError={onFullImageError}
    />
  );
}

/**
 * Presentational half — no network, no effects beyond focus. Rendered to a string in tests.
 */
export function ScenePhotosView({
  lookupCode,
  state,
  openIndex = -1,
  refreshing = false,
  notice = null,
  fullImageBroken = false,
  open = () => {},
  close = () => {},
  retry = () => {},
  onFullImageError = () => {},
}: {
  lookupCode: string;
  state: ScenePhotosState;
  /** Index into `state.items` of the photo shown full size; -1 = none. */
  openIndex?: number;
  refreshing?: boolean;
  notice?: string | null;
  fullImageBroken?: boolean;
  open?: (photoId: string) => void;
  close?: () => void;
  retry?: () => void;
  onFullImageError?: (photoId: string) => void;
}) {
  const headingId = `tieu-de-anh-phieu-${lookupCode}`;
  const items = state.kind === "list" ? state.items : [];
  const opened = openIndex >= 0 ? items[openIndex] : undefined;
  const previousPhoto = openIndex > 0 ? items[openIndex - 1] : undefined;
  const nextPhoto = openIndex >= 0 ? items[openIndex + 1] : undefined;

  return (
    <section className="khoi-chi-tiet scene-photos" aria-labelledby={headingId}>
      <h4 id={headingId}>{SCENE_PHOTOS_TITLE}</h4>

      <h5>{SCENE_PHOTOS_BEFORE}</h5>
      {state.kind === "loading" && <p role="status">{SCENE_PHOTOS_LOADING}</p>}
      {state.kind === "empty" && <p className="trang-thai-rong">{SCENE_PHOTOS_EMPTY}</p>}
      {(state.kind === "unavailable" || state.kind === "error") && (
        <>
          <p className="thong-bao-loi" role="alert">
            {state.kind === "unavailable" ? SCENE_PHOTOS_UNAVAILABLE : state.message}
          </p>
          <button type="button" className="nut-phu" onClick={retry}>
            {SCENE_PHOTOS_RETRY}
          </button>
        </>
      )}
      {state.kind === "list" && (
        <ul aria-label={SCENE_PHOTOS_BEFORE}>
          {items.map((p, i) => {
            const thumbSrc = scenePhotoSrc(p);
            return (
              <li key={p.id}>
                <button
                  type="button"
                  id={thumbId(lookupCode, p.id)}
                  aria-label={scenePhotoOpenLabel(i, items.length)}
                  aria-pressed={i === openIndex}
                  onClick={() => open(p.id)}
                >
                  {thumbSrc !== null ? (
                    // eslint-disable-next-line @next/next/no-img-element -- see the file header
                    <img
                      src={thumbSrc}
                      alt={scenePhotoAlt(i, items.length)}
                      referrerPolicy="no-referrer"
                      decoding="async"
                    />
                  ) : (
                    <span>{scenePhotoAlt(i, items.length)}</span>
                  )}
                </button>
              </li>
            );
          })}
        </ul>
      )}
      {refreshing && <p role="status">{SCENE_PHOTOS_REFRESHING}</p>}
      {notice !== null && <p className="ghi-chu">{notice}</p>}

      {opened !== undefined && (
        <FullSizePhoto
          lookupCode={lookupCode}
          photo={opened}
          index={openIndex}
          total={items.length}
          broken={fullImageBroken}
          previous={previousPhoto === undefined ? undefined : () => open(previousPhoto.id)}
          next={nextPhoto === undefined ? undefined : () => open(nextPhoto.id)}
          close={() => {
            close();
            document.getElementById(thumbId(lookupCode, opened.id))?.focus();
          }}
          retry={retry}
          onError={() => onFullImageError(opened.id)}
        />
      )}

      <h5>{SCENE_PHOTOS_AFTER}</h5>
      <p className="trang-thai-rong">{SCENE_PHOTOS_AFTER_NOT_BUILT}</p>
    </section>
  );
}

function thumbId(lookupCode: string, photoId: string): string {
  return `anh-phieu-${lookupCode}-${photoId}`;
}

function FullSizePhoto({
  lookupCode,
  photo,
  index,
  total,
  broken,
  previous,
  next,
  close,
  retry,
  onError,
}: {
  lookupCode: string;
  photo: petitions_photoLinkOut;
  index: number;
  total: number;
  broken: boolean;
  previous?: () => void;
  next?: () => void;
  close: () => void;
  retry: () => void;
  onError: () => void;
}) {
  const titleRef = useRef<HTMLHeadingElement>(null);
  const titleId = `tieu-de-anh-lon-${lookupCode}`;
  const src = scenePhotoSrc(photo);
  const label = scenePhotoAlt(index, total);

  useEffect(() => {
    titleRef.current?.focus();
  }, [photo.id]);

  function onKeyDown(e: KeyboardEvent<HTMLElement>): void {
    if (e.key === "Escape") {
      e.preventDefault();
      close();
    }
  }

  return (
    <section
      className="khoi-chi-tiet scene-photo-full"
      role="dialog"
      aria-labelledby={titleId}
      onKeyDown={onKeyDown}
    >
      <h5 id={titleId} ref={titleRef} tabIndex={-1}>
        {label}
      </h5>
      <p className="ghi-chu">Người dân gửi lúc {nhanThoiDiem(photo.created_at)}</p>
      {src !== null && !broken ? (
        // eslint-disable-next-line @next/next/no-img-element -- see the file header
        <img src={src} alt={label} referrerPolicy="no-referrer" decoding="async" onError={onError} />
      ) : (
        <>
          <p className="thong-bao-loi" role="alert">
            {SCENE_PHOTO_BROKEN}
          </p>
          <button type="button" className="nut-phu" onClick={retry}>
            {SCENE_PHOTOS_RETRY}
          </button>
        </>
      )}
      <div className="cum-nut">
        {previous !== undefined && (
          <button type="button" className="nut-phu" onClick={previous}>
            {SCENE_PHOTO_PREVIOUS}
          </button>
        )}
        {next !== undefined && (
          <button type="button" className="nut-phu" onClick={next}>
            {SCENE_PHOTO_NEXT}
          </button>
        )}
        <button type="button" className="nut-phu" onClick={close}>
          {SCENE_PHOTO_CLOSE}
        </button>
      </div>
    </section>
  );
}
