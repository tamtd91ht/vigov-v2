/**
 * HỢP ĐỒNG VỚI ViGov CHO PHIẾU PHẢN ÁNH CỦA CÔNG DÂN — tệp DUY NHẤT biết đường dẫn, tên trường
 * gửi đi và hình dạng phản hồi. Viết tay: `citizen-app` không có kiểu sinh từ openapi.
 *
 *   POST /api/v1/my-citizen-reports            Bearer + Idempotency-Key  → 201 phieuCuaToiRa
 *   GET  /api/v1/my-citizen-reports/{maTraCuu} Bearer                    → 200 phieuCuaToiRa · 404
 *   GET  /api/v1/my-citizen-reports            Bearer                    → 200 trang phieuCuaToiTomTatRa
 *
 * TUYẾN DANH SÁCH (26/09/2026), NGUYÊN VĂN HỢP ĐỒNG ĐÃ GIAO:
 *
 *   GET /api/v1/my-citizen-reports — CitizenOnly; citizen and commune come ONLY from the ViGov
 *   citizen session token (never send tenant/phone/citizen id in query/body/header);
 *   phone_verified_required: true.
 *    Query: limit (1-100, default 20), cursor (opaque, pass back next_cursor verbatim),
 *    sort=received_at (only), order=desc (only desc accepted; omit it), status (optional, one of
 *    the nine codes).
 *    200: {items: phieuCuaToiTomTatRa[], next_cursor: string ("" when has_more false),
 *    has_more: boolean}; items always [] when none.
 *    phieuCuaToiTomTatRa: {code: string (lookup code), status: string, field: string,
 *    field_label: string, content_excerpt: string (≤140 chars, "…" if cut), clock_from: date-time,
 *    acknowledge_due: date-time|null, resolve_due: date-time|null} — all keys always present.
 *    Errors: 400 invalid params; 401 no/invalid session or no commune chosen; 500. No 403/404.
 *
 *   Client gửi ĐÚNG HAI tham số: `limit` và (từ trang thứ hai) `cursor`. `sort`/`order` bỏ đi vì
 *   máy chủ chỉ có một cách xếp; `status` chưa màn nào dùng.
 *
 * Nguồn đối chiếu (đọc, không sửa): `service-petitions/internal/http/gui_phan_anh.go`,
 * `phieu_cua_toi.go`, `routes_cong_dan.go`.
 *
 * ⚠ NĂM TRƯỜNG, VÀ CHỈ NĂM TRƯỜNG. `guiPhanAnhVao` khai thêm mười trường CHỈ ĐỂ TỪ CHỐI:
 * `citizen_id`/`cong_dan_id` (người gửi — lấy từ phiên, luật 4), `field`/`linh_vuc` (lĩnh vực — cán
 * bộ chốt, ADR 0028 / #23), `channel`, `code`, `status`, `clock_from`, `acknowledge_due`,
 * `resolve_due`. Nhắc tới BẤT KỲ trường nào trong đó là 400. Xã thì không có trường nào cả — xã lấy
 * từ phiên (ADR 0022). `cong-dan.test.ts` khẳng định thân gửi đi đúng bằng năm khoá dưới.
 */

import { diaChiViGov } from "./dia-chi-vigov";

/** Đường dẫn tài nguyên. Một hằng — GET ghép mã tra cứu vào sau nó, không gõ lại. */
export const DUONG_DAN_PHAN_ANH_CUA_TOI = "/api/v1/my-citizen-reports";

/** Năm khoá máy chủ NHẬN. Xuất ra để phép kiểm đối chiếu, không để tệp khác dựng thân. */
export const TRUONG_DUOC_NHAN = [
  "content",
  "address",
  "reporter_name",
  "reporter_phone",
  "anonymous",
] as const;

/**
 * Giới hạn độ dài — CHÉP từ `service-petitions/internal/domain/gui_phan_anh.go` (đếm theo KÝ TỰ,
 * không theo byte). Client kiểm trước để nói bằng câu của người dân thay vì câu kỹ thuật của 400;
 * máy chủ vẫn là bên quyết định. Máy chủ đổi số thì sửa ở đây.
 */
export const DO_DAI_TOI_DA = {
  noi_dung: 4000,
  dia_chi: 500,
  ho_ten: 200,
  dien_thoai: 32,
} as const;

/** Thứ công dân gõ — đặt tên theo việc, không theo dây. */
export type PhanAnhMoi = {
  noi_dung: string;
  dia_chi: string;
  ho_ten: string;
  dien_thoai: string;
  an_danh: boolean;
};

