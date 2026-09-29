/**
 * MÀN "GỬI PHẢN ÁNH" — hành vi DUY NHẤT một công dân ghi vào hệ thống này (luật 10).
 *
 * BỐN BƯỚC, MỖI MÀN MỘT VIỆC (`skills/accessibility-elderly` #4):
 *
 *   nhập  →  xác nhận xã  →  đang gửi  →  mã tra cứu   (hoặc câu lỗi kèm việc cần làm)
 *
 * ⚠ BƯỚC XÁC NHẬN XÃ LÀ LUẬT NGHIỆP VỤ, KHÔNG PHẢI TRANG TRÍ (README §Non-negotiables #5): gửi nhầm
 * xã là xã ấy nhận việc ngoài địa bàn, phải chuyển hoặc từ chối, còn người dân chờ vô ích. Tên xã ở
 * bước ấy là `commune_name` CỦA PHIÊN — đúng xã máy chủ sẽ ghi phiếu.
 *
 * ⚠ KHÔNG CÓ Ô CHỌN LĨNH VỰC: lĩnh vực do cán bộ chốt (ADR 0028, #23). Không có ô chọn xã: xã lấy
 * từ phiên (ADR 0022). Không có ảnh: chưa có kho lưu ảnh, và màn hình nói thẳng điều đó.
 *
 * ⚠ CHƯA CÓ PHIÊN ViGov THÌ KHÔNG VẼ BIỂU MẪU và KHÔNG GỌI MẠNG (`api/vigov-session.ts`). The session
 * bridge exists end to end — `vihat-miniapp` opens a ViGov citizen session for a login carrying
 * `communeHostHint` and forwards `vigovSession`, which `api/open-vigov-session.ts` records — but in practice
 * the session is still empty: its Zalo account-id step always refuses (`vihat-miniapp`
 * `internal/zalo/ma_tai_khoan.go:45-47`, ADR 0045 UNKNOWN #2) and the bridge answers 503.
 */
import { type ReactNode, useEffect, useRef, useState } from "react";

import { submitReport, type CallResult } from "../api/vigov-client";
import {
  MAX_LENGTH,
  type NewReport,
  type MyReport,
  type SceneLocation,
  submitReportBody,
} from "../api/citizen-report-contract";
import { type SendAttempt, createSendAttempt } from "../api/send-attempt";
import type { ReopenWithPhone } from "../api/open-vigov-session";
import { getVigovSession } from "../api/vigov-session";

import { CommuneBanner, ChannelNotOpen, ReportCard } from "./frame";
import { SEND, EMERGENCY, SEND_ERROR, statusLabel, BACK, SEND_LOCATION_WORDS } from "./copy";
import { TextAreaField, TextField } from "./input-field";
import { PhoneVerificationPanel, usePhoneVerification } from "./phone-verification";
import {
  formatCoordinates,
  type GetSceneLocation,
  SceneLocationControl,
  type SceneLocationFailure,
  useSceneLocation,
} from "./scene-location";
import { vnDateTime } from "../../lib/date-time";

export const EMPTY_REPORT: NewReport = {
  content: "",
  address: "",
  full_name: "",
  phone: "",
  anonymous: false,
};

/** Câu cần sửa, hoặc `null` khi gửi được. Đếm theo KÝ TỰ như máy chủ, không theo byte. */
export function checkReport(report: NewReport): string | null {
  const len = (s: string) => [...s.trim()].length;
  if (len(report.content) === 0) return SEND.missing_content;
  if (len(report.content) > MAX_LENGTH.content) return SEND.too_long("Nội dung", MAX_LENGTH.content);
  if (len(report.address) > MAX_LENGTH.address) return SEND.too_long("Nơi xảy ra", MAX_LENGTH.address);
  if (!report.anonymous && len(report.full_name) > MAX_LENGTH.full_name) {
    return SEND.too_long("Họ và tên", MAX_LENGTH.full_name);
  }
  if (!report.anonymous && len(report.phone) > MAX_LENGTH.phone) {
    return SEND.too_long("Số điện thoại", MAX_LENGTH.phone);
  }
  return null;
}

