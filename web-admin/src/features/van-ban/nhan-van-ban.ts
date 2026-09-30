/**
 * Câu chữ và phép suy ra của hai quyển sổ văn bản. **Hàm thuần**: không gọi mạng, không dựng DOM,
 * và thời điểm hiện tại được TRUYỀN VÀO chứ không đọc từ đồng hồ ở đây — một hàm tự đọc đồng hồ là
 * một hàm không kiểm được ở đúng lúc quan trọng nhất: ngay trước và ngay sau hạn.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * QUÁ HẠN LÀ SUY RA, KHÔNG PHẢI MỘT TRƯỜNG MÁY CHỦ TRẢ VỀ (luật 10, bất biến 3). Hợp đồng CỐ Ý
 * không có `overdue`, và tệp này cố ý không dựng lại nó dưới một cái tên khác: `trangThaiHanVanBan`
 * nhận `due_at` cùng thời điểm hiện tại rồi trả về một trong ba ca, mỗi lần vẽ một lần.
 *
 * VÀ Ở ĐÂY CHỈ SUY RA "ĐÃ QUA MỐC HAY CHƯA", KHÔNG SUY RA "TRỄ MẤY NGÀY". Đặc tả §3.1 vẽ ô hạn
 * thành `d/M/yyyy (trễ N ngày)`, nhưng con số N ấy đếm bằng GIỜ LÀM VIỆC: nó cần lịch làm việc,
 * ngày nghỉ lễ và ngày làm bù của chính xã ấy — ba bảng do `identity` sở hữu cùng với phép cộng
 * (ADR 0007, luật 10 bất biến 4). Đếm bằng giờ đồng hồ ở trình duyệt sẽ ra một con số khác con số
 * của máy chủ vào đúng dịp lễ, và con số hiện trên màn hình cán bộ là con số được báo cáo lên
 * trên. So hai mốc tuyệt đối thì khác: đó là một phép so sánh, và nó đúng ở mọi lịch.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 */

// HAI BỘ ĐỊNH DẠNG NGÀY GIỜ ĐÃ CÓ CHỦ TRONG KHO NÀY, NÊN Ở ĐÂY KHÔNG CÓ BẢN THỨ BA. `nhanThoiDiem`
// ghim múi giờ Việt Nam (một bản thứ hai sẽ là bản quên ghim, và hai cán bộ đọc cùng một hạn ra hai
// giờ khác nhau); `nhanNgay` CẮT CHUỖI thay vì dựng `Date` (một ngày không mang múi giờ, cho qua
// `new Date("2026-09-02")` là lùi một ngày ở mọi máy phía tây London). Nhập chéo thư mục tính năng
// là cái giá rẻ hơn hẳn một bản sao thứ ba của cùng hai quyết định ấy (luật 9, cấm #2).
import { nhanNgay } from "@/features/cau-hinh/nhan-lich-lam-viec";
import type { KetTra } from "@/features/cau-hinh/tra-danh-muc";
import { nhanThoiDiem } from "@/features/phan-anh/nhan-phieu";

/* ---- danh sách đóng của máy chủ ---------------------------------------------------------- */

/**
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * HAI BẢNG NHÃN DƯỚI ĐÂY LÀ BẢN CHÉP TAY, VÀ ĐÓ LÀ LỖ HỔNG CỦA HỢP ĐỒNG CHỨ KHÔNG PHẢI MỘT LỰA
 * CHỌN Ở ĐÂY — cùng một lỗ hổng `features/phan-anh/nhan-phieu.ts` đã đo và đã báo.
 *
 * `openapi.json` khai `status` và `urgency` là `string` trơn, không kèm `enum`, dù bộ sinh kiểu CÓ
 * dịch `enum` chuỗi thành hợp của chuỗi hằng. Danh sách đóng thì có thật và nằm trong mã Go:
 * `service-documents/internal/domain/van_ban.go:33` (sáu trạng thái) và `:66` (bốn độ khẩn).
 *
 * HỆ QUẢ: máy chủ thêm một mã thì màn hình này KHÔNG đỏ ở `tsc`, nó chỉ rơi xuống nhánh dự phòng.
 * Nên nhánh dự phòng được viết để NÓI RA điều đó chứ không để giấu đi — nó hiện nguyên mã và nói
 * rõ mã ấy chưa có nhãn, để cán bộ gọi hỏi chứ không tự suy diễn.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 */
