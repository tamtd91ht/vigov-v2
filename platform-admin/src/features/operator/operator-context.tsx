"use client";

import { useRouter } from "next/navigation";
import { createContext, useContext, useEffect, useState, type ReactNode } from "react";

import { getCurrentOperator, type CurrentOperator } from "@/lib/api";
import { handleGuardedError } from "@/lib/errors";

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
 */

export type OperatorState =
  | { status: "loading" }
  | { status: "ready"; operator: CurrentOperator }
  | { status: "error"; message: string };

const OperatorContext = createContext<OperatorState>({ status: "loading" });

export function OperatorProvider({ children }: { children: ReactNode }) {
  const router = useRouter();
  const [state, setState] = useState<OperatorState>({ status: "loading" });

  useEffect(() => {
    let alive = true;
    getCurrentOperator().then(
      (operator) => {
        if (alive) setState({ status: "ready", operator });
      },
      (err: unknown) => {
        if (!alive) return;
        const message = handleGuardedError(err, "xem thông tin tài khoản", (p) => router.replace(p));
        if (message !== null) setState({ status: "error", message });
      },
    );
    return () => {
      alive = false;
    };
  }, [router]);

  return <OperatorContext.Provider value={state}>{children}</OperatorContext.Provider>;
}

export function useOperator(): OperatorState {
  return useContext(OperatorContext);
}

/** The keys to hint with — empty until the server answered. */
export function usePermissionKeys(): readonly string[] {
  const state = useOperator();
  return state.status === "ready" ? state.operator.permission_keys : [];
}