/**
 * CHỖ ĐẶT TIÊU ĐIỂM KHI ĐỔI BƯỚC — một `id` cho mỗi bước, phần tử mang nó có `tabIndex={-1}`.
 *
 * VÌ SAO: bấm "Tiếp tục" / "Gửi tới …" là nút vừa bấm BIẾN MẤT cùng bước cũ. Người dùng trình đọc
 * màn hình bị bỏ lại trên một nút không còn, và không biết màn đã sang bước nào. Đưa tiêu điểm tới
 * đầu bước mới thì trình đọc đọc ngay bước ấy nói gì (`skills/accessibility-elderly`).
 *
 * Bước "nhập" trỏ vào tiêu đề chung của màn: không trỏ vào ô nhập, vì tiêu điểm ở ô nhập là bàn
 * phím điện thoại bật lên che nửa màn.
 *
 * Các khoá là CHÍNH giá trị `Step["kind"]` (`STEP_HEADING_ID[step.kind]`), nên đổi tên chúng là đổi
 * giá trị của bước — việc ấy thuộc lượt đổi giá trị enum, không thuộc lượt đổi tên định danh này.
 */
export const STEP_HEADING_ID = {
  nhap: "cd-gui-tieu-de",
  "xac-nhan": "cd-xac-nhan-tieu-de",
  "dang-gui": "cd-dang-gui",
  xong: "cd-xong-tieu-de",
  loi: "cd-loi-gui",
  "can-so": "cd-xac-thuc-so",
} as const;

/* ─────────────────────────── bước 1: nhập ─────────────────────────── */

/**
 * The location control's state on the input step. `null` = the shell injected no location function
 * (outside Zalo, tests): no button at all, rather than a button that can only fail.
 */
export type InputStepLocation = {
  locating: boolean;
  failure: SceneLocationFailure | null;
  onLocate: () => void;
} | null;

export function EntryStep(props: {
  report: NewReport;
  error: string | null;
  onChange: (report: NewReport) => void;
  onNext: () => void;
  location?: InputStepLocation;
}) {
  const { report, onChange } = props;
  const location = props.location ?? null;
  return (
    <div className="cd-buoc">
      <p className="cd-khan-cap" role="note">
        {EMERGENCY}
      </p>

      <TextAreaField
        id="cd-noi-dung"
        label={SEND.content_label}
        suggestion={SEND.hint_content}
        value={report.content}
        max={MAX_LENGTH.content}
        required
        onChange={(v) => onChange({ ...report, content: v })}
      />
      <TextField
        id="cd-dia-chi"
        label={SEND.address_label}
        suggestion={SEND.hint_address}
        value={report.address}
        max={MAX_LENGTH.address}
        onChange={(v) => onChange({ ...report, address: v })}
      />
      {/* UNDER the address box, which stays optional and editable: the location adds a point, it never
          fills or replaces the words the citizen typed (`scene-location.tsx`). */}
      {location !== null && (
        <SceneLocationControl
          words={SEND_LOCATION_WORDS}
          look="shared"
          locating={location.locating}
          location={report.scene_location ?? null}
          failure={location.failure}
          onLocate={location.onLocate}
        />
      )}

      {/* NÚT BẬT/TẮT, KHÔNG PHẢI Ô ĐÁNH DẤU: trạng thái nói bằng CHỮ ("Đang bật"), không bằng một
          dấu tích nhỏ hay một màu (README §Non-negotiables #6), và đích chạm to bằng cả dòng. */}
      <button
        type="button"
        className="cd-cong-tac"
        aria-pressed={report.anonymous}
        onClick={() => onChange({ ...report, anonymous: !report.anonymous })}
      >
        <span className="cd-cong-tac__ten">{SEND.anonymous}</span>
        <span className="cd-cong-tac__trang-thai">{report.anonymous ? SEND.anonymous_on : SEND.anonymous_off}</span>
      </button>
      <p className="cd-ghi-chu">{SEND.anonymous_explanation}</p>

      {!report.anonymous && (
        <>
          <TextField
            id="cd-ho-ten"
            label={SEND.name_label}
            value={report.full_name}
            max={MAX_LENGTH.full_name}
            onChange={(v) => onChange({ ...report, full_name: v })}
          />
          <TextField
            id="cd-dien-thoai"
            label={SEND.phone_label}
            value={report.phone}
            max={MAX_LENGTH.phone}
            input_mode="tel"
            onChange={(v) => onChange({ ...report, phone: v })}
          />
        </>
      )}

      <p className="cd-ghi-chu">{SEND.photos_not_supported}</p>

      {props.error !== null && (
        <p className="cd-loi" role="alert">
          {props.error}
        </p>
      )}

      <button type="button" className="cd-nut" onClick={props.onNext}>
        {SEND.next_button}
      </button>
    </div>
  );
}

