/**
 * Mười ba tuyến của sổ Phản ánh người dân (`docs/ui-ux/09-phan-anh-nguoi-dan.md`), đúng bộ tuyến
 * `service-petitions/internal/http/routes.go` khai — không nhiều hơn, không ít hơn.
 *
 *   GET  /api/v1/citizen-reports                            feedback.read
 *   GET  /api/v1/citizen-reports/{maTraCuu}                 feedback.read
 *   POST /api/v1/citizen-reports/{maTraCuu}/classification  feedback.classify
 *   POST /api/v1/citizen-reports/{maTraCuu}/assignment      feedback.assign
 *   POST /api/v1/citizen-reports/{maTraCuu}/status          feedback.read + LUẬT NẮM GIỮ
 *   POST /api/v1/citizen-reports/{maTraCuu}/closure         feedback.resolve
 *   POST /api/v1/citizen-reports/{maTraCuu}/rejection       feedback.classify
 *   POST /api/v1/citizen-reports/{maTraCuu}/referral        feedback.classify
 *   PUT  /api/v1/citizen-reports/{maTraCuu}/publication     feedback.assign
 *   GET  /api/v1/citizen-reports/{maTraCuu}/log-entries     feedback.read
 *   POST /api/v1/citizen-reports/{maTraCuu}/log-entries     feedback.read + luật nghiệp vụ, Idempotency-Key
 *   POST /api/v1/citizen-reports/{maTraCuu}/tasks           task.create + feedback.read, Idempotency-Key
 *   GET  /api/v1/citizen-reports/{maTraCuu}/photos          feedback.read (signed links, ≤ 15 min)
 *
 * Added 02/10/2026 (backend of the same day):
 *
 *   GET  /api/v1/citizen-report-intake-fields               feedback.create
 *   POST /api/v1/citizen-reports                            feedback.create, Idempotency-Key
 *   POST …/{maTraCuu}/log-attachments                       feedback.read + note rule, Idempotency-Key
 *   POST …/{maTraCuu}/log-attachments/{id}/completion       feedback.read
 *   GET  …/{maTraCuu}/log-attachments/{id}/download         feedback.read (audited)
 *   GET  …/{maTraCuu}/verification-photos                   feedback.read (audited, ≤ 15 min)
 *   POST …/{maTraCuu}/verification-photos                   feedback.resolve, Idempotency-Key
 *   POST …/{maTraCuu}/verification-photos/{id}/completion   feedback.resolve
 *
 * Added 09/10/2026 (ADR 0053 §Sửa đổi 09/10/2026, 0072 §Trả lời 09/10/2026, 0088):
 *
 *   GET    /api/v1/citizen-report-counts                     feedback.read (same filters as the list)
 *   GET    /api/v1/citizen-report-points                     feedback.read (same filters; 422 over 5000)
 *   GET    /api/v1/citizen-report-breakdown                  feedback.read + report.read
 *   DELETE …/{maTraCuu}/log-attachments/{id}                 feedback.read gate; uploader OR feedback.resolve
 *
 * The KPI cards read GET /api/v1/citizen-report-summary through `lib/api/dashboard.ts`
 * (`fetchCitizenReportSummary`) — one client for that route, not two.
 *
 * KIỂU LẤY TỪ HỢP ĐỒNG, KHÔNG GÕ TAY: `petitions_phieuPhanAnhRa`, `petitions_phanLoaiVao`,
 * `petitions_phanCongVao`, `petitions_dongPhieuVao` đều đến từ `schema.gen.ts`.
 *
 * ⚠ CÂU MỞ ĐẦU CŨ CỦA TỆP NÀY — *"KHÔNG CÓ TUYẾN DANH SÁCH"* — ĐÃ SAI TỪ 23/09/2026, và nó được
 * ghi lại ở đây thay vì xoá lặng lẽ: một chú thích nói sai về việc "hợp đồng không có gì" là thứ
 * người sau đọc rồi dựng một màn hình nghèo đi theo. Năm tuyến xử lý phía cán bộ nay đã có.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * PHẢN HỒI ĐÃ CHE SẴN DỮ LIỆU CÁ NHÂN, VÀ MÀN HÌNH KHÔNG "LÀM ĐẸP" LẠI.
 *
 * `reporter_name` về dạng `Nguyễn V. A.`, `reporter_phone` về dạng `09****5678`, và cả hai RỖNG
 * khi người dân gửi ẩn danh. Ở TUYẾN DANH SÁCH thì che là TUYỆT ĐỐI — kể cả tài khoản có
 * `feedback.unmask` cũng nhận bản đã che, vì một lời gọi mở hai mươi người gửi thì không viết
 * nổi một dòng vết kiểm toán trung thực (luật 6 bất biến 7; `xu_ly_phan_anh.go`, khối trên
 * `DanhSachPhieu`). Cán bộ cần số thật thì MỞ TỪNG PHIẾU. Không có chỗ nào trong ứng dụng này
 * ghép lại, đoán lại, hay hiện thêm chữ số nào.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 *
 * THAM SỐ TRUY VẤN CỦA TUYẾN DANH SÁCH NAY ĐÃ ĐƯỢC HỢP ĐỒNG KHAI: `petitions_get_citizen_reports
 * ["truyVan"]` có `status` · `channel` · `field` · `hamlet` · `unit` · `q` · `late` · `scope` cùng
 * `limit` · `cursor` · `sort` · `order`. Khối này từng viết rằng đối tượng ấy RỖNG — đúng lúc viết,
 * hết đúng khi tuyến có chú thích `@query`.
 *
 * ⚠ ĐIỀU VẪN CÒN ĐÚNG: `themLocVaoTruyVan` ghép tên tham số bằng CHUỖI TRẦN, nên gõ sai một tên thì
 * `tsc` vẫn im lặng và máy chủ trả cả sổ. Tên được gom vào đúng một chỗ và có bài kiểm đọc lại từng
 * tên (`phieu-phan-anh.test.ts`).
 */

import { isPeriodMetric, type CitizenReportMetric } from "@/lib/drill-down";

import {
  CHUNG,
  docJSON,
  docThanLoiGoi,
  goiGhi,
  LOI_KHONG_RO,
  stripTechnicalPrefix,
  thamSoTheoHopDong,
  thongBaoLoi,
  type KetQua,
} from "./goi"; // vi-name-ok: existing exports of goi.ts (rule 12 inv 3)
import { UPLOAD_FORM_MISSING, type CallResult } from "./task-attachments";
import type {
  httpx_Error,
  petitions_citizenFieldListOut,
  petitions_citizenReportBreakdownOut,
  petitions_citizenReportCountsOut,
  petitions_citizenReportLogAttachmentRemoveIn,
  petitions_citizenReportPointsOut,
  petitions_delete_citizen_reports_by_maTraCuu_log_attachments_by_id,
  petitions_get_citizen_report_breakdown,
  petitions_get_citizen_report_counts,
  petitions_get_citizen_report_intake_fields,
  petitions_get_citizen_report_points,
  petitions_get_citizen_reports_by_maTraCuu_log_attachments_by_id_download,
  petitions_get_citizen_reports_by_maTraCuu_verification_photos,
  petitions_photoOut,
  petitions_photoUploadIn,
  petitions_photoUploadOut,
  petitions_post_citizen_reports,
  petitions_post_citizen_reports_by_maTraCuu_log_attachments,
  petitions_post_citizen_reports_by_maTraCuu_log_attachments_by_id_completion,
  petitions_post_citizen_reports_by_maTraCuu_verification_photos,
  petitions_post_citizen_reports_by_maTraCuu_verification_photos_by_id_completion,
  petitions_staffIntakeIn,
  petitions_taskAttachmentDownloadOut,
  petitions_taskAttachmentOut,
  petitions_taskAttachmentUploadIn,
  petitions_taskAttachmentUploadOut,
  petitions_chuyenCapTrenVao,
  petitions_dongPhieuVao,
  petitions_get_citizen_reports,
  petitions_get_citizen_reports_by_maTraCuu,
  petitions_get_citizen_reports_by_maTraCuu_log_entries,
  petitions_get_citizen_reports_by_maTraCuu_photos,
  petitions_ghiChuPhieuVao,
  petitions_khongTiepNhanVao,
  petitions_nhatKyPhieuRa,
  petitions_nhiemVuRa,
  petitions_petitionTaskIn,
  petitions_phanCongVao,
  petitions_phanLoaiVao,
  petitions_phieuPhanAnhRa,
  petitions_photoListOut,
  petitions_post_citizen_reports_by_maTraCuu_assignment,
  petitions_post_citizen_reports_by_maTraCuu_classification,
  petitions_post_citizen_reports_by_maTraCuu_closure,
  petitions_post_citizen_reports_by_maTraCuu_log_entries,
  petitions_post_citizen_reports_by_maTraCuu_referral,
  petitions_post_citizen_reports_by_maTraCuu_rejection,
  petitions_post_citizen_reports_by_maTraCuu_status,
  petitions_post_citizen_reports_by_maTraCuu_tasks,
  petitions_publicationIn,
  petitions_put_citizen_reports_by_maTraCuu_publication,
  page_Result_petitions_nhatKyPhieuRa,
  page_Result_petitions_phieuPhanAnhRa,
} from "./schema.gen";

