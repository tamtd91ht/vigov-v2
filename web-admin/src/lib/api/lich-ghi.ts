/**
 * Mười một tuyến **GHI** lịch làm việc của một xã — ca làm việc trong tuần, ngày nghỉ lễ, ngày
 * làm bù. Phần ĐỌC của đúng ba bảng ấy đã có chủ ở `lich-lam-viec.ts` và được dùng lại chứ không
 * viết lần thứ hai (luật 9, cấm #2).
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * BA BẢNG NÀY LÀ CẤU HÌNH CỦA TỪNG XÃ, VÀ LÀ NỀN CỦA MỌI HẠN ĐÃ HỨA VỚI NGƯỜI DÂN. Hạn đếm theo
 * GIỜ LÀM VIỆC (ADR 0007, luật 10 bất biến 4): không chạy ngoài giờ làm, không chạy ngày nghỉ lễ,
 * có chạy trong ngày làm bù. Lịch tuần rỗng nghĩa là `identity.AdvanceWorkingHours` từ chối, và
 * xã không vào sổ được văn bản đến cũng không nhận được phản ánh.
 *
 * KHÔNG MỘT PHÉP CỘNG GIỜ LÀM VIỆC NÀO Ở PHÍA WEB, kể cả để "xem trước". `identity` sở hữu cả ba
 * bảng và sở hữu phép cộng; một bản tính thứ hai ở trình duyệt sẽ lệch bản của máy chủ vào đúng
 * ngày lễ, và con số lệch ấy là con số cán bộ đọc rồi báo cáo lên trên.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 *
 * MƯỜI MỘT TUYẾN GHI ĐỀU ĐÒI `admin.sla`, TRONG KHI BA TUYẾN ĐỌC LÀ `any-authenticated`. Bất đối
 * xứng ấy đến từ máy chủ, không từ đây (`service-identity/internal/http/lich_ghi.go`): giờ làm
 * việc nằm dưới mọi hạn HIỆN TRÊN MÀN HÌNH, nên đòi quyền cấu hình ở lượt đọc sẽ làm trắng những
 * màn hình đó cho mọi tài khoản không phải quản trị. Ghi thì ngược lại — đó là hành vi dịch
 * chuyển cam kết của cả xã, và đúng một chức danh làm việc ấy.
 *
 * ⚠ XOÁ Ở ĐÂY LÀ XOÁ MỀM, VÀ GIỜ MỞ CA BỊ GIỮ VĨNH VIỄN. `UNIQUE (tenant_id, thu, bat_dau)` cố ý
 * KHÔNG có `WHERE deleted_at IS NULL` — một khoá duy nhất có điều kiện là thứ cho phép cấp lại
 * một giá trị đã cấp, điều luật 7 bất biến 3 cấm. Nên xã muốn quay lại một giờ cũ phải **SỬA
 * dòng đang có**, không xoá rồi thêm lại. Máy chủ trả 409 kèm đúng câu ấy; màn hình đưa nguyên
 * văn ra trang, không viết lại (`goi.ts`).
 *
 * VÌ SAO ĐƯỜNG DẪN TƯƠNG ĐỐI, VÌ SAO KHÔNG ĐỤNG COOKIE, VÌ SAO KHÔNG CÓ `tenant_id`: nói một lần
 * ở `goi.ts`.
 */

import { LOI_KHONG_RO, goiGhi, type KetQua } from "./goi";
import type {
  identity_caLamBuRa,
  identity_caLamViecRa,
  identity_delete_public_holidays_by_id,
  identity_delete_swap_working_days_by_id,
  identity_delete_working_hours_by_id,
  identity_gieoLichRa,
  identity_gieoNgayNghiLeVao,
  identity_ngayNghiLeRa,
  identity_patch_public_holidays_by_id,
  identity_patch_swap_working_days_by_id,
  identity_patch_working_hours_by_id,
  identity_post_public_holidays,
  identity_post_public_holidays_defaults,
  identity_post_swap_working_days,
  identity_post_working_hours,
  identity_post_working_hours_defaults,
  identity_suaCaLamViecVao,
  identity_suaNgayLamBuVao,
  identity_suaNgayNghiLeVao,
  identity_themCaLamViecVao,
  identity_themNgayLamBuVao,
  identity_themNgayNghiLeVao,
  identity_xoaLichVao,
} from "./schema.gen";

