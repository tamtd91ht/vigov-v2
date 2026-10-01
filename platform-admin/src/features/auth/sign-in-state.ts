import { ApiError, type EnrollmentStart } from "@/lib/api";
import { NETWORK_ERROR, passwordRejectionText, traceLine, waitMessage } from "@/lib/errors";

/**
 * The sign-in page's steps, as a pure reducer so the transitions — and above all what each one
 * FORGETS — are tested without a browser (`sign-in.test.ts`).
 *
 *   sign_in ──403 enrollment_required──▶ enroll_start ──201──▶ enroll_finish ──201──▶ recovery_codes
 *      │                                                                                   │
 *      └──────────────── 201 ──────────────▶ done ◀──────────── "Tôi đã lưu" ──────────────┘
 *
 * WHERE CREDENTIALS LIVE: in this state, in memory, for as long as a step needs them — the
 * enrolment completion has to resend the email and temporary password, so they survive step 1.
 * Never a URL, never Web Storage, never a cookie this app writes. Every exit
 * (done, restart) blanks them; the recovery codes are blanked by the confirmation that they were
 * saved, which is what "shown once" means.
 */

export type SignInStep = "sign_in" | "enroll_start" | "enroll_finish" | "recovery_codes" | "done";

export type SignInState = {
  step: SignInStep;
  email: string;
  /** The password typed at sign-in; at enrolment it IS the temporary password (operator.proto). */
  password: string;
  /** The pending TOTP secret, only during `enroll_finish`. */
  enrollment: EnrollmentStart | null;
  /** Only during `recovery_codes`. */
  recoveryCodes: string[];
};

export type SignInAction =
  | { type: "set_email"; value: string }
  | { type: "set_password"; value: string }
  | { type: "signed_in" }
  | { type: "enrollment_required" }
  | { type: "enrollment_started"; start: EnrollmentStart }
  | { type: "enrollment_completed"; recoveryCodes: string[] }
  | { type: "recovery_codes_saved" }
  | { type: "restart" };

export const INITIAL_SIGN_IN_STATE: SignInState = {
  step: "sign_in",
  email: "",
  password: "",
  enrollment: null,
  recoveryCodes: [],
};

export function signInReducer(state: SignInState, action: SignInAction): SignInState {
  switch (action.type) {
    case "set_email":
      return { ...state, email: action.value };
    case "set_password":
      return { ...state, password: action.value };
    case "signed_in":
      return { ...INITIAL_SIGN_IN_STATE, step: "done" };
    case "enrollment_required":
      // The password just accepted is the temporary one: carried into step 1 so it is not retyped.
      return { ...state, step: "enroll_start", enrollment: null };
    case "enrollment_started":
      return { ...state, step: "enroll_finish", enrollment: action.start };
    case "enrollment_completed":
      // The session exists now; the temporary password and the TOTP secret have no further use.
      return { ...INITIAL_SIGN_IN_STATE, step: "recovery_codes", recoveryCodes: action.recoveryCodes };
    case "recovery_codes_saved":
      return { ...INITIAL_SIGN_IN_STATE, step: "done" };
    case "restart":
      return { ...INITIAL_SIGN_IN_STATE, email: state.email };
  }
}

/**
 * ONE sentence for every refusal. The server already answers a wrong email, a wrong password, a
 * wrong code and a locked account alike (`sign_in_refused`); a second sentence here would rebuild
 * the account-existence oracle it hides.
 */
export const SIGN_IN_REFUSED =
  "Thông tin đăng nhập không đúng, hoặc tài khoản tạm thời không đăng nhập được. Kiểm tra lại rồi thử lại.";

export const ENROLLMENT_FINISH_REFUSED =
  "Không hoàn tất được đăng ký. Mã trên ứng dụng xác thực đổi sau mỗi 30 giây: gõ mã đang hiện rồi thử lại. Nếu vẫn không được, bấm “Bắt đầu lại”.";

export const SIGN_IN_UNAVAILABLE = "Hệ thống xác thực tạm không phản hồi. Vui lòng thử lại sau ít phút.";

export const SECOND_FACTOR_AMBIGUOUS = "Chỉ nhập mã trên ứng dụng xác thực hoặc mã khôi phục, không nhập cả hai.";

export type FailureText = { text: string; trace: string | null };

/**
 * Failure of a PUBLIC sign-in step (sign in, begin / complete enrolment). `refused` is the step's
 * own 401 sentence. 403 `enrollment_required` is not a failure — the caller branches on it first.
 */
export function signInFailure(err: unknown, refused: string): FailureText {
  const trace = traceLine(err);
  if (!(err instanceof ApiError)) return { text: SIGN_IN_REFUSED, trace };
  switch (true) {
    case err.status === 0:
      return { text: NETWORK_ERROR, trace };
    case err.status === 401:
      return { text: refused, trace };
    case err.status === 429:
      return { text: waitMessage(err.retryAfterSeconds), trace };
    case err.status === 503:
      return { text: SIGN_IN_UNAVAILABLE, trace };
    case err.code === "second_factor_ambiguous":
      return { text: SECOND_FACTOR_AMBIGUOUS, trace };
    case err.code === "new_password_rejected":
      return { text: passwordRejectionText(err.passwordRejection, "mật khẩu tạm"), trace };
    default:
      return { text: SIGN_IN_REFUSED, trace };
  }
}

export function isEnrollmentRequired(err: unknown): boolean {
  return err instanceof ApiError && err.status === 403 && err.code === "enrollment_required";
}