/**
 * Điền mã tra cứu vào khuôn đường dẫn CỦA HỢP ĐỒNG.
 *
 * Nhận khuôn (`/api/v1/citizen-reports/{maTraCuu}/closure`) chứ không tự ghép chuỗi: khuôn ấy lấy
 * từ kiểu sinh ra, nên đổi đường dẫn ở máy chủ là `tsc` đỏ tại chỗ gọi thay vì 404 lúc chạy.
 *
 * `encodeURIComponent` vì mã tra cứu là chuỗi người dân cầm trên tay và gõ lại — một dấu `/` hay
 * một khoảng trắng lọt vào sẽ đổi hẳn tuyến được gọi.
 */
function duongDanPhieu(mau: string, maTraCuu: string): string {
  return mau.replace("{maTraCuu}", encodeURIComponent(maTraCuu));
}

/**
 * Ghi chú NỘI BỘ đi kèm sáu thao tác xử lý — trường `note` tuỳ chọn, máy chủ ghi nó vào nhật ký xử
 * lý của phiếu (không gửi người dân, không vào `reason`/`result`).
 *
 * TRỐNG HAY TOÀN KHOẢNG TRẮNG THÌ TRƯỜNG VẮNG MẶT HẲN, không gửi `note: ""`: một dòng nhật ký với
 * ghi chú rỗng trông như cán bộ đã định viết gì rồi bị mất. Cắt khoảng trắng ở đây vì cùng lý do với
 * `reason` bên dưới — máy chủ đếm giới hạn 2000 ký tự trên chuỗi đã cắt.
 *
 * Chữ này là chữ cán bộ gõ về việc của một công dân: chỉ đi trong THÂN POST (luật 3, cấm #4).
 */
function thanGhiChu(ghiChu: string | undefined): { note?: string } {
  const gon = ghiChu?.trim() ?? "";
  return gon === "" ? {} : { note: gon };
}

/**
 * Files that go WITH an act (ADR 0088 §2): ids of completed log attachments, linked by the server to the
 * act's own log row in the act's transaction. ABSENT when there is none — the act's body is then
 * byte-for-byte what it was before files existed. Only STORED ids reach this list (`storedIds`).
 */
function actAttachments(attachments: readonly string[] | undefined): { attachments?: string[] } {
  return attachments === undefined || attachments.length === 0 ? {} : { attachments: [...attachments] };
}

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * ĐỌC
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

/**
 * GET /api/v1/citizen-reports/{maTraCuu}.
 *
 * MỘT MÃ 404 DUY NHẤT CHO BỐN TÌNH HUỐNG, và giao diện không được dựng lại sự phân biệt ấy: mã
 * không tồn tại · mã của xã khác · phiếu đã xoá mềm · phiếu thuộc lĩnh vực hạn chế (phản ánh về
 * tác phong cán bộ) mà tài khoản thiếu `feedback.restricted`. Máy chủ trả cùng một câu cho cả
 * bốn, có chủ ý: phân biệt được chúng là nói cho người đang thử mã biết họ gần tới đâu, và ở ca
 * thứ tư là nói cho một đồng nghiệp biết có người vừa phản ánh về họ (luật 4, cấm #2).
 *
 * Nên ở đây KHÔNG rẽ nhánh theo `code`, không đổi câu chữ, không thêm gợi ý nào — hiện đúng
 * `message` của máy chủ (`lib/api/goi.ts`).
 */
export function layPhieuPhanAnh(maTraCuu: string): Promise<KetQua<petitions_phieuPhanAnhRa>> {
  const mau: petitions_get_citizen_reports_by_maTraCuu["duongDan"] =
    "/api/v1/citizen-reports/{maTraCuu}";
  return docJSON<petitions_phieuPhanAnhRa>(duongDanPhieu(mau, maTraCuu));
}

/**
 * Bộ lọc của sổ phản ánh — bảy bộ lọc `locPhieuTuQuery` đọc, phạm vi, cộng phân trang.
 *
 * `LIÊN QUAN ĐẾN TÔI` KHÔNG CÓ Ở ĐÂY VÀ KIỂU `phamVi` KHÔNG CHO GỬI NÓ: máy chủ trả **400** cho
 * `scope=related` (thế nào là "liên quan" chưa được chốt — `xu_ly_phan_anh.go`, `errPhamVi…`), nên
 * một tab gửi nó là một tab biến quyển sổ thành trang lỗi — ra tới màn hình qua `PHAN_CHUA_DUNG`.
 * Lọc `Bị đánh giá thấp` thì NAY ĐÃ CÓ: `ratingMax` → `rating_max` (ADR 0050 điểm 2).
 */
export type LocPhanAnh = {
  /**
   * §4 tab Phạm vi. `mine` = "Giao cho tôi": phiếu mà người gọi đang là cán bộ được giao, và MÁY
   * CHỦ tự lấy mã cán bộ từ PHIÊN — `?scope=mine` **không** mang danh tính nào. Một tab gửi
   * `?assignee=CB-…` của chính mình là client tự khai mình là ai (luật 1, cấm #2).
   */
  phamVi?: "all" | "mine";
  /** Một trong chín mã trạng thái. Sai mã là **400**, không phải bị bỏ qua. */
  trangThai?: string;
  /** Một trong bốn mã kênh tiếp nhận. Sai mã là **400**. */
  kenh?: string;
  /** Mã lĩnh vực tầng 1. KHÔNG được kiểm ở máy chủ: mã lạ trả về trang rỗng. */
  linhVuc?: string;
  /** ULID thôn/tổ dân phố. */
  thonID?: string;
  /** ULID bộ phận đang giữ phiếu. */
  boPhanID?: string;
  /** Tìm theo nội dung, mã phiếu, địa chỉ. Tối đa 200 ký tự, dài hơn là 400. */
  tim?: string;
  /** Chỉ phiếu đã quá hạn XỬ LÝ XONG. Máy chủ chỉ nhận đúng chuỗi `true`. */
  chiTreHan?: boolean;
  /**
   * Only petitions the citizen rated AT MOST this many stars; unrated ones are excluded by the server.
   * The server accepts exactly the digits 1–5 and answers 400 to anything else, so the type admits
   * nothing else. The screen's `Bị đánh giá thấp` box sends 2 — spec §4 "phiếu 1–2 sao", the same
   * threshold that reopens a petition (ADR 0050 point 2).
   */
  ratingMax?: 1 | 2 | 3 | 4 | 5;
  limit?: number;
  cursor?: string | null;
  /**
   * Lọc theo MỘT số liệu của trang Tổng quan (SRS M7.2.2). Máy chủ đọc `feedback.restricted` qua
   * cùng một hàm cho số đếm và danh sách, nên con số và số dòng khớp nhau với mọi tài khoản.
   */
  metric?: CitizenReportMetric;
  /** Kỳ nửa mở [from, to), RFC3339. CHỈ gửi với số liệu theo kỳ. */
  from?: string;
  to?: string;
};