/* ─────────────────────── bước 2: xác nhận xã ─────────────────────── */

export function ConfirmStep(props: {
  commune_name: string;
  onSend: () => void;
  onEdit: () => void;
  /** The location that goes with the petition, if any — said here, at the last step, before sending. */
  location?: SceneLocation | null;
}) {
  const location = props.location ?? null;
  return (
    <div className="cd-buoc" aria-labelledby={STEP_HEADING_ID["xac-nhan"]}>
      <h2 className="cd-tieu-de-phu" id={STEP_HEADING_ID["xac-nhan"]} tabIndex={-1}>
        {SEND.confirm_title}
      </h2>
      <p className="cd-cau">{SEND.confirm_text}</p>
      <p className="cd-xa-xac-nhan">{props.commune_name}</p>
      <p className="cd-ghi-chu">{SEND.confirm_consequence}</p>
      {location !== null && <p className="cd-cau">{SEND.confirm_location(formatCoordinates(location))}</p>}
      <button type="button" className="cd-nut" onClick={props.onSend}>
        {SEND.send_button(props.commune_name)}
      </button>
      <button type="button" className="cd-nut-phu" onClick={props.onEdit}>
        {SEND.edit_button}
      </button>
    </div>
  );
}

/* ─────────────────────── bước 4: mã tra cứu ─────────────────────── */

/**
 * MÃ TRA CỨU TO, ĐỨNG ĐẦU (luật 10, bất biến 1): đó là thứ duy nhất người dân có để hỏi lại về
 * phiếu này. Dưới nó là tình trạng và mốc cán bộ phải xem phiếu — `acknowledge_due`, giờ Việt Nam.
 *
 * `role="status"` CHỈ bọc câu và mã, không bọc cả khối: tiêu điểm đã tới tiêu đề (`STEP_HEADING_ID`),
 * nên vùng thông báo chỉ cần đọc thêm mã. Bọc cả khối là trình đọc đọc lại cả thẻ phiếu lẫn nút.
 */
export function SendResult(props: { report: MyReport; onSendAnother: () => void }) {
  const { report } = props;
  const view_due = report.acknowledge_due === null ? null : vnDateTime(report.acknowledge_due);
  return (
    <div className="cd-buoc">
      <h2 className="cd-tieu-de-phu" id={STEP_HEADING_ID.xong} tabIndex={-1}>
        {SEND.done_title}
      </h2>
      <div role="status">
        <p className="cd-cau">{SEND.done_code}</p>
        <p className="cd-ma-tra-cuu">{report.lookup_code}</p>
      </div>
      <p className="cd-ghi-chu">{SEND.done_keep_code}</p>
      <p className="cd-cau">
        <strong>{statusLabel(report.status)}</strong>
      </p>
      {view_due !== null && <p className="cd-cau">{SEND.will_view_by(view_due)}</p>}
      <ReportCard report={report} />
      <button type="button" className="cd-nut-phu" onClick={props.onSendAnother}>
        {SEND.send_another}
      </button>
    </div>
  );
}

type ErrorBranch = keyof typeof SEND_ERROR;

/** Câu lỗi kèm việc cần làm. "Gửi lại" dùng lại CÙNG lần gửi — cùng thân, cùng khoá. */
export function SendError(props: { branch: ErrorBranch; onResend: () => void; onEdit: () => void }) {
  const error = SEND_ERROR[props.branch];
  return (
    <div className="cd-buoc">
      <p className="cd-loi" role="alert" id={STEP_HEADING_ID.loi} tabIndex={-1}>
        {error.text}
      </p>
      {error.can_resend && (
        <button type="button" className="cd-nut" onClick={props.onResend}>
          {SEND.resend_button}
        </button>
      )}
      <button type="button" className="cd-nut-phu" onClick={props.onEdit}>
        {SEND.edit_button}
      </button>
    </div>
  );
}

