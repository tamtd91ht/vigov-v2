/**
 * Ba tuyến của thông tin đăng nhập cán bộ — cấp · đặt lại · tự đổi.
 *
 * TÁCH KHỎI `can-bo.ts` THEO THỨ CHÚNG CHẠM VÀO, không theo màn hình nào gọi. `can-bo.ts` là
 * DANH BẠ: họ tên, chức danh, bộ phận, vai trò — dữ liệu một cơ quan công bố nội bộ. Tệp này là
 * THÔNG TIN ĐĂNG NHẬP, và một giá trị ở đây lọt ra là một tài khoản bị chiếm, không phải một
 * dòng danh bạ hiện sai. Hai mức hậu quả khác nhau thì để cạnh nhau là mời người sau đọc lướt và
 * xử lý chúng như nhau.
 *
 *   POST /api/v1/staff/{id}/account     quản trị viên cấp tài khoản (201)
 *   PUT  /api/v1/staff/{id}/password    quản trị viên đặt lại hộ (#17, 200)
 *   PUT  /api/v1/staff/current/password tự đổi (204) — tuyến DUY NHẤT gỡ cờ bắt đổi
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * MẬT KHẨU TẠM VỀ TỚI ĐÂY LÀ MỘT GIÁ TRỊ SỐNG ĐÚNG MỘT LẦN, và máy chủ KHÔNG dựng lại được nó:
 * CSDL chỉ giữ chuỗi băm argon2id, không có tuyến GET nào trả nó, không đường mã nào tính lại
 * được. Mất thì phải ĐẶT LẠI — sinh giá trị khác và để lại vết thứ hai.
 *
 * Nên ba điều dưới đây không phải lời khuyên, chúng là điều kiện để tệp này đúng:
 *
 *   1. KHÔNG `console.*` BẤT KỲ ĐÂU trong tệp này — kể cả trong nhánh `catch` mạng hỏng. Console
 *      của trình duyệt đi vào ảnh chụp màn hình cán bộ gửi cho hỗ trợ, và vào mọi bộ thu lỗi
 *      phía client mà kho này sẽ gắn một ngày nào đó (luật 3, cấm #1).
 *   2. KHÔNG `localStorage`, `sessionStorage`, không biến module giữ lại giá trị. Bề mặt duy
 *      nhất được giữ nó là state của đúng một thành phần React đang hiện nó, và biến mất khi
 *      thành phần ấy đóng.
 *   3. KHÔNG đưa nó vào URL, vào `title`, vào `aria-label` — luật 3, cấm #4.
 *
 * TỆP NÀY KHÔNG TỰ ĐI ĐÂU CẢ SAU KHI GHI. Không `router.push`, không `location.reload`. Một lần
 * điều hướng ngay sau `capTaiKhoan` là lần mật khẩu tạm biến mất khỏi màn hình trước khi quản
 * trị viên kịp đọc — và không có cách nào lấy lại. Màn hình quyết định khi nào rời đi.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 */

import { goiGhi, LOI_KHONG_RO, type KetQua } from "./goi";
import type {
  identity_capTaiKhoanRa,
  identity_doiMatKhauVao,
  identity_post_staff_by_id_account,
  identity_put_staff_by_id_password,
  identity_put_staff_current_password,
} from "./schema.gen";

/** Đường dẫn của một cán bộ cụ thể. `encodeURIComponent` vì id đi vào ĐƯỜNG DẪN, không vào thân. */
function duongDanMotCanBo(mau: string, id: string): string {
  return mau.replace("{id}", encodeURIComponent(id));
}

/**
 * Đọc `capTaiKhoanRa` từ một lần ghi thành công.
 *
 * `goiGhi` trả `Response` thô, nên phần đọc kiểu nằm ở đây — cùng lý do `can-bo.ts` có lớp mỏng
 * của nó: `DELETE` danh mục thành công bằng 204 KHÔNG THÂN, nên hàm dùng chung không được phép
 * luôn gọi `.json()`.
 */
