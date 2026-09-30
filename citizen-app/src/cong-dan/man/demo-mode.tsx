/**
 * DEMO BUILD OF THE COMMUNE'S OWN APP — the words and the two pieces of sample data, in one place.
 *
 * Owner's decision 30/09/2026, confirmed after the consequences were explained: while the commune app has not
 * been submitted to Zalo, `deploy.mjs --vao-thang --demo` builds a version that can be shown end to end even
 * though the App ID does not yet have Zalo's name/phone permissions. The owner drops the flag at submission.
 *
 * WHAT IT DOES, AND ONLY IN THAT BUILD (`DEMO_BUILD`, a compile-time constant — every other build folds these
 * branches away):
 *   · the name Zalo did not give → `DEMO_CITIZEN_NAME`, silently (no card, no error line). A real Zalo name wins;
 *   · the phone step Zalo refused → the screen the citizen asked for opens anyway, its phone field pre-filled
 *     with `DEMO_PHONE` — the repo's agreed fake number (rule 3 #5), never a plausible real one;
 *   · anything that needs the server with a session → ONE sentence (`DEMO_WORDS.task`), never a fake success,
 *     never an invented petition code, never the gate again;
 *   · a band "Chế độ demo" on every screen, so a screenshot cannot be taken for the real app.
 *
 * WHAT IT NEVER DOES: open, fake or widen a session. The server is unchanged — a real ViGov session still
 * needs Zalo's phone token, and when Zalo gives it everything works exactly as in a normal build.
 */
import { DEMO_BUILD } from "../../lib/demo-build";

import type { PhoneVerificationTask } from "./noi-dung";
import type { NameAtEntry } from "./trai-nghiem";

/** Sample name given by the owner for the demo. Pre-fill and greeting only — it grants nothing (rule 4). */
export const DEMO_CITIZEN_NAME = "Nguyễn Văn Hùng";

/** The repo's agreed fake number (rule 3 #5). Pre-fill only; it is never sent — there is no session to send with. */
export const DEMO_PHONE = "0900000000";

export const DEMO_WORDS = {
  band_title: "Chế độ demo",
  band_body: "Bản trình diễn, chưa phải ứng dụng chính thức của xã.",
  notice_title: "Chế độ demo",
  back: "Quay lại",
  /** One sentence per act that needs the server: what did not happen, and why. */
  task: {
    submit: "Chế độ demo: phản ánh chưa gửi được lên xã vì Zalo chưa cấp quyền số điện thoại cho ứng dụng.",
    mine: "Chế độ demo: chưa tải được phản ánh của bạn từ xã vì Zalo chưa cấp quyền số điện thoại cho ứng dụng.",
    lookup: "Chế độ demo: chưa tra cứu được phiếu với xã vì Zalo chưa cấp quyền số điện thoại cho ứng dụng.",
    rate: "Chế độ demo: đánh giá chưa gửi được lên xã vì Zalo chưa cấp quyền số điện thoại cho ứng dụng.",
  } satisfies Record<PhoneVerificationTask, string>,
  /** Step 1 of the send form: the commune's field list needs a session, and there is no built-in list (ADR 0060 §3). */
  fields:
    "Chế độ demo: chưa tải được danh sách lĩnh vực của xã vì Zalo chưa cấp quyền số điện thoại; bấm “Tiếp tục” để xem biểu mẫu.",
  fields_continue: "Tiếp tục",
} as const;

/**
 * The entry name state, with the demo name where Zalo gave none. PURE. Outside a demo build: unchanged.
 * `needs-consent` is settled at once — the card that would ask is exactly the nagging the demo removes.
 */
export function withDemoName(state: NameAtEntry): NameAtEntry {
  if (!DEMO_BUILD) return state;
  if (state.kind === "needs-consent") return { kind: "settled", name: DEMO_CITIZEN_NAME };
  if (state.kind === "settled" && state.name === null) return { kind: "settled", name: DEMO_CITIZEN_NAME };
  return state;
}

/** The phone field of a fresh send form: the fake number in a demo open without a session, empty otherwise. */
export function demoPhonePrefill(demoWithoutSession: boolean): string {
  return DEMO_BUILD && demoWithoutSession ? DEMO_PHONE : "";
}

/**
 * The band on every commune-app screen. Words, not colour alone (`role="note"`); `--text-small` or larger;
 * ink on light orange, measured in `accessibility.test.ts`. Nothing at all outside a demo build.
 */
export function DemoBand() {
  if (!DEMO_BUILD) return null;
  return (
    <p className="xa-demo" role="note">
      <strong>{DEMO_WORDS.band_title}</strong> · {DEMO_WORDS.band_body}
    </p>
  );
}
