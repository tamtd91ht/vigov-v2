/**
 * THE SHORT PERMISSION QUESTION asked BEFORE a Zalo dialog (owner, 09/10/2026) — "Cho phép" / "Không".
 *
 * Used for the scene photos ("Chụp ảnh", "Chọn ảnh có sẵn" — `scene-photos.tsx`) and for the Zalo name when
 * "Gửi phản ánh" opens on a session already verified (`TrangXa.tsx`). Zalo policy 3.3.4 wants the purpose said
 * before the platform's own dialog: this one sentence is that, and only "Cho phép" leads to Zalo.
 *
 * WHY A MODAL, AND WHY THESE KEYS (`skills/accessibility-elderly`):
 *   · `role="alertdialog"` + `aria-modal`: a screen reader reads the question and stays in it;
 *   · the focus moves INTO the dialog, onto "Không" — the answer that grants nothing — and Tab / Shift+Tab cycle
 *     between the two buttons only, so a keyboard or switch user cannot wander behind the dimmed screen;
 *   · Escape and a tap on the dimmed area mean "Không" — leaving the question never grants anything;
 *   · two full-width `xa-nut` buttons (≥ 48px, body-size text), words only.
 * The focus goes back where it was when the dialog closes.
 *
 * The look is the leave-app question's (`leave-app.tsx`): the same overlay and box classes, so the app has ONE
 * dialog style. PURE apart from the callbacks and the focus.
 */
import { type KeyboardEvent, useLayoutEffect, useRef } from "react";

import { CONSENT_DIALOG } from "./noi-dung";

export function ConsentDialog(props: {
  /** The `id` of the question — unique on the screen, so `aria-labelledby` names this dialog. */
  id: string;
  question: string;
  onAllow: () => void;
  onDeny: () => void;
}) {
  const allowRef = useRef<HTMLButtonElement>(null);
  const denyRef = useRef<HTMLButtonElement>(null);

  // A layout effect, not `autoFocus`: the focus is taken before paint, and given back on close (StrictMode's
  // simulated unmount gives it back and takes it again, so the dialog still ends with the focus).
  useLayoutEffect(() => {
    const before = document.activeElement as { focus?: () => void } | null;
    denyRef.current?.focus();
    return () => before?.focus?.();
  }, []);

  function onKeyDown(e: KeyboardEvent) {
    if (e.key === "Escape") {
      e.preventDefault();
      props.onDeny();
      return;
    }
    if (e.key !== "Tab") return;
    // Two buttons: either direction lands on the other one; from anywhere else, on the first.
    e.preventDefault();
    const next = document.activeElement === allowRef.current ? denyRef.current : allowRef.current;
    next?.focus();
  }

  return (
    <div
      className="xa-leave-app"
      onKeyDown={onKeyDown}
      onClick={(e) => {
        if (e.target === e.currentTarget) props.onDeny();
      }}
    >
      <div className="xa-the xa-leave-app__box" role="alertdialog" aria-modal="true" aria-labelledby={props.id}>
        <h2 className="xa-dau-khoi__tieu-de xa-leave-app__question" id={props.id}>
          {props.question}
        </h2>
        <button type="button" className="xa-nut" ref={allowRef} onClick={props.onAllow}>
          {CONSENT_DIALOG.allow}
        </button>
        <button type="button" className="xa-nut xa-nut--phu" ref={denyRef} onClick={props.onDeny}>
          {CONSENT_DIALOG.deny}
        </button>
      </div>
    </div>
  );
}