async function docCapTaiKhoan(
  kq: KetQua<Response>,
): Promise<KetQua<identity_capTaiKhoanRa>> {
  if (!kq.ok) return kq;

  // MỘT LẦN PHÁT LẠI CHỐNG TRÙNG KHÔNG MANG MẬT KHẨU VỀ, và nó về với ĐÚNG mã 200/201 của lần
  // đầu. `core/idem` cố ý không lưu thân của lần trả lời đầu tiên — *"the first response's body
  // is never stored, so it cannot be reproduced, and must not be"* (`core/idem/idem.go:421-422`) —
  // nên thân của lần phát lại là `{"code":…,"replayed":true}`.
  //
  // KHÔNG PHÁT HIỆN BẰNG CÁCH ĐOÁN HÌNH DẠNG THÂN. `.json()` chạy trót lọt, TypeScript ép kiểu
  // sạch sang `identity_capTaiKhoanRa`, và bên gọi nhận `ok: true` với `temporary_password` lẫn
  // `staff` đều `undefined` — `kq.duLieu.staff.full_name` viết theo cách hiển nhiên là một
  // `TypeError` ném giữa `then`, không ai bắt, màn hình đứng im. Máy chủ đã phát sẵn tín hiệu
  // cho đúng việc này: `Idempotent-Replay: true` (`core/idem`, `HeaderPhatLai`), đọc được mà
  // không cần chạm vào thân.
  //
  // TRẢ `ok: false` DÙ HTTP LÀ 200, và đó là lựa chọn có chủ ý: xét theo giao thức thì lời gọi
  // thành công, nhưng xét theo việc mà bên gọi cần — cầm một mật khẩu tạm để đọc cho cán bộ —
  // thì không có gì cả. Một `ok: true` ở đây là mời màn hình mở ô mật khẩu rỗng.
  if (kq.duLieu.headers.get("Idempotent-Replay") === "true") {
    return {
      ok: false,
      thongBao:
        "Máy chủ cho biết yêu cầu này đã được xử lý trước đó, nên không gửi kèm mật khẩu tạm " +
        "— mật khẩu chỉ đi ra đúng một lần, ở lần gọi đầu tiên. Hãy đóng ô này rồi bấm Đặt lại " +
        "mật khẩu một lần nữa: lần ấy sinh một mật khẩu KHÁC và hiện ra cho bạn đọc.",
    };
  }

  try {
    return { ok: true, duLieu: (await kq.duLieu.json()) as identity_capTaiKhoanRa };
  } catch {
    // Đúng mã mong đợi mà thân không đọc được là chỗ TỆ NHẤT của cả tệp này: máy chủ CÓ THỂ đã
    // cấp tài khoản và sinh mật khẩu tạm, mà giá trị ấy vừa mất vĩnh viễn. Câu thông báo không
    // được nói "thất bại" — bên gọi phải mời quản trị viên mở lại dòng và ĐẶT LẠI mật khẩu.
    return { ok: false, thongBao: LOI_KHONG_RO };
  }
}

/**
 * POST /api/v1/staff/{id}/account — cấp tài khoản đăng nhập. 201, trả về dòng danh bạ + mật khẩu tạm.
 *
 * KHÔNG CÓ THÂN VÀ KHÔNG CÓ MẬT KHẨU ĐI LÊN. Câu mở #9 chốt 22/09/2026: hệ thống sinh giá trị,
 * không phải quản trị viên đặt. `identity_post_staff_by_id_account["than"]` sinh từ hợp đồng là
 * `never`, nên một ô nhập mật khẩu ở màn hình sẽ không có chỗ nào gửi đi — `tsc` đỏ ngay.
 *
 * KHÔNG CÓ `Idempotency-Key`, VÀ ĐÓ LÀ QUYẾT ĐỊNH CỦA MÁY CHỦ chứ không phải thiếu sót ở đây
 * (`idempotency: khong-can` trong hợp đồng). Máy chủ chặn bằng `AND NOT co_tai_khoan` trong
 * WHERE: lần cấp thứ hai lên cùng một người trả 409 và KHÔNG đổi chuỗi băm. Đó là phép chặn
 * đúng hơn một khoá chống trùng — nó đúng cả khi hai quản trị viên khác nhau bấm, mà một khoá
 * theo biểu mẫu thì không.
 *
 * 409 Ở ĐÂY CÓ NGHĨA CỤ THỂ VÀ MÀN HÌNH PHẢI ĐƯA NGUYÊN CÂU RA: người này ĐÃ có tài khoản. Việc
 * cần làm khi ấy là ĐẶT LẠI (`datLaiMatKhau`), không phải bấm lại.
 */
export function capTaiKhoan(id: string): Promise<KetQua<identity_capTaiKhoanRa>> {
  const mau: identity_post_staff_by_id_account["duongDan"] = "/api/v1/staff/{id}/account";
  return goiGhi(duongDanMotCanBo(mau, id), "POST", undefined, 201).then(docCapTaiKhoan);
}

