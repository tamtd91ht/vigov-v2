"use client";

import { useRouter } from "next/navigation";
import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from "react";

import { getCurrentOperator, onPasswordChangeRequired, type CurrentOperator } from "@/lib/api";
import { handleGuardedError } from "@/lib/errors";
import { SIGN_IN_PATH } from "@/lib/route-access";

/**
 * Who is signed in, loaded once per page load from `GET /operator-sessions/current`.
 *
 * WHAT IT IS FOR: showing the `VH-…` code, and HINTING which controls to show from the
 * `permission_keys` (`lib/permissions.ts`). It is not an authorisation source — every API call is
 * checked by service-platform. Until it loads, the keys are EMPTY: a gated control appears only
 * once the server has said the operator holds the key, never "until proven otherwise".
 *
 * 401 here means the cookie the proxy saw is not a live session: go to sign-in. 503 is an outage
 * and says so; it never signs the person out.
 *
 * RESTRICTED SESSION (ADR 0048 §13/10 #6): after the owner resets an operator's password, the
 * operator signs in with the temporary one + their own TOTP and must change it before anything
 * else. `mustChangePassword` is true when the session answer says `must_change_password: true`, OR
 * when any call came back 403 `password_change_required` (`onPasswordChangeRequired`) — the second
 * covers a session answer that failed or predates the field. The console shell then shows only the
 * forced change screen. This is UX: the server refuses every other route either way.
 */

export type OperatorState =
  | { status: "loading" }
  | { status: "ready"; operator: CurrentOperator }
  | { status: "error"; message: string };

/** What one session read came to. `signed_out` = 401: the session is gone. */
export type OperatorLoad = { kind: "ready"; operator: CurrentOperator } | { kind: "signed_out" } | { kind: "error"; message: string };

/** One `GET /operator-sessions/current`, classified. Navigates nowhere: the caller decides. */
export async function loadOperator(): Promise<OperatorLoad> {
  try {
    return { kind: "ready", operator: await getCurrentOperator() };
  } catch (err) {
    let signedOut = false;
    const message = handleGuardedError(err, "xem thông tin tài khoản", () => {
      signedOut = true;
    });
    if (signedOut) return { kind: "signed_out" };
    // null without a 401 = a 403 password_change_required; the listener already flagged it.
    return { kind: "error", message: message ?? "" };
  }
}

/** Only an explicit `true` from the server, or a refusal saying so, restricts the console. */
export function mustChangePasswordOf(state: OperatorState, refusedForPassword: boolean): boolean {
  return refusedForPassword || (state.status === "ready" && state.operator.must_change_password === true);
}

/** After the forced change: `continue` = the console opens; `signed_out` = sign in again; `unknown` = could not tell. */
export type AfterPasswordChange = "continue" | "signed_out" | "unknown";

/** A session that still says `must_change_password` after a 204 is not "continue": never open the console on a guess. */
export function afterPasswordChangeOf(r: OperatorLoad): AfterPasswordChange {
  if (r.kind === "signed_out") return "signed_out";
  if (r.kind !== "ready" || r.operator.must_change_password === true) return "unknown";
  return "continue";
}

type OperatorContextValue = {
  state: OperatorState;
  mustChangePassword: boolean;
  /** Re-reads the session after the forced change and says where the operator goes next. */
  reloadAfterPasswordChange: () => Promise<AfterPasswordChange>;
};

const OperatorContext = createContext<OperatorContextValue>({
  state: { status: "loading" },
  mustChangePassword: false,
  reloadAfterPasswordChange: async () => "unknown",
});

export function OperatorProvider({ children }: { children: ReactNode }) {
  const router = useRouter();
  const [state, setState] = useState<OperatorState>({ status: "loading" });
  const [refusedForPassword, setRefusedForPassword] = useState(false);

  useEffect(() => onPasswordChangeRequired(() => setRefusedForPassword(true)), []);

  useEffect(() => {
    let alive = true;
    void loadOperator().then((r) => {
      if (!alive) return;
      if (r.kind === "signed_out") router.replace(SIGN_IN_PATH);
      else if (r.kind === "ready") setState({ status: "ready", operator: r.operator });
      else if (r.message !== "") setState({ status: "error", message: r.message });
    });
    return () => {
      alive = false;
    };
  }, [router]);

  const reloadAfterPasswordChange = useCallback(async (): Promise<AfterPasswordChange> => {
    const r = await loadOperator();
    const next = afterPasswordChangeOf(r);
    if (next === "continue" && r.kind === "ready") {
      setRefusedForPassword(false);
      setState({ status: "ready", operator: r.operator });
    }
    return next;
  }, []);

  const value = useMemo(
    () => ({ state, mustChangePassword: mustChangePasswordOf(state, refusedForPassword), reloadAfterPasswordChange }),
    [state, refusedForPassword, reloadAfterPasswordChange],
  );

  return <OperatorContext.Provider value={value}>{children}</OperatorContext.Provider>;
}

export function useOperator(): OperatorState {
  return useContext(OperatorContext).state;
}

/** True only when the server said so (session answer or a 403 `password_change_required`). */
export function useMustChangePassword(): boolean {
  return useContext(OperatorContext).mustChangePassword;
}

export function useReloadAfterPasswordChange(): () => Promise<AfterPasswordChange> {
  return useContext(OperatorContext).reloadAfterPasswordChange;
}

/** The keys to hint with — empty until the server answered. */
export function usePermissionKeys(): readonly string[] {
  const state = useOperator();
  return state.status === "ready" ? state.operator.permission_keys : [];
}

/**
 * Whether to SHOW the owner-only controls (ADR 0048 §Sửa đổi 13/10/2026). False until the server said
 * `is_owner: true` — loading, an error, an absent field all read as "not the owner". The owner-only
 * routes check it again on the server, so this is a hint like the keys.
 */
export function useIsOwner(): boolean {
  const state = useOperator();
  return state.status === "ready" && state.operator.is_owner === true;
}