/**
 * Gom TÊN tham số truy vấn về đúng một chỗ.
 *
 * Bảy cái tên dưới đây là bảy chuỗi KHÔNG có kiểu nào của hợp đồng canh giúp (xem khối chú thích
 * đầu tệp). Gõ sai một cái thì máy chủ bỏ qua nó và trả về cả quyển sổ, và màn hình trông hoàn
 * toàn bình thường — nên chúng đứng một chỗ và có bài kiểm đọc lại từng tên.
 */
function themLocVaoTruyVan(truyVan: URLSearchParams, loc: LocPhanAnh): void {
  addFilters(truyVan, loc);

  if (loc.limit !== undefined) truyVan.set("limit", String(loc.limit));
  // Con trỏ rỗng nghĩa là trang đầu. Gửi `cursor=` rỗng thì máy chủ trả 400 "con trỏ không hợp
  // lệ" — đúng vào lần mở màn hình đầu tiên.
  if (loc.cursor !== undefined && loc.cursor !== null && loc.cursor !== "") {
    truyVan.set("cursor", loc.cursor);
  }
}

/**
 * The FILTER half of the list's query — everything but paging. ONE builder for the list, the count
 * (`citizen-report-counts`) and the heat-map points (`citizen-report-points`): the three routes read the
 * same filter parameters with the same parser on the server, so a second builder here would be the copy
 * that forgets a filter and draws a heat map, or a total, of a different slice than the cards below it.
 *
 * The keyword (`q`) goes into the REQUEST only — never the address bar, storage or a log (rule 3,
 * forbidden #4): the screen keeps it in component state and nothing here writes a location.
 */
function addFilters(truyVan: URLSearchParams, loc: LocPhanAnh): void {
  // `scope=all` là mặc định của máy chủ, nên tab "Toàn xã" gửi tham số VẮNG MẶT HẲN — cùng cách
  // sổ Nhiệm vụ làm (`lib/api/nhiem-vu.ts`).
  if (loc.phamVi === "mine") truyVan.set("scope", "mine");

  if (loc.trangThai !== undefined && loc.trangThai !== "") truyVan.set("status", loc.trangThai);
  if (loc.kenh !== undefined && loc.kenh !== "") truyVan.set("channel", loc.kenh);
  if (loc.linhVuc !== undefined && loc.linhVuc !== "") truyVan.set("field", loc.linhVuc);
  if (loc.thonID !== undefined && loc.thonID !== "") truyVan.set("hamlet", loc.thonID);
  if (loc.boPhanID !== undefined && loc.boPhanID !== "") truyVan.set("unit", loc.boPhanID);
  if (loc.tim !== undefined && loc.tim !== "") truyVan.set("q", loc.tim);
  // `late` CHỈ NHẬN ĐÚNG CHUỖI `true`. Gửi `false` là **400**, không phải "không lọc" — nên ô
  // chưa tích thì tham số vắng mặt hẳn (`locPhieuTuQuery`, `errLocTreHanKhongHopLe`).
  if (loc.chiTreHan === true) truyVan.set("late", "true");

  // SỐ LIỆU CỦA TỔNG QUAN — ba tên mới đi qua `thamSoTheoHopDong`, nên `tsc` đối chiếu chúng với
  // kiểu `truyVan`. Kỳ chỉ đi lên với số liệu theo kỳ, cùng quy tắc sổ Nhiệm vụ.
  const dat = thamSoTheoHopDong<petitions_get_citizen_reports["truyVan"]>(truyVan);
  // Absent when the box is unticked — never `rating_max=` empty, which the server answers with 400.
  dat("rating_max", loc.ratingMax === undefined ? undefined : String(loc.ratingMax));
  dat("metric", loc.metric);
  if (loc.metric !== undefined && isPeriodMetric("citizen-reports", loc.metric)) {
    dat("from", loc.from);
    dat("to", loc.to);
  }
}

/** A filter without paging: what the count and the heat-map points take. */
export type CitizenReportFilter = Omit<LocPhanAnh, "limit" | "cursor">;

function withFilters(path: string, loc: CitizenReportFilter): string {
  const q = new URLSearchParams();
  addFilters(q, loc);
  const s = q.toString();
  return s === "" ? path : `${path}?${s}`;
}

/** Path of GET /api/v1/citizen-report-counts for these filters. Split from the call so it can be tested. */
export function citizenReportCountsPath(loc: CitizenReportFilter = {}): string {
  const path: petitions_get_citizen_report_counts["duongDan"] = "/api/v1/citizen-report-counts";
  return withFilters(path, loc);
}

/**
 * GET /api/v1/citizen-report-counts — how many petitions the list's filters match, counted by the SAME
 * predicate as the list (`feedback.restricted` included), so the tab's "(n)" and the pages agree. The
 * register itself pages by cursor and returns no total; this is the only total the screen shows.
 */
export function countCitizenReports(
  loc: CitizenReportFilter = {},
): Promise<KetQua<petitions_citizenReportCountsOut>> {
  return docJSON<petitions_citizenReportCountsOut>(citizenReportCountsPath(loc));
}

/** Path of GET /api/v1/citizen-report-points for these filters. */
export function citizenReportPointsPath(loc: CitizenReportFilter = {}): string {
  const path: petitions_get_citizen_report_points["duongDan"] = "/api/v1/citizen-report-points";
  return withFilters(path, loc);
}

/**
 * 422 of the points route: more located petitions than the cap (5000, ADR 0072 §Trả lời 09/10/2026).
 * READ BY THE SCREEN for one reason: the answer is "narrow the filters", not "try again" — so the heat-map
 * tab draws no map and offers no `Tải lại` (reloading the same filters gets the same refusal). The
 * sentence stays the server's, verbatim.
 */
export const TOO_MANY_POINTS_CODE =
  "too_many_points" satisfies petitions_get_citizen_report_points["errorCodes"][422];

export type PointsResult =
  | { readonly ok: true; readonly data: petitions_citizenReportPointsOut }
  | { readonly ok: false; readonly message: string; readonly tooManyPoints: boolean };

/**
 * GET /api/v1/citizen-report-points — every located petition the filters match: latitude, longitude,
 * status, NOTHING else (no code, no content, no sender — rule 3). All of them or a refusal: the server
 * never cuts the set, because a heat map of part of the points looks like the commune's and is not.
 *
 * ⚠ The answer is citizens' coordinates: held in component state for the map and nowhere else — never
 * logged, never cached, never put in a URL.
 */
export async function listCitizenReportPoints(loc: CitizenReportFilter = {}): Promise<PointsResult> {
  let res: Response;
  try {
    res = await fetch(citizenReportPointsPath(loc), { ...CHUNG, method: "GET" });
  } catch {
    return { ok: false, message: LOI_KHONG_RO, tooManyPoints: false };
  }
  if (res.status === 200) {
    try {
      return { ok: true, data: (await res.json()) as petitions_citizenReportPointsOut };
    } catch {
      return { ok: false, message: LOI_KHONG_RO, tooManyPoints: false };
    }
  }
  const err = await readError(res);
  return { ok: false, message: err.message, tooManyPoints: res.status === 422 && err.code === TOO_MANY_POINTS_CODE };
}

/** Path of GET /api/v1/citizen-report-breakdown for one period `[from, to)`. */
export function citizenReportBreakdownPath(period: { from: string; to: string }): string {
  const path: petitions_get_citizen_report_breakdown["duongDan"] = "/api/v1/citizen-report-breakdown";
  const q = new URLSearchParams();
  const set = thamSoTheoHopDong<petitions_get_citizen_report_breakdown["truyVan"]>(q);
  set("from", period.from);
  set("to", period.to);
  const s = q.toString();
  return s === "" ? path : `${path}?${s}`;
}

/**
 * GET /api/v1/citizen-report-breakdown — the Báo cáo tab (ADR 0053 §Sửa đổi 09/10/2026, C1–C5): on time /
 * late in the period and the current overdue stock, by field, by unit (working hours), by hamlet. 503
 * `working_hours_unavailable` and 409 `working_calendar_not_configured` come back as the server's sentence,
 * verbatim — nothing is counted on this side. `feedback.read` + `report.read`.
 */
