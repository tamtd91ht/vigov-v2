"use client";

import { KeyRound } from "lucide-react";
import type { ReactNode } from "react";

import { Sidebar } from "@/components/sidebar";
import { Topbar } from "@/components/topbar";
import { Notice } from "@/components/ui/notice";
import { PageHeader } from "@/components/ui/page-header";
import { ChangePasswordForm } from "@/features/account/change-password-form";

import {
  useMustChangePassword,
  useOperator,
  useReloadAfterPasswordChange,
  type AfterPasswordChange,
  type OperatorState,
} from "./operator-context";

/**
 * The signed-in frame, decided by the session answer (ADR 0048 §13/10 #6):
 *
 *   restricted (`must_change_password`, or a 403 `password_change_required`) → topbar (code +
 *     Đăng xuất) and the forced password change ONLY. No sidebar, and the page's own screen is not
 *     mounted — so it fires none of its calls, each of which the server would refuse anyway.
 *   loading → topbar and a status line. The page is held until the session answer, because mounting
 *     it first would send its calls before the console knows whether the session is restricted.
 *     Cost: on a full page load the content waits one extra round trip.
 *   ready, or the session read failed (503/network — the topbar says so) → the normal console. A
 *     restricted session behind a failed read is still caught: the page's first call comes back
 *     403 `password_change_required` and the frame switches.
 *
 * FRAME (web-admin's `.khung-trang`): a grid of sidebar + content column. `minmax(0, 1fr)`, not
 * `1fr`: a bare `1fr` track grows to its widest child, so one wide table would push the whole page
 * sideways; with `minmax(0, …)` the column shrinks and the table scrolls inside its own region.
 */

export const FORCED_CHANGE_TEXT = "Mật khẩu của bạn vừa được đặt lại. Hãy đặt mật khẩu mới trước khi tiếp tục.";

export function ConsoleShell({ children }: { children: ReactNode }) {
  return (
    <ConsoleShellView state={useOperator()} mustChangePassword={useMustChangePassword()}>
      {children}
    </ConsoleShellView>
  );
}

/** The frame for a given session answer; no context read for the decision, so tests render each case. */
export function ConsoleShellView({
  state,
  mustChangePassword,
  children,
}: {
  state: OperatorState;
  mustChangePassword: boolean;
  children: ReactNode;
}) {
  const open = !mustChangePassword && state.status !== "loading";
  return (
    // Below 1024px the sidebar is a strip on its own row: `auto 1fr` rows, or a short page would
    // share its spare height with the strip and the strip would grow.
    <div
      className={
        open
          ? "grid min-h-dvh grid-cols-[minmax(0,1fr)] grid-rows-[auto_1fr] lg:grid-cols-[auto_minmax(0,1fr)] lg:grid-rows-[1fr]"
          : "grid min-h-dvh grid-cols-[minmax(0,1fr)] grid-rows-[1fr]"
      }
    >
      {open ? <Sidebar /> : null}
      <div className="flex min-w-0 flex-col">
        <Topbar />
        <main className="w-full min-w-0 max-w-[90rem] px-4 pt-6 pb-10 md:px-7">
          {mustChangePassword ? (
            <ForcedPasswordChange />
          ) : state.status === "loading" ? (
            <p role="status" className="m-0 text-sm text-ink-500">
              Đang tải thông tin tài khoản…
            </p>
          ) : (
            children
          )}
        </main>
      </div>
    </div>
  );
}

export function ForcedPasswordChange() {
  return <ForcedPasswordChangeView onChanged={useReloadAfterPasswordChange()} />;
}

/** Reuses the account page's form, refusals included; only what happens after a 204 differs. */
export function ForcedPasswordChangeView({ onChanged }: { onChanged: () => Promise<AfterPasswordChange> }) {
  return (
    <div className="mx-auto flex w-full max-w-[36rem] min-w-0 flex-col gap-4">
      <PageHeader icon={KeyRound} title="Đổi mật khẩu" className="mb-0" />
      <Notice tone="legal" role="status">
        <p>{FORCED_CHANGE_TEXT}</p>
      </Notice>
      <ChangePasswordForm onChanged={onChanged} />
    </div>
  );
}
