"use client";

import { useRouter } from "next/navigation";
import { useCallback, useEffect, useRef, useState, type Ref } from "react";

import {
  listNotifications,
  markAllNotificationsRead,
  markNotificationRead,
  unreadCount,
  type StaffNotification,
} from "@/lib/api/notifications";

import {
  EMPTY_SENTENCE,
  LOADING_SENTENCE,
  MORE_BUTTON,
  PAGE_SIZE,
  PANEL_TITLE,
  POLL_MS,
  READ_ALL_BUTTON,
  UNREAD_MARK,
  badgeText,
  bellLabel,
  itemTime,
  kindLabel,
  safeLink,
} from "./notification-bell-logic";

/** What the panel holds: the pages read so far, or the one sentence of a failed read. */
export type BellFeed =
  | { readonly phase: "closed" }
  | { readonly phase: "loading" }
  | { readonly phase: "failed"; readonly message: string }
  | {
      readonly phase: "ready";
      readonly items: readonly StaffNotification[];
      readonly nextCursor: string | null;
    };

/**
 * The header bell — any signed-in staff account (08-thong-bao §8; ADR 0058 §3).
 *
 * WHOSE NOTICES: the server's decision, from the session (`lib/api/notifications.ts`). Nothing here
 * names a recipient, so there is nothing a modified client could change to read another inbox.
 *
 * THE BADGE IS POLLED MODESTLY: every 60 s while the tab is visible, and again when the window regains
 * focus — the moment a person comes back to the screen is the moment a stale count misleads. Hidden
 * tabs do not poll: 300 communes of open tabs polling at night is load for nobody.
 */
export function NotificationBell() {
  const router = useRouter();
  const [unread, setUnread] = useState<number | null>(null);
  const [feed, setFeed] = useState<BellFeed>({ phase: "closed" });
  const [actionError, setActionError] = useState("");
  const panelRef = useRef<HTMLDivElement | null>(null);
  const buttonRef = useRef<HTMLButtonElement | null>(null);

  // Bumped after a read / "Đọc hết" so the effect below re-reads the badge at once.
  const [countRead, setCountRead] = useState(0);
  const refreshCount = useCallback(() => setCountRead((n) => n + 1), []);

  useEffect(() => {
    let gone = false;
    const read = () => {
      void unreadCount().then((r) => {
        // A failed count draws NO number rather than a stale or invented one.
        if (!gone) setUnread(r.ok ? r.duLieu : null);
      });
    };
    read();
    const tick = window.setInterval(() => {
      if (document.visibilityState === "visible") read();
    }, POLL_MS);
    const onVisible = () => {
      if (document.visibilityState === "visible") read();
    };
    window.addEventListener("focus", read);
    document.addEventListener("visibilitychange", onVisible);
    return () => {
      gone = true;
      window.clearInterval(tick);
      window.removeEventListener("focus", read);
      document.removeEventListener("visibilitychange", onVisible);
    };
  }, [countRead]);

  const close = useCallback(() => {
    setFeed({ phase: "closed" });
    setActionError("");
  }, []);

  // Escape closes the panel and gives focus back to the bell (WAI-ARIA disclosure pattern).
  useEffect(() => {
    if (feed.phase === "closed") return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        close();
        buttonRef.current?.focus();
      }
    };
    const onDown = (e: MouseEvent) => {
      const t = e.target as Node | null;
      if (t !== null && !panelRef.current?.contains(t) && !buttonRef.current?.contains(t)) close();
    };
    document.addEventListener("keydown", onKey);
    document.addEventListener("mousedown", onDown);
    return () => {
      document.removeEventListener("keydown", onKey);
      document.removeEventListener("mousedown", onDown);
    };
  }, [feed.phase, close]);

  async function loadPage(cursor: string | null, before: readonly StaffNotification[]) {
    const r = await listNotifications(cursor, PAGE_SIZE);
    if (!r.ok) {
      setFeed({ phase: "failed", message: r.thongBao });
      return;
    }
    setFeed({
      phase: "ready",
      items: [...before, ...r.duLieu.items],
      nextCursor: r.duLieu.has_more && r.duLieu.next_cursor !== "" ? r.duLieu.next_cursor : null,
    });
  }

  function toggle() {
    if (feed.phase !== "closed") {
      close();
      return;
    }
    setFeed({ phase: "loading" });
    setActionError("");
    void loadPage(null, []);
    refreshCount();
  }

  async function open(item: StaffNotification) {
    setActionError("");
    if (!item.read) {
      const r = await markNotificationRead(item.id);
      if (!r.ok) {
        // Still follow the link: the person asked to go there; the dot stays, and the sentence says why.
        setActionError(r.thongBao);
      } else {
        setFeed((f) =>
          f.phase === "ready" ? { ...f, items: f.items.map((n) => (n.id === item.id ? r.duLieu : n)) } : f,
        );
        refreshCount();
      }
    }
    const to = safeLink(item.link);
    if (to !== null) {
      close();
      router.push(to);
    }
  }

  async function readAll() {
    setActionError("");
    const r = await markAllNotificationsRead();
    if (!r.ok) {
      setActionError(r.thongBao);
      return;
    }
    // Re-read rather than patch every row by hand: the server is the state, and its count is the badge.
    void loadPage(null, []);
    refreshCount();
  }

  return (
    <NotificationBellView
      unread={unread}
      feed={feed}
      actionError={actionError}
      buttonRef={buttonRef}
      panelRef={panelRef}
      onToggle={toggle}
      onOpen={(n) => void open(n)}
      onReadAll={() => void readAll()}
      onMore={() => {
        if (feed.phase === "ready" && feed.nextCursor !== null) void loadPage(feed.nextCursor, feed.items);
      }}
    />
  );
}

