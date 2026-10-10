"use client";

import { CloudOff, ImageUp, Lock, RefreshCw, ShieldX, X } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { toast } from "sonner";

import { Notice } from "@/components/ui/notice";
import { cn } from "@/lib/cn";
import { listVerificationPhotos, uploadVerificationPhoto } from "@/lib/api/phieu-phan-anh";
import type { petitions_photoLinkOut, petitions_photoListOut, petitions_photoOut } from "@/lib/api/schema.gen";
import type { CallResult } from "@/lib/api/task-attachments";
import { isTransientUpload, type UploadResult } from "@/lib/api/upload";
import { formatBytes } from "@/features/nhiem-vu/task-attachments";

import {
  AFTER_PHOTO_ACCEPT,
  AFTER_PHOTO_CLOSED,
  AFTER_PHOTO_INPUT_LABEL,
  AFTER_PHOTO_NOTE,
  AFTER_PHOTO_TOAST,
  AFTER_PHOTO_UPLOAD_BUTTON,
  AFTER_PHOTO_UPLOAD_DENIED,
  AFTER_PHOTOS_EMPTY,
  AFTER_PHOTOS_LOADING,
  AFTER_PHOTOS_UNAVAILABLE,
  afterPhotoAlt,
  afterPhotoType,
  afterPhotoUploadOpen,
  nhanThoiDiem,
  SCENE_PHOTO_BROKEN,
  SCENE_PHOTO_CLOSE,
  SCENE_PHOTO_GONE,
  SCENE_PHOTOS_REFRESHING,
  SCENE_PHOTOS_RETRY,
  scenePhotoSrc,
} from "./nhan-phieu";
import { buttonClass, Glyph } from "./petition-ui";
import { EmptyPhotoBox, freshLinksFor, scenePhotosState, THUMB_GRID, type ScenePhotosState } from "./scene-photos";

/**
 * §8.4 `Sau khi xử lý` — the verification photos staff upload (owner, 02/10/2026; ADR 0047 row "Ảnh
 * 'sau xử lý' của cán bộ — THAY G8"; ADR 0008 decision 3).
 *
 *   list    GET  …/verification-photos                 feedback.read, AUDITED, links ≤ 15 min
 *   upload  POST …/verification-photos (multipart)     feedback.resolve, every status but the 3 endings
 *
 * THE SAME DISCIPLINE AS THE "BEFORE" COLUMN (`scene-photos.tsx`, whose helpers are reused): one list
 * call when the column is shown, one after a stored upload, one when a link has expired or a full-size
 * image failed — NEVER on a timer (each call signs fresh links over a photograph and writes an audit
 * entry). The URL goes into `<img src>` only, `referrerPolicy="no-referrer"`, never `loading="lazy"`.
 *
 * THE UPLOAD IS ONE MULTIPART REQUEST (ADR 0052 §Sửa đổi 09/10/2026, `lib/api/upload.ts`), one
 * Idempotency-Key per attempt; the server scans, re-encodes without EXIF and stores in that request. A
 * refusal of the moment (`isTransientUpload`) offers "Gửi lại" — the same file again, nothing was stored;
 * every other refusal is final. Every refusal is the server's sentence verbatim (409 `photo_limit`: five
 * per processing round; 409 `petition_state`: the petition ended).
 *
 * THE BUTTON IS UX ONLY (rule 5, forbidden #1): it is drawn with `feedback.resolve` on a status the
 * server accepts; the server checks both on every call.
 */

type PhotoList = CallResult<petitions_photoListOut>;

/** One chosen file and where it stands. Local key. */
type UploadState =
  | { readonly kind: "uploading"; readonly percent: number }
  | { readonly kind: "checking" }
  | { readonly kind: "stored" }
  /** A refusal of the moment: nothing was stored — "Gửi lại" sends the same file again. */
  | { readonly kind: "retry"; readonly message: string }
  | { readonly kind: "refused"; readonly message: string };

type UploadItem = { readonly key: string; readonly name: string; readonly size: number; readonly state: UploadState };

export const AFTER_PHOTO_RETRY_BUTTON = "Gửi lại";

/** An upload's answer → the file's state. `stored` is the only success (`domain.StoredFileStored`). */
export function afterPhotoUpload(r: UploadResult<petitions_photoOut>): UploadState {
  if (r.ok) {
    return r.data.status === "stored"
      ? { kind: "stored" }
      : { kind: "refused", message: "Ảnh chưa được lưu. Hãy chọn ảnh và tải lên lại." };
  }
  if (isTransientUpload(r)) return { kind: "retry", message: r.message };
  return { kind: "refused", message: r.message };
}