/* ───────────────────────────── màn ───────────────────────────── */

type Step =
  | { kind: "nhap"; error: string | null }
  | { kind: "xac-nhan" }
  | { kind: "dang-gui" }
  | { kind: "xong"; report: MyReport }
  | { kind: "loi"; branch: ErrorBranch }
  /** Máy chủ cần số điện thoại đã xác thực — khung `phone-verification.tsx`. Nháp `report` vẫn nguyên. */
  | { kind: "can-so" };

/** Thân bước "đang gửi". `role="status"`: người không nhìn màn hình nghe được là đang gửi. */
export function Sending() {
  return (
    <p className="cd-cau" role="status" id={STEP_HEADING_ID["dang-gui"]} tabIndex={-1}>
      {SEND.sending}
    </p>
  );
}

/** Nhánh kết quả của lớp gọi → bước tiếp theo của màn. */
export function stepAfterSend(result: CallResult): Step | "kenh-chua-mo" {
  switch (result.kind) {
    case "xong":
      return { kind: "xong", report: result.report };
    case "chua-co-phien":
    case "chua-cau-hinh":
      return "kenh-chua-mo";
    case "khong-thay":
      return { kind: "loi", branch: "loi-may-chu" };
    case "can-xac-thuc-so":
      return { kind: "can-so" };
    default:
      return { kind: "loi", branch: result.kind };
  }
}

