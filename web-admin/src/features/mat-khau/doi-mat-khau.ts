import { tuDoiMatKhau } from "@/lib/api/tai-khoan";

/**
 * Phần QUYẾT ĐỊNH của màn tự đổi mật khẩu, tách hẳn khỏi phần vẽ.
 *
 * TÁCH RA VÌ MÔI TRƯỜNG TEST CỦA KHO NÀY LÀ NODE, KHÔNG PHẢI TRÌNH DUYỆT (`vitest.config.ts`):
 * không có jsdom, nên không có cách nào bấm một nút và xem nó gọi gì. Ba tính chất phải được
 * canh ở đây — lệch hai ô thì KHÔNG gọi mạng, thân gửi lên có ĐÚNG hai khoá, ô mật khẩu hiện
 * tại là bắt buộc — đều là tính chất của một hàm, và gói chúng trong một `onSubmit` là gói
 * chúng ở nơi không phép kiểm nào với tới.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * BA ĐIỀU TỆP NÀY KHÔNG LÀM, và cả ba đều là thứ sẽ được đề xuất vì nghe hợp lý:
 *
 *   1. KHÔNG rẽ nhánh theo `code` lỗi, không hiện `trace_id`, không hiện số hiệu HTTP. `goiGhi`
 *      đã lấy đúng câu máy chủ viết; viết lại nó ở đây là dựng bản sao thứ hai của một quy tắc
 *      nghiệp vụ, và bản sao ấy trôi mà không ai thấy (`lib/api/goi.ts`).
 *   2. KHÔNG `console.*` ở bất kỳ nhánh nào, kể cả nhánh mạng hỏng. Ba giá trị đi qua hàm này
 *      đều LÀ thông tin đăng nhập, và console của trình duyệt đi vào ảnh chụp màn hình cán bộ
 *      gửi cho hỗ trợ (luật 3, cấm #1).
 *   3. KHÔNG giữ lại giá trị nào sau khi trả về: không biến module, không `localStorage`,
 *      không `sessionStorage`.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 */

/**
 * Ba ô trên màn hình. `nhapLai` LÀ CỦA CLIENT và không có chỗ nào gửi nó đi: hợp đồng
 * `identity_doiMatKhauVao` chỉ có `current_password` và `new_password`, nên `tsc` đỏ nếu ai đó
 * cố nhét nó vào thân.
 *
 * Nó tồn tại vì một lần gõ nhầm MẬT KHẨU MỚI là một tài khoản không đăng nhập lại được, và
 * người dùng không biết mình đã gõ gì — máy chủ băng argon2id rồi, không ai dựng lại được giá
 * trị ấy, và đường ra duy nhất là nhờ quản trị viên đặt lại (#17).
 */
export type NhapDoiMatKhau = {
  hienTai: string;
  moi: string;
  nhapLai: string;
};

/** Đích sau khi đổi xong. Xem `doiMatKhau` để biết vì sao KHÔNG phải trang chủ. */
export const DUONG_DAN_SAU_KHI_DOI = "/dang-nhap";

/**
 * Độ dài tối thiểu, ĐẾM THEO KÝ TỰ chứ không theo byte và không theo đơn vị UTF-16.
 *
 * ĐÂY LÀ BẢN SAO CỦA MỘT HẰNG SỐ Ở MÁY CHỦ — `service-identity/internal/domain/mat_khau.go`,
 * `DaiMatKhauToiThieu = 12` — và nói thẳng ra là bản sao thì tốt hơn để nó nằm im: hợp đồng REST
 * sinh ra `current_password: string`, không mang theo con số nào, nên KHÔNG có đường nào để
 * client đọc được giá trị thật lúc chạy. Hệ quả phải chấp nhận: máy chủ đổi số thì số ở đây trôi.
 *
 * VÌ SAO VẪN NHẮC TRƯỚC Ở CLIENT: một cán bộ gõ đủ ba ô, bấm gửi, rồi bị từ chối vì quá ngắn là
 * một vòng ba ô phải gõ lại — mà hai trong ba ô ấy là mật khẩu, tức gõ mù. Nhắc trước là rẻ.
 *
 * VÀ CÂU TỪ CHỐI CUỐI CÙNG VẪN LÀ CỦA MÁY CHỦ: nếu hai số lệch nhau, phía nghiêm hơn thắng và
 * người dùng đọc câu của máy chủ. Không có nhánh nào ở đây "cho qua" thay máy chủ.
 *
 * KHÔNG CÓ QUY TẮC THÀNH PHẦN NÀO — không đòi chữ hoa, chữ số hay ký tự đặc biệt. Máy chủ không
 * có, và một quy tắc chỉ sống ở client là một quy tắc không tồn tại: nó từ chối một mật khẩu máy
 * chủ sẵn sàng nhận, và không ai thấy vì phép kiểm nào cũng xanh.
 */
