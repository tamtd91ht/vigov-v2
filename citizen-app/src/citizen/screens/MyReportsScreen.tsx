/**
 * MÀN "PHẢN ÁNH CỦA TÔI" — danh sách phiếu của CHÍNH người dân, mới nhất trước.
 *
 * ⚠ CHỈ PHIẾU CỦA MÌNH, VÀ MÁY CHỦ QUYẾT ĐIỀU ĐÓ: lời gọi không mang một tham số nào nói "của ai"
 * hay "xã nào" (`api/vigov-client.ts` `myReports`). Công dân và xã lấy từ phiên (luật 4, bất biến 2).
 *
 * ⚠ CHƯA CÓ PHIÊN ViGov THÌ KHÔNG GỌI MẠNG (`api/vigov-session.ts`). Màn nói "kênh chưa mở" như hai màn
 * kia. In practice the session is still empty: the bridge is wired (`api/open-vigov-session.ts` records
 * `vigovSession`) but `vihat-miniapp`'s Zalo account-id step always refuses
 * (`internal/zalo/ma_tai_khoan.go:45-47`, ADR 0045 UNKNOWN #2) → 503.
 *
 * ⚠ NÚT "XEM THÊM", KHÔNG CUỘN VÔ HẠN (`skills/accessibility-elderly`): danh sách tự dài ra khi
 * ngón tay chỉ định kéo xuống là danh sách người lớn tuổi không tìm lại được chỗ mình đang đọc.
 *
 * ⚠ KHÔNG LƯU XUỐNG MÁY. Danh sách sống trong `useState` và mất khi đóng màn (`two-halves-boundary`
 * §3b). Không ghi mã tra cứu vào nhật ký.
 *
 * ⚠ KHÔNG CÓ "QUÁ HẠN N NGÀY": quá hạn đếm bằng giờ làm việc (ADR 0007) và chỉ `identity` đếm được.
 * Thẻ chỉ hiện mốc hạn cố định máy chủ đã ghi.
 */
import { useEffect, useRef, useState } from "react";

import { type ListResult, myReports } from "../api/vigov-client";
import type { MyReportSummary } from "../api/citizen-report-contract";
import type { ReopenWithPhone } from "../api/open-vigov-session";
import { getVigovSession } from "../api/vigov-session";

import { CommuneBanner, ChannelNotOpen, fieldLabelOf } from "./frame";
import { MY_REPORTS, SEND, statusLabel, BACK, REPORT_CARD } from "./copy";
import { PhoneVerificationPanel, usePhoneVerification } from "./phone-verification";
import { UNREADABLE_TIME, vnDateTime } from "../../lib/date-time";

/** Ba câu lỗi người dân đọc được. Mọi mã lạ khác rơi vào `loi-may-chu`. */
export type ListError = "het-phien" | "loi-mang" | "loi-may-chu";

/** Trạng thái màn, THUẦN — test dựng thẳng từng bước mà không cần DOM. */
export type ReportList = {
  readonly entries: readonly MyReportSummary[];
  readonly cursor: string;
  readonly has_more: boolean;
  /** Đã nhận trang đầu. Trước đó, "không có dòng nào" chưa có nghĩa là "chưa gửi phản ánh nào". */
  readonly has_first_page: boolean;
  readonly loading: boolean;
  readonly error: ListError | null;
  /** Máy chủ nói chưa có phiên / chưa cấu hình — màn thành "kênh chưa mở". */
  readonly channel_closed: boolean;
};

export const FIRST_LIST: ReportList = {
  entries: [],
  cursor: "",
  has_more: false,
  has_first_page: false,
  loading: true,
  error: null,
  channel_closed: false,
};

export function startLoading(list: ReportList): ReportList {
  return { ...list, loading: true, error: null };
}

/**
 * Kết quả một lần tải → trạng thái kế. Trang mới NỐI VÀO SAU, không thay: "Xem thêm" là thêm.
 *
 * BỎ DÒNG TRÙNG MÃ: con trỏ không trả trùng, nhưng hai lần tải chồng nhau (bấm nhanh, dựng lại
 * màn) thì có thể — và hai thẻ cùng mã là người dân tưởng mình gửi hai lần.
 */
