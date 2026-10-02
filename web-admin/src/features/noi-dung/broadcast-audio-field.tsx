"use client";

import { AudioLines } from "lucide-react";
import { useEffect, useState } from "react";

import { CONTENT_TYPE_BROADCAST, layMotNoiDung, removeContentAudio } from "@/lib/api/noi-dung";
import type { comms_audioOut } from "@/lib/api/schema.gen";

import {
  AUDIO_ACCEPT,
  AUDIO_ATTACHES_AT_ONCE,
  AUDIO_DURATION_HINT,
  AUDIO_DURATION_LABEL,
  AUDIO_DURATION_NEEDED,
  AUDIO_HINT,
  AUDIO_LABEL,
  AUDIO_NONE,
  AUDIO_PICK_BUTTON,
  AUDIO_PREVIEW_EXPIRED,
  AUDIO_PREVIEW_LABEL,
  AUDIO_PREVIEW_MISSING,
  AUDIO_REFRESH_BUTTON,
  AUDIO_REMOVE_BUTTON,
  AUDIO_REMOVE_CANCEL_BUTTON,
  AUDIO_REMOVE_CONFIRM_BUTTON,
  AUDIO_REMOVE_QUESTION,
  AUDIO_REPLACE_NOTE,
  AUDIO_RETRY_BUTTON,
  AUDIO_TYPE_CHANGE_DETACHES,
  audioInFlight,
  audioPreviewSrc,
  audioStateText,
  parseAudioDuration,
  previewExpiresInMs,
  retryAudioCompletion,
  runAudioUpload,
  savedAudioText,
  type AudioUploadState,
} from "./broadcast-audio";

/** The largest delay `setTimeout` keeps (2^31 − 1 ms); a later expiry is re-checked at the next render. */
const MAX_TIMER_MS = 2_147_483_647;

/**
 * §7 `Truyền thanh` — the audio of a SAVED broadcast (ADR 0067 §4). Rendered only for an item whose
 * SAVED type is `truyen-thanh`; the form shows `AUDIO_SAVE_FIRST` instead otherwise.
 *
 * THE AUDIO IS NOT PART OF THE FORM'S LƯU. The completion attaches the file to the item on the server,
 * and `Gỡ âm thanh` is its own PATCH — so this block keeps its own copy of the item's audio, refreshed
 * from the detail route, and the form's `thanSua` never carries an audio field.
 *
 * STAYS MOUNTED WHILE THE TYPE BOX MOVES AWAY (`currentType`): it then shows only the warning that Lưu
 * will remove the audio, and an upload done earlier in this sitting is not forgotten if the type comes back.
 *
 * THE `<audio>` PLAYS THE SERVER'S SIGNED LINK AND NOTHING ELSE (`audioPreviewSrc`), with
 * `preload="none"` so the short-lived link is only fetched when the officer presses play. When it expires
 * (or fails to play) the player is replaced by a button that asks the detail route for a fresh one.
 */
