/**
 * Câu chữ và phép đổi hình dạng cho ĐƯỜNG GHI của danh bạ cán bộ.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * TÁCH RA KHỎI COMPONENT VÌ MỘT LÝ DO ĐÃ ĐO ĐƯỢC, không phải vì gọn: một quyết định nằm trong
 * module thuần thì kiểm được bằng một phép so chuỗi, còn cùng quyết định ấy viết thẳng trong JSX
 * thì chỉ kiểm được bằng cách kết xuất cả cây. Cả hai loại phép kiểm đều có mặt ở đây — module
 * này giữ QUYẾT ĐỊNH, còn `bieu-mau-ghi-can-bo.tsx` có bài kiểm riêng canh việc quyết định ấy
 * thật sự RA TỚI TRANG (`vitest.config.mts` giải thích vì sao cần cả hai).
 *
 * KHÔNG CÓ PHÉP KIỂM ĐẦU VÀO NÀO Ở ĐÂY, VÀ ĐÓ LÀ CHỦ Ý. Máy chủ đã chuẩn hoá và từ chối từng
 * trường kèm một câu tiếng Việt nói rõ phải sửa gì: gom khoảng trắng trong họ tên, hạ chữ thường
 * thư điện tử (vì `UNIQUE (tenant_id, email)` phân biệt hoa thường), lọc ký tự không thể có trong
 * một số điện thoại (`service-identity/internal/domain/danh_ba_ghi.go`). Chép bộ quy tắc ấy xuống
 * client là dựng bản sao thứ hai của một quy tắc nghiệp vụ — và bản sao ấy trôi mà không bài test
 * nào đỏ (luật 9, cấm #2).
 * ─────────────────────────────────────────────────────────────────────────────────────────
 */

import type {
  identity_canBoTomTat,
  identity_suaCanBoVao,
  identity_themCanBoVao,
} from "@/lib/api/schema.gen";

/* ---- nhãn các ô nhập ----------------------------------------------------------------------- */

export const O_HO_TEN = "Họ và tên";
export const O_CHUC_DANH = "Chức danh";
export const O_EMAIL = "Thư điện tử công vụ";
export const O_BO_PHAN = "Bộ phận";
export const O_VAI_TRO = "Vai trò";

/**
 * HAI NHÃN ĐIỆN THOẠI, MỖI NHÃN NÓI RÕ LOẠI SỐ — câu mở #16, khách chốt 22/09/2026.
 *
 * Máy bàn cơ quan là THÔNG TIN CÔNG VỤ; di động cá nhân là DỮ LIỆU CÁ NHÂN theo Nghị định 13. Hai
 * địa vị pháp lý khác nhau nghĩa là hai luật che, hai luật xuất Excel, hai luật công khai ra Mini
 * App. Một nhãn trung tính "Điện thoại" là chỗ người đang gõ không biết mình đang nhập loại nào —
 * và một ô nhập nhãn sai là cách hai loại số bị đổi chỗ cho nhau ngay từ lúc nhập, tức trước cả
 * khi có luật che nào chạy.
 */
export const O_MAY_BAN = "Máy bàn cơ quan";
export const O_DI_DONG = "Di động cá nhân";

/** Mục "không chọn gì" của hai ô chọn. `""` là một giá trị THẬT của hợp đồng, không phải chỗ trống. */
export const CHON_KHONG_BO_PHAN = "— Chưa phân bộ phận —";
export const CHON_KHONG_VAI_TRO = "— Không giữ vai trò nào —";

/* ---- nhãn các nút -------------------------------------------------------------------------- */

export const NUT_THEM_CAN_BO = "Thêm cán bộ";
export const NUT_SUA = "Sửa hồ sơ";
export const NUT_DOI_VAI_TRO = "Đổi vai trò";
export const NUT_KHOA = "Khoá tài khoản";
export const NUT_MO_KHOA = "Mở khoá tài khoản";
export const NUT_LUU = "Lưu";
export const NUT_HUY = "Huỷ";
export const NUT_XAC_NHAN_KHOA = "Xác nhận khoá";
export const NUT_XAC_NHAN_MO_KHOA = "Xác nhận mở khoá";

/**
 * MÀN HÌNH NÀY KHÔNG CÓ NÚT XOÁ, và đây là hằng ghi lại lý do ở chỗ người ta sẽ đi tìm nó.
 *
 * Câu mở #10 (chốt 22/09/2026) tách KHOÁ khỏi XOÁ: nghỉ hưu / chuyển công tác là KHOÁ, người vẫn
 * còn trong danh bạ; xoá một dòng nhập trùng là XOÁ MỀM và mang QUYỀN RIÊNG. Bảng `quyen` không
 * có khoá nào mang nghĩa ấy, nên tuyến xoá chưa tồn tại — và mượn tạm `admin.user` cho nó chính
 * là hình dạng #10 vừa từ chối (luật 5, bất biến 3c). Đó là phát hiện cho câu mở #27.
 *
 * Vẽ một nút `🗑 Xoá khỏi danh bạ` như đặc tả (`docs/ui-ux/12-danh-ba-can-bo.md:61`) là một lời
 * hứa suông: người quản trị bấm, nhận 404 hoặc 403, và kết luận hệ thống hỏng — trong khi thứ
 * đang thiếu là một quyết định của khách.
 */