const NHAN_TRANG_THAI: Readonly<Record<string, string>> = {
  // Sáu nhãn của `docs/ui-ux/05-van-ban-don-thu.md §3.2`, chép đúng chữ đặc tả.
  "moi-vao-so": "Mới vào sổ",
  "da-phan-cong": "Đã phân công",
  "dang-xu-ly": "Đang xử lý",
  "da-giai-quyet": "Đã giải quyết",
  "chuyen-cap-tren": "Chuyển cấp trên",
  "luu-khong-thu-ly": "Lưu, không thụ lý",
};

/** Sáu mã trạng thái để dựng ô lọc. KHÔNG phải nguồn sự thật — xem khối chú thích trên. */
export const MA_TRANG_THAI: readonly string[] = Object.keys(NHAN_TRANG_THAI);

/**
 * Bốn mức độ khẩn của Nghị định 30/2020/NĐ-CP, cộng ca "không ghi".
 *
 * CHUỖI RỖNG LÀ MỘT CÂU TRẢ LỜI THẬT, không phải thiếu dữ liệu: "xã không ghi độ khẩn" khác hẳn
 * "Thường". Mặc định thành `thuong` là xếp loại một văn bản nhà nước bằng một lựa chọn không ai
 * làm (`van_ban.go:61`).
 */
const NHAN_DO_KHAN: Readonly<Record<string, string>> = {
  thuong: "Thường",
  khan: "Khẩn",
  "thuong-khan": "Thượng khẩn",
  "hoa-toc": "Hoả tốc",
};

/** Bốn mã độ khẩn để dựng ô chọn. Ô chọn tự thêm mục rỗng "Không ghi" ở đầu. */
export const MA_DO_KHAN: readonly string[] = Object.keys(NHAN_DO_KHAN);

function traNhan(bang: Readonly<Record<string, string>>, ma: string, loai: string): string {
  const nhan = bang[ma];
  if (nhan !== undefined) return nhan;
  if (ma === "") return `Không ghi ${loai}`;
  return `${ma} (mã ${loai} chưa có nhãn trên màn hình này)`;
}

export function nhanTrangThai(ma: string): string {
  return traNhan(NHAN_TRANG_THAI, ma, "trạng thái");
}

export function nhanDoKhan(ma: string): string {
  return traNhan(NHAN_DO_KHAN, ma, "độ khẩn");
}

/* ---- số vào sổ và ngày ------------------------------------------------------------------- */

/**
 * Số vào sổ đọc thành `7/2026`.
 *
 * SỐ LUÔN ĐI KÈM NĂM, không bao giờ đứng một mình. Dãy số chạy lại từ 1 mỗi năm, nên "số 7" không
 * chỉ ra một văn bản nào cả — và đây là con số cán bộ đọc qua điện thoại cho một cơ quan khác.
 */
export function nhanSoVaoSo(so: number, nam: number): string {
  return `${so}/${nam}`;
}

/** Ngày TUỲ CHỌN của hợp đồng: chuỗi rỗng nghĩa là văn bản không mang ngày của riêng nó. */
export function nhanNgayCoThe(ngayISO: string): string {
  return ngayISO === "" ? "Không ghi" : nhanNgay(ngayISO);
}

/* ---- tra danh mục ra chữ đọc được ---------------------------------------------------------- */

/**
 * Cột "Loại văn bản" — tra MÃ trong danh mục Loại văn bản của xã.
 *
 * TRA THEO MÃ, KHÔNG THEO ID: hồ sơ giữ `document_type` là **giá trị** chứ không phải khoá ngoại
 * (`migration 0004`), đúng để đổi nhãn danh mục không đụng tới hồ sơ lưu trữ. Vì vậy bảng tra của
 * màn này khoá theo `code`, khác bảng tra bộ phận vốn khoá theo `id`.
 *
 * NĂM CA, NĂM CÂU, KHÔNG CÂU NÀO LÀ Ô TRỐNG — xem `features/cau-hinh/tra-danh-muc.ts`. Ca
 * `khongTraDuoc` ở đây là ca có thật và hay gặp: xã tắt một loại văn bản đã dùng, nên hồ sơ cũ
 * trỏ tới một mã không còn trong danh mục đang dùng. Hồ sơ vẫn đúng; chỉ có nhãn là không tra được.
 */
