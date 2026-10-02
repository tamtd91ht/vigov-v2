"use client";

import { KeyRound, LogIn } from "lucide-react";
import { useState, type FormEvent } from "react";

import { FormMessage, TextField } from "@/components/form-parts";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardFooter } from "@/components/ui/card";
import { Notice } from "@/components/ui/notice";
import { useGuardedError } from "@/features/operator/use-guarded-error";
import { changePassword } from "@/lib/api";
import { passwordRejectionText } from "@/lib/errors";
import { goTo } from "@/lib/navigate";
import { SIGN_IN_PATH } from "@/lib/route-access";

/**
 * `/tai-khoan/doi-mat-khau` — change the operator's own password, re-proving the current one and
 * a TOTP code (`PUT /operators/current/password`).
 *
 * On success identity revokes EVERY session of the account, this one included, and the server
 * clears the cookie: the screen says so and offers sign-in — it does not pretend the person is
 * still signed in.
 *
 * 403 `credentials_refused` is a typo, not a sign-out (operator_sessions.go `changePassword`): the
 * person stays here.
 */

export const CHANGE_PASSWORD_DONE =
  "Đã đổi mật khẩu. Mọi phiên đăng nhập của tài khoản, kể cả phiên này, đã kết thúc. Hãy đăng nhập lại bằng mật khẩu mới.";

export const CREDENTIALS_REFUSED = "Mật khẩu hiện tại hoặc mã xác thực không đúng. Kiểm tra lại rồi thử lại.";

export function ChangePasswordDone() {
  return (
    <Card as="section" role="status">
      <CardContent className="flex flex-col items-start gap-4">
        <p className="m-0 text-sm text-ink-700">{CHANGE_PASSWORD_DONE}</p>
        <Button
          type="button"
          variant="primary"
          onClick={() => goTo(SIGN_IN_PATH)}
          icon={<LogIn aria-hidden="true" focusable="false" strokeWidth={1.8} />}
        >
          Đăng nhập lại
        </Button>
      </CardContent>
    </Card>
  );
}

export function ChangePasswordForm() {
  const guarded = useGuardedError();
  const [current, setCurrent] = useState("");
  const [next, setNext] = useState("");
  const [confirm, setConfirm] = useState("");
  const [totp, setTotp] = useState("");
  const [mismatch, setMismatch] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [done, setDone] = useState(false);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (busy) return;
    setError(null);
    if (next !== confirm) {
      setMismatch(true);
      return;
    }
    setMismatch(false);
    setBusy(true);
    const code = totp;
    setTotp("");
    try {
      await changePassword({ currentPassword: current, newPassword: next, totpCode: code });
      setCurrent("");
      setNext("");
      setConfirm("");
      setDone(true);
    } catch (err) {
      setCurrent("");
      setError(
        guarded(err, "đổi mật khẩu", (e) => {
          if (e.code === "credentials_refused") return CREDENTIALS_REFUSED;
          if (e.code === "new_password_rejected") return passwordRejectionText(e.passwordRejection, "mật khẩu hiện tại");
          return null;
        }),
      );
    } finally {
      setBusy(false);
    }
  }

  if (done) return <ChangePasswordDone />;

  return (
    <Card as="form" method="post" onSubmit={submit} noValidate>
      <CardContent className="flex flex-col gap-4">
        <Notice tone="info">
          <p>Sau khi đổi, mọi phiên đăng nhập của tài khoản sẽ kết thúc và bạn cần đăng nhập lại.</p>
        </Notice>
        <TextField
          label="Mật khẩu hiện tại"
          name="current_password"
          type="password"
          autoComplete="current-password"
          value={current}
          onChange={(e) => setCurrent(e.target.value)}
          disabled={busy}
        />
        <TextField
          label="Mật khẩu mới"
          name="new_password"
          type="password"
          autoComplete="new-password"
          value={next}
          onChange={(e) => setNext(e.target.value)}
          disabled={busy}
        />
        <TextField
          label="Nhập lại mật khẩu mới"
          name="confirm_password"
          type="password"
          autoComplete="new-password"
          value={confirm}
          onChange={(e) => setConfirm(e.target.value)}
          disabled={busy}
          error={mismatch ? "Hai lần nhập mật khẩu mới không khớp. Hãy nhập lại." : null}
        />
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
          icon={<KeyRound aria-hidden="true" focusable="false" strokeWidth={1.8} />}
        >
          {busy ? "Đang đổi mật khẩu…" : "Đổi mật khẩu"}
        </Button>
      </CardFooter>
    </Card>
  );
}
