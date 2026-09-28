/**
 * Tám tuyến của sổ Nhiệm vụ (`docs/ui-ux/02-nhiem-vu.md`), đúng bộ tuyến
 * `service-petitions/internal/http/routes.go:773-982` khai — không nhiều hơn, không ít hơn.
 *
 *   GET    /api/v1/tasks                                       task.read
 *   GET    /api/v1/tasks/{ma}                                  task.read
 *   POST   /api/v1/tasks                                       task.create  + Idempotency-Key
 *   PATCH  /api/v1/tasks/{ma}                                  task.update
 *   POST   /api/v1/tasks/{ma}/status                           task.update  (+ task.approve ở tầng
 *                                                              nghiệp vụ cho bước `hoan-thanh`)
 *   POST   /api/v1/tasks/{ma}/assignment                       task.assign  (§5.7 — giao lại /
 *                                                              chuyển tiếp, quyết định 28/09/2026)
 *   DELETE /api/v1/tasks/{ma}                                  task.delete
 *   POST   /api/v1/tasks/{ma}/extensions                       task.update  ← KHÔNG phải task.extend
 *   POST   /api/v1/tasks/{ma}/extensions/{deNghiID}/decision   task.extend  + ADR 0038 lớp hai
 *   GET    /api/v1/task-extensions                             task.read    (hàng chờ duyệt §5.8,
 *                                                              đến sau tám tuyến trên; `task=` lọc
 *                                                              theo một nhiệm vụ)
 *   GET    /api/v1/task-counts                                 task.read    (số thật đầu cột Kanban)
 *   POST   /api/v1/tasks/{ma}/log-entries                      task.read    + Idempotency-Key (ô ghi tay
 *                                                              §5.9; ai ghi được là việc của dòng)
 *
 * HAI DÒNG CUỐI KHÔNG ĐƯỢC GỘP, và đó là toàn bộ ADR 0038: `task.extend` nhãn là **"Duyệt gia
 * hạn"** — quyền QUYẾT ĐỊNH. Gắn nó lên tuyến ĐỀ NGHỊ sẽ thành "chỉ người duyệt được mới xin
 * được", đúng điều ngược lại với §5.8, nơi ô đề nghị nằm trên drawer của người đang làm việc.
 *
 * KIỂU LẤY TỪ HỢP ĐỒNG, KHÔNG GÕ TAY: `petitions_nhiemVuRa`, `petitions_taoNhiemVuVao`,
 * `petitions_suaNhiemVuVao`, `petitions_doiTrangThaiVao`, `petitions_xoaNhiemVuVao`,
 * `petitions_deNghiLuiHanVao`, `petitions_quyetDinhLuiHanVao`, `petitions_deNghiLuiHanRa`,
 * `page_Result_petitions_nhiemVuRa`, `petitions_taskAssignmentIn` đều đến từ `schema.gen.ts`.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * THAM SỐ TRUY VẤN CỦA TUYẾN DANH SÁCH NAY ĐÃ ĐƯỢC HỢP ĐỒNG KHAI: `petitions_get_tasks["truyVan"]`
 * (`schema.gen.ts`) có `status` · `source` · `type` · `bloc` · `priority` · `unit` · `assignee` ·
 * `q` · `late` · `scope` · `soon` cùng `limit` · `cursor` · `sort` · `order`. Khối này từng viết
 * rằng đối tượng ấy RỖNG — đúng lúc viết, hết đúng khi tuyến có chú thích `@query`.
 *
 * ⚠ ĐIỀU VẪN CÒN ĐÚNG: `themLocVaoTruyVan` ghép tên tham số bằng CHUỖI TRẦN vào `URLSearchParams`,
 * nên `tsc` CHƯA đối chiếu những cái tên ấy với kiểu `truyVan` — gõ sai một tên thì máy chủ bỏ
 * qua nó và trả cả quyển sổ trong khi cán bộ tin mình đang xem một lát cắt. Vì thế mười cái tên
 * nằm trong ĐÚNG MỘT hàm và có bài kiểm đọc lại từng tên; nối chúng vào kiểu `truyVan` là việc
 * nên làm tiếp, ở đúng hàm ấy. `sort` và `order` (thêm 27/09/2026) đã đi qua `thamSoTheoHopDong`.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 *
 * ĐỔI CHIỀU CÓ CHỦ Ý 28/09/2026 (W5, máy chủ 3a4e60f): `scope=related` và `soon=true` NAY ĐƯỢC PHỤC
 * VỤ. Máy chủ tự hỏi `identity` — bộ phận của người đăng nhập (từ PHIÊN), ngưỡng sắp đến hạn của XÃ
 * — và không bao giờ bỏ lọc khi hỏi hỏng: 409 `due_soon_not_configured` khi xã chưa đặt ngưỡng, 503
 * `task_filter_unavailable` khi không hỏi được. Câu của máy chủ ra NGUYÊN VĂN ở chỗ đáng lẽ là trang.
 */

import { isPeriodMetric, type TaskMetric } from "@/lib/drill-down";

import {
  CHUNG,
  LOI_KHONG_RO,
  docJSON,
  docThanLoiGoi,
  goiGhi,
  thamSoTheoHopDong,
  thongBaoLoi,
  type KetQua,
} from "./goi";
import type {
  page_Result_petitions_deNghiChoDuyetRa,
  page_Result_petitions_nhatKyNhiemVuRa,
  page_Result_petitions_nhiemVuRa,
  petitions_deNghiLuiHanRa,
  petitions_deNghiLuiHanVao,
  petitions_delete_tasks_by_ma,
  petitions_doiTrangThaiVao,
  petitions_get_task_counts,
  petitions_get_task_extensions,
  petitions_get_tasks,
  petitions_get_tasks_by_ma,
  petitions_get_tasks_by_ma_log_entries,
  petitions_get_tasks_register_export,
  petitions_nhatKyNhiemVuRa,
  petitions_nhiemVuRa,
  petitions_patch_tasks_by_ma,
  petitions_post_tasks,
  petitions_post_tasks_by_ma_extensions,
  petitions_post_tasks_by_ma_extensions_by_deNghiID_decision,
  petitions_post_tasks_by_ma_assignment,
  petitions_post_tasks_by_ma_log_entries,
  petitions_post_tasks_by_ma_status,
  petitions_quyetDinhLuiHanVao,
  petitions_suaNhiemVuVao,
  petitions_taoNhiemVuVao,
  petitions_taskAssignmentIn,
  petitions_taskCountsOut,
  petitions_taskLogEntryIn,
  petitions_vanBanNhiemVuVao,
  petitions_xoaNhiemVuVao,
} from "./schema.gen";