export function nhanLoaiVanBan(ket: KetTra): string {
  switch (ket.loai) {
    case "dangDoc":
      return "Đang tải…";
    case "chuaGan":
      return "Không ghi loại văn bản";
    case "coTen":
      return ket.ten;
    case "khongTraDuoc":
      return "Mã không còn trong danh mục đang dùng";
    case "khongCoDanhMuc":
      return "Chưa đọc được danh mục loại văn bản";
  }
}

/**
 * Cột "Bộ phận đang giữ" — tra `holding_unit` trong danh mục bộ phận.
 *
 * `chuaGan` KHÔNG ĐƯỢC ĐỌC THÀNH MỘT Ô TRỐNG: "chưa chuyển cho bộ phận nào" là trạng thái của mọi
 * văn bản vừa vào sổ, và nó đúng là dòng cán bộ văn phòng đi tìm để chuyển tiếp.
 */
export function nhanBoPhanDangGiu(ket: KetTra): string {
  switch (ket.loai) {
    case "dangDoc":
      return "Đang tải…";
    case "chuaGan":
      return "Chưa chuyển bộ phận nào";
    case "coTen":
      return ket.ten;
    case "khongTraDuoc":
      return "Không tra được trong danh mục bộ phận";
    case "khongCoDanhMuc":
      return "Chưa đọc được danh mục bộ phận";
  }
}

/* ---- hạn xử lý ---------------------------------------------------------------------------- */

/**
 * Hạn xử lý của MỘT văn bản đến.
 *
 * `chuaCo` KHÔNG PHẢI CA THÔNG THƯỜNG và không được đọc thành "không đặt hạn": mọi văn bản vào sổ
 * qua tuyến hôm nay đều có hạn, vì máy chủ TỪ CHỐI vào sổ khi chưa ấn định được (409
 * `sla_chua_cau_hinh`). Một ô hạn rỗng ở đây là một dòng cũ hơn quy tắc ấy, và câu chữ nói đúng
 * như vậy thay vì hàm ý xã được phép có văn bản không hạn.
 */
export type HanVanBan =
  | { loai: "chuaCo" }
  | { loai: "conHan"; moc: string }
  | { loai: "quaHan"; moc: string };

export function trangThaiHanVanBan(hanISO: string, bayGio: Date): HanVanBan {
  if (hanISO === "") return { loai: "chuaCo" };

  const moc = new Date(hanISO);
  // Chuỗi không đọc được là hợp đồng hỏng, không phải một trạng thái nghiệp vụ: hiện nguyên văn
  // chuỗi máy chủ gửi, và KHÔNG gọi nó là quá hạn — đoán sai theo chiều ấy là báo một xã trễ hạn.
  if (Number.isNaN(moc.getTime())) return { loai: "conHan", moc: hanISO };

  return moc.getTime() < bayGio.getTime()
    ? { loai: "quaHan", moc: nhanThoiDiem(hanISO) }
    : { loai: "conHan", moc: nhanThoiDiem(hanISO) };
}

export function nhanHanVanBan(h: HanVanBan): string {
  switch (h.loai) {
    case "chuaCo":
      return "Chưa ấn định hạn";
    case "conHan":
      return `Hạn xử lý ${h.moc}`;
    case "quaHan":
      // CHỮ ĐÃ NÓI ĐỦ, màu chỉ là dấu hiệu THỨ HAI (a11y, `15-phu-luc §8`). Và không có con số
      // "trễ N ngày" — xem khối chú thích đầu tệp.
      return `Quá hạn · hạn xử lý ${h.moc}`;
  }
}

export function lopHanVanBan(h: HanVanBan): string | undefined {
  return h.loai === "quaHan" ? "chip chip-cham" : undefined;
}

/* ---- phép kiểm của biểu mẫu --------------------------------------------------------------- */

/** Bản nháp của khối "Chuyển cho bộ phận khác". Chuỗi hết — ô nhập của trình duyệt trả về chuỗi. */
export type BanChuyen = {
  denBoPhan: string;
  canBoXuLy: string;
  lyDo: string;
};

export const BAN_CHUYEN_TRONG: BanChuyen = { denBoPhan: "", canBoXuLy: "", lyDo: "" };

/** Kết quả soạn một yêu cầu: hoặc thân để gửi, hoặc MỘT câu nói rõ còn thiếu gì. */
export type Soan<T> = { ok: true; than: T } | { ok: false; loi: string };