export const DAI_TOI_THIEU = 12;

/** Trần của máy chủ (`DaiMatKhauToiDa = 512`), cùng tệp, cùng lý do bản sao ở trên. */
export const DAI_TOI_DA = 512;

/**
 * Đếm ký tự như Go đếm rune.
 *
 * `"a".length` ĐẾM ĐƠN VỊ UTF-16, KHÔNG ĐẾM KÝ TỰ. Tiếng Việt có dấu nằm trong BMP nên hai cách
 * đếm trùng nhau — đúng tới ngày một người dán vào đây một emoji, thứ chiếm hai đơn vị UTF-16 và
 * một rune. Khi ấy client đếm dư và cho qua một mật khẩu máy chủ từ chối là quá ngắn, ở đúng
 * chiều sai không ai thử.
 */
function demKyTu(chuoi: string): number {
  return Array.from(chuoi).length;
}

/**
 * Điều duy nhất client được phép từ chối một mình — hoặc `null` nếu không có gì để nói.
 *
 * THỨ TỰ CÁC NHÁNH LÀ THỨ TỰ CÁC Ô TRÊN MÀN HÌNH, có chủ ý: chỉ có MỘT vùng thông báo, nên câu
 * hiện ra phải trỏ vào ô đầu tiên còn sai. Nhảy cóc xuống ô thứ ba trong khi ô thứ nhất đang
 * trống là bắt người dùng đoán mình sai ở đâu.
 *
 * Mỗi câu nói RA VIỆC PHẢI LÀM, không nói tên lỗi (`skills/accessibility-elderly`, REQUIRED #5).
 */
export function loiTaiCho(nhap: NhapDoiMatKhau): string | null {
  // MẬT KHẨU HIỆN TẠI LÀ BẮT BUỘC, KỂ CẢ KHI VÀO MÀN NÀY QUA ĐƯỜNG BẮT ĐỔI LẦN ĐẦU — và ở đó
  // "mật khẩu hiện tại" chính là mật khẩu tạm quản trị viên vừa đọc cho họ. Bỏ ô này đi để tiết
  // kiệm một lần gõ là bỏ đúng thứ chặn một trình duyệt bỏ quên thành một lần chiếm tài khoản
  // vĩnh viễn: máy ở bộ phận một cửa là máy DÙNG CHUNG (câu mở #18), nên "trình duyệt này đang
  // giữ một phiên" và "người này biết mật khẩu" thường xuyên là hai người khác nhau.
  if (nhap.hienTai === "") return "Nhập mật khẩu hiện tại của bạn.";

  if (nhap.moi === "") return "Nhập mật khẩu mới.";

  if (demKyTu(nhap.moi) < DAI_TOI_THIEU) {
    return `Mật khẩu mới phải có ít nhất ${DAI_TOI_THIEU} ký tự.`;
  }

  if (demKyTu(nhap.moi) > DAI_TOI_DA) {
    return `Mật khẩu mới không được quá ${DAI_TOI_DA} ký tự.`;
  }

  // Máy chủ từ chối ca này (`ErrMatKhauMoiTrungCu`). Nhắc trước ở đây vì câu trả lời không phụ
  // thuộc dữ liệu nào ngoài hai ô người dùng vừa gõ.
  if (nhap.moi === nhap.hienTai) return "Mật khẩu mới phải khác mật khẩu hiện tại.";

  // VẾ CHỊU LỰC CỦA CẢ HÀM. So sánh BẰNG `!==` trên chuỗi thô: không `trim`, không chuẩn hoá
  // Unicode, không bỏ qua hoa thường. Mọi phép nới ở đây đều biến "hai ô giống nhau" thành một
  // câu không còn đúng — người dùng gõ một dấu cách cuối ở ô thứ hai sẽ được cho qua, rồi phát
  // hiện mật khẩu thật khác thứ họ nghĩ mình đã đặt, ở lần đăng nhập sau.
  if (nhap.nhapLai !== nhap.moi) {
    return "Hai ô mật khẩu mới chưa giống nhau. Nhập lại cho khớp.";
  }

  return null;
}

