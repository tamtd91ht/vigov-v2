/**
 * Ba mảnh dùng chung của hai màn nửa nhà nước, THUẦN — nhận mọi thứ qua tham số, để dựng được bằng
 * `react-dom/server` trong test (không có DOM để bấm).
 */
import type { ReactNode } from "react";

import type { MyReport } from "../api/citizen-report-contract";

import { statusExplanation, CHANNEL_NOT_OPEN, SENDING_TO_COMMUNE_LABEL, statusLabel, REPORT_CARD } from "./copy";
import { UNREADABLE_TIME, vnDateTime } from "../../lib/date-time";

/**
 * TÊN XÃ CỦA PHIÊN, ĐẦU MỖI MÀN (README §Non-negotiables #2). Xã ấy là xã MÁY CHỦ sẽ ghi phiếu vào
 * — không phải xã lớp khám phá gợi ý. `role="note"` để trình đọc màn hình đọc nó ra.
 */
export function CommuneBanner({ commune_name }: { commune_name: string }) {
  return (
    <p className="cd-xa" role="note">
      <span className="cd-xa__nhan">{SENDING_TO_COMMUNE_LABEL}</span>
      <strong className="cd-xa__ten">{commune_name}</strong>
    </p>
  );
}

/** Kênh chưa mở — hiện khi không có phiên ViGov. Không có biểu mẫu, không có nút gửi. */
export function ChannelNotOpen() {
  return (
    <section className="cd-dong" role="status" aria-labelledby="cd-dong-tieu-de">
      <h2 className="cd-dong__tieu-de" id="cd-dong-tieu-de">
        {CHANNEL_NOT_OPEN.title}
      </h2>
      <p className="cd-dong__cau">{CHANNEL_NOT_OPEN.text}</p>
    </section>
  );
}

function formatTime(iso: string): string {
  return vnDateTime(iso) ?? UNREADABLE_TIME;
}

function Row({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="cd-phieu__dong">
      <dt className="cd-phieu__nhan">{label}</dt>
      <dd className="cd-phieu__gia-tri">{children}</dd>
    </div>
  );
}

/** Chữ cán bộ viết cho người dân, hoặc câu "chưa ghi" — một ô trống im lặng không nói gì với ai. */
function StaffText({ text }: { text: string }) {
  return text !== "" ? (
    <span className="cd-phieu__ly-do">{text}</span>
  ) : (
    <span className="cd-phieu__phu">{REPORT_CARD.not_recorded}</span>
  );
}

/**
 * LÝ DO VÀ CƠ QUAN NHẬN CỦA HAI NHÁNH KẾT THÚC — đặt NGAY SAU tình trạng, vì đó là câu trả lời người
 * dân mở phiếu để đọc. Quyết định vẽ dựa trên TRẠNG THÁI, không dựa trên việc chuỗi có rỗng không:
 * ở trạng thái khác thẻ không có ô nào cho hai trường này (`readReport` cũng đã bỏ chúng đi).
 *
 * Chữ dài XUỐNG DÒNG, không cắt (`cd-phieu__ly-do`): lý do từ chối bị cắt giữa câu là một quyết định
 * hành chính người dân chỉ đọc được một nửa.
 */
function BranchEnd({ report }: { report: MyReport }) {
  if (report.status === "khong-tiep-nhan") {
    return (
      <Row label={REPORT_CARD.rejection_reason}>
        <StaffText text={report.reason} />
      </Row>
    );
  }
  if (report.status === "chuyen-cap-tren") {
    return (
      <>
        <Row label={REPORT_CARD.receiving_body_label}>
          <StaffText text={report.receiving_body} />
          {report.receiving_body !== "" && <span className="cd-phieu__phu">{REPORT_CARD.contact_body}</span>}
        </Row>
        <Row label={REPORT_CARD.referral_reason}>
          <StaffText text={report.reason} />
        </Row>
      </>
    );
  }
  return null;
}

/**
 * MỘT PHIẾU, CHỈ NHỮNG GÌ `phieuCuaToiRa` TRẢ VỀ. Không có ghi chú cán bộ, lịch sử chuyển hay tên
 * người xử lý — máy chủ không gửi, và thẻ này không có ô nào chờ chúng (luật 10, bất biến 7).
 *
 * Trạng thái bằng CHỮ, không bằng màu (README §Non-negotiables #6). Hai `null` của hai hạn nói hai
 * câu khác nhau: `acknowledge_due` null là KHÔNG ÁP DỤNG, `resolve_due` null là CHƯA CÓ.
 */
/**
 * Lĩnh vực như người dân đọc: tên xã đặt, hoặc một câu — KHÔNG BAO GIỜ mã thô. Chung cho thẻ phiếu
 * và danh sách "Phản ánh của tôi", để hai màn nói cùng một câu cho cùng một phiếu.
 */
export function fieldLabelOf(field: string, field_label: string): string {
  if (field_label !== "") return field_label;
  return field !== "" ? REPORT_CARD.classified : REPORT_CARD.not_classified;
}

export function ReportCard({ report }: { report: MyReport }) {
  const explanation = statusExplanation(report.status);
  const field = fieldLabelOf(report.field, report.field_label);

  const reporter = report.anonymous
    ? REPORT_CARD.anonymous
    : [report.masked_reporter_name, report.masked_reporter_phone].filter((p) => p !== "").join(" · ") ||
      REPORT_CARD.name_not_given;

  return (
    <dl className="cd-phieu">
      <Row label={REPORT_CARD.status}>
        <strong className="cd-phieu__trang-thai">{statusLabel(report.status)}</strong>
        {explanation !== null && <span className="cd-phieu__phu">{explanation}</span>}
      </Row>
      <BranchEnd report={report} />
      <Row label={REPORT_CARD.field}>{field}</Row>
      <Row label={REPORT_CARD.sent_at}>
        {formatTime(report.clock_from)} {REPORT_CARD.vn_time}
      </Row>
      <Row label={REPORT_CARD.view_due}>
        {report.acknowledge_due === null
          ? REPORT_CARD.view_due_not_applicable
          : `${formatTime(report.acknowledge_due)} ${REPORT_CARD.vn_time}`}
      </Row>
      <Row label={REPORT_CARD.due_at}>
        {report.resolve_due === null
          ? REPORT_CARD.due_at_not_yet
          : `${formatTime(report.resolve_due)} ${REPORT_CARD.vn_time}`}
      </Row>
      {report.result !== "" && (
        <Row label={REPORT_CARD.result}>
          <span className="cd-phieu__ket-qua">{report.result}</span>
        </Row>
      )}
      <Row label={REPORT_CARD.content}>
        <span className="cd-phieu__van-ban">{report.content}</span>
      </Row>
      <Row label={REPORT_CARD.address}>{report.address !== "" ? report.address : REPORT_CARD.address_empty}</Row>
      <Row label={REPORT_CARD.reporter}>{reporter}</Row>
    </dl>
  );
}