export const LOI_THIEU_BO_PHAN = "Chọn bộ phận nhận văn bản trước khi chuyển.";

/**
 * Lý do chuyển là BẮT BUỘC, và câu này nói ra VÌ SAO chứ không chỉ nói "bắt buộc".
 *
 * Máy chủ cũng đòi (`domain.ErrThieuLyDoChuyen`), nhưng phép kiểm ở đây không phải bản sao thừa
 * của phép kiểm ấy: nó chặn TRƯỚC khi có một lời gọi mạng nào, nên một lần bấm thiếu ô không tiêu
 * một vòng mạng và không để cán bộ chờ để rồi đọc một câu từ chối.
 */
export const LOI_THIEU_LY_DO_CHUYEN =
  "Ghi lý do chuyển trước khi bấm. Dòng lịch sử chuyển xử lý KHÔNG sửa được sau khi ghi, " +
  "nên đây là chỗ duy nhất giải thích vì sao văn bản này đi tới bộ phận ấy.";

/**
 * Soạn thân của một lần chuyển xử lý — hai phép kiểm, không một.
 *
 * KHÔNG KIỂM GÌ THÊM. Bộ phận có tồn tại hay không, văn bản đã kết thúc hay chưa: máy chủ kiểm cả
 * hai kèm một câu tiếng Việt nói rõ phải làm gì, và chép chúng xuống client là dựng bản sao thứ
 * hai của một bộ quy tắc nghiệp vụ (luật 9, cấm #2).
 */
export function soanChuyen(ban: BanChuyen): Soan<{
  to_unit: string;
  assignee: string;
  reason: string;
}> {
  const denBoPhan = ban.denBoPhan.trim();
  if (denBoPhan === "") return { ok: false, loi: LOI_THIEU_BO_PHAN };

  const lyDo = ban.lyDo.trim();
  if (lyDo === "") return { ok: false, loi: LOI_THIEU_LY_DO_CHUYEN };

  return { ok: true, than: { to_unit: denBoPhan, assignee: ban.canBoXuLy.trim(), reason: lyDo } };
}

export const LOI_THIEU_LY_DO_GO =
  "Ghi lý do gỡ trước khi bấm. Lý do được lưu cùng bản ghi và là câu trả lời khi có người hỏi " +
  "vì sao quyển sổ thiếu một số.";

/** Soạn thân của một lần gỡ khỏi sổ. Một phép kiểm: lý do rỗng — kể cả toàn dấu cách — thì không gửi. */
export function soanGo(lyDo: string): Soan<{ reason: string }> {
  const sach = lyDo.trim();
  if (sach === "") return { ok: false, loi: LOI_THIEU_LY_DO_GO };
  return { ok: true, than: { reason: sach } };
}

/* ---- câu chữ trên màn hình ---------------------------------------------------------------- */

/**
 * ⚠ CÂU QUAN TRỌNG NHẤT CỦA CẢ HAI QUYỂN SỔ, và nó phải đứng ở chỗ người dùng SẮP BẤM XOÁ.
 *
 * Máy chủ cấp số dưới một khoá dòng và không bao giờ cấp lại một số đã cấp, kể cả sau khi bản ghi
 * bị gỡ (luật 7, bất biến 3; `store/day_so_van_ban.go`). Không có câu này, cán bộ gỡ một dòng nhập
 * nhầm và đinh ninh số ấy sẽ được cấp lại cho văn bản tiếp theo — rồi báo cáo cuối năm thiếu một
 * số mà không ai giải thích được khoảng trống ấy.
 */
export const CANH_BAO_GO_KHONG_TRA_SO =
  "Gỡ một văn bản KHÔNG trả số về dãy. Số đã cấp mất vĩnh viễn khỏi quyển sổ và không bao giờ " +
  "được cấp lại cho văn bản khác, nên sổ sẽ có một khoảng trống nhìn thấy được. Bản ghi không bị " +
  "xoá khỏi hệ thống: nó được giữ lại kèm người gỡ và lý do gỡ.";

export const GIAI_THICH_LY_DO_GO =
  "Lý do này được lưu vĩnh viễn cùng bản ghi. Ghi rõ vì sao gỡ, ví dụ “nhập trùng với số 12/2026”.";

