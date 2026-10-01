"use client";

import { useReducer, useState, type FormEvent } from "react";

import { FormMessage, TextField } from "@/components/form-parts";
import { QrCode } from "@/components/qr-code";
import { beginEnrollment, completeEnrollment, signIn, type SecondFactor } from "@/lib/api";
import { goTo } from "@/lib/navigate";

import { RecoveryCodesOnce } from "./recovery-codes-once";
import {
  ENROLLMENT_FINISH_REFUSED,
  INITIAL_SIGN_IN_STATE,
  isEnrollmentRequired,
  SIGN_IN_REFUSED,
  signInFailure,
  signInReducer,
  type FailureText,
  type SignInAction,
  type SignInState,
} from "./sign-in-state";

/**
 * `/dang-nhap`: sign-in, and the first-sign-in enrolment it branches into.
 *
 * Password + a second factor that is ALWAYS required (ADR 0048 §28/09 #10) — a 6-digit TOTP code,
 * or a single-use recovery code. No "skip", no "remember this device".
 *
 * EVERY FORM IS `method="post"`. `onSubmit` prevents the browser submission; `method="post"` is
 * for the moment it does not run (script blocked, hydration failed): the browser's default GET
 * would put the password in the address bar, the history and every proxy log on the way.
 *
 * No `required` attributes, for the reason web-admin records: the browser's own "fill in this
 * field" is a second, different message beside the server's one.
 *
 * Where the console opens after signing in is a fixed path. There is no "return to" parameter yet;
 * one would need the open-redirect check web-admin carries (ledger `chua-dung-man-hinh-nao`).
 */

export const CONSOLE_HOME = "/xa";

export function SignInFlow() {
  const [state, dispatch] = useReducer(signInReducer, INITIAL_SIGN_IN_STATE);

  switch (state.step) {
    case "sign_in":
      return <SignInStep state={state} dispatch={dispatch} />;
    case "enroll_start":
      return <EnrollStartStep state={state} dispatch={dispatch} />;
    case "enroll_finish":
      return <EnrollFinishStep state={state} dispatch={dispatch} />;
    case "recovery_codes":
      return (
        <div className="sign-in-form wide">
          <h1 className="form-title">Đăng ký hoàn tất</h1>
          <RecoveryCodesOnce
            codes={state.recoveryCodes}
            continueLabel="Tiếp tục vào khu vận hành"
            onSaved={() => {
              dispatch({ type: "recovery_codes_saved" });
              goTo(CONSOLE_HOME);
            }}
          />
        </div>
      );
    case "done":
      return (
        <div className="sign-in-form" role="status">
          <p>Đang mở khu vận hành…</p>
        </div>
      );
  }
}

type StepProps = { state: SignInState; dispatch: (a: SignInAction) => void };

export function SignInStep({ state, dispatch }: StepProps) {
  const [factor, setFactor] = useState<SecondFactor["kind"]>("totp");
  const [code, setCode] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [failure, setFailure] = useState<FailureText | null>(null);

  function switchFactor() {
    // A code typed for one factor is meaningless for the other; carrying it over would submit a
    // TOTP as a recovery code.
    setFactor(factor === "totp" ? "recovery" : "totp");
    setCode("");
  }

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (submitting) return;
    setSubmitting(true);
    setFailure(null);
    const secondFactor = { kind: factor, code };
    // The one-time code is spent whatever the answer: never left in the field to be resent.
    setCode("");
    try {
      await signIn({ email: state.email, password: state.password, secondFactor });
      dispatch({ type: "signed_in" });
      goTo(CONSOLE_HOME);
    } catch (err) {
      if (isEnrollmentRequired(err)) {
        dispatch({ type: "enrollment_required" });
        return;
      }
      dispatch({ type: "set_password", value: "" });
      setFailure(signInFailure(err, SIGN_IN_REFUSED));
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <form className="sign-in-form" method="post" onSubmit={submit} noValidate>
      <h1 className="form-title">Đăng nhập khu vận hành</h1>
      <TextField
        label="Thư điện tử"
        name="email"
        type="email"
        autoComplete="username"
        autoCapitalize="none"
        spellCheck={false}
        value={state.email}
        onChange={(e) => dispatch({ type: "set_email", value: e.target.value })}
        disabled={submitting}
      />
      <TextField
        label="Mật khẩu"
        name="password"
        type="password"
        autoComplete="current-password"
        value={state.password}
        onChange={(e) => dispatch({ type: "set_password", value: e.target.value })}
        disabled={submitting}
      />
      <TextField
        label={factor === "totp" ? "Mã xác thực (6 số trên ứng dụng xác thực)" : "Mã khôi phục"}
        name={factor === "totp" ? "totp_code" : "recovery_code"}
        type="text"
        inputMode={factor === "totp" ? "numeric" : "text"}
        autoComplete={factor === "totp" ? "one-time-code" : "off"}
        autoCapitalize="none"
        spellCheck={false}
        maxLength={factor === "totp" ? 6 : undefined}
        value={code}
        onChange={(e) => setCode(e.target.value)}
        disabled={submitting}
      />
      <button type="button" className="link-button field-toggle" onClick={switchFactor} disabled={submitting}>
        {factor === "totp" ? "Dùng mã khôi phục" : "Dùng mã trên ứng dụng xác thực"}
      </button>
      <FormMessage text={failure?.text ?? null} trace={failure?.trace} />
      <button type="submit" className="primary-button" disabled={submitting} aria-busy={submitting}>
        {submitting ? "Đang đăng nhập…" : "Đăng nhập"}
      </button>
      <p className="field-hint form-footnote">
        Lần đầu đăng nhập: nhập thư điện tử và mật khẩu tạm được cấp, để trống mã xác thực. Hệ thống sẽ hướng dẫn
        đăng ký ứng dụng xác thực.
      </p>
    </form>
  );
}