export function readCitizenReportBreakdown(period: {
  from: string;
  to: string;
}): Promise<KetQua<petitions_citizenReportBreakdownOut>> {
  return docJSON<petitions_citizenReportBreakdownOut>(citizenReportBreakdownPath(period));
}

/** Dựng đường dẫn đọc sổ. Tách khỏi lời gọi mạng để kiểm được mà không cần thay `fetch`. */
export function duongDanSoPhanAnh(loc: LocPhanAnh = {}): string {
  const duongDan: petitions_get_citizen_reports["duongDan"] = "/api/v1/citizen-reports";
  const truyVan = new URLSearchParams();
  themLocVaoTruyVan(truyVan, loc);
  const chuoi = truyVan.toString();
  return chuoi === "" ? duongDan : `${duongDan}?${chuoi}`;
}

/**
 * GET /api/v1/citizen-reports — một trang của sổ phản ánh.
 *
 * PHIẾU THUỘC LĨNH VỰC `can-bo` KHÔNG NẰM TRONG TRANG khi tài khoản thiếu `feedback.restricted`,
 * và chúng cũng không nằm trong con trỏ — máy chủ lọc trong câu truy vấn chứ không cắt sau khi
 * đọc. Màn hình vì thế **không được** nói "có n phiếu bị ẩn": số đếm suy ra từ việc lật trang đã
 * không có chúng, và nói ra là nói cho một đồng nghiệp biết có người vừa phản ánh về họ.
 */
export function laySoPhanAnh(
  loc: LocPhanAnh = {},
): Promise<KetQua<page_Result_petitions_phieuPhanAnhRa>> {
  return docJSON<page_Result_petitions_phieuPhanAnhRa>(duongDanSoPhanAnh(loc));
}

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * BỐN THAO TÁC GHI CỦA LUỒNG CHÍNH (hai nhánh rẽ ở cuối tệp)
 *
 * CẢ SÁU TUYẾN GHI TRẢ VỀ 200 KÈM NGUYÊN PHIẾU SAU KHI GHI, và màn hình dùng đúng phiếu ấy thay vì vá tại
 * chỗ: trạng thái vừa tới và hạn vừa được ấn định quyết định lần sau vẽ nút nào.
 *
 * 409 LÀ CÂU TRẢ LỜI BÌNH THƯỜNG CỦA BA TUYẾN, không phải sự cố — "phiếu đã chuyển trạng thái
 * trong lúc bạn mở màn hình", hoặc "xã chưa cấu hình thời hạn cho lĩnh vực này". Câu của máy chủ
 * đi thẳng ra màn hình; viết lại nó ở đây là dựng bản sao thứ hai của một quy tắc nghiệp vụ.
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

/**
 * POST …/classification — chốt lĩnh vực. **Hành vi ẤN ĐỊNH hạn xử lý xong** theo SLA của xã.
 *
 * ĐÚNG MỘT TRƯỜNG, VÀ HAI SỰ VẮNG MẶT LÀ TỪ CHỐI CHỨ KHÔNG PHẢI BỎ SÓT (`phanLoaiVao`): không có
 * `due_at` — hạn là cam kết của xã tính theo giờ làm việc của chính xã, client đặt được hạn là
 * client đặt được xã ấy chậm bao lâu; không có `status` — hành vi này đưa phiếu sang
 * `dang-phan-loai` và không sang đâu khác.
 */
export function phanLoaiPhieu(
  maTraCuu: string,
  linhVuc: string,
  ghiChu?: string,
  extra: ClassifyExtras = {},
): Promise<KetQua<petitions_phieuPhanAnhRa>> {
  const mau: petitions_post_citizen_reports_by_maTraCuu_classification["duongDan"] =
    "/api/v1/citizen-reports/{maTraCuu}/classification";
  const than: petitions_phanLoaiVao = {
    field: linhVuc,
    ...thanGhiChu(ghiChu),
    ...actAttachments(extra.attachments),
    // ABSENT when nothing is picked: the server keeps what the petition holds (there is no "remove").
    // The stored value sent back is a CONFIRMATION; a different one is checked with identity and
    // audited (ADR 0088 §1) — its 400 / 503 come back verbatim.
    ...(extra.residentialUnitId !== undefined && extra.residentialUnitId !== ""
      ? { residential_unit_id: extra.residentialUnitId }
      : {}),
  };
  return docThanLoiGoi<petitions_phieuPhanAnhRa>(
    goiGhi(duongDanPhieu(mau, maTraCuu), "POST", than, 200),
  );
}

/** What the classification act may carry besides the field and the note (ADR 0088). */
export type ClassifyExtras = {
  /** ULID of the thôn / tổ dân phố the officer confirms or picks; `""` / absent = keep what is stored. */
  readonly residentialUnitId?: string;
  readonly attachments?: readonly string[];
};

/**
 * POST …/assignment — khối `Chuyển xử lý, không đổi trạng thái` của §8.5.
 *
 * `assignee` LÀ TUỲ CHỌN VÀ ĐÓ LÀ MỘT LỰA CHỌN CÓ THẬT trên màn hình: `— Để bộ phận phân công —`.
 * Trường vắng mặt hẳn khi không chọn ai, chứ không gửi chuỗi rỗng.
 *
 * `assignee` LÀ **MÃ CÁN BỘ** (`code` của `GET /api/v1/staff-directory`, dạng `CB-00123`), KHÔNG
 * PHẢI ULID NỘI BỘ. Máy chủ ghi nguyên chuỗi ấy vào `can_bo_xu_ly_id`, và luật nắm giữ so cột ấy với
 * `Principal.Ma` (`service-petitions/internal/app/xu_ly_phan_anh.go:184-212`). Gửi một ULID thì phiếu
 * vẫn "được giao" trên màn hình, còn người được giao thì không bao giờ tiến được nó.
 */
export function chuyenXuLyPhieu(
  maTraCuu: string,
  boPhanID: string,
  maCanBo?: string,
  ghiChu?: string,
  attachments?: readonly string[],
): Promise<KetQua<petitions_phieuPhanAnhRa>> {
  const mau: petitions_post_citizen_reports_by_maTraCuu_assignment["duongDan"] =
    "/api/v1/citizen-reports/{maTraCuu}/assignment";
  const than: petitions_phanCongVao = {
    ...(maCanBo !== undefined && maCanBo !== ""
      ? { unit: boPhanID, assignee: maCanBo }
      : { unit: boPhanID }),
    ...thanGhiChu(ghiChu),
    ...actAttachments(attachments),
  };
  return docThanLoiGoi<petitions_phieuPhanAnhRa>(
    goiGhi(duongDanPhieu(mau, maTraCuu), "POST", than, 200),
  );
}

/**
 * POST …/status — tiến phiếu MỘT bước trên luồng chính.
 *
 * KHÔNG CÓ THÂN, VÀ ĐÓ LÀ THIẾT KẾ CỦA MÁY CHỦ: một trạng thái đích đi trên dây là một client nhảy
 * được bước — đẩy thẳng phiếu sang `da-xu-ly` mà không ai làm gì — và những bước ở giữa sẽ thành
 * tuỳ chọn trên thực tế trong khi bản đồ vòng đời vẫn trông như bắt buộc. Máy chủ giữ bản đồ;
 * người gọi chỉ nói "tiến".
 *
 * KHÔNG GỬI `Content-Type`: tuyên bố có một thân là mời người sau điền vào đó một trạng thái đích.
 *
 * ⚠ TUYẾN NÀY KHAI `feedback.read`, VÀ ĐIỀU KIỆN THẬT NẰM Ở TẦNG NGHIỆP VỤ: `feedback.resolve`
 * **HOẶC** chính là cán bộ được phân công phiếu ấy (`app.duocTienTrangThai`). Trưởng thôn nhận một
 * phiếu phải tiến được phiếu ấy. Giao diện KHÔNG dựng lại phép kiểm đó: `assignee` và `staff.code`
 * của phiên đều là mã cán bộ nên so được, nhưng một phép so ở giao diện chỉ là một bản sao thứ hai
 * của luật nắm giữ, và bản sao ấy sẽ lệch vào ngày luật đổi. Nút luôn hiện với người xem được sổ,
 * và câu 403 của máy chủ ra thẳng màn hình.
 *
 * ⚠ NGOẠI LỆ DUY NHẤT CỦA "KHÔNG CÓ THÂN": ghi chú nội bộ `{note}`, từ 26/09/2026. Máy chủ nhận một
 * thân JSON TUỲ CHỌN chỉ mang `note` (không trạng thái đích nào). Kiểu `{ note: string }` dưới đây
 * GÕ TAY, và đó là lỗ hổng của công cụ chứ không phải lựa chọn: `tools/apidoc` không diễn tả được
 * một thân tuỳ chọn, nên hợp đồng khai tuyến này `than: never`. Hệ quả: máy chủ đổi tên trường thì
 * `tsc` KHÔNG đỏ ở đây. Ghi chú trống thì vẫn KHÔNG thân, KHÔNG `Content-Type` — đúng hình dạng cũ.
 */