/**
 * Câu dẫn của khối chuyển xử lý.
 *
 * NÓI RA TÍNH KHÔNG SỬA ĐƯỢC TRƯỚC KHI GHI, không phải sau. Dòng lịch sử là bản ghi chỉ-thêm ở
 * tầng CSDL (trigger `lich_su_chuyen_chi_them`, migration 0004), nên không nút nào trên màn hình
 * này sửa hay xoá được nó — và người viết lý do cần biết điều đó lúc đang gõ.
 */
export const DAN_CHUYEN_XU_LY =
  "Mỗi lần chuyển ghi một dòng lịch sử KHÔNG sửa được và KHÔNG xoá được. Bấm hai lần là hai lần " +
  "chuyển, không phải một lần gửi lại.";

/* ---- ngăn chi tiết văn bản đến -------------------------------------------------------------- */

/**
 * Tiêu đề ngăn chi tiết: `Văn bản đến số 7/2026 · nhận ngày 22/09/2026`.
 *
 * SỐ VÀ NGÀY, KHÔNG TRÍCH YẾU: tiêu đề này đi vào `aria-labelledby` — tức vào cây trợ năng — còn
 * trích yếu là chữ tự do có thể nhắc tên một công dân (luật 3, cấm #4). Ngày dùng `nhanNgay` như cột
 * "Ngày đến" của bảng, để một văn bản không mang hai cách viết ngày trên cùng một màn hình.
 */
export function nhanTieuDeVanBanDen(so: number, nam: number, ngayDenISO: string): string {
  return `Văn bản đến số ${nhanSoVaoSo(so, nam)} · nhận ngày ${nhanNgayCoThe(ngayDenISO)}`;
}

/**
 * Bộ phận chuyển ĐI của một dòng lịch sử. Rỗng ở lần chuyển đầu — chưa bộ phận nào giữ văn bản —
 * và câu ấy khác câu "Chưa chuyển bộ phận nào" của ô đang giữ: dòng này CHÍNH LÀ một lần chuyển.
 */
export function nhanTuBoPhan(ket: KetTra): string {
  return ket.loai === "chuaGan" ? "Chưa bộ phận nào giữ" : nhanBoPhanDangGiu(ket);
}

/**
 * Cán bộ được giao, hiện bằng MÃ CÁN BỘ (`CB-00123`), không bằng họ tên.
 *
 * KHÔNG TRA TÊN, có chủ ý: danh bạ cán bộ đòi `admin.user`, và một người có `document.read` chưa
 * chắc đọc được danh bạ — cùng lẽ ô "Cán bộ xử lý" của khối chuyển là ô chữ. Tuyến danh bạ hẹp là
 * việc của backend (`service-identity/tuyen-danh-ba-can-bo-hep`); đến lúc ấy chỉ hàm này đổi.
 */
export function nhanCanBo(ma: string): string {
  return ma === "" ? "Để bộ phận tự phân công" : ma;
}

export const TIEU_DE_DONG_THOI_GIAN = "Dòng thời gian chuyển tiếp";
export const TIEU_DE_KHOI_CHUYEN = "Chuyển cho bộ phận khác";

/**
 * Dòng thời gian rỗng là một CÂU TRẢ LỜI của máy chủ (`items: []`), không phải thiếu dữ liệu — và
 * nó phải nói ra thành chữ. Một danh sách trống trơn không phân biệt được "chưa chuyển lần nào"
 * với "chưa tải xong" hay "tải hỏng".
 */
export const LICH_SU_RONG = "Chưa chuyển xử lý lần nào.";
export const DANG_TAI_CHI_TIET = "Đang tải văn bản…";
export const DANG_TAI_LICH_SU = "Đang tải dòng thời gian chuyển tiếp…";
export const NUT_DONG_CHI_TIET = "Đóng chi tiết văn bản";
export const NUT_XEM = "Xem chi tiết";

/** Câu dẫn của ô hạn xử lý trên biểu mẫu vào sổ — nói rõ hạn KHÔNG do người nhập đặt. */
export const DAN_HAN_DO_MAY_CHU_AN_DINH =
  "Hạn xử lý do hệ thống ấn định theo bảng Thời hạn xử lý và Lịch làm việc của xã, một lần duy " +
  "nhất lúc vào sổ; biểu mẫu này không có ô hạn và sửa lại bảng ấy cũng không đổi hạn của văn bản " +
  "đã vào sổ.";