/**
 * `PhanAnhMoi` → thân yêu cầu. CHỖ DUY NHẤT năm tên trường gửi đi được viết ra.
 *
 * ẨN DANH THÌ KHÔNG GỬI HỌ TÊN VÀ SỐ ĐIỆN THOẠI — gửi chuỗi rỗng. Người bấm "Gửi ẩn danh" đã nói
 * họ không muốn tên mình gắn với phiếu; gửi hai ô ấy đi rồi trông vào máy chủ che là giữ lời hứa
 * bằng hệ thống của người khác (luật 3, bất biến 6: chỉ gửi đúng thứ cần).
 */
export function thanGuiPhanAnh(pa: PhanAnhMoi): string {
  return JSON.stringify({
    content: pa.noi_dung.trim(),
    address: pa.dia_chi.trim(),
    reporter_name: pa.an_danh ? "" : pa.ho_ten.trim(),
    reporter_phone: pa.an_danh ? "" : pa.dien_thoai.trim(),
    anonymous: pa.an_danh,
  });
}

/**
 * Một phiếu như máy chủ trả cho CHÍNH NGƯỜI GỬI (`phieuCuaToiRa`). Không có ghi chú cán bộ, không
 * có lịch sử chuyển, không có người xử lý — máy chủ không gửi, và kiểu này không có chỗ cho chúng
 * (luật 4, cấm #5; luật 10, bất biến 7).
 *
 * Họ tên và số điện thoại về ĐÃ CHE (`Nguyễn V. A.`, `09****0000`), và RỖNG khi gửi ẩn danh.
 */
export type PhieuCuaToi = {
  readonly ma_tra_cuu: string;
  readonly trang_thai: string;
  readonly linh_vuc: string;
  readonly nhan_linh_vuc: string;
  readonly noi_dung: string;
  readonly dia_chi: string;
  readonly ho_ten_da_che: string;
  readonly dien_thoai_da_che: string;
  readonly an_danh: boolean;
  readonly goc_dem_han: string;
  /** `null` = KHÔNG ÁP DỤNG (phiếu cán bộ nhập hộ). */
  readonly han_tiep_nhan: string | null;
  /** `null` = CHƯA CÓ (phiếu chưa được phân loại). Hai `null` nghĩa trái nhau — đừng gộp. */
  readonly han_xu_ly_xong: string | null;
  readonly ket_qua: string;
  /**
   * Lý do xã KHÔNG TIẾP NHẬN hoặc CHUYỂN CẤP TRÊN — viết CHO công dân (migration 0011). RỖNG ở mọi
   * trạng thái khác, kể cả khi máy chủ lỡ gửi: `docPhieu` bỏ nó đi (xem `laNhanhKetThuc`).
   */
  readonly ly_do: string;
  /** Cơ quan nhận phiếu — chỉ có ở `chuyen-cap-tren`, RỖNG ở mọi trạng thái khác. */
  readonly co_quan_nhan: string;
};

/**
 * Giới hạn hai trường của hai nhánh kết thúc — CHÉP từ `service-petitions/internal/domain/
 * xu_ly_phan_anh.go` (`LyDoToiDa`, `CoQuanNhanToiDa`), đếm theo KÝ TỰ như `utf8.RuneCountInString`.
 * Ở chiều ĐỌC, vượt giới hạn nghĩa là máy chủ trả thứ nó không bao giờ ghi được — sai khuôn, không
 * phải một câu dài để cắt bớt.
 */
export const DO_DAI_NHANH_KET_THUC = {
  ly_do: 2000,
  co_quan_nhan: 200,
} as const;

/**
 * Hai trạng thái mang `reason`/`receiving_body`. Máy chủ tự kiểm điều kiện này trước khi gửi
 * (`phieu_cua_toi.go`); client kiểm LẠI vì đây là chỗ chữ cán bộ viết tới tay người dân — một lý do
 * từ chối hiện trên phiếu đang xử lý là một câu sai do cơ quan nhà nước nói ra.
 */
export function laNhanhKetThuc(trang_thai: string): boolean {
  return trang_thai === "khong-tiep-nhan" || trang_thai === "chuyen-cap-tren";
}

/** Số KÝ TỰ (code point), không phải số đơn vị UTF-16: "ă" tổ hợp hay emoji không bị đếm đôi. */
const soKyTu = (s: string): number => [...s].length;

/**
 * Thân trả lời → `PhieuCuaToi`, hoặc `null` nếu sai khuôn.
 *
 * KIỂM TỪNG TRƯỜNG, không ép kiểu: một `as` cho `undefined` đi tiếp và hiện ra màn hình thành chữ
 * "undefined" — ở đúng chỗ đáng lẽ là mã tra cứu của người dân.
 */