export function tienTrangThaiPhieu(
  maTraCuu: string,
  ghiChu?: string,
  attachments?: readonly string[],
): Promise<KetQua<petitions_phieuPhanAnhRa>> {
  const mau: petitions_post_citizen_reports_by_maTraCuu_status["duongDan"] =
    "/api/v1/citizen-reports/{maTraCuu}/status";
  // Hand-typed like `note` (the contract declares this route `than: never`): `attachments` is the
  // server's `tienTrangThaiVao.Attachments` (`service-petitions/internal/http/xu_ly_phan_anh.go`).
  const extra = { ...thanGhiChu(ghiChu), ...actAttachments(attachments) };
  const than: { note?: string; attachments?: string[] } | undefined =
    Object.keys(extra).length === 0 ? undefined : extra;
  return docThanLoiGoi<petitions_phieuPhanAnhRa>(
    goiGhi(duongDanPhieu(mau, maTraCuu), "POST", than, 200),
  );
}

/**
 * POST …/closure — đóng phiếu kèm **kết quả người dân đọc được**.
 *
 * `result` BẮT BUỘC VÀ MÁY CHỦ CƯỠNG CHẾ (luật 10, bất biến 6). Nó là câu người dân đọc khi tra
 * phiếu của mình, và là thứ duy nhất phân biệt một phiếu đã được xử lý với một phiếu bị xếp lại
 * trong im lặng. "Đã xử lý" trống trơn không phải một kết quả.
 *
 * ⚠ TUYẾN NÀY ĐỨNG SAU `feedback.resolve` VÀ **KHÔNG** ĐƯỢC NỚI THEO LUẬT NẮM GIỮ Ở `…/status`.
 * Hai dòng ấy khác nhau chính là điểm chính: câu hỏi mở #7 chốt ngày 16/09/2026 —
 * *"`feedback.resolve` quyết định ai đóng được"*.
 *
 * HAI ĐIỂM ĐÓNG, MÁY CHỦ QUYẾT (`domain.DongDuoc`): `cho-dan-xac-nhan`, và `da-xu-ly` khi phiếu
 * KHÔNG có tài khoản công dân nào đứng sau (không ai để xác nhận — quyết định ngày 24/09/2026).
 */
export async function dongPhieu(
  maTraCuu: string,
  ketQua: string,
  ghiChu?: string,
  attachments?: readonly string[],
): Promise<CloseResult> {
  const mau: petitions_post_citizen_reports_by_maTraCuu_closure["duongDan"] =
    "/api/v1/citizen-reports/{maTraCuu}/closure";
  const than: petitions_dongPhieuVao = { result: ketQua, ...thanGhiChu(ghiChu), ...actAttachments(attachments) };
  let res: Response;
  try {
    res = await fetch(duongDanPhieu(mau, maTraCuu), {
      ...CHUNG,
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(than),
    });
  } catch {
    return { ok: false, thongBao: LOI_KHONG_RO, afterPhotoRequired: false };
  }
  if (res.status === 200) {
    try {
      return { ok: true, duLieu: (await res.json()) as petitions_phieuPhanAnhRa };
    } catch {
      return { ok: false, thongBao: LOI_KHONG_RO, afterPhotoRequired: false };
    }
  }
  const err = await readError(res);
  return {
    ok: false,
    thongBao: err.message,
    afterPhotoRequired: res.status === 409 && err.code === AFTER_PHOTO_REQUIRED_CODE,
  };
}

/**
 * The closure's answer. A refusal additionally says whether it is THE commune's verification-photo
 * gate (409 `after_photo_required`, ADR 0008 decision 3) — the ONE code of this route the screen reads.
 *
 * WHY THIS CODE IS READ when `goi.ts` says never to branch on `code`: the server gives this refusal a
 * code of its own precisely so the screen can point at the "Sau khi xử lý" upload instead of telling
 * the officer to reload (`service-petitions/internal/http/xu_ly_phan_anh.go`, the
 * `ErrVerificationPhotoRequired` case). The SENTENCE is still the server's, verbatim — it is the
 * commune's "Lời hệ thống" wording of `feedback.after_photo_required`, and rewriting it here would be a
 * second copy of the commune's voice. Every other refusal is `afterPhotoRequired: false` and reads
 * exactly as before.
 *
 * Assignable to `KetQua<petitions_phieuPhanAnhRa>`, so a caller that ignores the flag is unchanged.
 */
export type CloseResult =
  | { ok: true; duLieu: petitions_phieuPhanAnhRa }
  | { ok: false; thongBao: string; afterPhotoRequired: boolean };

export const AFTER_PHOTO_REQUIRED_CODE = "after_photo_required";

/** `message` (verbatim, else `LOI_KHONG_RO`) and `code` of an `httpx.Error` body. Never logged. */
async function readError(res: Response): Promise<{ message: string; code: string }> {
  try {
    const body = (await res.json()) as httpx_Error;
    return {
      message:
        typeof body?.message === "string" && body.message !== ""
          ? stripTechnicalPrefix(body.message)
          : LOI_KHONG_RO,
      code: typeof body?.code === "string" ? body.code : "",
    };
  } catch {
    return { message: LOI_KHONG_RO, code: "" };
  }
}

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * HAI NHÁNH RẼ — `khong-tiep-nhan` và `chuyen-cap-tren`
 *
 * CHỈ TỪ `dang-phan-loai`, và cả hai là TRẠNG THÁI CUỐI: không tuyến nào đưa phiếu ra khỏi chúng
 * (`domain.chuyenDuocSang`). Nơi khác là **409** — câu của máy chủ ra thẳng màn hình.
 *
 * `reason` LÀ CÂU NGƯỜI DÂN ĐỌC khi tra phiếu của mình, cùng vai trò với `result` của …/closure.
 * Nó chỉ đi trong THÂN POST — không lên URL, không vào bộ nhớ trình duyệt, không vào console: đó
 * là chữ cán bộ gõ về việc của một công dân, và có thể nhắc tên hay địa chỉ người ấy (luật 3).
 *
 * CẮT KHOẢNG TRẮNG Ở ĐÂY chứ không để màn hình tự nhớ: máy chủ đếm trên chuỗi đã cắt, nên gửi đi
 * đúng chuỗi được đếm là cách duy nhất để bộ đếm ký tự trên màn hình nói cùng một con số.
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

/** POST …/rejection — `Không tiếp nhận`. Đúng một trường. */
export function khongTiepNhanPhieu(
  maTraCuu: string,
  lyDo: string,
  ghiChu?: string,
  attachments?: readonly string[],
): Promise<KetQua<petitions_phieuPhanAnhRa>> {
  const mau: petitions_post_citizen_reports_by_maTraCuu_rejection["duongDan"] =
    "/api/v1/citizen-reports/{maTraCuu}/rejection";
  const than: petitions_khongTiepNhanVao = {
    reason: lyDo.trim(),
    ...thanGhiChu(ghiChu),
    ...actAttachments(attachments),
  };
  return docThanLoiGoi<petitions_phieuPhanAnhRa>(
    goiGhi(duongDanPhieu(mau, maTraCuu), "POST", than, 200),
  );
}