/**
 * Điền mã nhiệm vụ vào khuôn đường dẫn CỦA HỢP ĐỒNG.
 *
 * Nhận khuôn (`/api/v1/tasks/{ma}/status`) chứ không tự ghép chuỗi: khuôn ấy lấy từ kiểu sinh ra,
 * nên đổi đường dẫn ở máy chủ là `tsc` đỏ tại chỗ gọi thay vì 404 lúc chạy.
 *
 * `encodeURIComponent` vì `ma` là **mã sổ của xã** (`NV19`) do người gõ — §7.1 cho phép tự nhập —
 * nên một dấu `/` hay một khoảng trắng lọt vào sẽ đổi hẳn tuyến được gọi.
 */
function duongDanNhiemVu(mau: string, ma: string): string {
  return mau.replace("{ma}", encodeURIComponent(ma));
}

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * ĐỌC
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

/**
 * Bộ lọc của sổ nhiệm vụ — ĐÚNG mười tham số `locNhiemVuTuQuery` đọc, cộng sắp xếp và phân trang.
 *
 * `related` và `dueSoon` có từ W5 — xem khối đầu tệp.
 *
 * KHÔNG CÓ `che_do_xem`: Kanban / Danh sách / Sổ theo dõi chọn CÁCH BÀY cùng một tập dòng, nên
 * máy chủ cố ý không đọc nó (`nhiem_vu.go:409`). Gửi lên là dựng một tham số không ai đọc.
 */
export type LocNhiemVu = {
  /**
   * §3 tab Phạm vi. `mine` = "Giao cho tôi", và MÁY CHỦ tự điền mã cán bộ từ PHIÊN —
   * `?scope=mine` **không** mang theo danh tính nào (`nhiem_vu.go:273-288`). Đó là điều kiện để
   * nó không phải một lời client tự khai mình là ai (luật 1, cấm #2).
   */
  phamVi?: "all" | "mine" | "related";
  /**
   * §3 `☐ Sắp đến hạn` — `soon=true`. The THRESHOLD is the commune's (identity, working hours); the
   * screen sends only the switch and never a number — `72 giờ` in the spec is a default the commune
   * overrides (rule 10, forbidden #3).
   */
  dueSoon?: boolean;
  /** Một trong bảy mã trạng thái §6. Sai mã là **400**, không phải bị bỏ qua. */
  trangThai?: string;
  /** Một trong bốn mã nguồn giao §3. Sai mã là **400**. */
  nguonGiao?: string;
  /** Mã danh mục `loai_nhiem_vu` (`code`, không phải `id`). KHÔNG kiểm ở máy chủ: mã lạ ⇒ trang rỗng. */
  loai?: string;
  /** Mã danh mục khối của `identity` (`code`). Không kiểm ở máy chủ. */
  khoi?: string;
  /** Mã danh mục `muc_uu_tien_nhiem_vu` (`code`). Không kiểm ở máy chủ. */
  mucUuTien?: string;
  /** ULID bộ phận thực hiện. */
  boPhanID?: string;
  /**
   * MÃ NGHIỆP VỤ CÁN BỘ (`CB-…`), không phải ULID — cột `nguoi_thuc_hien_ma` giữ mã cán bộ
   * (luật 6, bất biến 8). Một ULID ở đây trả về trang rỗng và không có gì đỏ ở đâu cả.
   */
  nguoiThucHienMa?: string;
  /** Tìm trong mã + tiêu đề. Tối đa 200 ký tự, dài hơn là 400. */
  tim?: string;
  /** Chỉ việc đã quá hạn. Máy chủ chỉ nhận đúng chuỗi `true`. */
  chiTreHan?: boolean;
  /**
   * Cột sắp xếp — HAI giá trị, đúng danh sách trắng `SapXepNhiemVu`
   * (`service-petitions/internal/store/nhiem_vu.go:92-95`). Kiểu lấy từ hợp đồng, nên máy chủ thêm
   * hay bớt một cột là `tsc` đỏ ở đây. Vắng mặt = mặc định của máy chủ (`created_at`).
   */
  sapXep?: CotSapXepNhiemVu;
  /** Chiều sắp xếp. Vắng mặt = mặc định của máy chủ, GIẢM DẦN cho mọi cột (`page.Desc`). */
  chieu?: ChieuSapXepNhiemVu;
  limit?: number;
  /**
   * ⚠ CON TRỎ THUỘC VỀ ĐÚNG MỘT CÁCH SẮP XẾP. Máy chủ ghi `sort`/`order` vào trong con trỏ và TỪ CHỐI
   * (400 `invalid_cursor`) một con trỏ gửi kèm cách sắp khác (`core/page/page.go:456-457`). Nên đổi
   * `sapXep` hay `chieu` mà giữ con trỏ cũ là một trang lỗi — bên gọi phải về trang đầu.
   */
  cursor?: string | null;
  /**
   * Lọc theo MỘT số liệu của trang Tổng quan (SRS M7.2.2) — tập dòng đứng sau con số ấy, cùng một
   * vị từ SQL với phép đếm (`service-petitions/internal/domain/summary_metrics.go:3-11`).
   */
  metric?: TaskMetric;
  /** Kỳ nửa mở [from, to), RFC3339. CHỈ gửi với số liệu theo kỳ — xem `themLocVaoTruyVan`. */
  from?: string;
  to?: string;
  /**
   * Chỉ những việc con TRỰC TIẾP còn sống của một việc cha — MÃ SỔ của cha (`NV19`), không phải id
   * nội bộ: hợp đồng không phát id, và `nhiemVuRa.parent` giữ đúng loại giá trị này.
   */
  parent?: string;
  /**
   * `include=documents` (90d12ff) — the three §5.4 document lists on EVERY row, for the Sổ theo dõi
   * view (§4.3) only. A PROJECTION, not a filter: it never goes to `/task-counts` (`appendTaskFilters`
   * does not know it), and Kanban / Danh sách never ask for it — payload two of three views ignore.
   */
  includeDocuments?: boolean;
};

