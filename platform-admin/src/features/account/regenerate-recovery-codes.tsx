"use client";

import { RotateCcwKey, TriangleAlert } from "lucide-react";
import { useState, type FormEvent } from "react";

import { FormMessage, TextField } from "@/components/form-parts";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardFooter } from "@/components/ui/card";
import { Notice } from "@/components/ui/notice";
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
 *
 * LOOK: web-admin's confirm box — the specific question as the title, the consequence, then the
 * existing TOTP field and the action-named button. It frames the existing one-step flow; it adds no
 * second confirmation step.
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
      <Card>
        <CardContent>
          <RecoveryCodesOnce
            codes={codes}
            continueLabel="Xong"
            onSaved={() => {
              setCodes(null);
              setSaved(true);
            }}
          />
        </CardContent>
      </Card>
    );
  }

  return (
    <Card as="form" method="post" onSubmit={submit} noValidate>
      <CardContent className="flex flex-col gap-4">
        {saved ? (
          <Notice tone="info" role="status">
            <p>Đã tạo bộ mã khôi phục mới. Bộ mã cũ không còn dùng được.</p>
          </Notice>
        ) : null}
        <div className="flex items-start gap-3">
          <span aria-hidden="true" className="grid size-9 shrink-0 place-items-center rounded-full bg-danger-50 text-danger-600">
            <TriangleAlert className="size-[18px]" strokeWidth={1.8} focusable="false" />
          </span>
          <div className="flex min-w-0 flex-col gap-1.5">
            <h2 className="m-0 text-base leading-snug font-semibold text-ink-900">Tạo bộ mã khôi phục mới?</h2>
            <p className="m-0 text-sm text-ink-700">{REGENERATE_WARNING}</p>
          </div>
        </div>
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
      </CardContent>
      <CardFooter className="justify-end">
        <Button
          type="submit"
          variant="primary"
          disabled={busy}
          aria-busy={busy}
          icon={<RotateCcwKey aria-hidden="true" focusable="false" strokeWidth={1.8} />}
        >
          {busy ? "Đang tạo…" : "Tạo bộ mã mới"}
        </Button>
      </CardFooter>
    </Card>
  );
}