/**
 * POST …/referral — `Chuyển cấp trên`. `receiving_body` là TÊN cơ quan nhận, chữ tự do: không có
 * danh mục cơ quan nào trong hợp đồng, và từ 7/2025 không còn cấp huyện để chọn sẵn.
 */
export function chuyenCapTrenPhieu(
  maTraCuu: string,
  lyDo: string,
  coQuanTiepNhan: string,
  ghiChu?: string,
  attachments?: readonly string[],
): Promise<KetQua<petitions_phieuPhanAnhRa>> {
  const mau: petitions_post_citizen_reports_by_maTraCuu_referral["duongDan"] =
    "/api/v1/citizen-reports/{maTraCuu}/referral";
  const than: petitions_chuyenCapTrenVao = {
    reason: lyDo.trim(),
    receiving_body: coQuanTiepNhan.trim(),
    ...thanGhiChu(ghiChu),
    ...actAttachments(attachments),
  };
  return docThanLoiGoi<petitions_phieuPhanAnhRa>(
    goiGhi(duongDanPhieu(mau, maTraCuu), "POST", than, 200),
  );
}

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * PUBLIC-PAGE MODERATION (§8.3, §14.4) — PUT …/publication, `feedback.assign`
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

/** The two acts a member of staff can record. `cho-duyet` is refused by the server (400): it means
 * "nobody decided", and no act can record that. */
export type PublicationTarget = "cong-khai" | "an";

/**
 * PUT …/publication — show the petition on the commune's public page, or hide it.
 *
 * IT MOVES NO LIFECYCLE STATUS and notifies nobody (server, `petition_publication.go`); the 200 body
 * is the petition as it now stands, and the screen re-renders from it rather than patching locally.
 *
 * 409 `never_public` is the NORMAL answer for a staff-conduct (`can-bo`) petition: the server's
 * sentence — the commune's `feedback.never_public`, rewordable on the Lời hệ thống tab — goes to the
 * screen verbatim. Rewriting it here would be a second copy of the rule and of the commune's wording.
 *
 * NO Idempotency-Key: the route declares none — it sets an absolute value, and sending the same value
 * again writes nothing.
 */
export function setPetitionPublication(
  maTraCuu: string,
  status: PublicationTarget,
): Promise<KetQua<petitions_phieuPhanAnhRa>> {
  const mau: petitions_put_citizen_reports_by_maTraCuu_publication["duongDan"] =
    "/api/v1/citizen-reports/{maTraCuu}/publication";
  const than: petitions_publicationIn = { status };
  return docThanLoiGoi<petitions_phieuPhanAnhRa>(
    goiGhi(duongDanPhieu(mau, maTraCuu), "PUT", than, 200),
  );
}

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * NHẬT KÝ XỬ LÝ (§8.7) — GET/POST …/log-entries
 *
 * BẢN GHI NGHIỆP VỤ CÁN BỘ ĐỌC, KHÁC `audit_log`: vết kiểm toán không hiện cho xã, còn nhật ký này là
 * thứ người nhận phiếu đọc để biết ai đã làm gì. Hai tuyến cùng khai `feedback.read`.
 *
 * 404 CHO HAI TÌNH HUỐNG, MỘT CÂU: mã không có, hoặc phiếu thuộc lĩnh vực hạn chế mà tài khoản thiếu
 * `feedback.restricted`. Cùng lý do với `layPhieuPhanAnh` — không rẽ nhánh theo `code`.
 *
 * `actor_code` LÀ MÃ CÁN BỘ (`CB-…`), không họ tên: máy chủ không trả tên, và màn hình hiện đúng mã
 * (luật 6, bất biến 8 — mã là thứ còn đọc được nhiều năm sau).
 *
 * EXCEPTION: rows the CITIZEN caused (`danh-gia`, `mo-lai-theo-danh-gia`) carry the fixed marker
 * `cong-dan`, not a staff code and never the citizen's id (`domain.CitizenLogActor`).
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

/** Phân trang của nhật ký. `order` mặc định của máy chủ là `desc` — mới nhất trước. */
export type TrangNhatKy = {
  limit?: number;
  cursor?: string | null;
};

/** Dựng đường dẫn đọc nhật ký. Tên tham số lấy từ hợp đồng — `tsc` đỏ khi máy chủ đổi tên. */
export function duongDanNhatKyPhieu(maTraCuu: string, trang: TrangNhatKy = {}): string {
  const mau: petitions_get_citizen_reports_by_maTraCuu_log_entries["duongDan"] =
    "/api/v1/citizen-reports/{maTraCuu}/log-entries";
  const truyVan = new URLSearchParams();
  const dat =
    thamSoTheoHopDong<petitions_get_citizen_reports_by_maTraCuu_log_entries["truyVan"]>(truyVan);
  dat("limit", trang.limit);
  // Con trỏ rỗng/`null` = trang đầu, và KHÔNG gửi `cursor=` rỗng (máy chủ trả 400).
  dat("cursor", trang.cursor);
  const chuoi = truyVan.toString();
  const duongDan = duongDanPhieu(mau, maTraCuu);
  return chuoi === "" ? duongDan : `${duongDan}?${chuoi}`;
}

/** GET …/log-entries — một trang nhật ký, mới nhất trước. */
export function layNhatKyPhieu(
  maTraCuu: string,
  trang: TrangNhatKy = {},
): Promise<KetQua<page_Result_petitions_nhatKyPhieuRa>> {
  return docJSON<page_Result_petitions_nhatKyPhieuRa>(duongDanNhatKyPhieu(maTraCuu, trang));
}

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * SCENE PHOTOS (§8.4 `TRƯỚC KHI XỬ LÝ`) — GET …/photos, `feedback.read`
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

/**
 * GET …/photos — the photos the citizen attached, each with a presigned GET of at most 15 minutes.
 *
 * RETURNS THE STATUS, unlike `docJSON`, for ONE reason: 503 `storage_not_configured` means "the object
 * store is not there", and the drawer must say that in its own sentence while everything else on the
 * petition keeps working. The server's 503 sentence is written for the CITIZEN's upload ("Phản ánh vẫn
 * được ghi nhận bình thường"), so it is the wrong sentence on a staff screen. Every other status shows
 * the server's message verbatim — 404 included, which is the detail's own four-cause 404 (rule 4,
 * forbidden #2); the screen does not branch on `code`.
 *
 * ⚠ EVERY `url` IN THE ANSWER IS A BEARER CREDENTIAL to a citizen's photograph (rule 3): it goes into an
 * `<img src>` and nowhere else — never logged, never stored beyond the component's state, never put in
 * the address bar. `no-store` comes from `CHUNG`. Called when the section is shown and when a link has
 * expired, NEVER on a timer: each call signs fresh links over personal data.
 */
export async function listPetitionPhotos(
  maTraCuu: string,
): Promise<CallResult<petitions_photoListOut>> {
  const mau: petitions_get_citizen_reports_by_maTraCuu_photos["duongDan"] =
    "/api/v1/citizen-reports/{maTraCuu}/photos";
  let res: Response;
  try {
    res = await fetch(duongDanPhieu(mau, maTraCuu), { ...CHUNG, method: "GET" });
  } catch {
    // Status 0 = no answer at all. Nothing logged: the answer would have held signed links.
    return { ok: false, status: 0, message: LOI_KHONG_RO };
  }
  if (res.status !== 200) {
    return { ok: false, status: res.status, message: await thongBaoLoi(res) };
  }
  try {
    return { ok: true, data: (await res.json()) as petitions_photoListOut };
  } catch {
    return { ok: false, status: res.status, message: LOI_KHONG_RO };
  }
}

/**
 * POST …/log-entries — một dòng `ghi-chu` vào nhật ký, không đổi trạng thái. Được ở MỌI trạng thái.
 *
 * `khoaChongTrung` LÀ THAM SỐ, KHÔNG SINH TẠI CHỖ. Hợp đồng đòi `Idempotency-Key` bắt buộc (400 khi
 * thiếu). Sinh khoá trong hàm này thì lần bấm lại sau lỗi mạng mang khoá MỚI — và nếu lần đầu thật ra
 * đã tới máy chủ, nhật ký có hai dòng giống hệt nhau. Khoá do biểu mẫu giữ: giữ nguyên khi gửi lại,
 * thay mới sau một lần thành công (`khoaSauLanGhi`).
 *
 * Quyền THẬT nằm ở tầng nghiệp vụ: người được phân công, hoặc người có `feedback.resolve` /
 * `feedback.assign` / `feedback.classify`. Câu 403 của máy chủ ra nguyên văn.
 */
