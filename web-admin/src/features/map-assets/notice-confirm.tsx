"use client";

import { useEffect, useRef, useState, type ReactNode } from "react";

import { Button } from "@/components/ui/button";
import { BUSY_SAVING, BusyLabel } from "@/features/danh-ba/busy-label";
import { OverlayDialog } from "@/features/noi-dung/overlay-dialog";
import { getMapFrame, type FrameWriteResult } from "@/lib/api/map-frame";
import type { comms_mapFrameOut } from "@/lib/api/schema.gen";

import {
  MAP_FRAME_NOTICE_ACK,
  MAP_FRAME_NOTICE_ITEMS,
  MAP_FRAME_NOTICE_TITLE,
  MAP_FRAME_NOTICE_VERSION,
  NOTICE_CONFIRM_BUTTON,
  NOTICE_RELOADED,
  noticeOutdated,
} from "./labels";

/**
 * The legal-notice step of every frame change (ADR 0072 K4): the K6 text, a checkbox, and "Xác nhận
 * thay đổi" — disabled until the box is ticked. "Huỷ" (or Esc) returns WITHOUT sending anything.
 *
 * WHAT IS ACKNOWLEDGED IS WHAT WAS SHOWN. The text lives in this bundle (`labels.ts`, one version); the
 * server names its current version in every reply. When the two differ the officer would be ticking a
 * text they never read, so the confirm stays disabled and the page asks for a reload. When they match,
 * the version sent IS the server's. A 422 `notice_not_acknowledged` (the text moved on between the GET
 * and the save) reloads the frame, hands the reply up, and shows the step again — unticked.
 *
 * The server refuses a save without the acknowledgement (K4, rule 5 #1): this step is the explanation,
 * not the gate.
 *
 * ESC: handled here and STOPPED, so the screen's own Esc (collapse the expanded map) never sees it —
 * the screen also defers to any open dialog, this is the second lock, not the only one.
 */
export function NoticeConfirm({
  titleId,
  serverNoticeVersion,
  lead,
  send,
  onDone,
  onReloaded,
  onCancel,
}: {
  titleId: string;
  /** `notice_version` of the latest map-frame reply. */
  serverNoticeVersion: string;
  /** What this change does, above the notice (the reset explains itself here). */
  lead?: ReactNode;
  /** The write, given the acknowledged version. */
  send: (noticeVersion: string) => Promise<FrameWriteResult>;
  onDone: (out: comms_mapFrameOut) => void;
  /** A fresh GET after a stale-notice refusal — the screen adopts it as the current frame. */
  onReloaded: (out: comms_mapFrameOut) => void;
  onCancel: () => void;
}) {
  const [checked, setChecked] = useState(false);
  const [sending, setSending] = useState(false);
  const [error, setError] = useState("");
  const [reloadedNote, setReloadedNote] = useState(false);
  const checkbox = useRef<HTMLInputElement>(null);
  const versionMatches = serverNoticeVersion === MAP_FRAME_NOTICE_VERSION;

  useEffect(() => {
    // First focusable of the step. `showModal()` would pick it too; jsdom has no `showModal`.
    checkbox.current?.focus();
  }, []);

  function cancel() {
    if (!sending) onCancel();
  }

  async function confirm() {
    if (sending || !checked || !versionMatches) return;
    setSending(true);
    setError("");
    setReloadedNote(false);
    const res = await send(MAP_FRAME_NOTICE_VERSION);
    if (res.ok) {
      setSending(false);
      return onDone(res.duLieu);
    }
    if (res.noticeStale) {
      const fresh = await getMapFrame();
      setSending(false);
      setChecked(false);
      if (!fresh.ok) return setError(fresh.thongBao);
      onReloaded(fresh.duLieu);
      if (fresh.duLieu.notice_version === MAP_FRAME_NOTICE_VERSION) setReloadedNote(true);
      checkbox.current?.focus();
      return;
    }
    setSending(false);
    setError(res.thongBao);
  }

  return (
    <OverlayDialog titleId={titleId} onDismiss={cancel}>
      <div
        className="flex min-w-0 flex-col gap-4"
        data-notice-confirm=""
        onKeyDown={(e) => {
          if (e.key !== "Escape") return;
          e.preventDefault();
          e.stopPropagation();
          cancel();
        }}
      >
        <h2 id={titleId} className="m-0 text-[15px] font-semibold text-ink-900">
          {MAP_FRAME_NOTICE_TITLE}
        </h2>
        {lead}
        <ol className="m-0 flex flex-col gap-2 pl-5 text-[13px] text-ink-700" data-notice-items="">
          {MAP_FRAME_NOTICE_ITEMS.map((item) => (
            <li key={item}>{item}</li>
          ))}
        </ol>
        {!versionMatches && (
          <p className="thong-bao-loi m-0" role="alert">
            {noticeOutdated(serverNoticeVersion)}
          </p>
        )}
        {reloadedNote && (
          <p className="m-0 text-sm font-medium text-ink-700" role="status">
            {NOTICE_RELOADED}
          </p>
        )}
        <label className="flex items-start gap-2 text-[13px] font-medium text-ink-900">
          <input
            ref={checkbox}
            type="checkbox"
            className="mt-0.5 size-4 shrink-0"
            checked={checked}
            disabled={sending || !versionMatches}
            onChange={(e) => setChecked(e.target.checked)}
            data-notice-ack=""
          />
          <span>{MAP_FRAME_NOTICE_ACK}</span>
        </label>
        {error !== "" && (
          <p className="thong-bao-loi m-0" role="alert">
            {error}
          </p>
        )}
        <div className="flex flex-wrap justify-end gap-2">
          <Button
            type="button"
            variant="primary"
            disabled={!checked || sending || !versionMatches}
            aria-busy={sending}
            onClick={() => void confirm()}
          >
            <BusyLabel busy={sending} label={NOTICE_CONFIRM_BUTTON} busyText={BUSY_SAVING} />
          </Button>
          <Button type="button" variant="secondary" onClick={cancel} disabled={sending}>
            Huỷ
          </Button>
        </div>
      </div>
    </OverlayDialog>
  );
}