export function afterLoad(list: ReportList, result: ListResult): ReportList {
  switch (result.kind) {
    case "xong": {
      const seen = new Set(list.entries.map((p) => p.lookup_code));
      const fresh = result.page.entries.filter((p) => !seen.has(p.lookup_code));
      return {
        ...list,
        entries: [...list.entries, ...fresh],
        cursor: result.page.cursor,
        has_more: result.page.has_more,
        has_first_page: true,
        loading: false,
        error: null,
      };
    }
    case "chua-co-phien":
    case "chua-cau-hinh":
      return { ...list, loading: false, channel_closed: true };
    case "het-phien":
    case "loi-mang":
      return { ...list, loading: false, error: result.kind };
    default:
      return { ...list, loading: false, error: "loi-may-chu" };
  }
}

function formatTime(iso: string): string {
  return `${vnDateTime(iso) ?? UNREADABLE_TIME} ${REPORT_CARD.vn_time}`;
}

/**
 * MỘT THẺ = MỘT NÚT to bằng cả thẻ: chạm đâu cũng mở phiếu. Mã tra cứu to nhất, đứng đầu (luật 10,
 * bất biến 1). Trạng thái bằng CHỮ, không bằng màu (README §Non-negotiables #6).
 *
 * "Hạn xử lý xong" chỉ hiện khi đã có: `null` là CHƯA CÓ cam kết, và danh sách không phải chỗ giải
 * thích điều đó — màn tra cứu có câu riêng.
 */
export function ReportSummaryCard({ report, onOpen }: { report: MyReportSummary; onOpen: (code: string) => void }) {
  return (
    <li className="cd-cua-toi__muc">
      <button type="button" className="cd-the-cua-toi" onClick={() => onOpen(report.lookup_code)}>
        <span className="cd-the-cua-toi__ma">{report.lookup_code}</span>
        <span className="cd-the-cua-toi__dong">
          {REPORT_CARD.status}: <strong className="cd-the-cua-toi__trang-thai">{statusLabel(report.status)}</strong>
        </span>
        <span className="cd-the-cua-toi__dong">
          {REPORT_CARD.field}: {fieldLabelOf(report.field, report.field_label)}
        </span>
        <span className="cd-the-cua-toi__trich">{report.content_excerpt}</span>
        <span className="cd-the-cua-toi__dong">
          {REPORT_CARD.sent_at}: {formatTime(report.clock_from)}
        </span>
        {report.resolve_due !== null && (
          <span className="cd-the-cua-toi__dong">
            {REPORT_CARD.due_at}: {formatTime(report.resolve_due)}
          </span>
        )}
        <span className="cd-the-cua-toi__xem">{MY_REPORTS.view_button}</span>
      </button>
    </li>
  );
}

const ERROR_TEXT: Readonly<Record<ListError, string>> = {
  "het-phien": MY_REPORTS.session_expired,
  "loi-mang": MY_REPORTS.network_error,
  "loi-may-chu": MY_REPORTS.server_error,
};

