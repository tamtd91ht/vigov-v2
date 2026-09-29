/**
 * MÀN "TRA CỨU PHIẾU CỦA TÔI" — nhập mã, xem tình trạng, các hạn và kết quả khi đã đóng.
 *
 * ⚠ MỘT CÂU CHO MỌI 404: "không có mã này", "phiếu của người khác", "phiếu của xã khác" là cùng một
 * câu trả lời từ máy chủ, và màn hình không được tách chúng (luật 4, cấm #2).
 *
 * ⚠ MÃ TRA CỨU KHÔNG ĐI VÀO NHẬT KÝ, không ghi xuống máy. Nó chỉ đi trên đường dẫn của đúng một
 * lời gọi, kèm bearer của phiên — hợp đồng đặt nó ở đó.
 *
 * ⚠ CHƯA CÓ PHIÊN ViGov THÌ KHÔNG VẼ Ô NHẬP và KHÔNG GỌI MẠNG. In practice the session is still empty:
 * the bridge is wired (`api/open-vigov-session.ts` records `vigovSession`) but `vihat-miniapp`'s Zalo account-id
 * step always refuses (`internal/zalo/ma_tai_khoan.go:45-47`, ADR 0045 UNKNOWN #2) → 503.
 */
import { type ReactNode, useEffect, useRef, useState } from "react";

import { type CallResult, lookupReport } from "../api/vigov-client";
import type { MyReport } from "../api/citizen-report-contract";
import type { ReopenWithPhone } from "../api/open-vigov-session";
import { getVigovSession } from "../api/vigov-session";

import { CommuneBanner, ChannelNotOpen, ReportCard } from "./frame";
import { BACK, LOOKUP } from "./copy";
import { TextField } from "./input-field";
import { CitizenReportRating } from "./CitizenReportRating";
import { PhoneVerificationPanel, usePhoneVerification } from "./phone-verification";

/** What the rating block needs from the screen that shows the petition. */
export type RatingHooks = {
  readonly onRated: (report: MyReport) => void;
  readonly onReload: (code: string) => void;
  readonly reopenWithPhone?: ReopenWithPhone;
};

/** Độ dài tối đa ô mã. Mã do máy chủ sinh ngẫu nhiên và ngắn hơn nhiều; đây chỉ là trần ô nhập. */
const CODE_MAX_LENGTH = 64;

/**
 * Kết quả tra → thứ hiện dưới ô nhập. THUẦN, để test dựng thẳng từng nhánh.
 *
 * `chua-co-phien` / `chua-cau-hinh` không tới được đây trong luồng thường (màn đã dừng trước), nên
 * chúng hiện đúng khối "kênh chưa mở" chứ không một câu lỗi.
 */
export function LookupResult({ result, rating }: { result: CallResult; rating?: RatingHooks }): ReactNode {
  switch (result.kind) {
    case "xong":
      // Câu ngắn trong `role="status"`, thẻ phiếu NGOÀI vùng thông báo: tìm thấy thì người không
      // nhìn màn hình cũng được báo, mà không bị đọc dồn cả thẻ một lượt (câu lỗi đã có `alert`).
      return (
        <>
          <p className="cd-cau" role="status">
            {LOOKUP.found}
          </p>
          <ReportCard report={result.report} />
          {/* Keyed by the code: the SAME block survives the re-render from its own 200 (so it can say
              "sent"), and a different petition gets a fresh one. */}
          {rating !== undefined && (
            <CitizenReportRating
              key={result.report.lookup_code}
              report={result.report}
              onRated={rating.onRated}
              onReload={() => rating.onReload(result.report.lookup_code)}
              reopenWithPhone={rating.reopenWithPhone}
            />
          )}
        </>
      );
    case "chua-co-phien":
    case "chua-cau-hinh":
      return <ChannelNotOpen />;
    case "khong-thay":
      return (
        <p className="cd-loi" role="alert">
          {LOOKUP.not_found}
        </p>
      );
    case "het-phien":
      return (
        <p className="cd-loi" role="alert">
          {LOOKUP.session_expired}
        </p>
      );
    case "loi-mang":
      return (
        <p className="cd-loi" role="alert">
          {LOOKUP.network_error}
        </p>
      );
    default:
      return (
        <p className="cd-loi" role="alert">
          {LOOKUP.server_error}
        </p>
      );
  }
}

