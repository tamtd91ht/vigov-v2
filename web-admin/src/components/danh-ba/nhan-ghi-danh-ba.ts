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

// Chỉ KIỂU, nên vòng nhập với component bị xoá khi biên dịch.
import type { DangMoGhi } from "./bieu-mau-ghi-can-bo";

/* ---- nhãn các ô nhập ----------------------------------------------------------------------- */

export const O_HO_TEN = "Họ và tên";
export const O_CHUC_DANH = "Chức danh";
export const O_EMAIL = "Thư điện tử công vụ";

/**
 * Hint under the email field. The field is OPTIONAL since 4cf87b6 (a commune often lists staff who
 * have no official mailbox yet), and the one consequence the person typing must know is the account:
 * the email IS the login name, so without it `POST .../account` answers 409 `staff_has_no_email`.
 */
export const EMAIL_HINT =
  "Không bắt buộc. Cán bộ chưa có thư điện tử thì chưa cấp được tài khoản đăng nhập " +
  "(thư điện tử là tên đăng nhập).";
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

/** Ô tick "Có Zalo" (đặc tả §5) và câu nói rõ nó KHÔNG công khai gì. */
export const O_CO_ZALO = "Số di động này có dùng Zalo";
export const MO_TA_CO_ZALO =
  "Chỉ ghi nhận để đồng nghiệp biết có thể liên hệ qua Zalo. Việc này không đưa số lên Zalo Mini App.";

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
 * MÀN `/nguoi-dung` (trước 05/10/2026 là màn `/nguoi-dung` (trước 05/10/2026 là tab `Cấu hình → Người dùng`)) KHÔNG CÓ NÚT XOÁ, và đây là hằng ghi lại lý do ở chỗ người ta sẽ đi
 * tìm nó.
 *
 * Câu mở #10 (chốt 22/09/2026) tách KHOÁ khỏi XOÁ: nghỉ hưu / chuyển công tác là KHOÁ, người vẫn
 * còn trong danh bạ; xoá một dòng nhập trùng là XOÁ MỀM và mang QUYỀN RIÊNG (`admin.user.delete`,
 * gieo ở migration 0010, ADR 0035). Nút xoá dòng trùng nằm ở màn `/danh-ba`, sau quyền ấy — không ở
 * tab này, nơi mọi nút đều đứng sau `admin.user`.
 *
 * Câu dưới hiện trên tab ấy, nên phải nói đúng chỗ việc xoá được làm: "chưa mở" đã sai từ 24/09/2026.
 */
export const VI_SAO_KHONG_CO_NUT_XOA =
  "Cán bộ nghỉ hưu hoặc chuyển công tác thì khoá tài khoản, không xoá: hồ sơ đã xử lý phải còn " +
  "đọc được tên người thực hiện. Dòng nhập trùng (không có tài khoản đăng nhập) được xoá ở màn " +
  "Danh bạ cán bộ, bởi người có quyền riêng cho việc ấy.";

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

/* ---- hai việc gỡ một người khỏi Mini App mà không qua nút "Rút" ------------------------------ */

/**
 * Quyết định của khách, 28/09/2026: ĐỔI SỐ DI ĐỘNG CÁ NHÂN của một người đang hiện trên Mini App thì
 * máy chủ TỰ GỠ người ấy khỏi Mini App, vì sự đồng ý đã hỏi là đồng ý công khai SỐ CŨ — không phải
 * số vừa gõ. Muốn hiện lại, phải hỏi ý lại và bấm công khai lại ở màn Danh bạ cán bộ.
 *
 * CHỈ LÀ LỜI BÁO, KHÔNG PHẢI PHÉP CHẶN: máy chủ là nơi gỡ thật (`service-identity`). Câu này có mặt
 * để người sửa hồ sơ không bấm Lưu rồi mới phát hiện đồng nghiệp biến khỏi danh bạ của bà con.
 */
export const CANH_BAO_DOI_DI_DONG_CONG_KHAI =
  "Cán bộ này đang hiện trên danh bạ Zalo Mini App. Đổi số di động sẽ gỡ cán bộ này khỏi danh bạ " +
  "trên Mini App. Muốn hiện lại, phải hỏi ý và bấm công khai lại.";

/**
 * Có hiện `CANH_BAO_DOI_DI_DONG_CONG_KHAI` hay không: chỉ ở biểu mẫu SỬA, người ĐANG công khai, và ô
 * di động KHÁC giá trị đang lưu.
 *
 * SO CHUỖI THÔ, KHÔNG CHUẨN HOÁ. Máy chủ lọc ký tự của số điện thoại theo quy tắc của nó
 * (`domain/danh_ba_ghi.go`); chép quy tắc ấy xuống đây là bản sao thứ hai sẽ trôi. Hệ quả của so
 * thô là báo THỪA (thêm một dấu cách cũng hiện lời báo) — chiều sai vô hại. Chiều ngược lại, báo
 * THIẾU, là để một người biến khỏi Mini App mà không ai được nói trước.
 */
export function coCanhBaoDoiDiDong(dangMo: DangMoGhi, ban: BanNhapCanBo): boolean {
  if (dangMo.kieu !== "sua") return false;
  return dangMo.canBo.published && ban.diDongCaNhan !== dangMo.canBo.mobile;
}

/**
 * Cùng quyết định 28/09/2026: KHOÁ một người đang hiện trên Mini App cũng gỡ người ấy, và MỞ KHOÁ
 * KHÔNG hiện lại. Nửa thứ hai là nửa phải nói ra: người mở khoá sẽ tưởng mọi thứ trở về như cũ.
 */