const PANEL_ID = "bang-thong-bao-chuong";

/**
 * Pure rendering — exported so every state has a test without a router. Every string it draws goes
 * through React as TEXT (`title`, `body`, `link` included): there is no HTML path here at all.
 */
export function NotificationBellView({
  unread,
  feed,
  actionError,
  buttonRef,
  panelRef,
  onToggle,
  onOpen,
  onReadAll,
  onMore,
}: {
  unread: number | null;
  feed: BellFeed;
  actionError: string;
  buttonRef?: Ref<HTMLButtonElement>;
  panelRef?: Ref<HTMLDivElement>;
  onToggle: () => void;
  onOpen: (n: StaffNotification) => void;
  onReadAll: () => void;
  onMore: () => void;
}) {
  const badge = badgeText(unread);
  const isOpen = feed.phase !== "closed";
  const anyUnread = feed.phase === "ready" && feed.items.some((n) => !n.read);
  return (
    <div className="chuong-thong-bao">
      <button
        ref={buttonRef}
        type="button"
        className="nut-chuong"
        aria-label={bellLabel(unread)}
        aria-expanded={isOpen}
        aria-controls={PANEL_ID}
        onClick={onToggle}
      >
        <svg aria-hidden="true" focusable="false" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
          <path d="M18 8a6 6 0 0 0-12 0c0 7-3 9-3 9h18s-3-2-3-9" />
          <path d="M13.73 21a2 2 0 0 1-3.46 0" />
        </svg>
        {badge !== null && (
          <span className="huy-hieu-chuong" aria-hidden="true">
            {badge}
          </span>
        )}
      </button>

      {isOpen && (
        <div ref={panelRef} id={PANEL_ID} className="bang-chuong" role="region" aria-label={PANEL_TITLE}>
          <div className="dau-bang-chuong">
            <h2>{PANEL_TITLE}</h2>
            {(anyUnread || (unread ?? 0) > 0) && (
              <button type="button" className="nut-phu" onClick={onReadAll}>
                {READ_ALL_BUTTON}
              </button>
            )}
          </div>

          {actionError !== "" && (
            <p className="thong-bao-loi" role="alert">
              {actionError}
            </p>
          )}

          {feed.phase === "loading" && <p role="status">{LOADING_SENTENCE}</p>}
          {feed.phase === "failed" && (
            <p className="thong-bao-loi" role="alert">
              {feed.message}
            </p>
          )}
          {feed.phase === "ready" && feed.items.length === 0 && (
            <p className="trang-thai-rong">{EMPTY_SENTENCE}</p>
          )}
          {feed.phase === "ready" && feed.items.length > 0 && (
            <ul className="ds-chuong">
              {feed.items.map((n) => (
                <li key={n.id} className={n.read ? "muc-chuong" : "muc-chuong chua-doc"}>
                  <button type="button" onClick={() => onOpen(n)}>
                    <span className="loai-chuong">{kindLabel(n.kind)}</span>
                    {!n.read && <span className="an-thi-giac">{UNREAD_MARK}</span>}
                    <span className="tieu-de-chuong">{n.title}</span>
                    {n.body !== "" && <span className="dong-phu-chuong">{n.body}</span>}
                    <span className="gio-chuong">{itemTime(n.created_at)}</span>
                  </button>
                </li>
              ))}
            </ul>
          )}
          {feed.phase === "ready" && feed.nextCursor !== null && (
            <p>
              <button type="button" className="nut-phu" onClick={onMore}>
                {MORE_BUTTON}
              </button>
            </p>
          )}
        </div>
      )}
    </div>
  );
}
