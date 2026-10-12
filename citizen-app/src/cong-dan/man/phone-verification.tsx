/**
 * XÁC NHẬN SỐ ĐIỆN THOẠI GIỮA CHỪNG — dùng chung cho ba màn phản ánh (gửi · tra cứu · phản ánh của tôi).
 *
 * KHI NÀO: một tuyến phản ánh trả 403 `chua_xac_thuc_so` (`api/goi-vigov.ts` nhánh `can-xac-thuc-so`) —
 * phiên ViGov mở lúc xác nhận xã KHÔNG kèm số điện thoại (ADR 0045 câu 2), còn ba tuyến này cần một công
 * dân đã xác thực số (`core/httpx/citizen.go:199-200`). Quyết định của người dùng 28/09/2026: hỏi công
 * dân, mở lại phiên KÈM mã số điện thoại, rồi gọi lại việc cũ ĐÚNG MỘT LẦN.
 *
 * ⚠ HỎI TRƯỚC, XIN SAU. Hộp thoại xin số của Zalo chỉ hiện SAU cú bấm "Đồng ý chia sẻ số điện thoại" trên
 * màn này — không bao giờ tự bật lên khi màn vừa mở hay khi một lời gọi vừa trả 403. Cú bấm ấy là hành
 * vi đồng ý của công dân; một hộp thoại tự bật là xin thứ người ta chưa hiểu vì sao phải đưa.
 *
 * ⚠ KHÔNG VÒNG LẶP. `createPhoneVerification` giữ một kết cục CUỐI: đã mở lại thành công mà máy chủ vẫn
 *   trả 403, hoặc máy chủ nói thẳng là không xác thực được / khác xã / cầu tắt / ngoài Zalo — thì lần 403
 *   sau chỉ hiện câu kết cục, không hỏi lại, không mở lại, không gọi lại. Chỉ HAI kết cục cho hỏi lại, vì
 *   chỉ ở hai kết cục ấy một cú bấm mới có thể đổi được gì: công dân từ chối (họ có quyền đổi ý), và mạng.
 *
 * ⚠ KHÔNG GIỮ GÌ CỦA SỐ ĐIỆN THOẠI. Mã số điện thoại không bao giờ tới tệp này (`api/mo-phien-vigov.ts`
 *   khối "mở lại"). Không lưu xuống máy (`ranh-gioi-hai-nua.test.ts` §3b), không log.
 *
 * ⚠ NHÁP KHÔNG MẤT. Việc gọi lại là CHÍNH hàm của màn, với CHÍNH lần gửi / mã / con trỏ đã dùng — nội
 *   dung phản ánh đang nằm trong `useState` của màn và không đi qua đây.
 */
import { useRef, useState } from "react";

import {
  type PhoneVerificationOutcome,
  type ReopenWithPhone,
  reopenSessionWithPhone,
  type ZaloFailure,
} from "../api/mo-phien-vigov";

import {
  type PhoneVerificationTask,
  type PhoneVerificationWords,
  SHARED_PHONE_WORDS,
  zaloFailureSentence,
  zaloSupportCode,
} from "./noi-dung";

/** Kết cục KHÔNG gọi lại được — mỗi cái một câu. */
export type PhoneVerificationStop = Exclude<PhoneVerificationOutcome["kieu"], "da-xac-thuc">;

/** Thứ khung hiện. `null` = không hiện gì (chưa gặp 403, hoặc đang gọi lại việc cũ). */
export type PhoneVerificationState =
  | { readonly kieu: "hoi" }
  | { readonly kieu: "dang-xac-nhan" }
  /** `zalo` (with `thu-lai` only): Zalo refused a step with a code — the sentence names it. */
  | { readonly kieu: "ket-qua"; readonly outcome: PhoneVerificationStop; readonly zalo?: ZaloFailure };

export type PhoneVerification = {
  /** Một lời gọi vừa trả `can-xac-thuc-so`. `rerun` lặp lại ĐÚNG lời gọi ấy. */
  onPhoneRequired(rerun: () => void): void;
  /** Công dân bấm "Đồng ý chia sẻ số điện thoại". */
  allow(): Promise<void>;
  /** Công dân bấm "Không chia sẻ". */
  decline(): void;
  /** Công dân tự bắt đầu một việc mới trên màn — gỡ câu cũ. Không gỡ kết cục cuối. */
  reset(): void;
};

/** Hai kết cục duy nhất một cú bấm mới có thể đổi được. */
const CAN_ASK_AGAIN: ReadonlySet<PhoneVerificationStop> = new Set(["tu-choi", "thu-lai"]);

/**
 * Máy trạng thái, THUẦN — không React, để test chạy từng bước mà không cần DOM (khung gắn tự dựng của
 * `hieu-ung-khong-phien.test.tsx` không mô phỏng cú bấm).
 */