/**
 * PUT /api/v1/staff/{id}/password — quản trị viên đặt lại mật khẩu hộ (#17). 200, mật khẩu tạm mới.
 *
 * `khoaChongTrung` LÀ THAM SỐ, KHÔNG SINH TẠI CHỖ — cùng lý do `themCanBo`, và ở đây đắt hơn.
 * Sinh khoá bên trong hàm thì mỗi lần bấm lại sau một lỗi mạng là một khoá mới, tức mỗi lần bấm
 * lại sinh một mật khẩu tạm KHÁC và vô hiệu cái trước. Quản trị viên vừa đọc một giá trị cho cán
 * bộ qua điện thoại, bấm lại vì màn hình trông như chưa xong, và giá trị vừa đọc hết hiệu lực —
 * không ai hiểu vì sao đăng nhập không được. Khoá do biểu mẫu giữ, sống bằng đời một lần mở, và
 * lần bấm lại dùng LẠI chính nó.
 *
 * `MoKhiHong` LÀ QUYẾT ĐỊNH CỦA MÁY CHỦ, ngược với `POST /api/v1/staff`: kho khoá chống trùng
 * hỏng thì tuyến này VẪN chạy. Tuyến này LÀ đường cứu hộ, từ chối nó lúc Redis hỏng là bỏ rơi
 * đúng người nó sinh ra để cứu, và nó không sinh hồ sơ lưu trữ nào.
 */
export function datLaiMatKhau(
  id: string,
  khoaChongTrung: string,
): Promise<KetQua<identity_capTaiKhoanRa>> {
  const mau: identity_put_staff_by_id_password["duongDan"] = "/api/v1/staff/{id}/password";
  return goiGhi(duongDanMotCanBo(mau, id), "PUT", undefined, 200, {
    "Idempotency-Key": khoaChongTrung,
  }).then(docCapTaiKhoan);
}

/**
 * PUT /api/v1/staff/current/password — cán bộ tự đổi mật khẩu của chính mình. 204, không thân.
 *
 * `current` LÀ CỦA MÁY CHỦ, LẤY TỪ PHIÊN. Không có `id` nào ở đây và không được thêm vào: một
 * tham số định danh trên tuyến đổi mật khẩu là đúng hình dạng luật 4 cấm #1, chỉ đổi chỗ sang
 * phía cán bộ — đổi một chuỗi trong URL là đổi mật khẩu người khác.
 *
 * MẬT KHẨU HIỆN TẠI LÀ BẮT BUỘC, KỂ CẢ Ở MÀN BẮT ĐỔI LẦN ĐẦU, và bỏ nó đi là thứ sẽ được đề
 * xuất vì tiết kiệm một ô. Câu mở #18 chốt máy ở bộ phận một cửa là máy DÙNG CHUNG, nên "trình
 * duyệt này đang giữ một phiên" và "người này biết mật khẩu" thường xuyên là hai người khác
 * nhau. Ô ấy là thứ chặn một trình duyệt bỏ quên thành một lần chiếm tài khoản vĩnh viễn.
 *
 * 204 KHÔNG THÂN, nên hàm này trả `KetQua<void>`: không có gì để đọc, và gọi `.json()` lên một
 * phản hồi rỗng sẽ biến lần đổi THÀNH CÔNG thành "không đọc được".
 *
 * ĐỔI XONG LÀ PHIÊN HIỆN TẠI BỊ THU HỒI — máy chủ thu hồi MỌI phiên của người này, kể cả phiên
 * vừa gọi. Bên gọi phải đưa người dùng về màn đăng nhập; không làm thế thì trang tiếp theo nhận
 * 401 và trông như hệ thống hỏng. Đó là đọc đúng nghĩa đen `session-and-token` required #7, và
 * nó đúng ở đây: phiên mở bằng mật khẩu cũ chính là chỗ kẻ tấn công đang ngồi.
 */
export async function tuDoiMatKhau(than: identity_doiMatKhauVao): Promise<KetQua<void>> {
  const duongDan: identity_put_staff_current_password["duongDan"] =
    "/api/v1/staff/current/password";

  // DỰNG TỪNG TRƯỜNG, KHÔNG `...than`. Một phép trải ở đây là đường để một trường lạ đọc được từ
  // biểu mẫu — kể cả một bản sao mật khẩu dưới tên khác — đi lên máy chủ.
  const thanGui: identity_doiMatKhauVao = {
    current_password: than.current_password,
    new_password: than.new_password,
  };

  const kq = await goiGhi(duongDan, "PUT", thanGui, 204);
  return kq.ok ? { ok: true, duLieu: undefined } : kq;
}