/** Thân JSON của một phản hồi đã thành công. Thân hỏng là "không đọc được", không phải 200. */
async function docThanRa<T>(phanHoi: Response): Promise<KetQua<T>> {
  try {
    return { ok: true, duLieu: (await phanHoi.json()) as T };
  } catch {
    return { ok: false, thongBao: LOI_KHONG_RO };
  }
}

/** Đường dẫn của một dòng. `encodeURIComponent` vì id đi vào ĐƯỜNG DẪN, không vào thân. */
function duongDanDong(mau: string, id: string): string {
  return mau.replace("{id}", encodeURIComponent(id));
}

/**
 * DELETE một dòng lịch — 204, không thân. `reason` BẮT BUỘC: luật 7, bất biến 1 nêu ba cột
 * `deleted_at` · `deleted_by` · `delete_reason`.
 *
 * LÝ DO ĐI TRONG THÂN CHỨ KHÔNG TRONG QUERY, và đó là quyết định của máy chủ: một câu tự do do
 * người gõ về một hồ sơ nhà nước nằm trong query string sẽ đi vào mọi access log và mọi bộ đệm
 * proxy.
 */
async function xoaDong(mau: string, id: string, lyDo: string): Promise<KetQua<null>> {
  const than: identity_xoaLichVao = { reason: lyDo };
  const kq = await goiGhi(duongDanDong(mau, id), "DELETE", than, 204);
  return kq.ok ? { ok: true, duLieu: null } : kq;
}

/* ---- ca làm việc trong tuần ---------------------------------------------------------------- */

const GOC_CA_LAM_VIEC: identity_post_working_hours["duongDan"] = "/api/v1/working-hours";
const MAU_CA_LAM_VIEC: identity_patch_working_hours_by_id["duongDan"] &
  identity_delete_working_hours_by_id["duongDan"] = "/api/v1/working-hours/{id}";

/**
 * POST /api/v1/working-hours — thêm một CA vào tuần.
 *
 * MỘT DÒNG LÀ MỘT CA, KHÔNG PHẢI MỘT NGÀY: nghỉ trưa là KHOẢNG HỞ giữa hai ca cùng một thứ, không
 * phải một cờ. Một thứ làm cả ngày có nghỉ trưa là HAI dòng.
 *
 * `weekday` THEO ISO 8601 — 1 = thứ Hai … 7 = Chủ nhật — KHÔNG phải `Date.getDay()` của
 * JavaScript (0 là Chủ nhật). Quy đổi nhầm giữa hai hệ dịch mọi ca đi một ngày và không báo lỗi ở
 * đâu cả; câu từ chối của máy chủ nói rõ điều đó.
 */
export async function themCaLamViec(
  than: identity_themCaLamViecVao,
): Promise<KetQua<identity_caLamViecRa>> {
  const thanGui: identity_themCaLamViecVao = {
    weekday: than.weekday,
    start: than.start,
    end: than.end,
    note: than.note,
  };
  const kq = await goiGhi(GOC_CA_LAM_VIEC, "POST", thanGui, 201);
  return kq.ok ? docThanRa<identity_caLamViecRa>(kq.duLieu) : kq;
}

/** Sửa một ca: trường nào vắng là KHÔNG ĐỔI. Khoá lấy từ hợp đồng, không gõ lại. */
export type SuaCaLamViecVao = {
  [K in keyof identity_suaCaLamViecVao]?: NonNullable<identity_suaCaLamViecVao[K]>;
};

/**
 * PATCH /api/v1/working-hours/{id} — dịch một ca.
 *
 * ⚠ KHÔNG HỒI TỐ lên hạn đã phát ra: hạn nằm trên chính hồ sơ, chốt một lần tại hành vi cố định
 * nó (luật 10, bất biến 2). Sửa giờ làm chỉ đổi cách đếm cho những hồ sơ tiếp nhận SAU khi lưu.
 */
export async function suaCaLamViec(
  id: string,
  than: SuaCaLamViecVao,
): Promise<KetQua<identity_caLamViecRa>> {
  const thanGui: { [K in keyof identity_suaCaLamViecVao]: NonNullable<
    identity_suaCaLamViecVao[K]
  > | undefined } = {
    weekday: than.weekday,
    start: than.start,
    end: than.end,
    note: than.note,
  };
  const kq = await goiGhi(duongDanDong(MAU_CA_LAM_VIEC, id), "PATCH", thanGui, 200);
  return kq.ok ? docThanRa<identity_caLamViecRa>(kq.duLieu) : kq;
}

