"use client";

import { LogOut, UserRound } from "lucide-react";
import { useState } from "react";

import { useOperator } from "@/features/operator/operator-context";
import { ApiError, signOut } from "@/lib/api";
import { NETWORK_ERROR } from "@/lib/errors";
import { goTo } from "@/lib/navigate";
import { SIGN_IN_PATH } from "@/lib/route-access";

import { Button } from "./ui/button";

/**
 * The signed-in topbar — drawn like web-admin's (`web-admin/src/components/dau-trang.tsx`, spec 01):
 * 64px, sticky, white with a hairline under it, the person block and sign-out on the right. Solid
 * white, NOT web-admin's `bg-white/95 backdrop-blur`: lần 6 #3 allowed the blur for web-admin's header
 * alone, and nothing scrolls under this one that the blur would show.
 *
 * WHAT IT SHOWS ABOUT THE PERSON: the `VH-…` operator code, and nothing else — exactly what the
 * sidebar showed before the redesign. No name, no email: the session answer carries the code for
 * display (`operator-context.tsx`), and the topbar is on every screen anyone may glance at.
 *
 * Sign-out moved here from the sidebar unchanged: same call, same answers. 401 means the session
 * was already gone — the person is signed out, which is what they asked. Anything else: the session
 * may still be live, so say so instead of pretending.
 *
 * No system-wide search placeholder (web-admin's "?" box): that is a commune-staff Phase-2 item
 * (ADR 0068 §14), not a feature anyone planned for this console.
 */

export const SIGN_OUT_UNAVAILABLE =
  "Chưa đăng xuất được: hệ thống xác thực tạm không phản hồi. Vui lòng thử lại sau ít phút.";

export function Topbar() {
  const operator = useOperator();
  const [signingOut, setSigningOut] = useState(false);
  const [signOutError, setSignOutError] = useState<string | null>(null);

  async function doSignOut() {
    if (signingOut) return;
    setSigningOut(true);
    setSignOutError(null);
    try {
      await signOut();
    } catch (err) {
      if (!(err instanceof ApiError && err.status === 401)) {
        setSignOutError(
          err instanceof ApiError && err.status === 0
            ? NETWORK_ERROR
            : err instanceof ApiError && err.status === 503
              ? SIGN_OUT_UNAVAILABLE
              : "Chưa đăng xuất được. Vui lòng thử lại.",
        );
        setSigningOut(false);
        return;
      }
    }
    goTo(SIGN_IN_PATH);
  }

  const code =
    operator.status === "ready" ? operator.operator.operator_code : operator.status === "loading" ? "…" : "—";

  return (
    <header className="sticky top-0 z-30 flex min-h-[var(--topbar-h)] min-w-0 flex-wrap items-center justify-end gap-x-4 gap-y-2 border-b border-line bg-surface px-4 py-2 md:px-7">
      <section aria-label="Tài khoản" className="ml-auto flex min-w-0 flex-wrap items-center justify-end gap-x-3 gap-y-1">
        {/* web-admin's person block (`.header-user`, `.topbar-avatar`): a 36px navy disc, the name
            line 13px/600 navy, the role line 11.5px ink-muted. Here the "name" is the operator code. */}
        <p className="m-0 flex min-w-0 items-center gap-2.5 leading-tight">
          <span aria-hidden="true" className="grid size-9 shrink-0 place-items-center rounded-full bg-navy text-white">
            <UserRound className="size-[18px]" strokeWidth={1.8} focusable="false" />
          </span>
          <span className="flex min-w-0 flex-col">
            <span className="text-[13px] font-semibold text-navy">{code}</span>
            <span className="text-[11.5px] text-ink-muted">Người vận hành</span>
          </span>
        </p>
        <span aria-hidden="true" className="hidden h-6 w-px bg-line sm:block" />
        <Button
          type="button"
          variant="secondary"
          size="sm"
          onClick={doSignOut}
          disabled={signingOut}
          aria-busy={signingOut}
          icon={<LogOut aria-hidden="true" focusable="false" strokeWidth={1.8} />}
        >
          {signingOut ? "Đang đăng xuất…" : "Đăng xuất"}
        </Button>
      </section>
      {operator.status === "error" ? (
        <p className="m-0 basis-full text-right text-[13px] text-danger-600" role="alert">
          {operator.message}
        </p>
      ) : null}
      {signOutError ? (
        <p className="m-0 basis-full text-right text-[13px] text-danger-600" role="alert">
          {signOutError}
        </p>
      ) : null}
    </header>
  );
}