export const CANH_BAO_KHOA_CONG_KHAI =
  "Cán bộ này đang hiện trên danh bạ Zalo Mini App. Khoá tài khoản sẽ gỡ cán bộ này khỏi danh bạ " +
  "trên Mini App, và mở khoá sau đó không tự hiện lại. Muốn hiện lại, phải hỏi ý và bấm công khai lại.";

/** Có hiện `CANH_BAO_KHOA_CONG_KHAI` hay không: chỉ khi KHOÁ (không phải mở khoá) một người đang công khai. */
export function coCanhBaoKhoaCongKhai(dangMo: DangMoGhi): boolean {
  return dangMo.kieu === "khoa" && dangMo.khoa && dangMo.canBo.published;
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
 * Bảy ô của biểu mẫu thêm/sửa — sáu ô hồ sơ và ô "Có Zalo" (chỉ hiện ở biểu mẫu SỬA).
 *
 * KHÔNG CÓ `ma`, KHÔNG CÓ `vaiTro`, KHÔNG CÓ `dangHoatDong`. Ba thứ ấy vắng mặt vì ba lý do khác
 * nhau và không lý do nào là "chưa làm tới": mã do hệ thống sinh (#15); vai trò đi qua tuyến
 * riêng mang hai ràng buộc của #14; trạng thái khoá đi qua tuyến riêng mang phép chặn của #13.
 * Thêm một trường vào kiểu này là mở lại đúng một trong ba cánh cửa ấy.
 *
 * KHÔNG CÓ Ô BẬT/TẮT CÔNG KHAI LÊN MINI APP, dù nay tuyến ấy đã có: công khai là
 * `PUT .../publication`, mang quyền RIÊNG (`content.update`, không phải `admin.user`) và bắt buộc
 * tick xác nhận đã hỏi ý từng người (#12). Một ô tick lẫn trong biểu mẫu sửa hồ sơ sẽ gộp hai quyền
 * vào một lần Lưu — và một lần sửa số điện thoại có thể công khai luôn số ấy.
 *
 * `coZalo` KHÔNG CÔNG KHAI GÌ: nó chỉ ghi nhận số di động có dùng Zalo, là thông tin liên hệ
 * (`can_bo_ghi.go`: `HasZalo` thuộc `admin.user`). Không có trong thân THÊM vì hợp đồng
 * `identity_themCanBoVao` không có trường ấy.
 */
export type BanNhapCanBo = {
  hoTen: string;
  chucDanh: string;
  email: string;
  boPhanID: string;
  mayBanCoQuan: string;
  diDongCaNhan: string;
  coZalo: boolean;
};

export const BAN_TRONG: BanNhapCanBo = {
  hoTen: "",
  chucDanh: "",
  email: "",
  boPhanID: "",
  mayBanCoQuan: "",
  diDongCaNhan: "",
  coZalo: false,
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
    coZalo: cb.has_zalo,
  };
}

/**
 * Thân `POST /api/v1/staff`. Năm trường bắt buộc của hợp đồng cộng `email` tuỳ chọn.
 *
 * A BLANK EMAIL IS OMITTED, not sent as `""`. The server treats absent, `""` and whitespace alike
 * (stored as no email), so either would work today — but omitting is the one shape the contract
 * declares (`email?: string`), and it keeps the body free of a value nobody typed. This is not a
 * format check: a non-blank value goes up verbatim and the server validates it.
 */
export function thanThem(ban: BanNhapCanBo): identity_themCanBoVao {
  const than: identity_themCanBoVao = {
    full_name: ban.hoTen,
    position: ban.chucDanh,
    org_unit_id: ban.boPhanID,
    office_phone: ban.mayBanCoQuan,
    mobile: ban.diDongCaNhan,
  };
  if (ban.email.trim() !== "") than.email = ban.email;
  return than;
}

/**
 * The `email` of a PATCH body.
 *
 * `""` ONLY WHEN THE USER CLEARED AN EMAIL THAT WAS THERE. For the server `""` means "remove the
 * email", which it refuses (409 `staff_email_is_login`) for anyone holding a login account. Sending
 * `""` for a row that never had an email would be a no-op write the audit trail records as a
 * change, so that case sends `null` — "unchanged". A whitespace-only field counts as blank.
 */
function patchEmail(typed: string, stored: string): string | null {
  if (typed.trim() !== "") return typed;
  return stored.trim() !== "" ? "" : null;
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
 *
 * EXCEPTION: `email`. Clearing it is refused for staff with a login account, so it follows
 * `patchEmail` — `""` only when a stored email was cleared, `null` when it was and stays blank.
 *
 * `has_zalo` THÌ NGƯỢC LẠI: CHỈ GỬI KHI ĐỔI so với dòng gốc `goc`. Nó là trường TUỲ CHỌN của hợp
 * đồng (vắng = không đổi), và một ô tick không có ca "xoá trắng" để lẫn với "không đổi" như ô chữ.
 * Gửi nó ở mọi lần Lưu thì một lần sửa chức danh ghi đè cờ Zalo mà người khác vừa đặt, và vết kiểm
 * toán ghi một thay đổi không ai làm.
 */
export function thanSua(ban: BanNhapCanBo, goc: identity_canBoTomTat): identity_suaCanBoVao {
  const than: identity_suaCanBoVao = {
    full_name: ban.hoTen,
    position: ban.chucDanh,
    email: patchEmail(ban.email, goc.email),
    org_unit_id: ban.boPhanID,
    office_phone: ban.mayBanCoQuan,
    mobile: ban.diDongCaNhan,
  };
  if (ban.coZalo !== goc.has_zalo) than.has_zalo = ban.coZalo;
  return than;
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