export function SubmitReportScreen({
  onBack,
  reopenWithPhone,
  getSceneLocation,
}: {
  onBack: () => void;
  /** Hàm mở lại phiên kèm số do lớp vỏ tiêm vào (`api/open-vigov-session.ts`). Vắng = không có đường ấy. */
  reopenWithPhone?: ReopenWithPhone;
  /**
   * "Lấy vị trí hiện tại" — injected by the shell (`App.tsx`: Zalo codes + `vihat-miniapp` exchange).
   * Needs no ViGov session: the route is public. Absent = no location button.
   */
  getSceneLocation?: GetSceneLocation;
}) {
  // Đọc một lần lúc dựng. `null` until the bridge issues a session (in practice still, see the header). Mở lại phiên kèm số không đổi
  // TÊN XÃ (`reopenSessionWithPhone` từ chối phiên khác xã), nên bản đọc một lần này vẫn đúng.
  const [session] = useState(getVigovSession);
  const phone = usePhoneVerification(reopenWithPhone);
  const [report, setReport] = useState<NewReport>(EMPTY_REPORT);
  const [step, setStep] = useState<Step>({ kind: "nhap", error: null });
  const [channelClosed, setChannelClosed] = useState(false);
  /** Lần gửi đang dở. Giữ qua "Gửi lại"; bỏ khi người dân quay lại sửa (`api/send-attempt.ts`). */
  const [attempt, setAttempt] = useState<SendAttempt | null>(null);
  /**
   * The location lives IN the form (`report.scene_location`) so the body is built from exactly what the
   * screen shows. Functional update: the citizen may type while the exchange runs, and their words must
   * not be overwritten by a copy of the form taken before the tap. A new location is a new body, so the
   * pending send (and its Idempotency-Key) is dropped, exactly as `onChange` does for a typed change.
   */
  const sceneLocation = useSceneLocation(getSceneLocation, (location) => {
    setReport((current) => ({ ...current, scene_location: location }));
    setAttempt(null);
  });

  // Đổi bước → tiêu điểm tới đầu bước mới (`STEP_HEADING_ID`). So với bước TRƯỚC, không dùng cờ "lần
  // đầu": StrictMode chạy hiệu ứng hai lần lúc gắn, và cờ ấy sẽ kéo tiêu điểm ngay khi mở màn.
  // Theo `kind`, không theo cả `step`: bấm "Tiếp tục" mà còn thiếu nội dung vẫn là bước nhập, câu
  // lỗi tự đọc ra (`role="alert"`) và tiêu điểm nên ở lại nút vừa bấm.
  const previous_step = useRef(step.kind);
  useEffect(() => {
    if (previous_step.current === step.kind) return;
    previous_step.current = step.kind;
    document.getElementById(STEP_HEADING_ID[step.kind])?.focus();
  }, [step.kind]);

  const backButton = (
    <button type="button" className="quay-lai" onClick={onBack}>
      {BACK}
    </button>
  );

  if (session === null || channelClosed) {
    return (
      <section className="cd-man" aria-label={SEND.title}>
        {backButton}
        <h1 className="cd-tieu-de">{SEND.title}</h1>
        <ChannelNotOpen />
      </section>
    );
  }

  async function send(send_attempt: SendAttempt) {
    setStep({ kind: "dang-gui" });
    const next = stepAfterSend(await submitReport(send_attempt));
    if (next === "kenh-chua-mo") setChannelClosed(true);
    else if (next.kind === "can-so") {
      // Gọi lại với CÙNG lần gửi — cùng thân, cùng `Idempotency-Key`: 403 trả về trước khi máy chủ ghi
      // gì, nên đây vẫn là lần gửi ấy, không phải một phiếu mới.
      setStep(next);
      phone.onPhoneRequired(() => void send(send_attempt));
    } else {
      if (next.kind === "xong") setAttempt(null);
      setStep(next);
    }
  }

  function sendFirst() {
    let send_attempt = attempt;
    if (send_attempt === null) {
      try {
        send_attempt = createSendAttempt(submitReportBody(report));
      } catch {
        setStep({ kind: "loi", branch: "khong-tao-duoc-khoa" });
        return;
      }
      setAttempt(send_attempt);
    }
    void send(send_attempt);
  }

  function editAgain() {
    phone.reset();
    setAttempt(null);
    setStep({ kind: "nhap", error: null });
  }

  let body: ReactNode;
  switch (step.kind) {
    case "nhap":
      body = (
        <EntryStep
          report={report}
          error={step.error}
          onChange={(fresh) => {
            setReport(fresh);
            // Nội dung đổi thì lần gửi cũ không còn đúng thân của nó.
            setAttempt(null);
          }}
          onNext={() => {
            const error = checkReport(report);
            setStep(error === null ? { kind: "xac-nhan" } : { kind: "nhap", error });
          }}
          location={
            getSceneLocation === undefined
              ? null
              : {
                  locating: sceneLocation.locating,
                  failure: sceneLocation.failure,
                  onLocate: () => void sceneLocation.locate(),
                }
          }
        />
      );
      break;
    case "xac-nhan":
      body = (
        <ConfirmStep
          commune_name={session.commune_name}
          onSend={sendFirst}
          onEdit={editAgain}
          location={report.scene_location ?? null}
        />
      );
      break;
    case "dang-gui":
      body = <Sending />;
      break;
    case "xong":
      body = (
        <SendResult
          report={step.report}
          onSendAnother={() => {
            setReport(EMPTY_REPORT);
            sceneLocation.reset();
            setStep({ kind: "nhap", error: null });
          }}
        />
      );
      break;
    case "loi":
      body = (
        <SendError
          branch={step.branch}
          onResend={() => {
            if (attempt !== null) void send(attempt);
          }}
          onEdit={editAgain}
        />
      );
      break;
    case "can-so":
      // `null` chỉ trong khoảnh khắc giữa "đã xác thực" và lần gọi lại — lần gọi ấy đưa màn sang
      // "đang gửi" ngay.
      body =
        phone.state === null ? (
          <Sending />
        ) : (
          <>
            <PhoneVerificationPanel
              state={phone.state}
              task="submit"
              focusId={STEP_HEADING_ID["can-so"]}
              draftKept
              onAllow={() => void phone.allow()}
              onDecline={phone.decline}
            />
            {phone.state.kind !== "dang-xac-nhan" && (
              <button type="button" className="cd-nut-phu" onClick={editAgain}>
                {SEND.edit_button}
              </button>
            )}
          </>
        );
      break;
  }

  return (
    <section className="cd-man" aria-label={SEND.title}>
      {backButton}
      <CommuneBanner commune_name={session.commune_name} />
      <h1 className="cd-tieu-de" id={STEP_HEADING_ID.nhap} tabIndex={-1}>
        {SEND.title}
      </h1>
      {body}
    </section>
  );
}