/**
 * Kết cục một lần bấm gửi. `xong` KHÔNG có nghĩa "ở lại màn hình": xem dưới.
 */
export type KetCucDoiMatKhau = { pha: "loi"; thongBao: string } | { pha: "xong" };

/**
 * Đổi mật khẩu: kiểm tại client trước, gọi máy chủ sau, rồi RỜI KHỎI ỨNG DỤNG.
 *
 * KIỂM TRƯỚC KHI GỌI, VÀ LỆCH THÌ KHÔNG CÓ LỜI GỌI NÀO. Không phải để tiết kiệm một vòng mạng:
 * gửi một cặp mật khẩu lệch nhau lên máy chủ là gửi một giá trị NGƯỜI DÙNG KHÔNG ĐỊNH ĐẶT đi qua
 * mạng và vào mọi tầng ghi log trên đường — trong khi câu trả lời đã biết trước khi rời trình
 * duyệt.
 *
 * `dieuHuong` LÀ THAM SỐ, KHÔNG PHẢI `window.location` GỌI THẲNG. Hai lý do, và lý do thứ hai
 * mới là lý do thật:
 *
 *   1. Hàm này chạy được trong môi trường Node của bộ kiểm, nơi không có `window`.
 *   2. ĐÍCH ĐẾN LÀ MỘT QUYẾT ĐỊNH, KHÔNG PHẢI MỘT CƠ CHẾ, nên nó ở lại đây và có ca kiểm riêng.
 *      Bên gọi chỉ cầm cái cách đi (`window.location.assign`), không cầm cái đi đâu.
 *
 * VÌ SAO ĐÍCH LÀ `/dang-nhap` CHỨ KHÔNG PHẢI TRANG CHỦ: đổi xong là máy chủ thu hồi MỌI phiên của
 * người này, KỂ CẢ phiên vừa gọi (`lib/api/tai-khoan.ts`). Ở lại ứng dụng nghĩa là trang kế tiếp
 * nhận 401 và trông như hệ thống hỏng, đúng vào lúc người dùng vừa làm một việc đúng. Rời bằng CẢ
 * TRANG, không điều hướng phía client: tiến trình cũ còn giữ nguyên ba giá trị vừa gõ trong bộ
 * nhớ, và trên một máy dùng chung thì người kế tiếp ngồi vào chính cái máy đó.
 */
export async function doiMatKhau(
  nhap: NhapDoiMatKhau,
  dieuHuong: (duongDan: string) => void,
): Promise<KetCucDoiMatKhau> {
  const loi = loiTaiCho(nhap);
  if (loi !== null) return { pha: "loi", thongBao: loi };

  // HAI TRƯỜNG, DỰNG TỪNG CÁI. `nhapLai` không có tên nào trong hợp đồng để đi lên, và không có
  // phép trải nào ở đây để nó lọt vào bằng một tên khác.
  const ketQua = await tuDoiMatKhau({
    current_password: nhap.hienTai,
    new_password: nhap.moi,
  });

  // Nguyên văn câu máy chủ viết. Mật khẩu hiện tại sai về tới đây dưới dạng một câu tiếng Việt
  // của máy chủ, và đó là câu phải ra trang.
  if (!ketQua.ok) return { pha: "loi", thongBao: ketQua.thongBao };

  dieuHuong(DUONG_DAN_SAU_KHI_DOI);
  return { pha: "xong" };
}