export function docPhieu(than: unknown): PhieuCuaToi | null {
  if (typeof than !== "object" || than === null) return null;
  const t = than as Record<string, unknown>;
  const chuoi = (k: string): string | null => (typeof t[k] === "string" ? (t[k] as string) : null);
  const chuoiHoacNull = (k: string): string | null | undefined =>
    t[k] === null ? null : typeof t[k] === "string" ? (t[k] as string) : undefined;

  const ma = chuoi("code");
  const trang_thai = chuoi("status");
  const linh_vuc = chuoi("field");
  const nhan = chuoi("field_label");
  const noi_dung = chuoi("content");
  const dia_chi = chuoi("address");
  const ho_ten = chuoi("reporter_name");
  const dien_thoai = chuoi("reporter_phone");
  const goc = chuoi("clock_from");
  const ket_qua = chuoi("result");
  const han_tiep_nhan = chuoiHoacNull("acknowledge_due");
  const han_xu_ly = chuoiHoacNull("resolve_due");
  const an_danh = t["anonymous"];
  // Hai trường TUỲ CHỌN (`omitempty`): vắng mặt là "", có mặt thì phải là chuỗi trong giới hạn.
  const tuyChon = (k: string, toi_da: number): string | null => {
    if (t[k] === undefined) return "";
    return typeof t[k] === "string" && soKyTu(t[k] as string) <= toi_da ? (t[k] as string) : null;
  };
  const ly_do = tuyChon("reason", DO_DAI_NHANH_KET_THUC.ly_do);
  const co_quan_nhan = tuyChon("receiving_body", DO_DAI_NHANH_KET_THUC.co_quan_nhan);

  if (
    ma === null ||
    ma === "" ||
    trang_thai === null ||
    linh_vuc === null ||
    nhan === null ||
    noi_dung === null ||
    dia_chi === null ||
    ho_ten === null ||
    dien_thoai === null ||
    goc === null ||
    ket_qua === null ||
    han_tiep_nhan === undefined ||
    han_xu_ly === undefined ||
    typeof an_danh !== "boolean" ||
    ly_do === null ||
    co_quan_nhan === null
  ) {
    return null;
  }

  const nhanh = laNhanhKetThuc(trang_thai);

  return {
    ma_tra_cuu: ma,
    trang_thai,
    linh_vuc,
    nhan_linh_vuc: nhan,
    noi_dung,
    dia_chi,
    ho_ten_da_che: ho_ten,
    dien_thoai_da_che: dien_thoai,
    an_danh,
    goc_dem_han: goc,
    han_tiep_nhan,
    han_xu_ly_xong: han_xu_ly,
    ket_qua,
    ly_do: nhanh ? ly_do : "",
    // Cơ quan nhận chỉ có nghĩa khi phiếu ĐƯỢC CHUYỂN; ở `khong-tiep-nhan` không ai nhận cả.
    co_quan_nhan: trang_thai === "chuyen-cap-tren" ? co_quan_nhan : "",
  };
}

/** Địa chỉ tuyến gửi trên host của `service-petitions`, hoặc RỖNG khi host ấy chưa có. */
export function diaChiGuiPhanAnh(): string {
  return diaChiViGov("petitions", DUONG_DAN_PHAN_ANH_CUA_TOI);
}

/**
 * MỘT DÒNG CỦA "PHẢN ÁNH CỦA TÔI" (`phieuCuaToiTomTatRa`). Ít hơn `PhieuCuaToi`: không địa chỉ,
 * không người gửi, không kết quả — chạm vào dòng là mở màn tra cứu, nơi đọc đủ phiếu.
 */
export type PhieuCuaToiTomTat = {
  readonly ma_tra_cuu: string;
  readonly trang_thai: string;
  readonly linh_vuc: string;
  readonly nhan_linh_vuc: string;
  /** Tối đa 140 ký tự, máy chủ đã cắt và thêm "…". */
  readonly trich_noi_dung: string;
  readonly goc_dem_han: string;
  readonly han_tiep_nhan: string | null;
  readonly han_xu_ly_xong: string | null;
};

/** Một trang. `con_nua = false` thì `con_tro` luôn rỗng. */
export type TrangPhieuCuaToi = {
  readonly muc: readonly PhieuCuaToiTomTat[];
  readonly con_tro: string;
  readonly con_nua: boolean;
};

/** Số dòng mỗi trang — trong khoảng 1..100 hợp đồng cho phép. */
export const SO_DONG_MOI_TRANG = 20;

