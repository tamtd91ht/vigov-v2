"use client";

import { useId, useState, type FormEvent } from "react";

import { NotWiredError, signIn, type SecondFactor } from "@/lib/api";

/**
 * Operator sign-in: password + a second factor that is ALWAYS required (ADR 0048 §28/09 #10) —
 * a 6-digit TOTP code, or one of the single-use recovery codes. There is no path without it, so
 * the form has no "skip" and no "remember this device".
 *
 * NOT WIRED YET: `signIn` throws `NotWiredError` until service-platform's operator sign-in route
 * exists (TASK-05 → TASK-06b). The form says so plainly instead of pretending to sign in.
 *
 * ONE MESSAGE FOR EVERY REFUSAL, when wired: the server is expected to answer a wrong email, a
 * wrong password and a wrong code with the same sentence; telling them apart here would rebuild
 * the account-existence oracle the server hides. No `required` attributes for the same reason
 * web-admin records — the browser's own "fill in this field" is a second, different message.
 *
 * COOKIES: nothing here reads or writes one. The session cookie is set by the server, HttpOnly.
 */

type FactorKind = SecondFactor["kind"];

const NOT_WIRED_MESSAGE = "Đăng nhập khu vận hành chưa hoạt động: máy chủ chưa mở chức năng này.";
const GENERIC_FAILURE = "Không đăng nhập được. Vui lòng thử lại.";

export function SignInForm() {
  const emailId = useId();
  const passwordId = useId();
  const codeId = useId();
  const errorId = useId();

  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [factor, setFactor] = useState<FactorKind>("totp");
  const [code, setCode] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  function switchFactor() {
    // The code typed for one factor is meaningless for the other; carrying it over would submit
    // a TOTP as a recovery code.
    setFactor(factor === "totp" ? "recovery" : "totp");
    setCode("");
  }

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (submitting) return;
    setSubmitting(true);
    setError(null);
    try {
      await signIn({ email, password, secondFactor: { kind: factor, code } });
    } catch (e) {
      setError(e instanceof NotWiredError ? NOT_WIRED_MESSAGE : GENERIC_FAILURE);
    } finally {
      setSubmitting(false);
    }
  }

  const describedBy = error ? errorId : undefined;

  return (
    <form className="sign-in-form" onSubmit={submit} noValidate>
      <h1 className="form-title">Đăng nhập khu vận hành</h1>

      <div className="field">
        <label htmlFor={emailId}>Thư điện tử</label>
        <input
          id={emailId}
          name="email"
          type="email"
          autoComplete="username"
          autoCapitalize="none"
          spellCheck={false}
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          disabled={submitting}
          aria-describedby={describedBy}
        />
      </div>

      <div className="field">
        <label htmlFor={passwordId}>Mật khẩu</label>
        <input
          id={passwordId}
          name="password"
          type="password"
          autoComplete="current-password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          disabled={submitting}
          aria-describedby={describedBy}
        />
      </div>

      <div className="field">
        <label htmlFor={codeId}>
          {factor === "totp" ? "Mã xác thực (6 số trên ứng dụng xác thực)" : "Mã khôi phục"}
        </label>
        <input
          id={codeId}
          name={factor === "totp" ? "totp" : "recovery_code"}
          type="text"
          inputMode={factor === "totp" ? "numeric" : "text"}
          autoComplete={factor === "totp" ? "one-time-code" : "off"}
          autoCapitalize="none"
          spellCheck={false}
          maxLength={factor === "totp" ? 6 : undefined}
          value={code}
          onChange={(e) => setCode(e.target.value)}
          disabled={submitting}
          aria-describedby={describedBy}
        />
        <button type="button" className="link-button" onClick={switchFactor} disabled={submitting}>
          {factor === "totp" ? "Dùng mã khôi phục" : "Dùng mã trên ứng dụng xác thực"}
        </button>
      </div>

      <p id={errorId} className="form-error" role="alert" aria-live="assertive">
        {error ?? ""}
      </p>

      <button type="submit" className="primary-button" disabled={submitting} aria-busy={submitting}>
        {submitting ? "Đang đăng nhập…" : "Đăng nhập"}
      </button>
    </form>
  );
}