/** DELETE /api/v1/working-hours/{id} — xoá mềm. Xem cảnh báo giờ mở ca ở đầu tệp. */
export function xoaCaLamViec(id: string, lyDo: string): Promise<KetQua<null>> {
  return xoaDong(MAU_CA_LAM_VIEC, id, lyDo);
}

/**
 * POST /api/v1/working-hours/defaults — gieo tuần làm việc mặc định.
 *
 * KHÔNG GHI ĐÈ: bấm lần thứ hai không đổi một giờ nào xã đã sửa và không dựng lại dòng xã đã cố ý
 * bỏ. Ba con số trả về khác nhau và màn hình phải phân biệt: `seeded` là đã ghi, `kept` là "xã đã
 * có, chúng tôi không đụng vào", `skipped` là "KHÔNG ghi vì ghi vào sẽ làm hỏng lịch" — người đọc
 * nợ một câu khác nhau cho `kept` và cho `skipped`.
 */
export async function gieoTuanMacDinh(): Promise<KetQua<identity_gieoLichRa>> {
  const duongDan: identity_post_working_hours_defaults["duongDan"] =
    "/api/v1/working-hours/defaults";
  // Hợp đồng khai `than: never`: không thân, không `Content-Type`.
  const kq = await goiGhi(duongDan, "POST", undefined, 200);
  return kq.ok ? docThanRa<identity_gieoLichRa>(kq.duLieu) : kq;
}

/* ---- ngày nghỉ lễ -------------------------------------------------------------------------- */

const GOC_NGAY_NGHI_LE: identity_post_public_holidays["duongDan"] = "/api/v1/public-holidays";
const MAU_NGAY_NGHI_LE: identity_patch_public_holidays_by_id["duongDan"] &
  identity_delete_public_holidays_by_id["duongDan"] = "/api/v1/public-holidays/{id}";

/**
 * POST /api/v1/public-holidays — thêm một ngày xã đóng cửa.
 *
 * `date` LÀ `YYYY-MM-DD`, MỘT NGÀY — không mang giờ, không mang múi giờ. Một mốc ISO-8601 bị từ
 * chối, và khác biệt là MỘT NGÀY chứ không phải cách viết: `2026-09-02T00:00:00Z` hiện ở phía tây
 * London thành ngày 01, và một ngày nghỉ lệch một ngày là một hạn đếm xuyên qua ngày trụ sở đóng
 * cửa.
 */
export async function themNgayNghiLe(
  than: identity_themNgayNghiLeVao,
): Promise<KetQua<identity_ngayNghiLeRa>> {
  const thanGui: identity_themNgayNghiLeVao = { date: than.date, name: than.name };
  const kq = await goiGhi(GOC_NGAY_NGHI_LE, "POST", thanGui, 201);
  return kq.ok ? docThanRa<identity_ngayNghiLeRa>(kq.duLieu) : kq;
}

export type SuaNgayNghiLeVao = {
  [K in keyof identity_suaNgayNghiLeVao]?: NonNullable<identity_suaNgayNghiLeVao[K]>;
};

/** PATCH /api/v1/public-holidays/{id}. Vắng trường nào là không đổi trường ấy. */
export async function suaNgayNghiLe(
  id: string,
  than: SuaNgayNghiLeVao,
): Promise<KetQua<identity_ngayNghiLeRa>> {
  const thanGui: { [K in keyof identity_suaNgayNghiLeVao]: NonNullable<
    identity_suaNgayNghiLeVao[K]
  > | undefined } = { date: than.date, name: than.name };
  const kq = await goiGhi(duongDanDong(MAU_NGAY_NGHI_LE, id), "PATCH", thanGui, 200);
  return kq.ok ? docThanRa<identity_ngayNghiLeRa>(kq.duLieu) : kq;
}

/** DELETE /api/v1/public-holidays/{id} — xoá mềm. Ngày đã khai vẫn bị chiếm sau khi xoá. */
export function xoaNgayNghiLe(id: string, lyDo: string): Promise<KetQua<null>> {
  return xoaDong(MAU_NGAY_NGHI_LE, id, lyDo);
}