function docTomTat(than: unknown): PhieuCuaToiTomTat | null {
  if (typeof than !== "object" || than === null) return null;
  const t = than as Record<string, unknown>;
  const chuoi = (k: string): string | null => (typeof t[k] === "string" ? (t[k] as string) : null);
  const chuoiHoacNull = (k: string): string | null | undefined =>
    t[k] === null ? null : typeof t[k] === "string" ? (t[k] as string) : undefined;

  const ma = chuoi("code");
  const trang_thai = chuoi("status");
  const linh_vuc = chuoi("field");
  const nhan = chuoi("field_label");
  const trich = chuoi("content_excerpt");
  const goc = chuoi("clock_from");
  const han_tiep_nhan = chuoiHoacNull("acknowledge_due");
  const han_xu_ly = chuoiHoacNull("resolve_due");
  if (
    ma === null ||
    ma === "" ||
    trang_thai === null ||
    linh_vuc === null ||
    nhan === null ||
    trich === null ||
    goc === null ||
    han_tiep_nhan === undefined ||
    han_xu_ly === undefined
  ) {
    return null;
  }
  return {
    ma_tra_cuu: ma,
    trang_thai,
    linh_vuc,
    nhan_linh_vuc: nhan,
    trich_noi_dung: trich,
    goc_dem_han: goc,
    han_tiep_nhan,
    han_xu_ly_xong: han_xu_ly,
  };
}

/**
 * Thân trả lời của tuyến danh sách → `TrangPhieuCuaToi`, hoặc `null` nếu sai khuôn.
 *
 * MỘT DÒNG SAI KHUÔN LÀ CẢ TRANG SAI KHUÔN: bỏ lặng lẽ một dòng là giấu một phiếu của chính người
 * dân khỏi danh sách của họ — họ sẽ tưởng phiếu ấy chưa từng được gửi.
 *
 * `has_more = true` mà con trỏ rỗng cũng là sai khuôn: bấm "Xem thêm" với con trỏ rỗng là tải lại
 * trang đầu và nhân đôi danh sách.
 */
export function docTrangPhieuCuaToi(than: unknown): TrangPhieuCuaToi | null {
  if (typeof than !== "object" || than === null) return null;
  const t = than as Record<string, unknown>;
  const items = t["items"];
  const con_tro = t["next_cursor"];
  const con_nua = t["has_more"];
  if (!Array.isArray(items) || typeof con_tro !== "string" || typeof con_nua !== "boolean") return null;
  if (con_nua && con_tro === "") return null;

  const muc: PhieuCuaToiTomTat[] = [];
  for (const mot of items) {
    const p = docTomTat(mot);
    if (p === null) return null;
    muc.push(p);
  }
  return { muc, con_tro: con_nua ? con_tro : "", con_nua };
}

/**
 * Địa chỉ tuyến danh sách, hoặc RỖNG.
 *
 * CHỈ `limit` VÀ `cursor`. Không một tham số nào nói "của ai" hay "xã nào": máy chủ lấy cả hai từ
 * phiên (luật 4, cấm #1; luật 1, cấm #2). Con trỏ đi NGUYÊN VĂN — `URLSearchParams` mã hoá nó để
 * một ký tự `&` hay `=` trong chuỗi mờ không đổi được tham số, và máy chủ giải mã về đúng chuỗi ấy.
 */
export function diaChiDanhSach(con_tro: string): string {
  const goc = diaChiViGov("petitions", DUONG_DAN_PHAN_ANH_CUA_TOI);
  if (goc === "") return "";
  const q = new URLSearchParams({ limit: String(SO_DONG_MOI_TRANG) });
  if (con_tro !== "") q.set("cursor", con_tro);
  return `${goc}?${q.toString()}`;
}

/**
 * Địa chỉ tuyến tra một mã, hoặc RỖNG.
 *
 * MÃ TRA CỨU ĐI TRÊN ĐƯỜNG DẪN vì hợp đồng đặt nó ở đó (`{maTraCuu}`). Nó là mã nghiệp vụ, không
 * phải dữ liệu cá nhân, và chỉ mở được phiếu khi đi kèm đúng phiên của người gửi (404 cho mọi
 * trường hợp khác). `encodeURIComponent` để một ký tự lạ người dân gõ không đổi được đường dẫn.
 */
export function diaChiTraCuu(ma_tra_cuu: string): string {
  const goc = diaChiViGov("petitions", DUONG_DAN_PHAN_ANH_CUA_TOI);
  return goc === "" ? "" : `${goc}/${encodeURIComponent(ma_tra_cuu)}`;
}