/** Ba cột máy chủ cho sắp — `petitions_get_tasks["truyVan"]["sort"]`, không gõ tay. */
export type CotSapXepNhiemVu = NonNullable<petitions_get_tasks["truyVan"]["sort"]>;
/** `asc` · `desc` — `petitions_get_tasks["truyVan"]["order"]`. */
export type ChieuSapXepNhiemVu = NonNullable<petitions_get_tasks["truyVan"]["order"]>;

/**
 * Gom TÊN tham số truy vấn về đúng một chỗ.
 *
 * Mười cái tên dưới đây là mười chuỗi KHÔNG có kiểu nào của hợp đồng canh giúp (xem khối chú
 * thích đầu tệp). Gõ sai một cái thì máy chủ bỏ qua nó và trả về cả quyển sổ, và màn hình trông
 * hoàn toàn bình thường — nên chúng đứng một chỗ và có bài kiểm đọc lại từng tên.
 */
function themLocVaoTruyVan(truyVan: URLSearchParams, loc: LocNhiemVu): void {
  appendTaskFilters(truyVan, loc);

  // SẮP XẾP ĐI QUA `thamSoTheoHopDong`, không ghép chuỗi trần: hai tên này mới thêm, nên chúng là
  // hai tên đầu tiên của hàm này được `tsc` đối chiếu với kiểu `truyVan` — cả tên lẫn giá trị.
  const dat = thamSoTheoHopDong<petitions_get_tasks["truyVan"]>(truyVan);
  dat("sort", loc.sapXep);
  dat("order", loc.chieu);
  // The ONE accepted spelling; anything else is a 400, never silently ignored (`nhiem_vu.go:540-550`).
  if (loc.includeDocuments === true) dat("include", "documents");

  if (loc.limit !== undefined) truyVan.set("limit", String(loc.limit));
  // Con trỏ rỗng nghĩa là trang đầu. Gửi `cursor=` rỗng thì máy chủ trả 400 "con trỏ không hợp
  // lệ" — đúng vào lần mở màn hình đầu tiên.
  if (loc.cursor !== undefined && loc.cursor !== null && loc.cursor !== "") {
    truyVan.set("cursor", loc.cursor);
  }
}

/**
 * The FILTER half of a task query — everything that decides WHICH rows, nothing that decides their
 * order or page. Shared by the register (`themLocVaoTruyVan`) and the Kanban counts
 * (`taskCountsPath`) on purpose: a column header counting under different filters than the cards
 * beneath it is a number that silently disagrees with the board, and it is the number that gets
 * read out to leadership. One builder means the two cannot drift.
 */
function appendTaskFilters(truyVan: URLSearchParams, loc: LocNhiemVu): void {
  // `scope=all` là mặc định của máy chủ và không cần predicate nào, nên tab "Toàn xã" gửi tham
  // số vắng mặt hẳn — ít một tham số là ít một chỗ có thể gõ sai.
  if (loc.phamVi === "mine") truyVan.set("scope", "mine");
  // `related` carries NO identity either: the server reads the caller's code from the session and
  // asks identity for their units (`nhiem_vu.go:602-611`). It narrows; it never grants.
  if (loc.phamVi === "related") truyVan.set("scope", "related");

  if (loc.trangThai !== undefined && loc.trangThai !== "") truyVan.set("status", loc.trangThai);
  if (loc.nguonGiao !== undefined && loc.nguonGiao !== "") truyVan.set("source", loc.nguonGiao);
  if (loc.loai !== undefined && loc.loai !== "") truyVan.set("type", loc.loai);
  if (loc.khoi !== undefined && loc.khoi !== "") truyVan.set("bloc", loc.khoi);
  if (loc.mucUuTien !== undefined && loc.mucUuTien !== "") truyVan.set("priority", loc.mucUuTien);
  if (loc.boPhanID !== undefined && loc.boPhanID !== "") truyVan.set("unit", loc.boPhanID);
  if (loc.nguoiThucHienMa !== undefined && loc.nguoiThucHienMa !== "") {
    truyVan.set("assignee", loc.nguoiThucHienMa);
  }
  if (loc.tim !== undefined && loc.tim !== "") truyVan.set("q", loc.tim);

  // `late` CHỈ NHẬN ĐÚNG CHUỖI `true`. Gửi `false` là **400**, không phải "không lọc" — nên ô
  // chưa tích thì tham số vắng mặt hẳn (`errLocTreHanNhiemVuKhongHopLe`).
  if (loc.chiTreHan === true) truyVan.set("late", "true");
  // Same rule as `late`: ONLY `true` is accepted, an unticked box sends nothing (`nhiem_vu.go:718-726`).
  if (loc.dueSoon === true) truyVan.set("soon", "true");

  // `petitions_get_task_counts["truyVan"]` carries the same filter names; typing against the
  // counts route checks the names against BOTH routes' shared subset, not only the list route.
  const dat = thamSoTheoHopDong<petitions_get_task_counts["truyVan"]>(truyVan);
  dat("parent", loc.parent);

  // SỐ LIỆU CỦA TỔNG QUAN. Kỳ đi lên CHỈ khi số liệu đếm theo kỳ: máy chủ bỏ qua kỳ ở số liệu tồn
  // (`summary.go:182-185`), nên gửi nó là gửi một câu hỏi máy chủ không trả lời — và `from`/`to` mà
  // không có `metric` thì càng không nghĩa gì.
  dat("metric", loc.metric);
  if (loc.metric !== undefined && isPeriodMetric("tasks", loc.metric)) {
    dat("from", loc.from);
    dat("to", loc.to);
  }
}

/** Dựng đường dẫn đọc sổ. Tách khỏi lời gọi mạng để kiểm được mà không cần thay `fetch`. */
export function duongDanSoNhiemVu(loc: LocNhiemVu = {}): string {
  const duongDan: petitions_get_tasks["duongDan"] = "/api/v1/tasks";
  const truyVan = new URLSearchParams();
  themLocVaoTruyVan(truyVan, loc);
  const chuoi = truyVan.toString();
  return chuoi === "" ? duongDan : `${duongDan}?${chuoi}`;
}