function uploadStateText(s: UploadState): string {
  switch (s.kind) {
    case "uploading":
      return `Đang tải ${s.percent}%`;
    case "checking":
      return "Đang kiểm tra (quét mã độc, xử lý ảnh)…";
    case "stored":
      return "Đã lưu vào phiếu.";
    case "retry":
      return `Chưa gửi được: ${s.message}`;
    case "refused":
      return `Bị từ chối: ${s.message}`;
  }
}

export type VerificationDeps = {
  load: (lookupCode: string) => Promise<PhotoList>;
  upload: typeof uploadVerificationPhoto;
};

const DEFAULT_DEPS: VerificationDeps = {
  load: listVerificationPhotos,
  upload: uploadVerificationPhoto,
};

export function VerificationPhotos({
  lookupCode,
  status,
  canUpload,
  deps = DEFAULT_DEPS,
}: {
  lookupCode: string;
  /** The petition's status — the upload is offered only where the server accepts one. */
  status: string;
  /** The session holds `feedback.resolve`. UX only. */
  canUpload: boolean;
  /** Injected only by tests; the screen always calls the contract routes. */
  deps?: VerificationDeps;
}) {
  const [reads, setReads] = useState(0);
  const [loaded, setLoaded] = useState<{ key: number; result: PhotoList } | null>(null);
  const [openId, setOpenId] = useState<string | null>(null);
  const [refreshing, setRefreshing] = useState(false);
  const [notice, setNotice] = useState<string | null>(null);
  const [retriedFor, setRetriedFor] = useState<string | null>(null);
  const [brokenFor, setBrokenFor] = useState<string | null>(null);
  const [uploads, setUploads] = useState<readonly UploadItem[]>([]);
  const seq = useRef(0);
  // The chosen files, kept (browser memory, this column only) so "Gửi lại" can send the same bytes.
  const picked = useRef(new Map<string, File>());

  useEffect(() => {
    let dropped = false;
    deps.load(lookupCode).then((result) => {
      if (!dropped) setLoaded({ key: reads, result });
    });
    return () => {
      dropped = true;
    };
  }, [deps, lookupCode, reads]);

  const current = loaded !== null && loaded.key === reads ? loaded.result : null;
  // While a re-read after an upload is in flight, keep showing the previous list rather than a blank.
  const shown = current ?? (loaded !== null && reads > 0 ? loaded.result : null);
  const state = scenePhotosState(shown);
  const items = state.kind === "list" ? state.items : [];

  async function show(photoId: string, force = false): Promise<void> {
    setNotice(null);
    const fresh = await freshLinksFor(
      items,
      photoId,
      new Date(),
      () => {
        setRefreshing(true);
        return deps.load(lookupCode).finally(() => setRefreshing(false));
      },
      force,
    );
    if (fresh.kind === "failed") {
      setOpenId(null);
      setLoaded({ key: reads, result: fresh.result });
      return;
    }
    if (fresh.refetched) setLoaded({ key: reads, result: { ok: true, data: { items: [...fresh.items] } } });
    if (fresh.items.some((p) => p.id === photoId)) {
      setOpenId(photoId);
    } else {
      setOpenId(null);
      setNotice(SCENE_PHOTO_GONE);
    }
  }

  const set = (key: string, s: UploadState) =>
    setUploads((list) => list.map((i) => (i.key === key ? { ...i, state: s } : i)));

  async function start(key: string, file: File): Promise<void> {
    const t = afterPhotoType(file);
    if (!t.ok) {
      set(key, { kind: "refused", message: t.message });
      return;
    }
    set(key, { kind: "uploading", percent: 0 });
    const next = afterPhotoUpload(
      await deps.upload(lookupCode, file, t.contentType, crypto.randomUUID(), {
        onProgress: (percent) => set(key, { kind: "uploading", percent }),
        onSent: () => set(key, { kind: "checking" }),
      }),
    );
    set(key, next);
    // A stored photo is on the petition now: read the list ONCE more to show it.
    if (next.kind === "stored") {
      setReads((n) => n + 1);
      toast.success(AFTER_PHOTO_TOAST);
    }
  }

  function add(files: readonly File[]): void {
    for (const f of files) {
      seq.current += 1;
      const key = `anh-sau-${seq.current}`;
      picked.current.set(key, f);
      setUploads((list) => [...list, { key, name: f.name, size: f.size, state: { kind: "uploading", percent: 0 } }]);
      void start(key, f);
    }
  }

  return (
    <VerificationPhotosView
      lookupCode={lookupCode}
      state={state}
      openIndex={openId === null ? -1 : items.findIndex((p) => p.id === openId)}
      refreshing={refreshing}
      notice={notice}
      fullImageBroken={openId !== null && brokenFor === openId}
      // An ended petition says so to everyone; the missing key is said only where an upload could happen.
      upload={!afterPhotoUploadOpen(status) ? "closed" : canUpload ? "open" : "denied"}
      uploads={uploads}
      open={(id) => {
        setRetriedFor(null);
        setBrokenFor(null);
        void show(id);
      }}
      close={() => setOpenId(null)}
      retry={() => {
        setOpenId(null);
        setNotice(null);
        setRetriedFor(null);
        setBrokenFor(null);
        setLoaded(null);
        setReads((n) => n + 1);
      }}
      onFullImageError={(id) => {
        if (retriedFor === id) {
          setBrokenFor(id);
          return;
        }
        setRetriedFor(id);
        void show(id, true);
      }}
      onAdd={add}
      onRetryUpload={(key) => {
        const it = uploads.find((i) => i.key === key);
        const f = picked.current.get(key);
        if (it !== undefined && it.state.kind === "retry" && f !== undefined) void start(key, f);
      }}
      onRemoveUpload={(key) => setUploads((list) => list.filter((i) => i.key !== key))}
    />
  );
}