/**
 * Câu đứng cạnh đường dẫn tới `/cau-hinh` trên biểu mẫu vào sổ.
 *
 * HIỆN THƯỜNG TRỰC, KHÔNG CHỈ KHI MÁY CHỦ TỪ CHỐI — và đó là một quyết định, không phải một sự
 * tiện tay: để biết một câu từ chối 409 có phải câu `sla_chua_cau_hinh` hay không thì phải DÒ CHỮ
 * trong câu máy chủ viết, tức dựng một bản sao thứ hai của quy tắc nghiệp vụ ở client — và bản sao
 * ấy im lặng hỏng vào ngày máy chủ sửa câu chữ (`goi.ts`: không rẽ nhánh theo `code`, và cũng
 * không rẽ nhánh theo lời văn). Một câu đúng ở mọi lúc thì không có ngày nào sai.
 */
export const DAN_DUONG_TOI_CAU_HINH =
  "Xã chưa khai bảng Thời hạn xử lý hoặc Giờ làm việc thì không vào sổ được văn bản đến. Chỗ sửa:";

export const NHAN_DUONG_TOI_CAU_HINH = "Cấu hình → Thời hạn xử lý";

/* ---- nhãn nút và ô nhập ------------------------------------------------------------------- */

export const NUT_VAO_SO = "Vào sổ văn bản đến";
export const NUT_CAP_SO = "Cấp số văn bản đi";
export const NUT_SUA = "Sửa";
export const NUT_GO = "Gỡ khỏi sổ";
export const NUT_CHUYEN = "Chuyển xử lý";
export const NUT_XAC_NHAN_GO = "Xác nhận gỡ khỏi sổ";
export const NUT_XAC_NHAN_CHUYEN = "Chuyển và ghi vết";
export const NUT_LUU = "Lưu";
export const NUT_HUY = "Huỷ";

export const O_NGAY_DEN = "Ngày đến";
export const O_SO_KY_HIEU = "Số, ký hiệu văn bản";
export const O_NGAY_VAN_BAN = "Ngày văn bản";
export const O_CO_QUAN_BAN_HANH = "Cơ quan ban hành";
export const O_LOAI_VAN_BAN = "Loại văn bản";
export const O_TRICH_YEU = "Trích yếu";
export const O_DO_KHAN = "Độ khẩn";
export const O_NOI_NHAN = "Nơi nhận";
export const O_NGUOI_KY = "Người ký";
export const O_DEN_BO_PHAN = "Chuyển đến bộ phận";
export const O_CAN_BO_XU_LY = "Cán bộ xử lý (không bắt buộc)";
export const O_LY_DO_CHUYEN = "Lý do chuyển";
export const O_LY_DO_GO = "Lý do gỡ";

/* ---- trạng thái rỗng ---------------------------------------------------------------------- */

/**
 * Sổ rỗng là trạng thái BÌNH THƯỜNG của một xã vừa nhận hệ thống, không phải một sự cố — nên câu
 * này nói việc phải làm tiếp, không báo động.
 */
export const SO_DEN_RONG =
  "Chưa có văn bản đến nào trong sổ theo bộ lọc đang chọn. Bấm “Vào sổ văn bản đến” để nhập văn " +
  "bản đầu tiên.";

export const SO_DI_RONG =
  "Chưa có văn bản đi nào trong sổ theo bộ lọc đang chọn. Bấm “Cấp số văn bản đi” khi phát hành " +
  "văn bản đầu tiên.";

/**
 * ⚠ SỔ VĂN BẢN ĐI KHÔNG CÓ ĐẶC TẢ, và câu này nói ra đúng điều máy chủ làm thay vì bịa một quy
 * trình. Không có “nháp → đã ký → đã phát hành”: `vanBanDiRa` không có `status` và không có
 * `due_at`, vì cấp số CHÍNH LÀ hành vi phát hành (`van_ban_di.go:40`). Vẽ thêm một vòng đời ở đây
 * là bắt mọi xã đi theo một quy trình không ai quyết định.
 */
export const DAN_SO_DI =
  "Sổ ghi các văn bản xã đã phát hành. Cấp số là hành vi phát hành: hệ thống cấp số ngay khi lưu, " +
  "và số đã cấp không bao giờ được cấp lại.";

export const DAN_SO_DEN =
  "Sổ vào công văn đến của xã. Hệ thống cấp số đến và ấn định hạn xử lý theo cấu hình của xã.";