export const VI_SAO_KHONG_CO_NUT_XOA =
  "Cán bộ nghỉ hưu hoặc chuyển công tác thì khoá tài khoản, không xoá: hồ sơ đã xử lý phải còn " +
  "đọc được tên người thực hiện. Việc xoá một dòng nhập trùng là thao tác khác và chưa mở.";

/* ---- tiêu đề và câu giải thích của từng biểu mẫu -------------------------------------------- */

export function tieuDeThem(): string {
  return "Thêm cán bộ vào danh bạ";
}

export function tieuDeSua(hoTen: string): string {
  return `Sửa hồ sơ: ${hoTen}`;
}

export function tieuDeVaiTro(hoTen: string): string {
  return `Đổi vai trò: ${hoTen}`;
}

export function tieuDeKhoa(hoTen: string, khoa: boolean): string {
  return khoa ? `Khoá tài khoản: ${hoTen}` : `Mở khoá tài khoản: ${hoTen}`;
}

/**
 * Hai điều người bấm "Thêm cán bộ" cần biết TRƯỚC khi gõ, vì cả hai đều trái với trực giác.
 *
 * Không có ô Mã (#15): mã do hệ thống cấp, và mã đã cấp thì không bao giờ cấp lại kể cả sau xoá
 * mềm (luật 7, bất biến 3). Người quen nhập hồ sơ nhân sự sẽ đi tìm ô ấy nếu không ai nói.
 *
 * Thêm vào danh bạ KHÔNG phải là cấp tài khoản (#9): máy chủ ghi `co_tai_khoan = false` làm hằng.
 * Không nói ra thì một xã tưởng đã tạo xong người dùng và ngồi chờ người ấy đăng nhập.
 */
export const GIAI_THICH_THEM =
  "Mã cán bộ do hệ thống cấp sau khi lưu, nên không có ô nhập mã. Thêm vào danh bạ chưa phải là " +
  "cấp tài khoản đăng nhập — đó là một việc riêng.";

/**
 * Hậu quả của việc khoá, nói ra TRƯỚC khi bấm (accessibility-elderly, REQUIRED #7).
 *
 * Câu này phải nói cả hai nửa. Nửa thứ hai — "vẫn còn trong danh bạ" — là nửa hay bị hiểu sai
 * nhất: người bấm đang nghĩ mình đang xoá một người khỏi hệ thống, còn thứ thật sự xảy ra là
 * người ấy không đăng nhập được nữa mà tên vẫn đọc được trên mọi hồ sơ cũ (#10).
 */
export function canhBaoKhoa(khoa: boolean): string {
  return khoa
    ? "Sau khi khoá, cán bộ này không đăng nhập được nữa. Người này VẪN CÒN trong danh bạ và tên " +
        "vẫn hiện trên những hồ sơ đã xử lý."
    : "Sau khi mở khoá, cán bộ này đăng nhập lại được bình thường.";
}

/* ---- câu xác nhận sau khi ghi xong ---------------------------------------------------------- */

export function daThem(hoTen: string): string {
  return `Đã thêm ${hoTen} vào danh bạ.`;
}

export function daLuuHoSo(hoTen: string): string {
  return `Đã lưu hồ sơ của ${hoTen}.`;
}

export function daDoiVaiTro(hoTen: string): string {
  return `Đã đổi vai trò của ${hoTen}.`;
}

export function daDatKhoa(hoTen: string, khoa: boolean): string {
  return khoa ? `Đã khoá tài khoản của ${hoTen}.` : `Đã mở khoá tài khoản của ${hoTen}.`;
}

/* ---- bản nháp của biểu mẫu ------------------------------------------------------------------ */

/**
 * Sáu ô của biểu mẫu thêm/sửa — và đúng sáu, không hơn.
 *
 * KHÔNG CÓ `ma`, KHÔNG CÓ `vaiTro`, KHÔNG CÓ `dangHoatDong`. Ba thứ ấy vắng mặt vì ba lý do khác
 * nhau và không lý do nào là "chưa làm tới": mã do hệ thống sinh (#15); vai trò đi qua tuyến
 * riêng mang hai ràng buộc của #14; trạng thái khoá đi qua tuyến riêng mang phép chặn của #13.
 * Thêm một trường vào kiểu này là mở lại đúng một trong ba cánh cửa ấy.
 *
 * KHÔNG CÓ Ô BẬT/TẮT CÔNG KHAI LÊN MINI APP, và sự vắng mặt ấy là có chủ ý: câu mở #12 chốt phải
 * HỎI Ý từng người và LƯU LẠI sự đồng ý kèm thời điểm, mà lược đồ chưa có cột nào giữ nó. Một ô
 * tick ở đây sẽ công khai số di động cá nhân của một người ra kênh công khai mà không có bằng
 * chứng nào cho thấy người ấy đã đồng ý — và phần đã công khai thì không thu lại được.
 */