/** GET /api/v1/tasks — một trang của sổ nhiệm vụ. */
export function laySoNhiemVu(
  loc: LocNhiemVu = {},
): Promise<KetQua<page_Result_petitions_nhiemVuRa>> {
  return docJSON<page_Result_petitions_nhiemVuRa>(duongDanSoNhiemVu(loc));
}

/**
 * Path of `GET /api/v1/task-counts` — the real per-status totals behind the Kanban headers (#15).
 *
 * FILTERS ONLY, through `appendTaskFilters` — the same builder as the list. `sort`, `order`,
 * `limit` and `cursor` are dropped even when the caller passes them: the counts route declares none
 * of them, and a total does not depend on order or page.
 */
export function taskCountsPath(loc: LocNhiemVu = {}): string {
  const duongDan: petitions_get_task_counts["duongDan"] = "/api/v1/task-counts";
  const truyVan = new URLSearchParams();
  appendTaskFilters(truyVan, loc);
  const chuoi = truyVan.toString();
  return chuoi === "" ? duongDan : `${duongDan}?${chuoi}`;
}

/**
 * Path of `GET /api/v1/tasks/register-export` — the Sổ theo dõi as .xlsx (backend P10).
 *
 * THE LIST'S FILTERS (`appendTaskFilters`, the same builder) AND ITS SORT — the file is exactly the
 * register on screen, in §4.3's column order, which the server writes. NO `limit`, `cursor`,
 * `include`: the export is every matching row (refused above 5 000 with 422, never truncated), and it
 * always carries the documents.
 */
export function registerExportPath(loc: LocNhiemVu = {}): string {
  const duongDan: petitions_get_tasks_register_export["duongDan"] = "/api/v1/tasks/register-export";
  const truyVan = new URLSearchParams();
  appendTaskFilters(truyVan, loc);
  const dat = thamSoTheoHopDong<petitions_get_tasks_register_export["truyVan"]>(truyVan);
  dat("sort", loc.sapXep);
  dat("order", loc.chieu);
  const chuoi = truyVan.toString();
  return chuoi === "" ? duongDan : `${duongDan}?${chuoi}`;
}

/** The file of one export: bytes and the name to save them under. */
export type RegisterFile = { readonly blob: Blob; readonly fileName: string };

/** Used when the reply names no file — never a name carrying the commune or a search term. */
export const REGISTER_EXPORT_FALLBACK_NAME = "so-theo-doi-nhiem-vu.xlsx";

/**
 * `filename="…"` / `filename*=UTF-8''…` of a Content-Disposition, or the fallback. A path component
 * in the name is dropped: the browser decides where to save, not the header.
 */
export function registerFileName(header: string | null): string {
  if (header === null) return REGISTER_EXPORT_FALLBACK_NAME;
  const star = /filename\*\s*=\s*UTF-8''([^;]+)/i.exec(header);
  let name: string | undefined;
  if (star?.[1] !== undefined) {
    try {
      name = decodeURIComponent(star[1].trim());
    } catch {
      name = undefined;
    }
  }
  if (name === undefined) name = /filename\s*=\s*"?([^";]+)"?/i.exec(header)?.[1]?.trim();
  const base = name?.split(/[\\/]/).pop()?.trim();
  return base === undefined || base === "" ? REGISTER_EXPORT_FALLBACK_NAME : base;
}

/**
 * GET /api/v1/tasks/register-export — `task.read`. Every export is audited by the server BEFORE the
 * file is sent (who, the filters, the row count); there is no idempotency key — a second click is a
 * second export, which is the truth.
 *
 * READ AS A BLOB, NOT NAVIGATED TO: a refusal — 422 `register_export_too_large`, 503
 * `register_names_unavailable` / `task_filter_unavailable`, 409 `due_soon_not_configured` — must come
 * back as the server's sentence on this screen, and a navigation would put the JSON error in a tab.
 */
export async function downloadTaskRegister(loc: LocNhiemVu = {}): Promise<KetQua<RegisterFile>> {
  let phanHoi: Response;
  try {
    phanHoi = await fetch(registerExportPath(loc), { ...CHUNG, method: "GET" });
  } catch {
    return { ok: false, thongBao: LOI_KHONG_RO };
  }
  if (phanHoi.status !== 200) return { ok: false, thongBao: await thongBaoLoi(phanHoi) };
  try {
    const blob = await phanHoi.blob();
    return { ok: true, duLieu: { blob, fileName: registerFileName(phanHoi.headers.get("Content-Disposition")) } };
  } catch {
    return { ok: false, thongBao: LOI_KHONG_RO };
  }
}

/** GET /api/v1/task-counts — `task.read`, all seven status codes, same filters as the list. */
export function getTaskCounts(loc: LocNhiemVu = {}): Promise<KetQua<petitions_taskCountsOut>> {
  return docJSON<petitions_taskCountsOut>(taskCountsPath(loc));
}

/**
 * GET /api/v1/tasks/{ma}.
 *
 * MỘT MÃ 404 DUY NHẤT CHO BA TÌNH HUỐNG, và giao diện không được dựng lại sự phân biệt ấy: mã
 * không tồn tại · mã của xã khác · nhiệm vụ đã xoá mềm. Phân biệt được chúng là nói cho người
 * đang thử mã biết những số nào đang tồn tại trong một quyển sổ họ không đọc được.
 */
export function layNhiemVu(ma: string): Promise<KetQua<petitions_nhiemVuRa>> {
  const mau: petitions_get_tasks_by_ma["duongDan"] = "/api/v1/tasks/{ma}";
  return docJSON<petitions_nhiemVuRa>(duongDanNhiemVu(mau, ma));
}

/**
 * Dựng đường dẫn đọc NHẬT KÝ & TRAO ĐỔI (§5.9) của một nhiệm vụ. Tách khỏi lời gọi mạng để kiểm
 * được mà không thay `fetch`.
 *
 * TÊN THAM SỐ ĐI QUA `thamSoTheoHopDong<…["truyVan"]>`, không ghép chuỗi trần như
 * `themLocVaoTruyVan`: máy chủ đổi tên `limit`/`cursor` là `tsc` đỏ ở đây. `order` không gửi —
 * mặc định của máy chủ đã là mới nhất trước, đúng thứ tự §5.9 vẽ.
 */
