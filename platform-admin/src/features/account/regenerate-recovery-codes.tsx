"use client";

import { useState, type FormEvent } from "react";

import { FormMessage, TextField } from "@/components/form-parts";
import { RecoveryCodesOnce } from "@/features/auth/recovery-codes-once";
import { useGuardedError } from "@/features/operator/use-guarded-error";
import { regenerateRecoveryCodes } from "@/lib/api";

import { CREDENTIALS_REFUSED_TOTP } from "./messages";

/**
 * `/tai-khoan/ma-khoi-phuc` — issue a new batch of recovery codes (`POST
 * /operators/current/recovery-codes`, re-proving a TOTP code). The previous batch stops working
 * at that moment, which the screen says BEFORE the button, not after.
 *
 * The new codes live in this component's state until "Tôi đã lưu", then are dropped.
 */

export const REGENERATE_WARNING =
  "Tạo bộ mã mới thì toàn bộ mã khôi phục cũ hết hiệu lực ngay, kể cả các mã chưa dùng.";

export function RegenerateRecoveryCodes() {
  const guarded = useGuardedError();
  const [totp, setTotp] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [codes, setCodes] = useState<string[] | null>(null);
  const [saved, setSaved] = useState(false);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (busy) return;
    setBusy(true);
    setError(null);
    const code = totp;
    setTotp("");
    try {
      const res = await regenerateRecoveryCodes({ totpCode: code });
      setCodes(res.recovery_codes);
    } catch (err) {
      setError(guarded(err, "tạo lại mã khôi phục", (e) => (e.code === "credentials_refused" ? CREDENTIALS_REFUSED_TOTP : null)));
    } finally {
      setBusy(false);
    }
  }

  if (codes !== null) {
    return (
      <div className="panel">
        <RecoveryCodesOnce
          codes={codes}
          continueLabel="Xong"
          onSaved={() => {
            setCodes(null);
            setSaved(true);
          }}
        />
      </div>
    );
  }

  return (
    <form className="panel narrow" method="post" onSubmit={submit} noValidate>
      {saved ? (
        <p className="notice-box" role="status">
          Đã tạo bộ mã khôi phục mới. Bộ mã cũ không còn dùng được.
        </p>
      ) : null}
      <p className="warning-box">{REGENERATE_WARNING}</p>
      <TextField
        label="Mã xác thực (6 số trên ứng dụng xác thực)"
        name="totp_code"
        type="text"
        inputMode="numeric"
        autoComplete="one-time-code"
        maxLength={6}
        value={totp}
        onChange={(e) => setTotp(e.target.value)}
        disabled={busy}
      />
      <FormMessage text={error} />
      <button type="submit" className="primary-button" disabled={busy} aria-busy={busy}>
        {busy ? "Đang tạo…" : "Tạo bộ mã mới"}
      </button>
    </form>
  );
}
