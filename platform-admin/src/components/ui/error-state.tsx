import { CloudOff } from "lucide-react";
import type { ReactNode } from "react";

import { cn } from "@/lib/cn";

/**
 * Load error — copied from `web-admin/src/components/ui/error-state.tsx` (change both).
 *
 * `message` IS PRINTED VERBATIM: it is the sentence the screen already built from the server's
 * answer (`lib/errors.ts`).
 *
 * DIFFERENCE FROM WEB-ADMIN: no "Tải lại" button. web-admin draws it only when the screen passes its
 * EXISTING reload mechanism; no screen of this console has one (each loads once, on mount), and a
 * "Tải lại" that reloads nothing is a promise the screen cannot keep. Add `onRetry` the day a
 * screen gains a reload — not before.
 */
export type ErrorStateProps = {
  title: ReactNode;
  message?: ReactNode;
  role?: "alert" | "status";
  className?: string;
};

export function ErrorState({ title, message, role, className }: ErrorStateProps) {
  return (
    <div role={role} className={cn("flex flex-col items-center gap-2 px-4 py-10 text-center", className)}>
      <span aria-hidden="true" className="mb-2 grid size-[72px] place-items-center rounded-full bg-danger-50 text-danger-600">
        <CloudOff className="size-8" strokeWidth={1.6} focusable="false" />
      </span>
      <p className="m-0 text-base font-semibold text-ink-900">{title}</p>
      {message !== undefined && message !== null && <p className="m-0 max-w-md text-[13px] text-ink-500">{message}</p>}
    </div>
  );
}