export function ghiNhatKyPhieu(
  maTraCuu: string,
  ghiChu: string,
  khoaChongTrung: string,
  attachments: readonly string[] = [],
): Promise<KetQua<petitions_nhatKyPhieuRa>> {
  const mau: petitions_post_citizen_reports_by_maTraCuu_log_entries["duongDan"] =
    "/api/v1/citizen-reports/{maTraCuu}/log-entries";
  // `attachments` ABSENT when there is none — the entry's shape before files existed, so a note
  // without a file is byte-for-byte the request it always was. Only STORED ids reach this list
  // (`storedIds`); a refused or unfinished upload is never named here.
  const than: petitions_ghiChuPhieuVao =
    attachments.length === 0
      ? { note: ghiChu.trim() }
      : { note: ghiChu.trim(), attachments: [...attachments] };
  return docThanLoiGoi<petitions_nhatKyPhieuRa>(
    goiGhi(duongDanPhieu(mau, maTraCuu), "POST", than, 201, {
      "Idempotency-Key": khoaChongTrung,
    }),
  );
}

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * TASK FROM A PETITION (§13) — POST …/tasks, `task.create` AND `feedback.read`
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

/**
 * POST …/tasks — book one task whose source is THIS petition. 201 with the task, including the register
 * number the server just issued (`NV12`), which the caller cannot know in advance.
 *
 * BUILT FIELD BY FIELD, NEVER `...than`: the shared `FormGiaoViec` produces `petitions_taoNhiemVuVao`,
 * which HAS `source` / `source_id`, and TypeScript lets it be passed as `petitions_petitionTaskIn`
 * because it only has more. A spread here would put that pair on the wire — and the server answers 400
 * to either key (`service-petitions/internal/http/petition_task.go`, `sourceKeysRefused`): the source is
 * the petition in the PATH, never one the client names.
 *
 * `idempotencyKey` IS A PARAMETER: the form holds it for one opening, so a retry after a network error
 * reuses it — the first send may already have booked a register number.
 *
 * EVERY 409 IS SHOWN VERBATIM (`petition_state`, `petition_not_classified`, `restricted_field_no_task`,
 * `code_taken`, `task_tree`, …): the list of refusals is the server's and grows; this function does not
 * branch on `code`.
 */
export function createTaskFromPetition(
  maTraCuu: string,
  than: petitions_petitionTaskIn,
  idempotencyKey: string,
): Promise<KetQua<petitions_nhiemVuRa>> {
  const mau: petitions_post_citizen_reports_by_maTraCuu_tasks["duongDan"] =
    "/api/v1/citizen-reports/{maTraCuu}/tasks";
  const sent: petitions_petitionTaskIn = {
    code: than.code,
    auto_code: than.auto_code,
    type: than.type,
    bloc: than.bloc,
    title: than.title,
    description: than.description,
    priority: than.priority,
    note: than.note,
    unit: than.unit,
    assignee: than.assignee,
    assigner: than.assigner,
    due_at: than.due_at,
    parent: than.parent,
    documents: than.documents,
  };
  return docThanLoiGoi<petitions_nhiemVuRa>(
    goiGhi(duongDanPhieu(mau, maTraCuu), "POST", sent, 201, { "Idempotency-Key": idempotencyKey }),
  );
}

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * STAFF INTAKE — "Nhập hộ phản ánh" (§11): GET …/citizen-report-intake-fields, POST …/citizen-reports
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

/**
 * GET /api/v1/citizen-report-intake-fields — the fields the commune takes NOW, in its order, `can-bo`
 * included (ADR 0028 Bổ sung 2026-10-02 row 4). The modal's select reads THIS list and never the
 * hand-copied `LINH_VUC_PHAN_ANH`: a code the list offers is a code the write accepts, because the
 * server checks the pick against the same predicate. 503 `field_catalogue_unavailable` comes back as
 * the server's sentence; there is no built-in fallback list (ADR 0060 §3).
 */
export function listIntakeFields(): Promise<KetQua<petitions_citizenFieldListOut>> {
  const duongDan: petitions_get_citizen_report_intake_fields["duongDan"] =
    "/api/v1/citizen-report-intake-fields";
  return docJSON<petitions_citizenFieldListOut>(duongDan);
}

/**
 * What the modal collects. Strings as typed; `clockFrom` is already RFC 3339 WITH a zone, or `""` for
 * "the booking instant".
 */
export type StaffIntakeInput = {
  readonly field: string;
  readonly content: string;
  readonly address: string;
  readonly reporterName: string;
  readonly reporterPhone: string;
  readonly anonymous: boolean;
  readonly clockFrom: string;
  /**
   * ULID of the thôn / tổ dân phố the officer picked, or `""` / absent for none. The server checks it is
   * one of THIS commune's units in use (identity gRPC, ADR 0088 §1) and answers 400
   * `residential_unit_not_offered` / 503 otherwise — verbatim on the form.
   */
  readonly residentialUnitId?: string;
};

/**
 * The body of POST /api/v1/citizen-reports, BUILT FIELD BY FIELD.
 *
 * `petitions_staffIntakeIn` also lists `citizen_id`, `channel`, `code`, `status`, the two deadlines,
 * `lat`/`lng` and their Vietnamese spellings — and the server answers 400 to EVERY one of
 * them (`service-petitions/internal/http/staff_intake.go`, `notTheClients` / `notAcceptedYet`): the
 * channel is always `can-bo-nhap-ho` and the petition is linked to no citizen (ADR 0028 Bổ sung
 * 2026-10-02 rows 1, 5). None of them can be set from here, whatever the caller passes.
 *
 * Optional strings that are blank are ABSENT, not `""`; `anonymous` is sent only when ticked. Every
 * string is trimmed — the server counts lengths on the trimmed string, and a stray space must not be
 * stored as the citizen's name.
 */
export function staffIntakeBody(input: StaffIntakeInput): petitions_staffIntakeIn {
  const body: petitions_staffIntakeIn = { field: input.field, content: input.content.trim() };
  const address = input.address.trim();
  const name = input.reporterName.trim();
  const phone = input.reporterPhone.trim();
  if (address !== "") body.address = address;
  if (name !== "") body.reporter_name = name;
  if (phone !== "") body.reporter_phone = phone;
  if (input.anonymous) body.anonymous = true;
  if (input.clockFrom !== "") body.clock_from = input.clockFrom;
  if (input.residentialUnitId !== undefined && input.residentialUnitId !== "") {
    body.residential_unit_id = input.residentialUnitId;
  }
  return body;
}

/**
 * POST /api/v1/citizen-reports — book one petition on behalf of a citizen. 201 with the petition.
 *
 * THE ANSWER IS READ FOR ITS `code` ONLY. A replay of the same `Idempotency-Key` answers 201 with
 * `{code, replayed}` and nothing else (`core/idem` `PhatLai`: the first body is never stored), so the
 * type promises nothing more than the lookup code — the one thing the officer must hand over.
 *
 * `idempotencyKey` IS A PARAMETER held by the form for one opening: a retry after a network error
 * reuses it (the first send may have booked a code already); a new key only after a 201. A double
 * click therefore books ONE petition, not two that could only be soft deleted (rule 7).
 *
 * Every refusal is the server's sentence verbatim — 400 `clock_from_out_of_range`, `field_not_offered`,
 * 503 `intake_not_configured`, `field_catalogue_unavailable`, 409 `request_in_progress` — each written
 * for the officer and saying what to do. Personal data (name, phone, content) travels in the BODY only.
 */