/**
 * `initial_code`: mã đã chọn từ "Phản ánh của tôi" — điền sẵn vào ô và tra ngay khi mở, để người dân
 * không phải gõ lại một mã họ vừa chạm vào. Vẫn đi qua ĐÚNG lời gọi tra cứu, với ĐÚNG phiên.
 */
export function ReportLookupScreen({
  onBack,
  initial_code = "",
  reopenWithPhone,
  onChanged,
}: {
  onBack: () => void;
  initial_code?: string;
  /** Hàm mở lại phiên kèm số do lớp vỏ tiêm vào (`api/open-vigov-session.ts`). Vắng = không có đường ấy. */
  reopenWithPhone?: ReopenWithPhone;
  /**
   * The citizen changed the petition from here (a rating — which may also have reopened it). The shell
   * uses it to reload "Phản ánh của tôi", which would otherwise still show the old status.
   */
  onChanged?: () => void;
}) {
  const [session] = useState(getVigovSession);
  const phone = usePhoneVerification(reopenWithPhone);
  const [typed_code, setTypedCode] = useState(initial_code);
  const [lookingUp, setLookingUp] = useState(false);
  const [codeMissing, setCodeMissing] = useState(false);
  const [result, setResult] = useState<CallResult | null>(null);
  // Tra mã điền sẵn ĐÚNG MỘT LẦN, kể cả khi React dựng hiệu ứng hai lần (StrictMode).
  const prefilled_lookup_done = useRef(false);

  useEffect(() => {
    if (session === null || initial_code.trim() === "" || prefilled_lookup_done.current) return;
    prefilled_lookup_done.current = true;
    void lookup();
  }, [session, initial_code]);

  const backButton = (
    <button type="button" className="quay-lai" onClick={onBack}>
      {BACK}
    </button>
  );

  if (session === null) {
    return (
      <section className="cd-man" aria-label={LOOKUP.title}>
        {backButton}
        <h1 className="cd-tieu-de">{LOOKUP.title}</h1>
        <ChannelNotOpen />
      </section>
    );
  }

  /** `code` absent = the code in the box; present = re-read that petition (the rating block's reload). */
  async function lookup(code?: string) {
    const target = code ?? typed_code;
    if (target.trim() === "") {
      setCodeMissing(true);
      return;
    }
    setCodeMissing(false);
    phone.reset();
    setLookingUp(true);
    setResult(null);
    const result = await lookupReport(target);
    setLookingUp(false);
    if (result.kind === "can-xac-thuc-so") {
      // Gọi lại với CÙNG mã đã tra.
      phone.onPhoneRequired(() => void lookup(target));
      return;
    }
    setResult(result);
  }

  const rating: RatingHooks = {
    onRated: (report) => {
      setResult({ kind: "xong", report });
      onChanged?.();
    },
    onReload: (code) => void lookup(code),
    reopenWithPhone,
  };

  return (
    <section className="cd-man" aria-label={LOOKUP.title}>
      {backButton}
      <CommuneBanner commune_name={session.commune_name} />
      <h1 className="cd-tieu-de">{LOOKUP.title}</h1>
      <TextField
        id="cd-ma-tra-cuu"
        label={LOOKUP.code_label}
        suggestion={LOOKUP.hint_code}
        value={typed_code}
        max={CODE_MAX_LENGTH}
        onChange={setTypedCode}
      />
      {codeMissing && (
        <p className="cd-loi" role="alert">
          {LOOKUP.missing_code}
        </p>
      )}
      <button
        type="button"
        className="cd-nut"
        disabled={lookingUp || phone.state?.kind === "dang-xac-nhan"}
        onClick={() => void lookup()}
      >
        {LOOKUP.lookup_button}
      </button>
      {lookingUp && (
        <p className="cd-cau" role="status">
          {LOOKUP.looking_up}
        </p>
      )}
      {phone.state !== null && (
        <PhoneVerificationPanel
          state={phone.state}
          task="lookup"
          onAllow={() => void phone.allow()}
          onDecline={phone.decline}
        />
      )}
      {result !== null && <LookupResult result={result} rating={rating} />}
    </section>
  );
}