/** Presentational half — no network. Rendered to a string in tests. */
export function VerificationPhotosView({
  lookupCode,
  state,
  openIndex = -1,
  refreshing = false,
  notice = null,
  fullImageBroken = false,
  upload,
  uploads = [],
  open = () => {},
  close = () => {},
  retry = () => {},
  onFullImageError = () => {},
  onAdd = () => {},
  onRetryUpload = () => {},
  onRemoveUpload = () => {},
}: {
  lookupCode: string;
  state: ScenePhotosState;
  openIndex?: number;
  refreshing?: boolean;
  notice?: string | null;
  fullImageBroken?: boolean;
  /** `open` = the button; `closed` = the petition ended; `denied` = no `feedback.resolve`. */
  upload: "open" | "closed" | "denied";
  uploads?: readonly UploadItem[];
  open?: (photoId: string) => void;
  close?: () => void;
  retry?: () => void;
  onFullImageError?: (photoId: string) => void;
  onAdd?: (files: File[]) => void;
  onRetryUpload?: (key: string) => void;
  onRemoveUpload?: (key: string) => void;
}) {
  const items = state.kind === "list" ? state.items : [];
  const opened: petitions_photoLinkOut | undefined = openIndex >= 0 ? items[openIndex] : undefined;
  const inputId = `anh-sau-xu-ly-${lookupCode}`;
  const busy = uploads.some((u) => u.state.kind === "uploading" || u.state.kind === "checking");

  return (
    <div className="verification-photos flex flex-col gap-3 [&>p]:m-0">
      {state.kind === "loading" && (
        <p role="status" className="text-sm text-ink-500">
          {AFTER_PHOTOS_LOADING}
        </p>
      )}
      {state.kind === "empty" && <EmptyPhotoBox>{AFTER_PHOTOS_EMPTY}</EmptyPhotoBox>}
      {(state.kind === "unavailable" || state.kind === "error") && (
        <div className="flex flex-wrap items-center gap-3">
          <Glyph icon={CloudOff} className="size-[18px] shrink-0 text-danger-600" />
          <p className="thong-bao-loi m-0 min-w-0 flex-1" role="alert">
            {state.kind === "unavailable" ? AFTER_PHOTOS_UNAVAILABLE : state.message}
          </p>
          <button type="button" className={buttonClass("secondary", "sm")} onClick={retry}>
            <Glyph icon={RefreshCw} />
            {SCENE_PHOTOS_RETRY}
          </button>
        </div>
      )}
      {state.kind === "list" && (
        <ul aria-label="Ảnh sau xử lý" className={cn(THUMB_GRID, "m-0 list-none p-0")}>
          {items.map((p, i) => {
            const src = scenePhotoSrc(p);
            return (
              <li key={p.id} className="min-w-0">
                <button
                  type="button"
                  aria-label={`Xem cỡ lớn ${afterPhotoAlt(i, items.length).toLowerCase()}`}
                  aria-pressed={i === openIndex}
                  onClick={() => open(p.id)}
                  className={cn(
                    "block h-32 w-full cursor-pointer overflow-hidden rounded-[10px] border border-line bg-surface-muted p-0",
                    "transition-[border-color,box-shadow] duration-150 hover:border-brand-500",
                    "focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-brand-500",
                    i === openIndex && "border-brand-600 ring-2 ring-brand-500",
                  )}
                >
                  {src !== null ? (
                    // eslint-disable-next-line @next/next/no-img-element -- signed link, see scene-photos.tsx
                    <img
                      src={src}
                      alt={afterPhotoAlt(i, items.length)}
                      referrerPolicy="no-referrer"
                      decoding="async"
                      className="block size-full object-cover"
                    />
                  ) : (
                    <span className="grid size-full place-items-center px-2 text-center text-xs text-ink-500">
                      {afterPhotoAlt(i, items.length)}
                    </span>
                  )}
                </button>
              </li>
            );
          })}
        </ul>
      )}
      {refreshing && (
        <p role="status" className="text-sm text-ink-500">
          {SCENE_PHOTOS_REFRESHING}
        </p>
      )}
      {notice !== null && <p className="ghi-chu">{notice}</p>}

      {opened !== undefined && (
        <section
          className="flex flex-col gap-3 rounded-xl border border-line bg-surface-muted p-3 [&>p]:m-0"
          aria-label={afterPhotoAlt(openIndex, items.length)}
        >
          <div className="flex flex-wrap items-baseline justify-between gap-2">
            <h6 className="m-0 text-sm font-semibold text-ink-900">{afterPhotoAlt(openIndex, items.length)}</h6>
            <p className="ghi-chu m-0">Tải lên lúc {nhanThoiDiem(opened.created_at)}</p>
          </div>
          {scenePhotoSrc(opened) !== null && !fullImageBroken ? (
            // eslint-disable-next-line @next/next/no-img-element -- signed link, see scene-photos.tsx
            <img
              src={scenePhotoSrc(opened) ?? undefined}
              alt={afterPhotoAlt(openIndex, items.length)}
              referrerPolicy="no-referrer"
              decoding="async"
              onError={() => onFullImageError(opened.id)}
              className="mx-auto block max-h-[70vh] w-auto max-w-full rounded-lg bg-surface object-contain"
            />
          ) : (
            <p className="thong-bao-loi" role="alert">
              {SCENE_PHOTO_BROKEN}
            </p>
          )}
          <div className="cum-nut justify-end">
            <button type="button" className={buttonClass("secondary", "sm")} onClick={close}>
              <Glyph icon={X} />
              {SCENE_PHOTO_CLOSE}
            </button>
          </div>
        </section>
      )}

      {upload === "open" && (
        <div className="flex flex-col gap-2">
          {/* The input first, visually hidden but focusable; its label is the visible button (same
              pattern as `AttachmentPicker`). Enter/Space on the focused input open the picker. */}
          <div>
            <input
              id={inputId}
              name={inputId}
              type="file"
              multiple
              accept={AFTER_PHOTO_ACCEPT}
              className="an-thi-giac peer"
              onChange={(e) => {
                const files = e.target.files === null ? [] : Array.from(e.target.files);
                e.target.value = "";
                if (files.length > 0) onAdd(files);
              }}
            />
            <label
              htmlFor={inputId}
              className={cn(
                buttonClass("secondary", "sm"),
                "peer-focus-visible:outline-2 peer-focus-visible:outline-offset-2 peer-focus-visible:outline-brand-500",
              )}
              aria-busy={busy}
            >
              <Glyph icon={ImageUp} />
              {AFTER_PHOTO_UPLOAD_BUTTON}
              <span className="an-thi-giac"> — {AFTER_PHOTO_INPUT_LABEL}</span>
            </label>
          </div>
          <p className="ghi-chu m-0">{AFTER_PHOTO_NOTE}</p>
          {uploads.length > 0 && (
            <ul aria-label="Ảnh sau xử lý đang tải lên" className="m-0 flex list-none flex-col gap-1.5 p-0 text-sm">
              {uploads.map((u) => {
                const refused = u.state.kind === "refused" || u.state.kind === "retry";
                return (
                  <li key={u.key} className="flex flex-wrap items-center gap-x-3 gap-y-1">
                    <span className="min-w-0 break-all">
                      {u.name} · {formatBytes(u.size)}
                    </span>
                    <span role={refused ? "alert" : "status"} className={refused ? "text-danger-600" : "text-ink-500"}>
                      {uploadStateText(u.state)}
                    </span>
                    {u.state.kind === "retry" && (
                      <button type="button" className={buttonClass("secondary", "sm")} onClick={() => onRetryUpload(u.key)}>
                        {AFTER_PHOTO_RETRY_BUTTON}
                      </button>
                    )}
                    {(refused || u.state.kind === "stored") && (
                      <button
                        type="button"
                        className={buttonClass("secondary", "sm")}
                        aria-label={`Bỏ ${u.name} khỏi danh sách`}
                        onClick={() => onRemoveUpload(u.key)}
                      >
                        Bỏ
                      </button>
                    )}
                  </li>
                );
              })}
            </ul>
          )}
        </div>
      )}
      {upload === "closed" && (
        <p className="inline-flex items-center gap-2 text-sm text-ink-500">
          <Glyph icon={Lock} className="size-4 shrink-0" />
          {AFTER_PHOTO_CLOSED}
        </p>
      )}
      {upload === "denied" && (
        <Notice tone="neutral" icon={ShieldX}>
          {AFTER_PHOTO_UPLOAD_DENIED}
        </Notice>
      )}
    </div>
  );
}
