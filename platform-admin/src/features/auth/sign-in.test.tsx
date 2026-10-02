import { renderToString } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { QrCode, qrPath } from "@/components/qr-code";
import { ApiError, type EnrollmentStart } from "@/lib/api";

import { RECOVERY_CODES_SAVED_LABEL, RecoveryCodesOnce, recoveryCodesText } from "./recovery-codes-once";
import { EnrollFinishStep, groupKey, SignInFlow } from "./sign-in-flow";
import {
  ENROLLMENT_FINISH_REFUSED,
  INITIAL_SIGN_IN_STATE,
  isEnrollmentRequired,
  SIGN_IN_REFUSED,
  SIGN_IN_UNAVAILABLE,
  signInFailure,
  signInReducer,
  type SignInState,
} from "./sign-in-state";

/**
 * The sign-in flow's decisions are pure (`sign-in-state.ts`) and tested here directly; the screens
 * are rendered to strings for what they must and must not contain. vitest runs in Node without a
 * DOM by design (`vitest.config.mts`), so a click is not simulated — the reducer IS what a click
 * dispatches.
 */

const apiError = (status: number, code: string, message = "", retryAfter: number | null = null) =>
  new ApiError(status, code, message, "", retryAfter);

describe("sign-in failures", () => {
  it("every refusal reads the SAME sentence, whatever the server's message says", () => {
    const a = signInFailure(apiError(401, "sign_in_refused", "wrong password"), SIGN_IN_REFUSED);
    const b = signInFailure(apiError(401, "sign_in_refused", "no such account"), SIGN_IN_REFUSED);
    expect(a.text).toBe(SIGN_IN_REFUSED);
    expect(b.text).toBe(a.text);
  });

  it("403 enrollment_required is the enrolment branch, not a failure", () => {
    expect(isEnrollmentRequired(apiError(403, "enrollment_required"))).toBe(true);
    expect(isEnrollmentRequired(apiError(403, "forbidden"))).toBe(false);
    expect(isEnrollmentRequired(apiError(401, "sign_in_refused"))).toBe(false);
  });

  it("429 shows the wait from Retry-After", () => {
    expect(signInFailure(apiError(429, "rate_limited", "", 60), SIGN_IN_REFUSED).text).toContain("1 phút");
    expect(signInFailure(apiError(429, "rate_limited", "", 42), SIGN_IN_REFUSED).text).toContain("42 giây");
  });

  it("503 says the auth service is down — never 'wrong password'", () => {
    for (const code of ["operator_auth_unavailable", "rate_limit_unavailable"]) {
      expect(signInFailure(apiError(503, code), SIGN_IN_REFUSED).text).toBe(SIGN_IN_UNAVAILABLE);
    }
  });

  it("completing enrolment uses its own refusal sentence and the server's password bounds", () => {
    expect(signInFailure(apiError(401, "sign_in_refused"), ENROLLMENT_FINISH_REFUSED).text).toBe(ENROLLMENT_FINISH_REFUSED);
    const rejected = new ApiError(422, "new_password_rejected", "", "", null, { problem: "too_short", minLength: 15, maxLength: 128 });
    expect(signInFailure(rejected, ENROLLMENT_FINISH_REFUSED).text).toContain("15 ký tự");
  });
});

const START: EnrollmentStart = {
  operator_code: "VH-00001",
  provisioning_uri: "otpauth://totp/ViGov:VH-00001?secret=JBSWY3DPEHPK3PXP&issuer=ViGov",
  manual_entry_key: "JBSWY3DPEHPK3PXP",
};

