/**
 * Sáu tuyến của sổ Phản ánh người dân (`docs/ui-ux/09-phan-anh-nguoi-dan.md`), đúng bộ tuyến
 * `service-petitions/internal/http/routes.go:529-680` khai — không nhiều hơn, không ít hơn.
 *
 *   GET  /api/v1/citizen-reports                            feedback.read
 *   GET  /api/v1/citizen-reports/{maTraCuu}                 feedback.read
 *   POST /api/v1/citizen-reports/{maTraCuu}/classification  feedback.classify
 *   POST /api/v1/citizen-reports/{maTraCuu}/assignment      feedback.assign
 *   POST /api/v1/citizen-reports/{maTraCuu}/status          feedback.read + LUẬT NẮM GIỮ
 *   POST /api/v1/citizen-reports/{maTraCuu}/closure         feedback.resolve
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
 * ⚠ HỢP ĐỒNG KHÔNG KHAI MỘT THAM SỐ TRUY VẤN NÀO CHO TUYẾN DANH SÁCH, VÀ HANDLER THẬT ĐỌC BẢY.
 * `petitions_get_citizen_reports["truyVan"]` sinh ra một đối tượng RỖNG, trong khi
 * `locPhieuTuQuery` (`xu_ly_phan_anh.go:255-294`) đọc và **kiểm** `status` · `channel` · `field` ·
 * `hamlet` · `unit` · `q` · `late`, còn `page.Parse` đọc `limit` · `cursor` · `sort` · `order`.
 * Nguyên nhân là `tools/apidoc` chưa có chú thích `@query` trên tuyến ấy, chứ không phải máy chủ
 * không nhận. Hệ quả có thật và được nói ra chứ không giấu: **bộ lọc §4 không có kiểu nào của hợp
 * đồng canh giúp** — gõ sai một tên tham số ở đây thì `tsc` im lặng và máy chủ trả cả sổ. Vì thế
 * tên tham số được gom vào đúng một chỗ (`themLocVaoTruyVan`) và có bài kiểm đọc lại từng tên.
 * Đã báo về để khai `@query` trên tuyến.
 */

import { docJSON, goiGhi, LOI_KHONG_RO, type KetQua } from "./goi";
import type {
  petitions_dongPhieuVao,
  petitions_get_citizen_reports,
  petitions_get_citizen_reports_by_maTraCuu,
  petitions_phanCongVao,
  petitions_phanLoaiVao,
  petitions_phieuPhanAnhRa,
  petitions_post_citizen_reports_by_maTraCuu_assignment,
  petitions_post_citizen_reports_by_maTraCuu_classification,
  petitions_post_citizen_reports_by_maTraCuu_closure,
  petitions_post_citizen_reports_by_maTraCuu_status,
  page_Result_petitions_phieuPhanAnhRa,
} from "./schema.gen";

/**
 * Đọc thân của một lần ghi thành công.
 *
 * ⚠ ĐÂY LÀ BẢN THỨ TƯ CỦA HÀM NÀY (`van-ban.ts:62`, `can-bo.ts:202`, `thu-chi.ts:61` là ba bản
 * kia), và nó được viết lại thay vì gộp về `goi.ts` vì `goi.ts` nằm NGOÀI ranh giới ghi của lượt
 * này. Ghi ra như một phát hiện chứ không im lặng: điều kiện `goi.ts` tự đặt cho mình —
 * *"màn hình ghi thứ hai xuất hiện là lúc hàm này chuyển sang goi.ts"* — nay đã thoả lần thứ ba.
 */
async function docThanRa<T>(goi: Promise<KetQua<Response>>): Promise<KetQua<T>> {
  const kq = await goi;
  if (!kq.ok) return kq;
  try {
    return { ok: true, duLieu: (await kq.duLieu.json()) as T };
  } catch {
    // Đúng mã mong đợi mà thân không phải JSON là máy chủ hoặc proxy đang trả thứ khác. Với cán
    // bộ thì đó vẫn là "không đọc được", không phải một trạng thái nghiệp vụ.
    return { ok: false, thongBao: LOI_KHONG_RO };
  }
}

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
 * Bộ lọc của sổ phản ánh — ĐÚNG bảy tham số `locPhieuTuQuery` đọc, cộng phân trang.
 *
 * KHÔNG CÓ `pham_vi` (`Toàn xã` · `Giao cho tôi` · `Liên quan đến tôi`, §4) và KHÔNG CÓ
 * `danh_gia_thap`: máy chủ không nhận hai tham số ấy, và một tham số không được nhận thì bị **bỏ
 * qua lặng lẽ** — màn hình sẽ hiện cả sổ trong khi cán bộ tin mình đang xem phần việc của riêng
 * mình. Hai thiếu sót ấy ra tới màn hình, xem `features/phan-anh/nhan-phieu.ts`.
 */
export type LocPhanAnh = {
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
  limit?: number;
  cursor?: string | null;
};