export function duongDanNhatKyNhiemVu(
  ma: string,
  trang: { limit?: number; cursor?: string | null } = {},
): string {
  const mau: petitions_get_tasks_by_ma_log_entries["duongDan"] = "/api/v1/tasks/{ma}/log-entries";
  const truyVan = new URLSearchParams();
  const dat = thamSoTheoHopDong<petitions_get_tasks_by_ma_log_entries["truyVan"]>(truyVan);
  dat("limit", trang.limit);
  // Con trỏ rỗng/`null` = trang đầu, và KHÔNG gửi `cursor=` rỗng (máy chủ trả 400).
  dat("cursor", trang.cursor);
  const chuoi = truyVan.toString();
  const duongDan = duongDanNhiemVu(mau, ma);
  return chuoi === "" ? duongDan : `${duongDan}?${chuoi}`;
}

/**
 * GET /api/v1/tasks/{ma}/log-entries — một trang nhật ký, mới nhất trước. `task.read`.
 *
 * 404 MỘT CÂU CHO MỌI CA ("Không tìm thấy nhiệm vụ."), cùng lý do `layNhiemVu`: máy chủ đọc nhiệm
 * vụ qua đúng bộ đọc của tuyến chi tiết trước khi chạm tới nhật ký.
 */
export function layNhatKyNhiemVu(
  ma: string,
  conTro?: string | null,
  limit?: number,
): Promise<KetQua<page_Result_petitions_nhatKyNhiemVuRa>> {
  return docJSON<page_Result_petitions_nhatKyNhiemVuRa>(
    duongDanNhatKyNhiemVu(ma, { limit, cursor: conTro }),
  );
}

/**
 * Bộ lọc của hàng chờ duyệt lùi hạn.
 *
 * `approver` CHỈ CÓ HAI TRẠNG THÁI: vắng mặt (mọi đề nghị đang chờ của xã) hoặc đúng chuỗi `me`.
 * `me` KHÔNG mang danh tính nào — máy chủ tự điền mã cán bộ từ PHIÊN; mọi giá trị khác là 400. Kiểu
 * hẹp hơn kiểu hợp đồng (`string`) có chủ ý: một mã `CB-…` gửi vào đây là client tự khai mình là ai
 * (luật 1, cấm #2), và `tsc` chặn nó trước khi máy chủ phải chặn.
 */
export type LocHangChoLuiHan = {
  approver?: "me";
  /** Register code (`NV19`): only that task's pending requests — the drawer's block (#11). */
  task?: string;
  cursor?: string | null;
  limit?: number;
};

/**
 * Dựng đường dẫn đọc hàng chờ duyệt lùi hạn. Tách khỏi lời gọi mạng để kiểm được mà không thay
 * `fetch`.
 *
 * Tên tham số đi qua `thamSoTheoHopDong`, nên máy chủ đổi tên là `tsc` đỏ ở đây. `sort`/`order`
 * KHÔNG gửi: mặc định của máy chủ đã là cũ nhất trước — đề nghị chờ lâu nhất lên đầu.
 */
export function duongDanHangChoLuiHan(loc: LocHangChoLuiHan = {}): string {
  const duongDan: petitions_get_task_extensions["duongDan"] = "/api/v1/task-extensions";
  const truyVan = new URLSearchParams();
  const dat = thamSoTheoHopDong<petitions_get_task_extensions["truyVan"]>(truyVan);
  dat("approver", loc.approver);
  dat("task", loc.task);
  dat("limit", loc.limit);
  // Con trỏ rỗng/`null` = trang đầu, và KHÔNG gửi `cursor=` rỗng (máy chủ trả 400).
  dat("cursor", loc.cursor);
  const chuoi = truyVan.toString();
  return chuoi === "" ? duongDan : `${duongDan}?${chuoi}`;
}

/**
 * GET /api/v1/task-extensions — một trang đề nghị lùi hạn ĐANG CHỜ của xã. `task.read`.
 *
 * `task=NV19` (ad7f821) lọc theo MỘT nhiệm vụ — lối của drawer (`task-extension-block.tsx`), thay
 * cho việc lật cả hàng chờ của xã để tìm một dòng.
 */
