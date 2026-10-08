"use client";

import { useState } from "react";

import { revealStaffEmail, type StaffEmailResult } from "@/lib/api/danh-ba-chon-nguoi";

/** The reveal button's word (owner 08/10/2026, ADR 0082 #3). */
export const STAFF_EMAIL_REVEAL_BUTTON = "Xem";

/** Accessible name: "Xem" alone does not say WHAT is revealed to a screen reader. */
export const STAFF_EMAIL_REVEAL_LABEL = "Xem email đầy đủ";

type RevealState =
  | { readonly kind: "masked" }
  | { readonly kind: "loading" }
  | { readonly kind: "shown"; readonly email: string }
  /** 403: the account may not reveal — the masked address stays, the button goes. */
  | { readonly kind: "forbidden" }
  | { readonly kind: "error"; readonly message: string };

/**
 * The assignee's address in the task drawer (ADR 0082 #3, #10 — the drawer is the ONLY place with the
 * button): the masked address `t***@vigov.vn`, then a small `Xem`. A click asks the server for the full
 * address, which replaces the masked one IN PLACE — no toast, the button gone.
 *
 * WHY THE ADDRESS LIVES IN THIS COMPONENT'S STATE ONLY: every reveal is an audited read of personal data
 * on the server (rule 6, invariant 7). A copy kept anywhere longer-lived (a module cache, the drawer's
 * reducer, the URL) would be a read shown again without a trail. The component is keyed by the task in
 * the drawer, so another task — or the same one reopened — starts masked again.
 *
 * `canReveal` is `task.read` (`QUYEN_XEM_NHIEM_VU`). Convenience only: the route checks the key itself
 * (rule 5, forbidden #1); a 403 that comes back anyway hides the button and keeps the masked text.
 */
export function StaffEmailReveal({
  code,
  masked,
  canReveal,
  reveal = revealStaffEmail,
}: {
  /** Business code `CB-…` of the officer — what the route is asked for. */
  code: string;
  /** `email_masked` from the staff directory. Never empty here: the caller draws another line then. */
  masked: string;
  canReveal: boolean;
  /** Injected by tests; the real call otherwise. */
  reveal?: (code: string) => Promise<StaffEmailResult>;
}) {
  const [state, setState] = useState<RevealState>({ kind: "masked" });

  function onReveal() {
    if (state.kind === "loading") return;
    setState({ kind: "loading" });
    reveal(code).then((r) => {
      if (r.ok) setState({ kind: "shown", email: r.duLieu.email });
      else if (r.forbidden === true) setState({ kind: "forbidden" });
      else setState({ kind: "error", message: r.thongBao });
    });
  }

  const showButton = canReveal && (state.kind === "masked" || state.kind === "loading");
  return (
    <>
      <span className="[overflow-wrap:anywhere]">{state.kind === "shown" ? state.email : masked}</span>
      {showButton && (
        <>
          {" "}
          <button
            type="button"
            aria-label={STAFF_EMAIL_REVEAL_LABEL}
            disabled={state.kind === "loading"}
            aria-busy={state.kind === "loading" || undefined}
            className="text-brand cursor-pointer border-0 bg-transparent p-0 text-[11px] font-semibold [font-family:inherit] underline-offset-2 hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-brand-500 disabled:cursor-wait disabled:opacity-60"
            onClick={onReveal}
          >
            {STAFF_EMAIL_REVEAL_BUTTON}
          </button>
        </>
      )}
      {state.kind === "error" && (
        <span className="text-danger block" role="alert">
          {state.message}
        </span>
      )}
    </>
  );
}