/**
 * POST /api/v1/public-holidays/defaults — gieo ngày nghỉ lễ CỐ ĐỊNH THEO DƯƠNG LỊCH của một năm.
 *
 * ⚠ BỐN NGÀY, KHÔNG PHẢI MƯỜI MỘT, VÀ MÀN HÌNH NỢ NGƯỜI BẤM MỘT CÂU. Tết Nguyên đán và Giỗ Tổ
 * Hùng Vương theo ÂM LỊCH — một phép quy đổi tự viết sai một ngày là một cam kết sai với người
 * dân — còn ngày liền kề 02/9 do Thủ tướng chọn từng năm giữa 01/9 và 03/9. Xã tự nhập cả ba theo
 * thông báo hằng năm. `seeded: 4` đứng một mình đọc ra là "xong", nên câu nói rõ còn thiếu gì
 * nằm ở `features/cau-hinh/nhan-thoi-han.ts` và hiện ngay cạnh kết quả gieo.
 *
 * NĂM LÀ BẮT BUỘC, và "năm nay" không phải mặc định mã này được phép chọn: đúng 0h ngày 01/01 một
 * màn hình không nêu năm sẽ lặng lẽ gieo sang năm khác mà không dòng mã nào thay đổi.
 */
export async function gieoNgayNghiLeMacDinh(nam: number): Promise<KetQua<identity_gieoLichRa>> {
  const duongDan: identity_post_public_holidays_defaults["duongDan"] =
    "/api/v1/public-holidays/defaults";
  const than: identity_gieoNgayNghiLeVao = { year: nam };
  const kq = await goiGhi(duongDan, "POST", than, 200);
  return kq.ok ? docThanRa<identity_gieoLichRa>(kq.duLieu) : kq;
}

/* ---- ngày làm bù --------------------------------------------------------------------------- */

const GOC_NGAY_LAM_BU: identity_post_swap_working_days["duongDan"] = "/api/v1/swap-working-days";
const MAU_NGAY_LAM_BU: identity_patch_swap_working_days_by_id["duongDan"] &
  identity_delete_swap_working_days_by_id["duongDan"] = "/api/v1/swap-working-days/{id}";

/**
 * POST /api/v1/swap-working-days — thêm một CA làm bù.
 *
 * GIỜ NẰM TRÊN CHÍNH DÒNG, không lấy từ lịch tuần — vì thứ mà một ngày làm bù rơi vào thường
 * không có ca nào cả. Và một dòng vẫn là một CA: ngày làm bù có nghỉ trưa là hai dòng cùng ngày.
 */
export async function themNgayLamBu(
  than: identity_themNgayLamBuVao,
): Promise<KetQua<identity_caLamBuRa>> {
  const thanGui: identity_themNgayLamBuVao = {
    date: than.date,
    start: than.start,
    end: than.end,
    name: than.name,
  };
  const kq = await goiGhi(GOC_NGAY_LAM_BU, "POST", thanGui, 201);
  return kq.ok ? docThanRa<identity_caLamBuRa>(kq.duLieu) : kq;
}

export type SuaNgayLamBuVao = {
  [K in keyof identity_suaNgayLamBuVao]?: NonNullable<identity_suaNgayLamBuVao[K]>;
};

/** PATCH /api/v1/swap-working-days/{id}. Vắng trường nào là không đổi trường ấy. */
export async function suaNgayLamBu(
  id: string,
  than: SuaNgayLamBuVao,
): Promise<KetQua<identity_caLamBuRa>> {
  const thanGui: { [K in keyof identity_suaNgayLamBuVao]: NonNullable<
    identity_suaNgayLamBuVao[K]
  > | undefined } = { date: than.date, start: than.start, end: than.end, name: than.name };
  const kq = await goiGhi(duongDanDong(MAU_NGAY_LAM_BU, id), "PATCH", thanGui, 200);
  return kq.ok ? docThanRa<identity_caLamBuRa>(kq.duLieu) : kq;
}

/** DELETE /api/v1/swap-working-days/{id} — xoá mềm. Giờ mở ca của ngày ấy vẫn bị chiếm. */
export function xoaNgayLamBu(id: string, lyDo: string): Promise<KetQua<null>> {
  return xoaDong(MAU_NGAY_LAM_BU, id, lyDo);
}