export function BroadcastAudioField({
  itemId,
  initialAudio,
  currentType,
  disabled,
  onBusyChange,
  initialState,
  initialConfirmRemove,
}: {
  itemId: string;
  /** The detail route's `audio` block when the form opened; `null` = none attached. */
  initialAudio: comms_audioOut | null;
  /** The form's type box right now. */
  currentType: string;
  disabled: boolean;
  /** Told whenever an upload starts or stops moving — the form holds Lưu meanwhile. */
  onBusyChange: (busy: boolean) => void;
  /** Tests only: render in a given upload state (this suite has no DOM events). */
  initialState?: AudioUploadState;
  /** Tests only: render the remove confirmation. */
  initialConfirmRemove?: boolean;
}) {
  const [saved, setSaved] = useState<comms_audioOut | null>(initialAudio);
  const [state, setState] = useState<AudioUploadState>(initialState ?? { kind: "idle" });
  const [duration, setDuration] = useState("");
  const [expired, setExpired] = useState(false);
  const [confirmRemove, setConfirmRemove] = useState(initialConfirmRemove ?? false);
  const [working, setWorking] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);

  useEffect(() => {
    const ms = previewExpiresInMs(saved, Date.now());
    if (ms === null) return;
    const t = setTimeout(() => setExpired(true), Math.min(Math.max(0, ms), MAX_TIMER_MS));
    return () => clearTimeout(t);
  }, [saved]);

  function refresh(): void {
    setWorking(true);
    void layMotNoiDung(itemId).then((kq) => {
      setWorking(false);
      if (!kq.ok) {
        setActionError(kq.thongBao);
        return;
      }
      setActionError(null);
      setExpired(false);
      setSaved(kq.duLieu.audio ?? null);
    });
  }

  function report(s: AudioUploadState): void {
    setState(s);
    onBusyChange(audioInFlight(s));
    // Attached on the server: read the item back for the player and the measured facts.
    if (s.kind === "ready") refresh();
  }

  function remove(): void {
    setWorking(true);
    void removeContentAudio(itemId).then((kq) => {
      setWorking(false);
      if (!kq.ok) {
        // The server's sentence verbatim (403, 409, 422 `audio_all_or_none`, …).
        setActionError(kq.thongBao);
        return;
      }
      setActionError(null);
      setConfirmRemove(false);
      setSaved(null);
      setExpired(false);
      setDuration("");
      setState({ kind: "idle" });
    });
  }

  if (currentType !== CONTENT_TYPE_BROADCAST) {
    return saved !== null ? <p className="ghi-chu">{AUDIO_TYPE_CHANGE_DETACHES}</p> : null;
  }

  const busy = audioInFlight(state);
  const parsed = parseAudioDuration(duration);
  const durationProblem = duration.trim() === "" ? null : parsed.ok ? null : parsed.message;
  const previewSrc = audioPreviewSrc(saved);
  const stateText = audioStateText(state);
  const refused = state.kind === "refused" || state.kind === "retry";

  return (
    // Same panel as the cover block above it (`CoverImageField`): one look for every upload of the form.
    <div
      className="o-nhap task-attachments m-0 flex flex-col items-start gap-2 rounded-xl border border-dashed border-line-strong bg-surface-muted p-4 [&_p]:m-0"
      role="group"
      aria-labelledby="am-thanh-nhan"
    >
      <span id="am-thanh-nhan" className="flex items-center gap-2 text-sm font-semibold text-ink-900">
        <AudioLines aria-hidden="true" focusable="false" strokeWidth={1.8} className="size-[18px] text-brand-600" />
        {AUDIO_LABEL}
      </span>
      <p className="ghi-chu">{AUDIO_ATTACHES_AT_ONCE}</p>

      {saved !== null ? (
        <>
          <p>{savedAudioText(saved)}</p>
          {previewSrc !== null && !expired ? (
            <audio
              controls
              preload="none"
              src={previewSrc}
              aria-label={AUDIO_PREVIEW_LABEL}
              onError={() => setExpired(true)}
            />
          ) : (
            <p className="ghi-chu">{expired ? AUDIO_PREVIEW_EXPIRED : AUDIO_PREVIEW_MISSING}</p>
          )}
          <p className="ghi-chu">{AUDIO_REPLACE_NOTE}</p>
          <div className="cum-nut">
            {(expired || previewSrc === null) && (
              <button type="button" className="nut-phu" disabled={disabled || working} onClick={refresh}>
                {AUDIO_REFRESH_BUTTON}
              </button>
            )}
            {!confirmRemove && (
              <button
                type="button"
                className="nut-phu"
                disabled={disabled || working}
                onClick={() => setConfirmRemove(true)}
              >
                {AUDIO_REMOVE_BUTTON}
              </button>
            )}
          </div>
          {confirmRemove && (
            <div role="group" aria-labelledby="go-am-thanh-hoi">
              <p id="go-am-thanh-hoi">{AUDIO_REMOVE_QUESTION}</p>
              <div className="cum-nut">
                <button type="button" className="nut-chinh" disabled={disabled || working} onClick={remove}>
                  {AUDIO_REMOVE_CONFIRM_BUTTON}
                </button>
                <button
                  type="button"
                  className="nut-phu"
                  disabled={working}
                  onClick={() => setConfirmRemove(false)}
                >
                  {AUDIO_REMOVE_CANCEL_BUTTON}
                </button>
              </div>
            </div>
          )}
        </>
      ) : (
        <>
          <p className="ghi-chu">{AUDIO_NONE}</p>
          <label htmlFor="thoi-luong-am-thanh">{AUDIO_DURATION_LABEL}</label>
          <input
            id="thoi-luong-am-thanh"
            name="thoi-luong-am-thanh"
            inputMode="numeric"
            placeholder="12:30"
            autoComplete="off"
            maxLength={8}
            value={duration}
            aria-describedby="thoi-luong-am-thanh-goi-y"
            aria-invalid={durationProblem !== null}
            disabled={disabled || busy}
            onChange={(e) => setDuration(e.target.value)}
          />
          <p className="ghi-chu" id="thoi-luong-am-thanh-goi-y">
            {AUDIO_DURATION_HINT}
          </p>
          {durationProblem !== null && (
            <p className="thong-bao-loi" role="alert">
              {durationProblem}
            </p>
          )}

          {/* Same control shape as the cover: the input first, visually hidden but focusable; the label
              right after it is the visible button. Held off until a valid duration is typed. */}
          <input
            id="tep-am-thanh"
            name="tep-am-thanh"
            type="file"
            accept={AUDIO_ACCEPT}
            className="an-thi-giac"
            aria-describedby="tep-am-thanh-goi-y"
            disabled={disabled || busy || !parsed.ok}
            onChange={(e) => {
              const f = e.target.files?.[0];
              e.target.value = ""; // the same file can be chosen again after a refusal
              if (f !== undefined) void runAudioUpload(f, itemId, duration, report);
            }}
          />
          <label htmlFor="tep-am-thanh" className="nut-phu" aria-disabled={!parsed.ok || busy}>
            {AUDIO_PICK_BUTTON}
            <span className="an-thi-giac"> — {AUDIO_LABEL}</span>
          </label>
          <p className="ghi-chu" id="tep-am-thanh-goi-y">
            {parsed.ok ? AUDIO_HINT : `${AUDIO_HINT} ${AUDIO_DURATION_NEEDED}`}
          </p>
        </>
      )}

      {stateText !== "" && (
        <p role={refused ? "alert" : "status"} className={refused ? "thong-bao-loi" : undefined}>
          {stateText}
        </p>
      )}

      {state.kind === "retry" && (
        <div className="cum-nut">
          <button
            type="button"
            className="nut-phu"
            disabled={disabled}
            onClick={() => void retryAudioCompletion(state.id, duration, report)}
          >
            {AUDIO_RETRY_BUTTON}
          </button>
        </div>
      )}

      {actionError !== null && (
        <p className="thong-bao-loi" role="alert">
          {actionError}
        </p>
      )}
    </div>
  );
}