export function EnrollStartStep({ state, dispatch }: StepProps) {
  const [submitting, setSubmitting] = useState(false);
  const [failure, setFailure] = useState<FailureText | null>(null);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (submitting) return;
    setSubmitting(true);
    setFailure(null);
    try {
      const start = await beginEnrollment({ email: state.email, temporaryPassword: state.password });
      dispatch({ type: "enrollment_started", start });
    } catch (err) {
      dispatch({ type: "set_password", value: "" });
      setFailure(signInFailure(err, SIGN_IN_REFUSED));
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <form className="sign-in-form" method="post" onSubmit={submit} noValidate>
      <h1 className="form-title">Đăng ký ứng dụng xác thực</h1>
      <p>
        Tài khoản của bạn chưa đăng ký ứng dụng xác thực. Mỗi lần đăng nhập khu vận hành đều cần mã 6 số từ ứng
        dụng này. Chuẩn bị điện thoại đã cài một ứng dụng xác thực rồi bấm “Tiếp tục”.
      </p>
      <TextField
        label="Thư điện tử"
        name="email"
        type="email"
        autoComplete="username"
        autoCapitalize="none"
        spellCheck={false}
        value={state.email}
        onChange={(e) => dispatch({ type: "set_email", value: e.target.value })}
        disabled={submitting}
      />
      <TextField
        label="Mật khẩu tạm được cấp"
        name="temporary_password"
        type="password"
        autoComplete="current-password"
        value={state.password}
        onChange={(e) => dispatch({ type: "set_password", value: e.target.value })}
        disabled={submitting}
      />
      <FormMessage text={failure?.text ?? null} trace={failure?.trace} />
      <button type="submit" className="primary-button" disabled={submitting} aria-busy={submitting}>
        {submitting ? "Đang xử lý…" : "Tiếp tục"}
      </button>
      <button type="button" className="link-button" onClick={() => dispatch({ type: "restart" })} disabled={submitting}>
        Quay lại đăng nhập
      </button>
    </form>
  );
}

/** Groups of four, so a key typed by hand can be checked group by group. */
export function groupKey(key: string): string {
  return key.replace(/\s+/g, "").replace(/(.{4})(?=.)/g, "$1 ");
}

export function EnrollFinishStep({ state, dispatch }: StepProps) {
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [totpCode, setTotpCode] = useState("");
  const [mismatch, setMismatch] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [failure, setFailure] = useState<FailureText | null>(null);
  const enrollment = state.enrollment;
  if (enrollment === null) return null;

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (submitting) return;
    setFailure(null);
    if (newPassword !== confirmPassword) {
      setMismatch(true);
      return;
    }
    setMismatch(false);
    setSubmitting(true);
    const code = totpCode;
    setTotpCode("");
    try {
      const done = await completeEnrollment({
        email: state.email,
        temporaryPassword: state.password,
        newPassword,
        totpCode: code,
      });
      setNewPassword("");
      setConfirmPassword("");
      dispatch({ type: "enrollment_completed", recoveryCodes: done.recovery_codes });
    } catch (err) {
      setFailure(signInFailure(err, ENROLLMENT_FINISH_REFUSED));
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <form className="sign-in-form wide" method="post" onSubmit={submit} noValidate>
      <h1 className="form-title">Đăng ký ứng dụng xác thực</h1>
      <ol className="steps">
        <li>
          <p>Mở ứng dụng xác thực trên điện thoại, chọn thêm tài khoản và quét mã QR dưới đây.</p>
          <QrCode value={enrollment.provisioning_uri} label="Mã QR để thêm tài khoản vận hành vào ứng dụng xác thực" />
          <p>Không quét được? Chọn nhập khoá thủ công và gõ khoá sau:</p>
          <p className="manual-key">
            <code>{groupKey(enrollment.manual_entry_key)}</code>
          </p>
          <p className="field-hint">Không chụp ảnh, không gửi mã QR hay khoá này cho bất kỳ ai.</p>
        </li>
        <li>
          <p>Đặt mật khẩu mới thay cho mật khẩu tạm.</p>
          <TextField
            label="Mật khẩu mới"
            name="new_password"
            type="password"
            autoComplete="new-password"
            value={newPassword}
            onChange={(e) => setNewPassword(e.target.value)}
            disabled={submitting}
          />
          <TextField
            label="Nhập lại mật khẩu mới"
            name="confirm_password"
            type="password"
            autoComplete="new-password"
            value={confirmPassword}
            onChange={(e) => setConfirmPassword(e.target.value)}
            disabled={submitting}
            error={mismatch ? "Hai lần nhập mật khẩu mới không khớp. Hãy nhập lại." : null}
          />
        </li>
        <li>
          <TextField
            label="Mã xác thực (6 số đang hiện trên ứng dụng)"
            name="totp_code"
            type="text"
            inputMode="numeric"
            autoComplete="one-time-code"
            maxLength={6}
            value={totpCode}
            onChange={(e) => setTotpCode(e.target.value)}
            disabled={submitting}
          />
        </li>
      </ol>
      <FormMessage text={failure?.text ?? null} trace={failure?.trace} />
      <button type="submit" className="primary-button" disabled={submitting} aria-busy={submitting}>
        {submitting ? "Đang hoàn tất…" : "Hoàn tất đăng ký"}
      </button>
      <button type="button" className="link-button" onClick={() => dispatch({ type: "restart" })} disabled={submitting}>
        Bắt đầu lại
      </button>
    </form>
  );
}