export function createPhoneVerification(
  reopen: ReopenWithPhone | undefined,
  setState: (s: PhoneVerificationState | null) => void,
  /**
   * COMMUNE APP ONLY (owner 12/10/2026: no explanation once Zalo holds the permission, for EVERY personal act — here
   * "Gửi đánh giá"): whether Zalo already holds the phone permission (`commune-session.ts` `ReadPhoneGrant`, the
   * shell's `getSetting`). Granted → straight to the reopen, which shows no Zalo dialog; not granted, or the read
   * fails → the explanation as before. Absent (the shared app, every QR path) → the explanation, always.
   * Every `onPhoneRequired` here follows the citizen's own tap on a screen that calls this machine, and a reopen that
   * succeeds sets the final outcome below — so a server that keeps answering 403 meets that sentence, never a loop.
   */
  readGrant?: () => Promise<boolean>,
): PhoneVerification {
  /** Việc chờ gọi lại. Chỉ sống giữa lúc gặp 403 và lúc có kết cục. */
  let pending: (() => void) | null = null;
  /** Kết cục cuối — có rồi thì 403 sau chỉ còn câu này. */
  let finalOutcome: PhoneVerificationStop | null = null;
  let busy = false;

  const machine: PhoneVerification = {
    onPhoneRequired(rerun) {
      if (finalOutcome !== null) {
        pending = null;
        setState({ kieu: "ket-qua", outcome: finalOutcome });
        return;
      }
      if (reopen === undefined) {
        // Không có đường mở lại (không qua bước xác nhận xã ở lần mở này). Hỏi số rồi không làm gì được
        // với nó là hỏi cho có — nói thẳng.
        pending = null;
        finalOutcome = "chua-mo";
        setState({ kieu: "ket-qua", outcome: "chua-mo" });
        return;
      }
      pending = rerun;
      if (readGrant !== undefined) {
        // The "working" words while the permission is read. A citizen who started something else meanwhile
        // (`reset`) is not followed, and neither is a second 403 that replaced this one.
        setState({ kieu: "dang-xac-nhan" });
        void readGrant()
          .catch(() => false)
          .then((granted) => {
            if (pending !== rerun || busy) return;
            if (granted) void machine.allow();
            else setState({ kieu: "hoi" });
          });
        return;
      }
      setState({ kieu: "hoi" });
    },

    async allow() {
      if (reopen === undefined || pending === null || busy) return;
      busy = true;
      setState({ kieu: "dang-xac-nhan" });
      const outcome = await reopenSessionWithPhone(reopen);
      busy = false;
      if (outcome.kieu === "da-xac-thuc") {
        // Gọi lại ĐÚNG MỘT LẦN. Nếu máy chủ vẫn trả 403, `onPhoneRequired` gặp kết cục cuối này.
        finalOutcome = "van-chua-xac-thuc";
        const rerun = pending;
        pending = null;
        setState(null);
        rerun();
        return;
      }
      if (!CAN_ASK_AGAIN.has(outcome.kieu)) finalOutcome = outcome.kieu;
      // Chỉ `thu-lai` giữ việc chờ: câu của nó mời bấm lại ngay trên khung này.
      if (outcome.kieu !== "thu-lai") pending = null;
      setState(
        outcome.kieu === "thu-lai" && outcome.zalo !== undefined
          ? { kieu: "ket-qua", outcome: outcome.kieu, zalo: outcome.zalo }
          : { kieu: "ket-qua", outcome: outcome.kieu },
      );
    },

    decline() {
      pending = null;
      setState({ kieu: "ket-qua", outcome: "tu-choi" });
    },

    reset() {
      if (busy) return;
      pending = null;
      setState(null);
    },
  };
  return machine;
}

/**
 * Máy trạng thái trong một màn. Dựng MỘT lần cho mỗi lần gắn màn (StrictMode dựng lại vẫn một). `readGrant`: the
 * commune app's permission reader (`createPhoneVerification`); the shared app never passes it.
 */
export function usePhoneVerification(
  reopen: ReopenWithPhone | undefined,
  readGrant?: () => Promise<boolean>,
): PhoneVerification & { state: PhoneVerificationState | null } {
  const [state, setState] = useState<PhoneVerificationState | null>(null);
  const machine = useRef<PhoneVerification | null>(null);
  if (machine.current === null) machine.current = createPhoneVerification(reopen, setState, readGrant);
  return { ...machine.current, state };
}

/**
 * Câu cho một kết cục, nói đúng việc của màn ấy. `zalo` (only meaningful with `thu-lai`): Zalo refused a
 * step with a code, and the sentence says which and whether pressing again can help.
 */
export function phoneVerificationMessage(
  outcome: PhoneVerificationStop,
  task: PhoneVerificationTask,
  zalo?: ZaloFailure,
  /** The voice: the shared app's by default, `COMMUNE_PHONE_WORDS` in the commune app ("bà con"). */
  PHONE_VERIFICATION: PhoneVerificationWords = SHARED_PHONE_WORDS,
): string {
  const t = PHONE_VERIFICATION.task[task];
  if (outcome === "thu-lai" && zalo !== undefined) {
    return PHONE_VERIFICATION.zalo_failed(zaloFailureSentence(zalo), t, zalo.transient);
  }
  switch (outcome) {
    case "tu-choi":
      return PHONE_VERIFICATION.refused(t);
    case "thu-lai":
      return PHONE_VERIFICATION.retry(t);
    case "ngoai-zalo":
      return PHONE_VERIFICATION.outside_zalo(t);
    case "chua-mo":
      return PHONE_VERIFICATION.unavailable(t);
    case "van-chua-xac-thuc":
      return PHONE_VERIFICATION.still_unverified(t);
    case "khac-xa":
      return PHONE_VERIFICATION.other_commune(t);
  }
}