export type BanNhapCanBo = {
  hoTen: string;
  chucDanh: string;
  email: string;
  boPhanID: string;
  mayBanCoQuan: string;
  diDongCaNhan: string;
};

export const BAN_TRONG: BanNhapCanBo = {
  hoTen: "",
  chucDanh: "",
  email: "",
  boPhanID: "",
  mayBanCoQuan: "",
  diDongCaNhan: "",
};

/**
 * Nạp một dòng danh bạ đã đọc vào bản nháp của biểu mẫu sửa.
 *
 * HAI CẶP TÊN TRƯỜNG LỆCH NHAU GIỮA ĐƯỜNG ĐỌC VÀ ĐƯỜNG GHI, và hàm này là chỗ DUY NHẤT dịch
 * chúng — vì vậy nó có bài kiểm riêng:
 *
 *   đọc `phone`         ↔ ghi `office_phone`
 *   đọc `department_id` ↔ ghi `org_unit_id`
 *
 * Sự lệch ấy đến từ hợp đồng chứ không từ đây (`kb/20-contracts/openapi.json`), nên không sửa
 * được ở phía web. Chỗ nguy hiểm là cặp thứ nhất: `phone` và `mobile` cùng kiểu chuỗi, cùng trông
 * như một số điện thoại, nên một lần lắp nhầm biên dịch sạch, kết xuất đẹp, và ghi số di động cá
 * nhân của một người vào ô máy bàn cơ quan — tức đổi địa vị pháp lý của dữ liệu ấy (#16).
 */
export function banTuCanBo(cb: identity_canBoTomTat): BanNhapCanBo {
  return {
    hoTen: cb.full_name,
    chucDanh: cb.position,
    email: cb.email,
    boPhanID: cb.department_id,
    mayBanCoQuan: cb.phone,
    diDongCaNhan: cb.mobile,
  };
}

/** Thân `POST /api/v1/staff`. Sáu trường bắt buộc của hợp đồng, không thừa một trường nào. */
export function thanThem(ban: BanNhapCanBo): identity_themCanBoVao {
  return {
    full_name: ban.hoTen,
    position: ban.chucDanh,
    email: ban.email,
    org_unit_id: ban.boPhanID,
    office_phone: ban.mayBanCoQuan,
    mobile: ban.diDongCaNhan,
  };
}

/**
 * Thân `PATCH /api/v1/staff/{id}`.
 *
 * GỬI ĐỦ SÁU TRƯỜNG, KHÔNG GỬI `null` CHO Ô KHÔNG ĐỔI. Biểu mẫu sửa hiện cả sáu ô và nạp sẵn giá
 * trị đang có, nên sáu giá trị đi lên đều là thứ người dùng vừa nhìn thấy trên màn hình. Một
 * nhánh "chỉ gửi ô đã đổi" ở đây sẽ phải so bản nháp với bản gốc, và phép so ấy là chỗ một ô vừa
 * bị XOÁ TRẮNG có chuỗi rỗng trông giống một ô không đổi — tức sửa xong thì số điện thoại vẫn còn
 * đó, và không ai báo gì.
 *
 * `null` của hợp đồng vẫn có nghĩa "không đổi" (máy chủ đọc sáu con trỏ), nhưng biểu mẫu này
 * không có trạng thái ấy: `""` là "ô này trống", một giá trị hợp lệ với chức danh, bộ phận và hai
 * số điện thoại.
 */
export function thanSua(ban: BanNhapCanBo): identity_suaCanBoVao {
  return {
    full_name: ban.hoTen,
    position: ban.chucDanh,
    email: ban.email,
    org_unit_id: ban.boPhanID,
    office_phone: ban.mayBanCoQuan,
    mobile: ban.diDongCaNhan,
  };
}

/**
 * Sinh một khoá chống trùng cho một lần mở biểu mẫu thêm.
 *
 * `crypto.randomUUID` có sẵn trong mọi trình duyệt chạy được ứng dụng này và không cần thư viện.
 * Gọi ở chỗ MỞ biểu mẫu chứ không ở chỗ GỬI: xem `themCanBo` trong `lib/api/can-bo.ts`.
 */
export function khoaChongTrungMoi(): string {
  return crypto.randomUUID();
}
