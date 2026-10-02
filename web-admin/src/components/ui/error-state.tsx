import { CloudOff, RotateCw } from "lucide-react";
import type { ReactNode } from "react";

import { cn } from "@/lib/cn";

import { Button } from "./button";

/**
 * Load error — spec v2 §8b "Lỗi tải": `CloudOff` in red, "Chưa tải được …", the server's sentence,
 * and a "Tải lại" button when the screen has a way to reload.
 *
 * `message` IS PRINTED VERBATIM. It is the sentence the screen already received from the server
 * (`thongBaoLoi` reads only `message`). Rewording it here would put a guess where the authority's
 * own answer was; mapping codes to friendlier text is the screen's call, made once, in its own file.
 *
 * `onRetry` is the screen's EXISTING reload mechanism, passed in. No `onRetry` = no button: a
 * "Tải lại" that reloads nothing is a promise the screen cannot keep. The button is `type="button"`
 * so an error state inside a form never submits it.
 *
 * `role` passes through for a screen that already announced its error (`role="alert"`); none by
 * default, so adopting this component never adds an announcement the screen did not make.
 */
export const RETRY_LABEL = "Tải lại";

export type ErrorStateProps = {
  title: ReactNode;
  /** The server's message, verbatim. */
  message?: ReactNode;
  onRetry?: () => void;
  retryLabel?: string;
  role?: "alert" | "status";
  className?: string;
};

export function ErrorState({ title, message, onRetry, retryLabel = RETRY_LABEL, role, className }: ErrorStateProps) {
  return (
    <div role={role} className={cn("error-state flex flex-col items-center gap-2 px-4 py-10 text-center", className)}>
      <span aria-hidden="true" className="mb-2 grid size-[72px] place-items-center rounded-full bg-danger-50 text-danger-600">
        <CloudOff className="size-8" strokeWidth={1.6} focusable="false" />
      </span>
      <p className="m-0 text-base font-semibold text-ink-900">{title}</p>
      {message !== undefined && <p className="m-0 max-w-md text-[13px] text-ink-500">{message}</p>}
      {onRetry !== undefined && (
        <div className="mt-2">
          <Button
            type="button"
            variant="secondary"
            onClick={onRetry}
            icon={<RotateCw aria-hidden="true" focusable="false" strokeWidth={1.8} />}
          >
            {retryLabel}
          </Button>
        </div>
      )}
    </div>
  );
}
