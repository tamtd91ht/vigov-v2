"use client";

import { TextAreaField, TextField } from "@/components/form-parts";
import { Badge } from "@/components/ui/badge";
import { useGuardedError } from "@/features/operator/use-guarded-error";
import type { ApiError } from "@/lib/api";

import { REASON_MAX, TEMP_PASSWORD_HINT, type OwnerError, type OwnerField, type StatusChip } from "./owner-model";

/**
 * Pieces the owner-only dialogs repeat. Temporary passwords live in the dialog's component state
 * only: type=password, autocomplete=new-password (no browser offers to fill an old one), never put
 * anywhere else, and gone when the dialog unmounts on close.
 */

/** `handleGuardedError` with the owner screens' refusals; null = the 401 already left the page. */
export function useOwnerError(map: (e: ApiError) => OwnerError | null) {
  const guarded = useGuardedError();
  return (err: unknown, action: string): OwnerError | null => {
    let field: OwnerField = "form";
    const text = guarded(err, action, (e) => {
      const known = map(e);
      if (known) field = known.field;
      return known?.text ?? null;
    });
    return text === null ? null : { field, text };
  };
}

export function at(error: OwnerError | null, field: OwnerField): string | null {
  return error?.field === field ? error.text : null;
}

/** The form-level line: an error whose field has no input of its own in this dialog. */
export function formLevel(error: OwnerError | null, ...inline: OwnerField[]): string | null {
  return error && !inline.includes(error.field) ? error.text : null;
}

export function TemporaryPasswordFields({
  password,
  confirm,
  onPassword,
  onConfirm,
  disabled,
  error,
}: {
  password: string;
  confirm: string;
  onPassword: (v: string) => void;
  onConfirm: (v: string) => void;
  disabled: boolean;
  error: OwnerError | null;
}) {
  return (
    <>
      <TextField
        label="Mật khẩu tạm"
        hint={TEMP_PASSWORD_HINT}
        name="temporary_password"
        type="password"
        autoComplete="new-password"
        autoCapitalize="none"
        spellCheck={false}
        value={password}
        onChange={(e) => onPassword(e.target.value)}
        disabled={disabled}
        error={at(error, "password")}
      />
      <TextField
        label="Nhập lại mật khẩu tạm"
        name="temporary_password_confirm"
        type="password"
        autoComplete="new-password"
        autoCapitalize="none"
        spellCheck={false}
        value={confirm}
        onChange={(e) => onConfirm(e.target.value)}
        disabled={disabled}
        error={at(error, "confirm")}
      />
    </>
  );
}

export function ReasonField({
  value,
  onChange,
  disabled,
  error,
  hint = `Bắt buộc, tối đa ${REASON_MAX} ký tự. Lý do được ghi vào nhật ký.`,
}: {
  value: string;
  onChange: (v: string) => void;
  disabled: boolean;
  error: OwnerError | null;
  hint?: string;
}) {
  return (
    <TextAreaField
      label="Lý do"
      hint={hint}
      name="reason"
      rows={3}
      maxLength={REASON_MAX}
      value={value}
      onChange={(e) => onChange(e.target.value)}
      disabled={disabled}
      error={at(error, "reason")}
    />
  );
}

export function StatusChips({ chips }: { chips: readonly StatusChip[] }) {
  return (
    <span className="flex flex-wrap gap-1.5">
      {chips.map((c) => (
        <Badge key={c.text} tone={c.tone}>
          {c.text}
        </Badge>
      ))}
    </span>
  );
}
