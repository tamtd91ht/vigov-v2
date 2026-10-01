"use client";

import { useRouter } from "next/navigation";
import { useCallback } from "react";

import type { ApiError } from "@/lib/api";
import { handleGuardedError } from "@/lib/errors";

/** `handleGuardedError` bound to the router: a 401 replaces the page with sign-in. */
export function useGuardedError() {
  const router = useRouter();
  return useCallback(
    (err: unknown, action: string, specific?: (e: ApiError) => string | null) =>
      handleGuardedError(err, action, (p) => router.replace(p), specific),
    [router],
  );
}
