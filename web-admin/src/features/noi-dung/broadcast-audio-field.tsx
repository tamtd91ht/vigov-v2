"use client";

import { Upload } from "lucide-react";
import { useState } from "react";

import { Field } from "@/components/ui/field";
import { cn } from "@/lib/cn";

import {
  AUDIO_ACCEPT,
  AUDIO_DURATION_LABEL,
  AUDIO_DURATION_MAX_SECONDS,
  AUDIO_DURATION_MIN_SECONDS,
  AUDIO_HAS_FILE,
  AUDIO_HINT,
  AUDIO_LABEL,
  AUDIO_PICK_BUTTON,
  declaredAudioType,
  formatAudioSize,
} from "./broadcast-audio";

/**
 * §7 `Truyền thanh` — the prototype's two boxes side by side (`ContentItemForm.tsx:312-336`): the
 * duration in whole seconds, then the file slot. The SAME two boxes on the create form and on the edit
 * form of a saved broadcast (`hasExisting` only changes the slot's words); there is no player and no
 * separate remove — a new file chosen REPLACES the attached one when Lưu is pressed (`savedAudioPatch`).
 *
 * The file is only HELD here (pre-checked by `declaredAudioType`, never sent): the form passes it to the
 * save, which uploads it with the item's id (`HeldAudio`). The duration is typed, never measured (ADR 0067
 * §4.1).
 *
 * KEYBOARD: same shape as the cover slot — the file input visually hidden and focusable FIRST, the dashed
 * slot right after it is its `<label>` and shows the focus ring.
 */
export function HeldAudioField({
  file,
  hasExisting = false,
  duration,
  problem,
  disabled,
  onFile,
  onDuration,
}: {
  file: File | null;
  /** The saved item already has its audio: the slot reads `Đã có tệp — chọn tệp mới để thay`. */
  hasExisting?: boolean;
  duration: string;
  /** `heldAudioProblem` — drawn under the duration box. */
  problem: string | null;
  disabled: boolean;
  onFile: (file: File) => void;
  onDuration: (duration: string) => void;
}) {
  // A refusal of the picked file (type, size, empty) — the file is then not held.
  const [refusal, setRefusal] = useState<string | null>(null);
  return (
    <div className="grid gap-3 sm:grid-cols-2">
      <Field label={AUDIO_DURATION_LABEL} htmlFor="thoi-luong-am-thanh" grow="auto" error={problem ?? undefined}>
        <input
          id="thoi-luong-am-thanh"
          name="thoi-luong-am-thanh"
          type="number"
          inputMode="numeric"
          min={AUDIO_DURATION_MIN_SECONDS}
          max={AUDIO_DURATION_MAX_SECONDS}
          step={1}
          autoComplete="off"
          value={duration}
          aria-invalid={problem !== null || undefined}
          disabled={disabled}
          onChange={(e) => onDuration(e.target.value)}
        />
      </Field>
      <div className="flex min-w-0 flex-col gap-1.5 [&_p]:m-0" role="group" aria-labelledby="tep-am-thanh-nhan">
        <span id="tep-am-thanh-nhan" className="text-[13px] leading-tight font-semibold text-navy">
          {AUDIO_LABEL}
        </span>
        <input
          id="tep-am-thanh"
          name="tep-am-thanh"
          type="file"
          accept={AUDIO_ACCEPT}
          className="peer an-thi-giac"
          aria-describedby="tep-am-thanh-goi-y"
          disabled={disabled}
          onChange={(e) => {
            const f = e.target.files?.[0];
            e.target.value = ""; // the same file can be chosen again after a refusal
            if (f === undefined) return;
            const t = declaredAudioType(f);
            setRefusal(t.ok ? null : t.message);
            if (t.ok) onFile(f);
          }}
        />
        <label
          htmlFor="tep-am-thanh"
          className={cn(
            "flex min-w-0 items-center gap-2.5 rounded-md border border-dashed border-line px-3 py-2.5 transition-colors",
            "peer-focus-visible:ring-3 peer-focus-visible:ring-ring/50",
            disabled ? "cursor-not-allowed opacity-60" : "cursor-pointer hover:border-brand/50 hover:bg-brand/4",
          )}
        >
          <Upload aria-hidden="true" focusable="false" className="size-4 shrink-0 text-ink-muted" />
          <span className="min-w-0 flex-1">
            <span className="block overflow-hidden text-[12.5px] font-medium text-ellipsis whitespace-nowrap text-navy">
              {file !== null ? file.name : hasExisting ? AUDIO_HAS_FILE : AUDIO_PICK_BUTTON}
            </span>
            <span className="block text-[11px] text-ink-muted" id="tep-am-thanh-goi-y">
              {AUDIO_HINT}
            </span>
          </span>
          {file !== null && (
            <span className="shrink-0 text-[11.5px] font-semibold text-leaf">{formatAudioSize(file.size)}</span>
          )}
        </label>
        {refusal !== null && (
          <p className="thong-bao-loi" role="alert">
            {refusal}
          </p>
        )}
      </div>
    </div>
  );
}