export function bookStaffIntake(
  input: StaffIntakeInput,
  idempotencyKey: string,
): Promise<KetQua<Pick<petitions_phieuPhanAnhRa, "code">>> {
  const duongDan: petitions_post_citizen_reports["duongDan"] = "/api/v1/citizen-reports";
  const than: petitions_post_citizen_reports["than"] = staffIntakeBody(input);
  return docThanLoiGoi<Pick<petitions_phieuPhanAnhRa, "code">>(
    goiGhi(duongDan, "POST", than, 201, { "Idempotency-Key": idempotencyKey }),
  );
}

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * FILES OF A PETITION — log attachments (§8.7) and verification photos (§8.4 `Sau khi xử lý`)
 *
 * The task attachment's flow (ADR 0052, `lib/api/task-attachments.ts`): declare → POST the bytes
 * straight to the object store (`uploadToStorage`, reused) → completion. Every call returns its HTTP
 * STATUS, because the status picks what the screen offers next (422 refused for good; 503 / 409 retry
 * the completion). The sentence is always the server's.
 *
 * ⚠ The upload form and every signed `url` are BEARER CREDENTIALS: used at once, never logged, never
 * stored beyond component state, `no-store`. A file on a petition is evidence about one citizen's case.
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

const NO_ANSWER = 0;

function filePath(template: string, maTraCuu: string, id?: string): string {
  const p = duongDanPhieu(template, maTraCuu);
  return id === undefined ? p : p.replace("{id}", encodeURIComponent(id));
}

async function callWithStatus<T>(path: string, init: RequestInit, want: number): Promise<CallResult<T>> {
  let res: Response;
  try {
    res = await fetch(path, { ...CHUNG, ...init });
  } catch {
    // Nothing logged: the answer would have held a signed link or an upload form.
    return { ok: false, status: NO_ANSWER, message: LOI_KHONG_RO };
  }
  if (res.status !== want) return { ok: false, status: res.status, message: await thongBaoLoi(res) };
  try {
    return { ok: true, data: (await res.json()) as T };
  } catch {
    return { ok: false, status: res.status, message: LOI_KHONG_RO };
  }
}

/** A declaration's 201 without its form is a replay (`core/idem` stores a code, never a body). */
function withForm<T extends { upload: { url: string; fields: Record<string, string> } }>(
  r: CallResult<T>,
): CallResult<T> {
  if (r.ok && (r.data?.upload?.url === undefined || r.data.upload.fields === undefined)) {
    return { ok: false, status: 201, message: UPLOAD_FORM_MISSING };
  }
  return r;
}

function jsonPost(body: unknown, idempotencyKey: string): RequestInit {
  return {
    method: "POST",
    headers: { "Content-Type": "application/json", "Idempotency-Key": idempotencyKey },
    body: JSON.stringify(body),
  };
}

/**
 * POST …/log-attachments — declare one file for the next log entry. ONE KEY PER ATTEMPT (minted by the
 * caller): a replay cannot carry the form again. Whether THIS officer may attach to THIS petition is the
 * note rule, decided by the server (the assignee, or feedback.resolve / .assign / .classify); its 403
 * comes back verbatim. Field by field — never `...body`.
 */
export async function requestLogAttachmentUpload(
  maTraCuu: string,
  body: petitions_taskAttachmentUploadIn,
  idempotencyKey: string,
): Promise<CallResult<petitions_taskAttachmentUploadOut>> {
  const template: petitions_post_citizen_reports_by_maTraCuu_log_attachments["duongDan"] =
    "/api/v1/citizen-reports/{maTraCuu}/log-attachments";
  const sent: petitions_taskAttachmentUploadIn = {
    file_name: body.file_name,
    content_type: body.content_type,
    size: body.size,
  };
  return withForm(
    await callWithStatus<petitions_taskAttachmentUploadOut>(
      filePath(template, maTraCuu),
      jsonPost(sent, idempotencyKey),
      201,
    ),
  );
}

/** POST …/log-attachments/{id}/completion — sniff, scan, store. Safe to repeat. */
export function completeLogAttachment(
  maTraCuu: string,
  id: string,
): Promise<CallResult<petitions_taskAttachmentOut>> {
  const template: petitions_post_citizen_reports_by_maTraCuu_log_attachments_by_id_completion["duongDan"] =
    "/api/v1/citizen-reports/{maTraCuu}/log-attachments/{id}/completion";
  return callWithStatus<petitions_taskAttachmentOut>(filePath(template, maTraCuu, id), { method: "POST" }, 200);
}

/**
 * DELETE …/log-attachments/{id} — soft-removes one file from the petition's log, reason REQUIRED (ADR
 * 0088 §2, rule 7: the file stays, with who and why). 204, no body. Who may is decided on the server —
 * the uploader, or `feedback.resolve` — and its refusal (403, 409 `legal_hold`) comes back verbatim.
 * The reason travels in the BODY only.
 */
export function removeLogAttachment(maTraCuu: string, id: string, reason: string): Promise<KetQua<void>> {
  const template: petitions_delete_citizen_reports_by_maTraCuu_log_attachments_by_id["duongDan"] =
    "/api/v1/citizen-reports/{maTraCuu}/log-attachments/{id}";
  const body: petitions_citizenReportLogAttachmentRemoveIn = { reason: reason.trim() };
  return goiGhi(filePath(template, maTraCuu, id), "DELETE", body, 204).then((r) =>
    r.ok ? ({ ok: true, duLieu: undefined } as KetQua<void>) : r,
  );
}

/**
 * GET …/log-attachments/{id}/download — a link of at most 15 minutes. AUDITED by the server (a file on
 * a petition's log is evidence about a citizen's case), so it is asked for AT THE CLICK, never
 * prefetched, and never kept.
 */
export function logAttachmentDownloadLink(
  maTraCuu: string,
  id: string,
): Promise<CallResult<petitions_taskAttachmentDownloadOut>> {
  const template: petitions_get_citizen_reports_by_maTraCuu_log_attachments_by_id_download["duongDan"] =
    "/api/v1/citizen-reports/{maTraCuu}/log-attachments/{id}/download";
  return callWithStatus<petitions_taskAttachmentDownloadOut>(
    filePath(template, maTraCuu, id),
    { method: "GET" },
    200,
  );
}

/**
 * GET …/verification-photos — the "after" photos staff uploaded, each with a signed link ≤ 15 minutes.
 * AUDITED per call (a photograph cannot be masked): called when the column is shown, after an upload,
 * and when a link has expired — NEVER on a timer. 503 keeps its status for the column's own sentence.
 */
export function listVerificationPhotos(maTraCuu: string): Promise<CallResult<petitions_photoListOut>> {
  const template: petitions_get_citizen_reports_by_maTraCuu_verification_photos["duongDan"] =
    "/api/v1/citizen-reports/{maTraCuu}/verification-photos";
  return callWithStatus<petitions_photoListOut>(filePath(template, maTraCuu), { method: "GET" }, 200);
}

/**
 * POST …/verification-photos — declare ONE photo (`feedback.resolve`). ONE KEY PER ATTEMPT. 409
 * `petition_state` (the petition ended) and `photo_limit` (five per processing round) are the server's
 * sentences verbatim; the screen counts nothing.
 */
export async function requestVerificationPhotoUpload(
  maTraCuu: string,
  body: petitions_photoUploadIn,
  idempotencyKey: string,
): Promise<CallResult<petitions_photoUploadOut>> {
  const template: petitions_post_citizen_reports_by_maTraCuu_verification_photos["duongDan"] =
    "/api/v1/citizen-reports/{maTraCuu}/verification-photos";
  const sent: petitions_photoUploadIn = { content_type: body.content_type, size: body.size };
  return withForm(
    await callWithStatus<petitions_photoUploadOut>(
      filePath(template, maTraCuu),
      jsonPost(sent, idempotencyKey),
      201,
    ),
  );
}

/** POST …/verification-photos/{id}/completion — scan, re-encode without EXIF, store. Safe to repeat. */
export function completeVerificationPhoto(
  maTraCuu: string,
  id: string,
): Promise<CallResult<petitions_photoOut>> {
  const template: petitions_post_citizen_reports_by_maTraCuu_verification_photos_by_id_completion["duongDan"] =
    "/api/v1/citizen-reports/{maTraCuu}/verification-photos/{id}/completion";
  return callWithStatus<petitions_photoOut>(filePath(template, maTraCuu, id), { method: "POST" }, 200);
}