describe("sign-in state — what each step keeps and what it forgets", () => {
  const typed: SignInState = { ...INITIAL_SIGN_IN_STATE, email: "op@example.vn", password: "tam-thoi" };

  it("enrolment carries the email and the just-accepted temporary password into step 1", () => {
    const s = signInReducer(typed, { type: "enrollment_required" });
    expect(s).toMatchObject({ step: "enroll_start", email: "op@example.vn", password: "tam-thoi" });
  });

  it("completion forgets the temporary password and the TOTP secret, and holds the codes", () => {
    const started = signInReducer(signInReducer(typed, { type: "enrollment_required" }), { type: "enrollment_started", start: START });
    expect(started.enrollment).toEqual(START);
    const done = signInReducer(started, { type: "enrollment_completed", recoveryCodes: ["a", "b"] });
    expect(done).toEqual({ step: "recovery_codes", email: "", password: "", enrollment: null, recoveryCodes: ["a", "b"] });
  });

  it("the codes are shown ONCE: confirming they were saved drops them", () => {
    const codes = signInReducer(INITIAL_SIGN_IN_STATE, { type: "enrollment_completed", recoveryCodes: ["a", "b"] });
    const after = signInReducer(codes, { type: "recovery_codes_saved" });
    expect(after.recoveryCodes).toEqual([]);
    expect(after.step).toBe("done");
  });

  it("signing in, or starting over, blanks the password", () => {
    expect(signInReducer(typed, { type: "signed_in" })).toMatchObject({ password: "", email: "" });
    const restarted = signInReducer({ ...typed, step: "enroll_finish", enrollment: START }, { type: "restart" });
    expect(restarted).toMatchObject({ step: "sign_in", password: "", enrollment: null, email: "op@example.vn" });
  });
});

describe("sign-in screens", () => {
  it("first screen: email, password, TOTP with the recovery-code toggle, POST form, no demo wording", () => {
    const html = renderToString(<SignInFlow />);
    expect(html).toContain("Thư điện tử");
    expect(html).toContain("Mật khẩu");
    expect(html).toContain("Mã xác thực");
    expect(html).toContain('autoComplete="one-time-code"');
    expect(html).toContain("Dùng mã khôi phục");
    // If the script never runs, the browser's default GET would put the password in the URL.
    expect(html).toContain('method="post"');
    expect(html.toLowerCase()).not.toMatch(/demo|trải nghiệm|dùng thử/);
  });

  it("enrolment step 2 draws the QR locally and shows the manual key, never the URI as text or a link", () => {
    const html = renderToString(
      <EnrollFinishStep state={{ ...INITIAL_SIGN_IN_STATE, step: "enroll_finish", enrollment: START }} dispatch={() => {}} />,
    );
    expect(html).toContain("<svg");
    expect(html).toContain(groupKey(START.manual_entry_key));
    expect(html).not.toContain("otpauth://");
    expect(html).not.toMatch(/<img/);
    expect(html).toContain('method="post"');
  });

  it("groupKey splits the key in fours", () => {
    expect(groupKey("JBSWY3DPEHPK3PXP")).toBe("JBSW Y3DP EHPK 3PXP");
    expect(groupKey("ABCDE")).toBe("ABCD E");
  });

  it("the recovery-code screen shows every code, and cannot be left before 'Tôi đã lưu'", () => {
    const codes = ["k7dq-2mxa", "p9tz-4hvr", "w3ne-8cjb"];
    const html = renderToString(<RecoveryCodesOnce codes={codes} onSaved={() => {}} continueLabel="Tiếp tục" />);
    for (const c of codes) expect(html).toContain(c);
    expect(html).toContain(RECOVERY_CODES_SAVED_LABEL);
    expect(html).toContain("một lần duy nhất");
    expect(html).toMatch(/<button[^>]*disabled=""[^>]*>Tiếp tục<\/button>/);
    expect(recoveryCodesText(codes)).toContain(codes.join("\n"));
  });

  // The production CSP refuses `style` attributes (`lib/csp.ts`); the redesign draws with classes.
  it("no sign-in screen draws a style attribute", () => {
    const screens = [
      <SignInFlow key="a" />,
      <EnrollFinishStep key="b" state={{ ...INITIAL_SIGN_IN_STATE, step: "enroll_finish", enrollment: START }} dispatch={() => {}} />,
      <RecoveryCodesOnce key="c" codes={["k7dq-2mxa"]} onSaved={() => {}} continueLabel="Tiếp tục" />,
    ];
    for (const s of screens) expect(renderToString(s)).not.toContain("style=");
  });
});

describe("QR code", () => {
  it("one square per dark module", () => {
    expect(qrPath([[true, false], [false, true]])).toBe("M0 0h1v1h-1zM1 1h1v1h-1z");
  });

  it("renders as SVG elements with attributes only — no style, no image source", () => {
    const html = renderToString(<QrCode value={START.provisioning_uri} label="QR" />);
    expect(html).toMatch(/^<svg[^>]*role="img"/);
    expect(html).not.toContain("style=");
    expect(html).not.toContain("href");
  });
});