export function layHangChoLuiHan(
  loc: LocHangChoLuiHan = {},
): Promise<KetQua<page_Result_petitions_deNghiChoDuyetRa>> {
  return docJSON<page_Result_petitions_deNghiChoDuyetRa>(duongDanHangChoLuiHan(loc));
}

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * SÁU THAO TÁC GHI
 *
 * 409 LÀ CÂU TRẢ LỜI BÌNH THƯỜNG CỦA NĂM TRONG SÁU TUYẾN, không phải sự cố. Hai lần từ chối nặng
 * nhất mang toàn bộ thông tin TRONG CÂU CHỮ, không trong mã lỗi:
 *
 *   hoàn thành cha còn con   "còn 3 việc con (NV20, NV21, NV22) — hoàn thành hết việc con rồi mới
 *                            hoàn thành việc cha"
 *   xoá cha còn con          "còn 3 việc con chưa xoá — xử lý hoặc xoá các việc con trước"
 *
 * DANH SÁCH MÃ VÀ CON SỐ LÀ TOÀN BỘ PHẦN CÓ ÍCH. Nuốt chúng thành "có lỗi xảy ra" để lại cho cán
 * bộ đúng thông tin bằng không: họ biết mình không được phép, và không biết còn vướng ở đâu. Nên
 * không hàm nào dưới đây viết lại một câu từ chối; `thongBao` của `goi.ts` đã mang nguyên văn
 * `message` máy chủ viết (luật 9, cấm #2).
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

/**
 * POST /api/v1/tasks — §7 "Giao việc mới". 201, trả về nhiệm vụ vừa tạo.
 *
 * `khoaChongTrung` LÀ THAM SỐ, KHÔNG SINH TẠI CHỖ. Hợp đồng đòi `Idempotency-Key` bắt buộc. Sinh
 * khoá bên trong hàm thì mỗi lần bấm lại sau một lỗi mạng là một khoá mới — tức đúng cái khoá
 * chống trùng sinh ra để chặn, vì lần gửi đầu CÓ THỂ đã tới máy chủ và đã cấp một số sổ. Khoá do
 * biểu mẫu giữ, sống bằng đời một lần mở form, và lần bấm lại dùng lại chính nó.
 *
 * DỰNG TỪNG TRƯỜNG, KHÔNG `...than`. Một phép trải ở đây là đường để một trường lạ — `status`,
 * `original_due_at`, `created_by` — đi lên máy chủ vào ngày ai đó truyền vào một dòng vừa đọc
 * được. Cả ba đều là những thứ hợp đồng CỐ Ý không nhận.
 */
export function taoNhiemVu(
  than: petitions_taoNhiemVuVao,
  khoaChongTrung: string,
): Promise<KetQua<petitions_nhiemVuRa>> {
  const duongDan: petitions_post_tasks["duongDan"] = "/api/v1/tasks";

  const thanGui: petitions_taoNhiemVuVao = {
    auto_code: than.auto_code,
    type: than.type,
    title: than.title,
  };
  // Trường tuỳ chọn chỉ có mặt khi có giá trị: gửi chuỗi rỗng là gửi một lựa chọn, còn vắng mặt
  // là "không khai" — hai điều khác nhau ở một cột nullable.
  if (than.code !== undefined && than.code !== "") thanGui.code = than.code;
  if (than.bloc !== undefined && than.bloc !== "") thanGui.bloc = than.bloc;
  if (than.description !== undefined && than.description !== "") {
    thanGui.description = than.description;
  }
  if (than.priority !== undefined && than.priority !== "") thanGui.priority = than.priority;
  if (than.source !== undefined && than.source !== "") thanGui.source = than.source;
  if (than.source_id !== undefined && than.source_id !== "") thanGui.source_id = than.source_id;
  if (than.unit !== undefined && than.unit !== "") thanGui.unit = than.unit;
  if (than.assignee !== undefined && than.assignee !== "") thanGui.assignee = than.assignee;
  if (than.assigner !== undefined && than.assigner !== "") thanGui.assigner = than.assigner;
  if (than.lead_unit !== undefined && than.lead_unit !== "") thanGui.lead_unit = than.lead_unit;
  if (than.monitor !== undefined && than.monitor !== "") thanGui.monitor = than.monitor;
  // `due_at` LÀ MỘT LẦN DUY NHẤT TRONG ĐỜI NHIỆM VỤ: `han_ban_dau` lấy cùng mốc và trigger
  // `nhiem_vu_bat_bien` từ chối mọi lần ghi sau. Việc tạo mà bỏ trống hạn thì KHÔNG bao giờ đặt
  // được hạn nữa — hệ quả có thật, và nó ra tới màn hình chứ không nằm ở đây.
  if (than.due_at !== undefined && than.due_at !== null && than.due_at !== "") {
    thanGui.due_at = than.due_at;
  }
  if (than.parent !== undefined && than.parent !== "") thanGui.parent = than.parent;
  // BA NHÓM VĂN BẢN §7.2 — cũng dựng từng trường, từng dòng. Chỉ bốn trường hợp đồng nhận cho một
  // dòng MỚI: không `id` (lúc tạo mọi dòng đều mới; một `id` gửi kèm là một dòng máy chủ không
  // tìm thấy), không `position` (số thứ tự do sổ cấp). Mảng rỗng thì VẮNG MẶT: lúc tạo, rỗng và vắng cùng nghĩa, và vắng thì thân
  // của loại `Nhiệm vụ cơ bản` không mang một khoá thuộc §7.2.
  if (than.documents !== undefined && than.documents.length > 0) {
    thanGui.documents = than.documents.map((d) => {
      const dong: petitions_vanBanNhiemVuVao = { group: d.group, summary: d.summary };
      if (d.reference !== undefined && d.reference !== "") dong.reference = d.reference;
      if (d.date !== undefined && d.date !== "") dong.date = d.date;
      return dong;
    });
  }

  return docThanLoiGoi<petitions_nhiemVuRa>(
    goiGhi(duongDan, "POST", thanGui, 201, { "Idempotency-Key": khoaChongTrung }),
  );
}

/**
 * PATCH /api/v1/tasks/{ma} — §5.4 nút `✎ Sửa`. 200, trả về nhiệm vụ sau khi sửa.
 *
 * `null` NGHĨA LÀ "KHÔNG ĐỔI", không phải "xoá trắng" — mọi trường là con trỏ ở máy chủ. Chuỗi
 * rỗng thì KHÁC HẲN: nó là "xoá nội dung ô này", một việc hợp lệ với ghi chú và tóm tắt kết quả.
 *
 * KHÔNG CÓ `assigner`, và sự vắng mặt ấy là TỪ CHỐI chứ không phải bỏ sót: `assigner` CHÍNH LÀ người
 * duyệt theo ADR 0038, nên một tài khoản `task.update` sửa được cột ấy là một tài khoản tự đặt mình
 * làm người duyệt đề nghị của chính mình.
 *
 * ĐỔI CHIỀU CÓ CHỦ Ý 28/09/2026 (W3): `code` (đổi mã, 3c3525f — 409 `code_taken`), `due_at` (SỬA hạn,
 * f27fd6e — một mốc RFC 3339, không phải lùi hạn) và `expected_updated_at` (khoá lạc quan, d2ed15e —
 * 409 `task_changed`) nay ĐI trên dây. Trước đó cả ba bị bỏ ở đây vì máy chủ chưa nhận.
 *
 * DỰNG TỪNG TRƯỜNG, KHÔNG GỬI THẲNG `than` — cùng lý do `taoNhiemVu`: một đối tượng mang thêm
 * `assigner` hay `status` vẫn qua được `tsc` (kiểu cấu trúc), và đi thẳng lên dây. Chỉ trường có giá
 * trị mới có mặt; `null` bị bỏ, vì với máy chủ nó cùng nghĩa với vắng mặt — nên một hạn không XOÁ được
 * qua đây, đúng như máy chủ (`due_at` null/vắng = giữ nguyên).
 *
 * ⚠ `documents: []` PHẢI SỐNG SÓT QUA ĐÂY. Trên tuyến này mảng rỗng là "gỡ hết mọi dòng" — khác hẳn
 * vắng mặt ("không đụng tới khối"). Mỗi dòng cũng dựng lại từng trường: `id` giữ nguyên (thiếu nó là
 * dòng cũ bị gỡ rồi thêm lại), `position` không bao giờ đi (số thứ tự do sổ cấp).
 */
export function suaNhiemVu(
  ma: string,
  than: petitions_suaNhiemVuVao,
): Promise<KetQua<petitions_nhiemVuRa>> {
  const mau: petitions_patch_tasks_by_ma["duongDan"] = "/api/v1/tasks/{ma}";

  const thanGui: petitions_suaNhiemVuVao = {};
  if (than.code !== undefined && than.code !== null) thanGui.code = than.code;
  if (than.due_at !== undefined && than.due_at !== null) thanGui.due_at = than.due_at;
  if (than.bloc !== undefined && than.bloc !== null) thanGui.bloc = than.bloc;
  if (than.title !== undefined && than.title !== null) thanGui.title = than.title;
  if (than.description !== undefined && than.description !== null) {
    thanGui.description = than.description;
  }
  if (than.priority !== undefined && than.priority !== null) thanGui.priority = than.priority;
  if (than.progress !== undefined && than.progress !== null) thanGui.progress = than.progress;
  if (than.result_summary !== undefined && than.result_summary !== null) {
    thanGui.result_summary = than.result_summary;
  }
  if (than.note !== undefined && than.note !== null) thanGui.note = than.note;
  if (than.leader_approved !== undefined && than.leader_approved !== null) {
    thanGui.leader_approved = than.leader_approved;
  }
  if (than.superior_acknowledged !== undefined && than.superior_acknowledged !== null) {
    thanGui.superior_acknowledged = than.superior_acknowledged;
  }
  if (than.parent !== undefined && than.parent !== null) thanGui.parent = than.parent;
  if (than.documents !== undefined && than.documents !== null) {
    thanGui.documents = than.documents.map((d) => {
      const dong: petitions_vanBanNhiemVuVao = { group: d.group, summary: d.summary };
      if (d.id !== undefined && d.id !== "") dong.id = d.id;
      if (d.reference !== undefined && d.reference !== "") dong.reference = d.reference;
      if (d.date !== undefined && d.date !== "") dong.date = d.date;
      return dong;
    });
  }
  // The token of the task AS THE SCREEN READ IT, sent back unchanged — never recomputed here.
  if (than.expected_updated_at !== undefined && than.expected_updated_at !== null) {
    thanGui.expected_updated_at = than.expected_updated_at;
  }

  return docThanLoiGoi<petitions_nhiemVuRa>(
    goiGhi(duongDanNhiemVu(mau, ma), "PATCH", thanGui, 200),
  );
}

/**
 * POST /api/v1/tasks/{ma}/status — §6 chuyển trạng thái. 200, trả về nhiệm vụ sau khi chuyển.
 *
 * TRẠNG THÁI ĐÍCH ĐI TRÊN DÂY, khác hẳn tuyến `…/status` của phiếu phản ánh — vì vòng đời §6 RẼ
 * NHÁNH ở mọi bước (`tam-dung` rời được cả bốn trạng thái làm việc), nên không có một "bước kế
 * tiếp" nào để máy chủ tự chọn. Cái máy chủ vẫn giữ là TẤM BẢN ĐỒ: một bước sơ đồ không vẽ thì trả
 * 409.
 *
 * `chuyen-tiep` KHÔNG CÒN LÀ ĐÍCH CỦA TUYẾN NÀY (764bb92): máy chủ trả 400 và chỉ sang
 * `giaoLaiNhiemVu`. Chuyển tiếp nay là giao CÙNG nhiệm vụ cho nơi khác, không phải một trạng thái.
 *
 * ⚠ BƯỚC `hoan-thanh` ĐI QUA HAI PHÉP KIỂM NỮA Ở MÁY CHỦ, và cả hai đều trả câu chữ mang thông
 * tin: `task.approve` (khoá hẹp hơn khoá cổng của tuyến), và **mọi việc con phải xong** — câu từ
 * chối LIỆT KÊ MÃ các việc con còn lại. Màn hình vẽ thẳng câu ấy.
 */
export function doiTrangThaiNhiemVu(
  ma: string,
  trangThai: string,
  ghiChu?: string,
): Promise<KetQua<petitions_nhiemVuRa>> {
  const mau: petitions_post_tasks_by_ma_status["duongDan"] = "/api/v1/tasks/{ma}/status";
  const than: petitions_doiTrangThaiVao =
    ghiChu !== undefined && ghiChu !== ""
      ? { status: trangThai, note: ghiChu }
      : { status: trangThai };
  return docThanLoiGoi<petitions_nhiemVuRa>(
    goiGhi(duongDanNhiemVu(mau, ma), "POST", than, 200),
  );
}

/**
 * POST /api/v1/tasks/{ma}/log-entries — §5.9 manual timeline entry (60011e8). 201, the new row.
 *
 * THE ROUTE'S GATE IS `task.read`; who may write is decided on the row (`TaskWorkRightFor`: assignee or
 * `task.update` fully, monitor / assigner / creator log-only; anybody else 403). The refusal comes back
 * verbatim.
 *
 * `idempotencyKey` IS A PARAMETER, NOT MADE HERE — same reason as `taoNhiemVu`: the route requires it,
 * and a key made per call would turn a retry after a lost response into a SECOND entry in an
 * append-only table, which nobody can ever remove. The form holds the key and reuses it on retry.
 *
 * The note is sent as given; the server trims it and refuses a blank or over-long one.
 */
export function addTaskLogEntry(
  code: string,
  note: string,
  idempotencyKey: string,
): Promise<KetQua<petitions_nhatKyNhiemVuRa>> {
  const mau: petitions_post_tasks_by_ma_log_entries["duongDan"] = "/api/v1/tasks/{ma}/log-entries";
  const body: petitions_taskLogEntryIn = { note };
  return docThanLoiGoi<petitions_nhatKyNhiemVuRa>(
    goiGhi(duongDanNhiemVu(mau, code), "POST", body, 201, { "Idempotency-Key": idempotencyKey }),
  );
}

/**
 * POST /api/v1/tasks/{ma}/assignment — §5.7 "Giao việc, chuyển việc", `task.assign`. 200, returns
 * the task after the act. Named after the server handler (`ReassignTask`).
 *
 * ABSENT MEANS "LEAVE IT", "" MEANS "CLEAR IT" — the server reads every field as a pointer. The
 * caller (`assignmentBody`) sends ONLY what differs from the task as read: every staff code sent is
 * checked with identity, so resending an unchanged monitor who has since left would refuse an
 * unrelated change.
 *
 * FIELD BY FIELD, NOT `body` AS IS — same reason as `taoNhiemVu`: a structurally typed object
 * carrying `status` or `due_at` would pass `tsc` and reach the wire. `note` goes only when it has
 * text; it lands on the timeline under the server's own from→to sentence.
 *
 * THE SERVER'S REFUSALS ARE SHOWN VERBATIM by the caller: 400 (nothing sent, unit cleared, staff
 * code not assignable — one sentence), 409 `no_change`, 409 `task_state` (terminal task), 503
 * `assignee_check_unavailable`.
 */
export function reassignTask(
  code: string,
  body: petitions_taskAssignmentIn,
): Promise<KetQua<petitions_nhiemVuRa>> {
  const template: petitions_post_tasks_by_ma_assignment["duongDan"] =
    "/api/v1/tasks/{ma}/assignment";
  const sent: petitions_taskAssignmentIn = {};
  if (body.unit !== undefined && body.unit !== null) sent.unit = body.unit;
  if (body.assignee !== undefined && body.assignee !== null) sent.assignee = body.assignee;
  if (body.lead_unit !== undefined && body.lead_unit !== null) sent.lead_unit = body.lead_unit;
  if (body.monitor !== undefined && body.monitor !== null) sent.monitor = body.monitor;
  if (body.note !== undefined && body.note !== "") sent.note = body.note;
  return docThanLoiGoi<petitions_nhiemVuRa>(
    goiGhi(duongDanNhiemVu(template, code), "POST", sent, 200),
  );
}

/**
 * DELETE /api/v1/tasks/{ma} — xoá MỀM, kèm lý do bắt buộc. **204 không thân.**
 *
 * TRẢ VỀ `KetQua<void>` CHỨ KHÔNG ĐỌC THÂN: một hàm luôn gọi `.json()` sẽ biến lần xoá thành
 * công thành "không đọc được".
 *
 * ⚠ 409 KHI CÒN VIỆC CON CHƯA XOÁ (ADR 0037 quyết định 3), và câu từ chối MANG SỐ VIỆC CON. Con
 * số ấy là toàn bộ phần có ích: từ chối mà không nói còn vướng gì là bắt cán bộ đi tìm.
 */
export function xoaNhiemVu(ma: string, lyDo: string): Promise<KetQua<void>> {
  const mau: petitions_delete_tasks_by_ma["duongDan"] = "/api/v1/tasks/{ma}";
  const than: petitions_xoaNhiemVuVao = { reason: lyDo };
  return goiGhi(duongDanNhiemVu(mau, ma), "DELETE", than, 204).then((kq) =>
    kq.ok ? ({ ok: true, duLieu: undefined } as KetQua<void>) : kq,
  );
}

/**
 * POST /api/v1/tasks/{ma}/extensions — §5.8 gửi đề nghị lùi hạn. 201, trả về đề nghị.
 *
 * ⚠ TUYẾN NÀY ĐỨNG SAU `task.update`, **KHÔNG** SAU `task.extend` (ADR 0038). Người ĐỀ NGHỊ là
 * người đang làm việc; người DUYỆT là lãnh đạo giao việc. Gộp hai khoá là biến ô đề nghị thành
 * thứ chỉ người duyệt mới dùng được.
 *
 * `new_due_at` PHẢI MUỘN HƠN HẠN ĐANG CÓ — máy chủ kiểm, và một "đề nghị lùi hạn" về phía trước
 * là một lần rút ngắn cam kết qua đường vòng không ai duyệt.
 *
 * Mỗi nhiệm vụ chỉ có MỘT đề nghị đang chờ (`UNIQUE (tenant_id, nhiem_vu_id, moc_cho_duyet)`),
 * nên lần gửi thứ hai là 409 chứ không phải một đề nghị thứ hai lãnh đạo duyệt được hai lần.
 */
export function deNghiLuiHan(
  ma: string,
  hanMoiISO: string,
  lyDo: string,
): Promise<KetQua<petitions_deNghiLuiHanRa>> {
  const mau: petitions_post_tasks_by_ma_extensions["duongDan"] = "/api/v1/tasks/{ma}/extensions";
  const than: petitions_deNghiLuiHanVao = { new_due_at: hanMoiISO, reason: lyDo };
  return docThanLoiGoi<petitions_deNghiLuiHanRa>(
    goiGhi(duongDanNhiemVu(mau, ma), "POST", than, 201),
  );
}

/**
 * POST /api/v1/tasks/{ma}/extensions/{deNghiID}/decision — §5.8, ADR 0038. 200.
 *
 * HAI GIÁ TRỊ VÀ ĐÚNG HAI: `approve` · `reject`. Máy chủ TỪ CHỐI mọi giá trị khác thay vì đọc
 * thành "từ chối" — một lỗi gõ im lặng bác một đề nghị lùi hạn là một cam kết với dân bị giữ
 * nguyên mà không ai biết vì sao.
 *
 * ⚠ HAI LỚP KIỂM Ở MÁY CHỦ, KHÔNG THAY NHAU: `task.extend` ở cổng, rồi
 * `Principal.Ma == lanh_dao_giao_viec_ma` trong giao dịch. Nhiệm vụ CHƯA GHI lãnh đạo giao việc
 * thì **không ai duyệt được** — hỏng theo chiều đóng, và câu từ chối chỉ thẳng thứ phải bổ sung.
 */
export function quyetDinhLuiHan(
  ma: string,
  deNghiID: string,
  duyet: boolean,
  ghiChu?: string,
): Promise<KetQua<petitions_deNghiLuiHanRa>> {
  const mau: petitions_post_tasks_by_ma_extensions_by_deNghiID_decision["duongDan"] =
    "/api/v1/tasks/{ma}/extensions/{deNghiID}/decision";
  const duongDan = duongDanNhiemVu(mau, ma).replace(
    "{deNghiID}",
    encodeURIComponent(deNghiID),
  );
  const than: petitions_quyetDinhLuiHanVao =
    ghiChu !== undefined && ghiChu !== ""
      ? { decision: duyet ? "approve" : "reject", note: ghiChu }
      : { decision: duyet ? "approve" : "reject" };
  return docThanLoiGoi<petitions_deNghiLuiHanRa>(goiGhi(duongDan, "POST", than, 200));
}