/**
 * Whether a screen that SENDS may offer "Gửi bằng họ tên và số điện thoại" under this outcome (ADR 0080 #1):
 * only when Zalo gave no number — the citizen declined, Zalo failed the PHONE step, or the server reopened the
 * session without a verified number. Never outside Zalo, never for a network failure or another commune. PURE.
 */
export function phoneVerificationOffersManual(outcome: PhoneVerificationStop, zalo?: ZaloFailure): boolean {
  if (outcome === "tu-choi" || outcome === "van-chua-xac-thuc") return true;
  return outcome === "thu-lai" && zalo?.capability === "phone";
}

/**
 * Khung hiện trên màn, THUẦN. `focusId` nằm trên phần tử ĐẦU TIÊN của mọi trạng thái, để màn nào dời
 * tiêu điểm khi đổi bước (màn gửi) thì trình đọc màn hình đọc ngay câu mới.
 *
 * Hai nút to (`cd-nut`, `cd-nut-phu`), chữ nói đúng việc — không "OK", không "Huỷ".
 */
export function PhoneVerificationPanel(props: {
  state: PhoneVerificationState;
  task: PhoneVerificationTask;
  focusId?: string;
  /** Màn gửi: nói thêm rằng nội dung đang viết vẫn còn. */
  draftKept?: boolean;
  /** The voice — the shared app's by default; the commune app passes `COMMUNE_PHONE_WORDS` ("bà con"). */
  words?: PhoneVerificationWords;
  /**
   * Where the number goes, said before Zalo's dialog. Default: the shared app's route (through
   * `vihat-miniapp`). The commune's own app passes `COMMUNE_APP_SESSION.zalo_asks`: its reopen goes straight
   * to ViGov identity (ADR 0066), and the shared sentence would name a server the number never reaches.
   */
  zaloAsks?: string;
  onAllow: () => void;
  onDecline: () => void;
  /**
   * "Gửi bằng họ tên và số điện thoại" (ADR 0080) — only the SEND screen passes it, and the button shows only
   * where `phoneVerificationOffersManual` says. Absent = never offered (lookup, "Phản ánh của tôi", rating).
   */
  onManual?: () => void;
}) {
  const { state } = props;
  const PHONE_VERIFICATION = props.words ?? SHARED_PHONE_WORDS;
  const draft = props.draftKept === true ? <p className="cd-ghi-chu">{PHONE_VERIFICATION.draft_kept}</p> : null;

  if (state.kieu === "dang-xac-nhan") {
    return (
      <p className="cd-cau" role="status" id={props.focusId} tabIndex={-1}>
        {PHONE_VERIFICATION.working}
      </p>
    );
  }

  if (state.kieu === "ket-qua") {
    // Same condition as the sentence's Zalo branch: the code line never stands under a sentence not built from it.
    const supportCode = state.outcome === "thu-lai" ? zaloSupportCode(state.zalo) : null;
    return (
      <div className="cd-buoc">
        <p className="cd-loi" role="alert" id={props.focusId} tabIndex={-1}>
          {phoneVerificationMessage(state.outcome, props.task, state.zalo, PHONE_VERIFICATION)}
        </p>
        {supportCode !== null && <p className="cd-ghi-chu">{supportCode}</p>}
        {draft}
        {state.outcome === "thu-lai" && (
          <button type="button" className="cd-nut" onClick={props.onAllow}>
            {PHONE_VERIFICATION.allow}
          </button>
        )}
        {props.onManual !== undefined && phoneVerificationOffersManual(state.outcome, state.zalo) && (
          <>
            <p className="cd-cau">{PHONE_VERIFICATION.typed_contact_offer}</p>
            <button type="button" className="cd-nut" onClick={props.onManual}>
              {PHONE_VERIFICATION.typed_contact_button}
            </button>
          </>
        )}
      </div>
    );
  }

  return (
    <div className="cd-buoc">
      <h2 className="cd-tieu-de-phu" id={props.focusId} tabIndex={-1}>
        {PHONE_VERIFICATION.title}
      </h2>
      <p className="cd-cau">{PHONE_VERIFICATION.why}</p>
      <p className="cd-ghi-chu">{props.zaloAsks ?? PHONE_VERIFICATION.zalo_asks}</p>
      {draft}
      <button type="button" className="cd-nut" onClick={props.onAllow}>
        {PHONE_VERIFICATION.allow}
      </button>
      <button type="button" className="cd-nut-phu" onClick={props.onDecline}>
        {PHONE_VERIFICATION.decline}
      </button>
    </div>
  );
}