/**
 * Gom TÊN tham số truy vấn về đúng một chỗ.
 *
 * Bảy cái tên dưới đây là bảy chuỗi KHÔNG có kiểu nào của hợp đồng canh giúp (xem khối chú thích
 * đầu tệp). Gõ sai một cái thì máy chủ bỏ qua nó và trả về cả quyển sổ, và màn hình trông hoàn
 * toàn bình thường — nên chúng đứng một chỗ và có bài kiểm đọc lại từng tên.
 */
function themLocVaoTruyVan(truyVan: URLSearchParams, loc: LocPhanAnh): void {
  if (loc.trangThai !== undefined && loc.trangThai !== "") truyVan.set("status", loc.trangThai);
  if (loc.kenh !== undefined && loc.kenh !== "") truyVan.set("channel", loc.kenh);
  if (loc.linhVuc !== undefined && loc.linhVuc !== "") truyVan.set("field", loc.linhVuc);
  if (loc.thonID !== undefined && loc.thonID !== "") truyVan.set("hamlet", loc.thonID);
  if (loc.boPhanID !== undefined && loc.boPhanID !== "") truyVan.set("unit", loc.boPhanID);
  if (loc.tim !== undefined && loc.tim !== "") truyVan.set("q", loc.tim);
  // `late` CHỈ NHẬN ĐÚNG CHUỖI `true`. Gửi `false` là **400**, không phải "không lọc" — nên ô
  // chưa tích thì tham số vắng mặt hẳn (`locPhieuTuQuery`, `errLocTreHanKhongHopLe`).
  if (loc.chiTreHan === true) truyVan.set("late", "true");

  if (loc.limit !== undefined) truyVan.set("limit", String(loc.limit));
  // Con trỏ rỗng nghĩa là trang đầu. Gửi `cursor=` rỗng thì máy chủ trả 400 "con trỏ không hợp
  // lệ" — đúng vào lần mở màn hình đầu tiên.
  if (loc.cursor !== undefined && loc.cursor !== null && loc.cursor !== "") {
    truyVan.set("cursor", loc.cursor);
  }
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
 * BỐN THAO TÁC GHI
 *
 * CẢ BỐN TRẢ VỀ 200 KÈM NGUYÊN PHIẾU SAU KHI GHI, và màn hình dùng đúng phiếu ấy thay vì vá tại
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
): Promise<KetQua<petitions_phieuPhanAnhRa>> {
  const mau: petitions_post_citizen_reports_by_maTraCuu_classification["duongDan"] =
    "/api/v1/citizen-reports/{maTraCuu}/classification";
  const than: petitions_phanLoaiVao = { field: linhVuc };
  return docThanRa<petitions_phieuPhanAnhRa>(
    goiGhi(duongDanPhieu(mau, maTraCuu), "POST", than, 200),
  );
}

/**
 * POST …/assignment — khối `Chuyển xử lý, không đổi trạng thái` của §8.5.
 *
 * `assignee` LÀ TUỲ CHỌN VÀ ĐÓ LÀ MỘT LỰA CHỌN CÓ THẬT trên màn hình: `— Để bộ phận phân công —`.
 * Trường vắng mặt hẳn khi không chọn ai, chứ không gửi chuỗi rỗng.
 */
export function chuyenXuLyPhieu(
  maTraCuu: string,
  boPhanID: string,
  canBoID?: string,
): Promise<KetQua<petitions_phieuPhanAnhRa>> {
  const mau: petitions_post_citizen_reports_by_maTraCuu_assignment["duongDan"] =
    "/api/v1/citizen-reports/{maTraCuu}/assignment";
  const than: petitions_phanCongVao =
    canBoID !== undefined && canBoID !== ""
      ? { unit: boPhanID, assignee: canBoID }
      : { unit: boPhanID };
  return docThanRa<petitions_phieuPhanAnhRa>(
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
 * phiếu phải tiến được phiếu ấy. Giao diện KHÔNG dựng lại phép kiểm đó — nó không thể: phản hồi
 * mang `assignee` là **ULID nội bộ** còn phiên hiện tại chỉ mang mã nghiệp vụ của cán bộ, hai thứ
 * không so được. Nút luôn hiện với người xem được sổ, và câu 403 của máy chủ ra thẳng màn hình.
 */
export function tienTrangThaiPhieu(
  maTraCuu: string,
): Promise<KetQua<petitions_phieuPhanAnhRa>> {
  const mau: petitions_post_citizen_reports_by_maTraCuu_status["duongDan"] =
    "/api/v1/citizen-reports/{maTraCuu}/status";
  return docThanRa<petitions_phieuPhanAnhRa>(
    goiGhi(duongDanPhieu(mau, maTraCuu), "POST", undefined, 200),
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
 */
export function dongPhieu(
  maTraCuu: string,
  ketQua: string,
): Promise<KetQua<petitions_phieuPhanAnhRa>> {
  const mau: petitions_post_citizen_reports_by_maTraCuu_closure["duongDan"] =
    "/api/v1/citizen-reports/{maTraCuu}/closure";
  const than: petitions_dongPhieuVao = { result: ketQua };
  return docThanRa<petitions_phieuPhanAnhRa>(
    goiGhi(duongDanPhieu(mau, maTraCuu), "POST", than, 200),
  );
}