/** Thân màn khi đã có phiên, THUẦN — nhận trạng thái và việc cần làm qua tham số. */
export function ListBody(props: {
  list: ReportList;
  onOpen: (code: string) => void;
  onLoad: () => void;
  onSubmitReport: () => void;
}) {
  const { list } = props;
  return (
    <>
      {!list.has_first_page && list.loading && (
        <p className="cd-cau" role="status">
          {MY_REPORTS.loading}
        </p>
      )}

      {list.has_first_page && list.entries.length === 0 && (
        <div className="cd-buoc">
          <p className="cd-cau">{MY_REPORTS.empty}</p>
          <button type="button" className="cd-nut" onClick={props.onSubmitReport}>
            {SEND.title}
          </button>
        </div>
      )}

      {list.entries.length > 0 && (
        <ul className="cd-cua-toi">
          {list.entries.map((p) => (
            <ReportSummaryCard key={p.lookup_code} report={p} onOpen={props.onOpen} />
          ))}
        </ul>
      )}

      {list.has_first_page && list.loading && (
        <p className="cd-cau" role="status">
          {MY_REPORTS.loading_more}
        </p>
      )}

      {list.error !== null && (
        <div className="cd-buoc">
          <p className="cd-loi" role="alert">
            {ERROR_TEXT[list.error]}
          </p>
          {/* Hết phiên thì bấm lại không đổi được gì — câu đã nói việc cần làm. */}
          {list.error !== "het-phien" && (
            <button type="button" className="cd-nut" disabled={list.loading} onClick={props.onLoad}>
              {MY_REPORTS.retry_button}
            </button>
          )}
        </div>
      )}

      {list.error === null && list.has_more && (
        <button type="button" className="cd-nut-phu" disabled={list.loading} onClick={props.onLoad}>
          {MY_REPORTS.load_more_button}
        </button>
      )}

      {list.has_first_page && !list.has_more && list.entries.length > 0 && (
        <p className="cd-ghi-chu">{MY_REPORTS.end_of_list}</p>
      )}
    </>
  );
}

export function MyReportsScreen(props: {
  onBack: () => void;
  onOpenReport: (code: string) => void;
  onSubmitReport: () => void;
  /** Hàm mở lại phiên kèm số do lớp vỏ tiêm vào (`api/open-vigov-session.ts`). Vắng = không có đường ấy. */
  reopenWithPhone?: ReopenWithPhone;
}) {
  // Đọc một lần lúc dựng. `null` until the bridge issues a session (in practice still, see the header).
  const [session] = useState(getVigovSession);
  const [list, setList] = useState<ReportList>(FIRST_LIST);
  const phone = usePhoneVerification(props.reopenWithPhone);
  // Tải trang đầu ĐÚNG MỘT LẦN, kể cả khi React dựng hiệu ứng hai lần (StrictMode).
  const first_loaded = useRef(false);

  /** Tải trang kế — trang đầu khi `cursor` còn rỗng. Dùng cho cả "Xem thêm" lẫn "Thử lại". */
  async function load(cursor: string) {
    phone.reset();
    setList(startLoading);
    const result = await myReports(cursor);
    if (result.kind === "can-xac-thuc-so") {
      // Không phải một lỗi của danh sách: dừng "đang tải", giữ nguyên những dòng đã có, và hỏi công dân.
      // Gọi lại với CÙNG con trỏ.
      setList((previous) => ({ ...previous, loading: false }));
      phone.onPhoneRequired(() => void load(cursor));
      return;
    }
    setList((previous) => afterLoad(previous, result));
  }

  useEffect(() => {
    if (session === null || first_loaded.current) return;
    first_loaded.current = true;
    void load("");
  }, [session]);

  const backButton = (
    <button type="button" className="quay-lai" onClick={props.onBack}>
      {BACK}
    </button>
  );

  if (session === null || list.channel_closed) {
    return (
      <section className="cd-man" aria-label={MY_REPORTS.title}>
        {backButton}
        <h1 className="cd-tieu-de">{MY_REPORTS.title}</h1>
        <ChannelNotOpen />
      </section>
    );
  }

  return (
    <section className="cd-man" aria-label={MY_REPORTS.title}>
      {backButton}
      <CommuneBanner commune_name={session.commune_name} />
      <h1 className="cd-tieu-de">{MY_REPORTS.title}</h1>
      <ListBody
        list={list}
        onOpen={props.onOpenReport}
        onLoad={() => {
          if (!list.loading) void load(list.cursor);
        }}
        onSubmitReport={props.onSubmitReport}
      />
      {phone.state !== null && (
        <PhoneVerificationPanel
          state={phone.state}
          task="mine"
          onAllow={() => void phone.allow()}
          onDecline={phone.decline}
        />
      )}
    </section>
  );
}
