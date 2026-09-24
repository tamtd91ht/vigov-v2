/**
 * Câu chữ của màn tra cứu phiếu phản ánh (`docs/ui-ux/09-phan-anh-nguoi-dan.md §8`). Hàm thuần:
 * không gọi mạng, không dựng DOM. Thời điểm hiện tại được TRUYỀN VÀO, không đọc từ đồng hồ ở
 * đây — một hàm tự đọc đồng hồ là một hàm không kiểm được ở đúng lúc quan trọng nhất: ngay trước
 * và ngay sau hạn.
 */

import type {
  identity_canBoChonNguoiRa,
  petitions_phieuPhanAnhRa,
} from "@/lib/api/schema.gen";

/**
 * Giờ Việt Nam, GHIM chứ không theo cài đặt của máy cán bộ.
 *
 * Một hạn xử lý là CAM KẾT của một cơ quan nhà nước với người dân. Máy đặt múi giờ khác sẽ hiện
 * cùng một mốc lệch đi vài giờ, và ở một hạn đếm bằng giờ làm việc thì vài giờ là cả buổi làm —
 * đủ để hai cán bộ đọc cùng một phiếu ra hai hạn khác nhau.
 */
const DINH_DANG_THOI_DIEM = new Intl.DateTimeFormat("vi-VN", {
  timeZone: "Asia/Ho_Chi_Minh",
  day: "2-digit",
  month: "2-digit",
  year: "numeric",
  hour: "2-digit",
  minute: "2-digit",
  hour12: false,
});

/** Mốc thời gian ISO-8601 của hợp đồng → `14:20 09/09/2026` theo giờ Việt Nam. */
export function nhanThoiDiem(iso: string): string {
  const moc = new Date(iso);
  // Chuỗi không đọc được là hợp đồng hỏng, không phải một trạng thái nghiệp vụ: hiện nguyên văn
  // chuỗi máy chủ gửi thay vì chữ "Invalid Date", để người báo lỗi có thứ để đọc qua điện thoại.
  if (Number.isNaN(moc.getTime())) return iso;
  return DINH_DANG_THOI_DIEM.format(moc);
}

/**
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * BẢNG NHÃN TRẠNG THÁI VÀ BẢNG NHÃN KÊNH LÀ HAI BẢN CHÉP TAY, VÀ ĐÓ LÀ MỘT LỖ HỔNG CỦA HỢP ĐỒNG
 * CHỨ KHÔNG PHẢI MỘT LỰA CHỌN Ở ĐÂY.
 *
 * `openapi.json` khai cả hai trường là `string` trơn, không kèm `enum` — dù bộ sinh kiểu CÓ dịch
 * `enum` chuỗi thành hợp của chuỗi hằng và đã làm đúng thế cho `sort` của `GET /api/v1/staff`
 * (`scripts/gen-api-types.mjs`). Danh sách đóng thì có thật và nằm trong mã Go
 * (`service-petitions/internal/domain/phieu_phan_anh.go:37` và `:115`).
 *
 * HỆ QUẢ: thêm một trạng thái ở máy chủ thì màn hình này KHÔNG đỏ ở `tsc` — nó chỉ lặng lẽ rơi
 * xuống nhánh dự phòng. Nên nhánh dự phòng được viết để NÓI RA điều đó chứ không để giấu đi: nó
 * hiện nguyên mã và nói rõ mã ấy chưa có nhãn. Đã báo lên để `tools/apidoc` phát `enum`.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 */
const NHAN_TRANG_THAI: Readonly<Record<string, string>> = {
  "da-tiep-nhan": "Đã tiếp nhận",
  "dang-phan-loai": "Đang phân loại",
  "da-chuyen-xu-ly": "Đã chuyển xử lý",
  "dang-xu-ly": "Đang xử lý",
  "da-xu-ly": "Đã xử lý",
  "cho-dan-xac-nhan": "Chờ dân xác nhận",
  "da-dong": "Đã đóng",
  "khong-tiep-nhan": "Không tiếp nhận",
  "chuyen-cap-tren": "Chuyển cấp trên",
};

const NHAN_KENH: Readonly<Record<string, string>> = {
  "zalo-mini-app": "Zalo Mini App",
  "zalo-oa": "Zalo OA",
  "web-xa": "Trang web của xã",
  "can-bo-nhap-ho": "Cán bộ nhập hộ",
};

function traNhan(bang: Readonly<Record<string, string>>, ma: string, loai: string): string {
  const nhan = bang[ma];
  if (nhan !== undefined) return nhan;
  if (ma === "") return `Không có ${loai}`;
  return `${ma} (mã ${loai} chưa có nhãn trên màn hình này)`;
}

export function nhanTrangThai(ma: string): string {
  return traNhan(NHAN_TRANG_THAI, ma, "trạng thái");
}

export function nhanKenh(ma: string): string {
  return traNhan(NHAN_KENH, ma, "kênh");
}

/**
 * Lĩnh vực phản ánh — BA ca, cùng hình dạng với cột Loại của tab Thôn/Tổ dân phố.
 *
 * `field_label` RỖNG LÀ CA THÔNG THƯỜNG, không phải lỗi: nhãn chỉ có khi xã đã đặt lại tên cho
 * mã ấy, còn mã gốc thuộc bộ danh mục tầng 1 do dịch vụ `platform` giữ và `petitions` chưa có
 * đường đọc (`service-petitions/internal/http/phieu_phan_anh.go`, chú thích trên `FieldLabel`).
 * `field` RỖNG lại là chuyện khác hẳn: phiếu CHƯA ĐƯỢC PHÂN LOẠI — và đó chính là lý do phiếu
 * ấy chưa có hạn xử lý xong.
 */
export type LinhVucPhanAnh =
  | { loai: "chuaPhanLoai" }
  | { loai: "coNhan"; nhan: string }
  | { loai: "chiCoMa"; ma: string };

export function linhVucPhanAnh(ma: string, nhan: string): LinhVucPhanAnh {
  if (ma === "") return { loai: "chuaPhanLoai" };
  if (nhan === "") return { loai: "chiCoMa", ma };
  return { loai: "coNhan", nhan };
}

export function nhanLinhVuc(l: LinhVucPhanAnh): string {
  switch (l.loai) {
    case "chuaPhanLoai":
      return "Chưa phân loại";
    case "coNhan":
      return l.nhan;
    case "chiCoMa":
      // Hiện MÃ, không kèm lời trách móc: xã chưa đặt lại tên cho mã là chuyện bình thường.
      return l.ma;
  }
}

/**
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * NGƯỜI GỬI — MÀN HÌNH HIỆN ĐÚNG THỨ MÁY CHỦ GỬI, KHÔNG GHÉP LẠI VÀ KHÔNG "LÀM ĐẸP".
 *
 * `reporter_name` về dạng `Nguyễn V. A.` và `reporter_phone` về dạng `09****5678` — TRỪ MỘT CA:
 * `GET /api/v1/citizen-reports/{maTraCuu}` trả bản ĐẦY ĐỦ cho tài khoản có `feedback.unmask` (khoá
 * gieo 20/09/2026), và mỗi lần đọc như thế máy chủ ghi một dòng vết kiểm toán
 * (`service-petitions/internal/http/phieu_phan_anh.go:240-251`). Tuyến danh sách và bốn tuyến ghi
 * thì LUÔN che, kể cả với khoá ấy (`xu_ly_phan_anh.go:156`, `:469`). Hàm này không biết và không cần
 * biết mình đang cầm bản nào: nó hiện đúng thứ máy chủ gửi, và quyết định che hay không nằm ở máy chủ.
 *
 * ẨN DANH THÌ CẢ HAI TRƯỜNG RỖNG, và màn hình KHÔNG được bù vào bằng gì cả: một cái tên đã che
 * vẫn là một cái tên — `Nguyễn V. A.` trong một xã vài nghìn người vẫn chỉ ra một người, và
 * đúng điều cờ ẩn danh bảo vệ là cán bộ đang xử lý không biết ai gửi.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 */
export function nhanNguoiGui(phieu: petitions_phieuPhanAnhRa): string {
  if (phieu.anonymous) return "Người gửi ẩn danh";

  const phan = [phieu.reporter_name, phieu.reporter_phone].filter((p) => p !== "");
  if (phan.length === 0) return "Không có thông tin người gửi";
  return phan.join(" · ");
}

/**
 * Trạng thái của MỘT hạn xử lý.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * QUÁ HẠN LÀ SUY RA, KHÔNG PHẢI MỘT TRƯỜNG ĐƯỢC GỬI VỀ (luật 10, bất biến 3) — và hợp đồng CỐ Ý
 * không có trường `overdue` vì đúng lý do ấy: một giá trị boolean đóng băng lúc dựng phản hồi
 * còn màn hình thì đứng trên máy cán bộ hàng giờ sau đó.
 *
 * NHƯNG Ở ĐÂY CHỈ SUY RA "ĐÃ QUA MỐC HAY CHƯA", KHÔNG SUY RA "QUÁ HẠN MẤY NGÀY". So hai mốc thời
 * gian tuyệt đối là một phép so sánh và nó đúng ở mọi lịch. Còn "quá hạn 3 ngày" mà đặc tả §8.3
 * vẽ là một khoảng đếm bằng GIỜ LÀM VIỆC — nó cần lịch làm việc, ngày nghỉ lễ và ngày làm bù của
 * chính xã ấy, ba bảng do `identity` sở hữu cùng với phép cộng giờ làm việc (ADR 0007). Đếm bằng
 * giờ đồng hồ ở trình duyệt sẽ ra một con số khác con số của máy chủ vào đúng dịp lễ, và con số
 * hiện trên màn hình cán bộ là con số được báo cáo lên trên.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 */
export type TrangThaiHan =
  /** `acknowledge_due = null`: phiếu do cán bộ nhập hộ — cán bộ CHÍNH LÀ người đọc, nên khoảng ấy không tồn tại. */
  | { loai: "khongApDung" }
  /** `resolve_due = null`: phiếu chưa được phân loại nên chưa có cam kết nào được ấn định. */
  | { loai: "chuaCo" }
  | { loai: "conHan"; moc: string }
  | { loai: "quaHan"; moc: string };

/**
 * `hanISO` là `null` với HAI nghĩa khác nhau tuỳ trường, nên nghĩa ấy được TRUYỀN VÀO chứ không
 * đoán ở đây: `khongApDung` cho hạn tiếp nhận, `chuaCo` cho hạn xử lý xong. Gộp hai cái làm một
 * là để màn hình nói "chưa có hạn" về một phiếu mà khoảng ấy không bao giờ tồn tại.
 */
export function trangThaiHan(
  hanISO: string | null,
  khiRong: "khongApDung" | "chuaCo",
  bayGio: Date,
): TrangThaiHan {
  if (hanISO === null) return { loai: khiRong };

  const moc = new Date(hanISO);
  if (Number.isNaN(moc.getTime())) return { loai: "conHan", moc: hanISO };

  return moc.getTime() < bayGio.getTime()
    ? { loai: "quaHan", moc: nhanThoiDiem(hanISO) }
    : { loai: "conHan", moc: nhanThoiDiem(hanISO) };
}

export function nhanHan(h: TrangThaiHan): string {
  switch (h.loai) {
    case "khongApDung":
      // KHÔNG hiện thành `0` và không hiện ô trống: một số không là một mẫu hợp lệ, và một xã
      // nhập hộ nhiều phiếu sẽ báo cáo thời gian tiếp nhận trung bình gần bằng không (ADR 0028).
      return "Không áp dụng";
    case "chuaCo":
      return "Chưa ấn định — phiếu chưa được phân loại";
    case "conHan":
      return `Hạn cuối ${h.moc}`;
    case "quaHan":
      return `Quá hạn · hạn cuối ${h.moc}`;
  }
}

export function lopHan(h: TrangThaiHan): string | undefined {
  return h.loai === "quaHan" ? "nhan-lech" : undefined;
}

/** Ô "Hiển thị với người dân" (§8.3), hai ca, cùng câu chú thích đặc tả ghi. */
export function nhanHienCongKhai(hien: boolean): string {
  return hien
    ? "Đang hiện công khai. Người dân xem được phiếu này trên trang công khai của xã."
    : "Chưa cho hiện công khai. Chỉ cán bộ trong xã xem được. Người gửi vẫn tra cứu được phiếu của mình.";
}

/** Câu dẫn của ô nhập mã. Màn tra cứu nay đứng CẠNH quyển sổ, không thay cho nó. */
export const HUONG_DAN_TRA_CUU =
  "Nhập mã tra cứu đã trả cho người dân để mở thẳng đúng một phiếu, kể cả phiếu không nằm trong " +
  "bộ lọc đang chọn ở quyển sổ bên trên.";

/** Chưa gõ mã nào. Trạng thái BÌNH THƯỜNG lúc mở màn, không phải lỗi. */
export const CHUA_TRA_CUU = "Chưa tra phiếu nào. Nhập mã tra cứu rồi bấm Tra cứu.";

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * DANH MỤC LĨNH VỰC PHẢN ÁNH — 12 mục, §5
 *
 * ⚠ ĐÂY LÀ BẢN CHÉP TAY THỨ BA CỦA TỆP NÀY, cùng loại với `NHAN_TRANG_THAI` và `NHAN_KENH` bên
 * trên, và nó là LỖ HỔNG CỦA HỢP ĐỒNG chứ không phải một lựa chọn ở đây: `openapi.json` không có
 * tuyến nào trả về danh mục lĩnh vực phản ánh. Bộ mã tầng 1 do dịch vụ `platform` giữ và
 * `service-petitions` cũng không đọc được nó (ADR 0026, điều kiện dừng #2) — chính vì thế handler
 * **không kiểm** `field` tồn tại hay không (`domain.KiemLinhVuc`).
 *
 * HAI HỆ QUẢ ĐƯỢC NÓI RA THAY VÌ GIẤU:
 *
 *   1. Xã đặt lại tên cho một mã thì NHÃN TRÊN THẺ vẫn đúng — nó lấy `field_label` máy chủ gửi.
 *      Chỉ ô CHỌN dưới đây là dùng nhãn chép tay, vì lúc chọn thì chưa có phiếu nào để hỏi.
 *   2. Khách chốt thêm mã thứ mười ba thì màn hình này KHÔNG đỏ ở `tsc` — ô chọn chỉ thiếu một
 *      dòng, lặng lẽ. Đã báo về.
 *
 * `can-bo` CÓ TRONG DANH SÁCH NÀY, VÀ ĐÓ KHÔNG PHẢI SƠ SUẤT. Nó là lĩnh vực hạn chế ở đường ĐỌC
 * (máy chủ trả 404 cho tài khoản thiếu `feedback.restricted`, và loại hẳn khỏi trang), nhưng chốt
 * lĩnh vực VÀO `can-bo` là hành vi được phép với người có `feedback.classify`
 * (`app.duocChamPhieuHanChe`) — đúng tình huống một cán bộ đọc phiếu rồi nhận ra nó nói về một
 * đồng nghiệp. Bỏ mục ấy khỏi ô chọn là bịt đường phân loại đúng.
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

export type MucChon = { readonly ma: string; readonly nhan: string };

export const LINH_VUC_PHAN_ANH: readonly MucChon[] = [
  { ma: "rac-thai", nhan: "Rác thải – Vệ sinh môi trường" },
  { ma: "giao-thong", nhan: "Hạ tầng giao thông" },
  { ma: "cap-thoat-nuoc", nhan: "Cấp thoát nước" },
  { ma: "dien", nhan: "Điện" },
  { ma: "trat-tu-do-thi", nhan: "Trật tự đô thị – lấn chiếm vỉa hè" },
  { ma: "an-ninh", nhan: "An ninh trật tự" },
  { ma: "xay-dung", nhan: "Xây dựng không phép" },
  { ma: "o-nhiem", nhan: "Ô nhiễm (tiếng ồn, khí thải, nước thải)" },
  { ma: "y-te-giao-duc", nhan: "Y tế – Giáo dục" },
  { ma: "can-bo", nhan: "Thái độ / tác phong cán bộ" },
  { ma: "an-toan-thuc-pham", nhan: "An toàn thực phẩm" },
  { ma: "khac", nhan: "Khác" },
];

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * CHÍN TRẠNG THÁI — danh sách ĐÓNG (ADR 0027; khách duyệt NGUYÊN VĂN từng ký tự 20/09/2026)
 *
 * Đổi một trong chín chuỗi mã là **di trú hồ sơ lưu trữ** (luật 7), không phải đổi tên. Bảy mã
 * luồng chính đi theo đúng thứ tự vòng đời; hai mã còn lại là rẽ nhánh.
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

export const LUONG_CHINH: readonly string[] = [
  "da-tiep-nhan",
  "dang-phan-loai",
  "da-chuyen-xu-ly",
  "dang-xu-ly",
  "da-xu-ly",
  "cho-dan-xac-nhan",
  "da-dong",
];

export const RE_NHANH: readonly string[] = ["khong-tiep-nhan", "chuyen-cap-tren"];

/** Chín mã, đúng thứ tự ô chọn `Tất cả trạng thái` của §4. */
export const MOI_TRANG_THAI: readonly string[] = [...LUONG_CHINH, ...RE_NHANH];

/** Bốn kênh tiếp nhận (§7, §12), cho ô lọc. */
export const MOI_KENH: readonly string[] = ["zalo-mini-app", "zalo-oa", "web-xa", "can-bo-nhap-ho"];

/** Một ô của StatusStepper §8.2. */
export type OBuoc = {
  readonly ma: string;
  readonly nhan: string;
  readonly vaiTro: "daQua" | "dangODay" | "chuaToi";
};

/**
 * Bảy ô luồng chính, với ô hiện tại đánh dấu `đang ở đây` (§8.2).
 *
 * PHIẾU ĐANG Ở MỘT RẼ NHÁNH (`khong-tiep-nhan`, `chuyen-cap-tren`) THÌ KHÔNG Ô NÀO LÀ "đang ở
 * đây", và bảy ô đều `chuaToi`. Không suy bừa một vị trí trên luồng chính cho một phiếu đã rời
 * khỏi luồng ấy: một ô tô màu sai nói với cán bộ rằng phiếu còn đang chạy.
 *
 * MỘT MÃ LẠ (hợp đồng mọc thêm trạng thái) cũng rơi vào đúng nhánh trên — không ô nào sáng — và
 * chuỗi trạng thái hiện nguyên mã ở chỗ khác. Không nhánh dự phòng nào ở đây tô ô đầu tiên.
 */
export function buocLuongChinh(trangThai: string): readonly OBuoc[] {
  const viTri = LUONG_CHINH.indexOf(trangThai);
  return LUONG_CHINH.map((ma, i) => ({
    ma,
    nhan: nhanTrangThai(ma),
    vaiTro: viTri < 0 ? "chuaToi" : i < viTri ? "daQua" : i === viTri ? "dangODay" : "chuaToi",
  }));
}

/**
 * Hai ô RẼ NHÁNH của §8.2 (`Không tiếp nhận`, `Chuyển cấp trên`), sáng theo TRẠNG THÁI của phiếu.
 *
 * Ô KHÔNG BẤM ĐƯỢC. Đặc tả vẽ nhãn `chuyển sang` (bấm để chuyển), nhưng hai nhánh này là hành vi
 * KẾT THÚC phiếu và phải mang một lý do người dân đọc được — nên chúng là hai biểu mẫu riêng bên
 * dưới (`BieuMauReNhanh`), không phải một cú bấm trên thanh bước.
 *
 * Không có `daQua`: hai nhánh là trạng thái cuối, không phiếu nào đi QUA chúng.
 */
export function buocReNhanh(trangThai: string): readonly OBuoc[] {
  return RE_NHANH.map((ma) => ({
    ma,
    nhan: nhanTrangThai(ma),
    vaiTro: ma === trangThai ? "dangODay" : "chuaToi",
  }));
}

/**
 * Câu giải thích trạng thái hiện tại (§8.2, dòng dưới stepper).
 *
 * ĐẶC TẢ CHO ĐÚNG MỘT CÂU TRONG CHÍN, và tám câu còn lại **không được bịa ra ở đây**: đó là chữ
 * hiện trên màn hình cán bộ của một cơ quan nhà nước, và §11 của chính chương này vừa cho thấy hậu
 * quả — một câu mô tả cũ, không ai duyệt lại, nói sai về thời hạn của phiếu nhập hộ. Trả `null`
 * thì màn hình không hiện dòng nào, chứ không hiện một câu do web nghĩ ra.
 */
export function cauGiaiThichTrangThai(trangThai: string): string | null {
  if (trangThai === "dang-phan-loai") {
    return "Đang xem phiếu thuộc lĩnh vực nào, có tiếp nhận không.";
  }
  return null;
}

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * BỐN THAO TÁC, VÀ **HAI CỔNG KHÁC NHAU** — điểm chính của màn hình này
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

/** Nút nào ĐƯỢC VẼ cho bộ quyền của phiên hiện tại. */
export type CongThaoTac = {
  readonly phanLoai: boolean;
  readonly phanCong: boolean;
  /**
   * Nút `Đóng phiếu`. `feedback.resolve` và **chỉ** khoá ấy — xem `QUYEN_DONG_PHAN_ANH` ở
   * `lib/quyen.ts`.
   */
  readonly dongPhieu: boolean;
};

/**
 * Bốn nút, và chỉ BA trong bốn có cổng ở giao diện.
 *
 * ⚠ `tienTrangThai` KHÔNG CÓ TRONG KIỂU NÀY, VÀ SỰ VẮNG MẶT ẤY LÀ NỘI DUNG CHÍNH CHỨ KHÔNG PHẢI
 * MỘT CHỖ CÒN THIẾU. Điều kiện thật của `…/status` là `feedback.resolve` **HOẶC** chính là cán bộ
 * được phân công phiếu ấy (LUẬT NẮM GIỮ). `assignee` của phiếu và `staff.code` của phiên đều là
 * MÃ CÁN BỘ (`CB-00123`) nên về kỹ thuật so được — nhưng giao diện CỐ Ý không so: luật nắm giữ là
 * của máy chủ (`service-petitions/internal/app/xu_ly_phan_anh.go:184-212`), và một bản sao ở đây
 * sẽ ẩn nút đúng vào ngày luật ấy được nới mà màn hình chưa biết.
 *
 * Nên nút tiến trạng thái HIỆN VỚI MỌI NGƯỜI XEM ĐƯỢC SỔ, và câu 403 của máy chủ
 * (*"Phiếu này không được phân công cho bạn…"*) ra thẳng màn hình. Gắn nó sau `feedback.resolve`
 * cho "gọn" là lấy mất đúng điều luật nắm giữ mở ra cho trưởng thôn.
 */
export function congThaoTac(coPhanLoai: boolean, coPhanCong: boolean, coDong: boolean): CongThaoTac {
  return { phanLoai: coPhanLoai, phanCong: coPhanCong, dongPhieu: coDong };
}

/**
 * Nút `Phân loại` chỉ có nghĩa khi phiếu CHƯA phân loại.
 *
 * Máy chủ chốt lĩnh vực bằng một câu `UPDATE` mang `trang_thai = 'da-tiep-nhan'`, nên lần thứ hai
 * trả **409** (`routes.go`, khối `PHÂN LOẠI`). Ẩn nút ở trạng thái khác là nói ra điều ấy trước,
 * chứ không phải dựng thêm một luật.
 */
export function phanLoaiDuoc(trangThai: string): boolean {
  return trangThai === "da-tiep-nhan";
}

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * HAI NHÁNH RẼ — `Không tiếp nhận`, `Chuyển cấp trên` (POST …/rejection, …/referral)
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

/**
 * Hai nhánh chỉ rời được từ `dang-phan-loai` — bước một người đọc phiếu lần đầu và quyết xã có
 * nhận việc này không (`domain.KetThucNhanhDuoc`). Nơi khác máy chủ trả 409; ẩn biểu mẫu là nói
 * ra điều ấy trước, không phải dựng thêm một luật.
 */
export function reNhanhDuoc(trangThai: string): boolean {
  return trangThai === "dang-phan-loai";
}

/** Giới hạn của máy chủ (`service-petitions`, 400 `invalid_request` khi vượt). */
export const LY_DO_TOI_THIEU = 10;
export const LY_DO_TOI_DA = 2000;
export const CO_QUAN_TOI_DA = 200;

/**
 * Đếm KÝ TỰ như máy chủ đếm: số điểm mã Unicode (rune của Go) của chuỗi ĐÃ CẮT khoảng trắng.
 *
 * KHÔNG dùng `.length`: `.length` đếm đơn vị UTF-16, nên một ký tự ngoài mặt phẳng cơ bản (biểu
 * tượng cảm xúc cán bộ dán từ Zalo) đếm thành hai, và bộ đếm trên màn hình lệch khỏi con số máy
 * chủ dùng để từ chối. Chữ Việt dựng sẵn (`ệ`, `ữ`) là một điểm mã, đúng như người đọc thấy.
 */
export function demKyTu(s: string): number {
  return Array.from(s.trim()).length;
}

/** Câu lỗi của ô lý do, hoặc `null` khi hợp lệ. */
export function loiLyDo(lyDo: string): string | null {
  const n = demKyTu(lyDo);
  if (n < LY_DO_TOI_THIEU) {
    return `Lý do cần ít nhất ${LY_DO_TOI_THIEU} ký tự (hiện có ${n}).`;
  }
  if (n > LY_DO_TOI_DA) {
    return `Lý do không được quá ${LY_DO_TOI_DA} ký tự (hiện có ${n}).`;
  }
  return null;
}

/** Câu lỗi của ô cơ quan tiếp nhận, hoặc `null` khi hợp lệ. */
export function loiCoQuan(coQuan: string): string | null {
  const n = demKyTu(coQuan);
  if (n === 0) return "Cần ghi tên cơ quan tiếp nhận.";
  if (n > CO_QUAN_TOI_DA) {
    return `Tên cơ quan tiếp nhận không được quá ${CO_QUAN_TOI_DA} ký tự (hiện có ${n}).`;
  }
  return null;
}

export const NHAN_KHONG_TIEP_NHAN = "Không tiếp nhận";
export const NHAN_CHUYEN_CAP_TREN = "Chuyển cấp trên";
export const NHAN_O_LY_DO = "Lý do (người dân sẽ đọc được)";
export const NHAN_O_CO_QUAN = "Cơ quan tiếp nhận";
export const GOI_Y_CO_QUAN = "Ví dụ: Công ty điện lực, Công an xã, Sở Xây dựng…";

export const CANH_BAO_RE_NHANH =
  "Thao tác này kết thúc xử lý phiếu tại xã và không hoàn tác được. Người dân được thông báo và " +
  "đọc được lý do này khi tra cứu phiếu của mình.";

/**
 * Nút `Đóng phiếu` có nghĩa ở phiếu này không — HAI ĐIỂM, cùng hai điểm `domain.DongDuoc` của máy
 * chủ, nhưng điểm thứ hai được SUY RA TỪ KÊNH chứ không từ đúng điều kiện máy chủ dùng:
 *
 *   cho-dan-xac-nhan                         luồng thường
 *   da-xu-ly  VÀ  kênh `can-bo-nhap-ho`      phiếu không có công dân nào để xác nhận
 *
 * ⚠ ĐIỀU KIỆN THẬT CỦA MÁY CHỦ LÀ `cong_dan_id` RỖNG, và hợp đồng KHÔNG trả trường ấy hay cờ nào
 * tương đương (`petitions_phieuPhanAnhRa`). Kênh `can-bo-nhap-ho` là phiếu cán bộ vào sổ thay dân,
 * nên không có tài khoản công dân — đó là tín hiệu DUY NHẤT màn hình đọc được. Sai lệch nếu có
 * chỉ theo một chiều an toàn: một phiếu kênh khác mà không có công dân sẽ THIẾU nút ở `da-xu-ly`
 * (máy chủ vẫn cho đóng), chứ không bao giờ có nút mà máy chủ từ chối vì lý do này. Đã báo về để
 * hợp đồng trả một cờ.
 */
export function dongDuocTrenManHinh(phieu: petitions_phieuPhanAnhRa): boolean {
  if (phieu.status === "cho-dan-xac-nhan") return true;
  return phieu.status === "da-xu-ly" && phieu.channel === "can-bo-nhap-ho";
}

/** Phiếu đã đóng hoặc đã rẽ nhánh thì không còn bước kế tiếp trên luồng chính. */
export function conBuocKeTiep(trangThai: string): boolean {
  const viTri = LUONG_CHINH.indexOf(trangThai);
  return viTri >= 0 && viTri < LUONG_CHINH.length - 1;
}

/** Câu dưới nút `Đóng phiếu` khi tài khoản thiếu `feedback.resolve`. Nói ĐÚNG tên khoá. */
export const CAU_THIEU_QUYEN_DONG =
  "Tài khoản của bạn không có quyền kết thúc xử lý phản ánh (feedback.resolve), nên không có nút " +
  "Đóng phiếu. Bạn vẫn tiến được trạng thái của phiếu được phân công cho mình.";

export const CAU_THIEU_QUYEN_PHAN_LOAI =
  "Tài khoản của bạn không có quyền chốt lĩnh vực phản ánh (feedback.classify), nên không có ô " +
  "phân loại và không có hai thao tác Không tiếp nhận, Chuyển cấp trên. Chốt lĩnh vực là hành vi " +
  "ấn định hạn xử lý xong của xã.";

export const CAU_THIEU_QUYEN_PHAN_CONG =
  "Tài khoản của bạn không có quyền chuyển xử lý phản ánh (feedback.assign), nên không có khối " +
  "Chuyển xử lý.";

/** Nhãn nút tiến trạng thái. Không nêu tên bước kế tiếp: máy chủ giữ bản đồ, không phải màn này. */
export const NHAN_TIEN_TRANG_THAI = "Chuyển sang bước kế tiếp";

export const GHI_CHU_TIEN_TRANG_THAI =
  "Máy chủ quyết định bước kế tiếp trên luồng chính. Phiếu được phân công cho bạn thì bạn tiến " +
  "được, dù tài khoản không có quyền xử lý phản ánh của cả xã.";

export const NHAN_O_KET_QUA = "Kết quả xử lý người dân đọc được";

export const GHI_CHU_O_KET_QUA =
  "Câu này hiện trên phiếu của người dân khi họ tra cứu. Bắt buộc phải có — một phiếu đóng mà " +
  "không nói kết quả là một phiếu bị xếp lại trong im lặng.";

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * QUYỂN SỔ — câu chữ của danh sách và bộ lọc (§2, §4)
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

export const SO_RONG =
  "Không có phiếu phản ánh nào khớp bộ lọc đang chọn. Bỏ bớt điều kiện lọc để xem rộng hơn.";

export const DANG_TAI_SO = "Đang tải sổ phản ánh…";

/** Nhãn "mọi giá trị" của từng ô lọc — nguyên văn §4. */
export const MOI_TRANG_THAI_NHAN = "Tất cả trạng thái";
export const MOI_LINH_VUC_NHAN = "Tất cả lĩnh vực";
export const MOI_DIA_BAN_NHAN = "Tất cả địa bàn";
export const MOI_BO_PHAN_NHAN = "Tất cả bộ phận";
export const MOI_KENH_NHAN = "Tất cả kênh tiếp nhận";
export const CHI_TRE_HAN_NHAN = "Chỉ phiếu trễ hạn";
export const TIM_PLACEHOLDER = "Tìm theo nội dung, mã phiếu, địa chỉ…";

/**
 * Bộ phận đang giữ phiếu, đọc ra thành chữ.
 *
 * `unit` LÀ ULID, KHÔNG PHẢI TÊN. Tra sang tên bằng danh mục `GET /api/v1/org-units` — tuyến
 * `any-authenticated`, nên mọi tài khoản xem được sổ đều tra được. Một id không có trong danh mục
 * (bộ phận vừa bị đổi, danh mục đọc hỏng) thì hiện câu nói ra điều đó, KHÔNG hiện id: một chuỗi
 * `01JB…` trên màn hình cán bộ là một chuỗi không ai làm gì được.
 */
export function nhanBoPhan(id: string, tenTheoID: ReadonlyMap<string, string>): string {
  if (id === "") return "Chưa chuyển bộ phận";
  return tenTheoID.get(id) ?? "Bộ phận không còn trong danh mục";
}

/** Danh bạ chọn người, tra theo MÃ CÁN BỘ (`code`). */
export type DanhBaTheoMa = ReadonlyMap<string, identity_canBoChonNguoiRa>;

/** Dựng bảng tra từ `items` của `GET /api/v1/staff-directory`. */
export function danhBaTheoMa(items: readonly identity_canBoChonNguoiRa[]): DanhBaTheoMa {
  return new Map(items.map((cb) => [cb.code, cb]));
}

/**
 * Ô `ĐANG GIAO CHO` của §8.3 — phần CÁN BỘ.
 *
 * `assignee` LÀ MÃ CÁN BỘ (`CB-00123`), và họ tên tra bằng danh bạ chọn người
 * (`GET /api/v1/staff-directory`, mọi cán bộ đăng nhập đọc được). BỐN CA:
 *
 *   rỗng                         phiếu giao cho bộ phận, bộ phận tự phân công
 *   có trong danh bạ             họ tên (họ tên rỗng thì hiện mã — không bao giờ một ô trống)
 *   danh bạ chưa tải / tải hỏng  hiện MÃ: mã là thứ duy nhất màn hình biết chắc
 *   không có trong danh bạ       hiện MÃ kèm một câu trung tính. Danh bạ chỉ gồm người có tài khoản
 *                                đang hoạt động, nên người đã khoá tài khoản (nghỉ hưu, chuyển công
 *                                tác) rơi vào ca này — và phiếu cũ của họ vẫn phải đọc được
 *
 * KHÔNG CÓ EMAIL như đặc tả vẽ (`Họ tên — email`): danh bạ chọn người cố ý không trả email.
 */
export function nhanCanBoXuLy(assignee: string, danhBa: DanhBaTheoMa | null): string {
  if (assignee === "") return "Chưa phân công cán bộ cụ thể";
  if (danhBa === null) return assignee;
  const cb = danhBa.get(assignee);
  if (cb === undefined) return `${assignee} (không có trong danh bạ cán bộ đang hoạt động)`;
  return cb.full_name === "" ? assignee : cb.full_name;
}

/**
 * Một dòng của ô chọn `Cán bộ xử lý` (§8.5). Đặc tả vẽ `Họ tên — email · Chức danh`; danh bạ không
 * có email nên dòng là `Họ tên · Chức danh`, hoặc chỉ họ tên khi chưa có chức danh.
 */
export function nhanLuaChonCanBo(cb: identity_canBoChonNguoiRa): string {
  const ten = cb.full_name === "" ? cb.code : cb.full_name;
  return cb.position === "" ? ten : `${ten} · ${cb.position}`;
}

/** Nhãn và lựa chọn mặc định của ô chọn cán bộ — nguyên văn §8.5. */
export const NHAN_CHON_CAN_BO = "Cán bộ xử lý";
export const DE_BO_PHAN_PHAN_CONG = "— Để bộ phận phân công —";

/** Hai tab phạm vi của §4. Tab thứ ba — `Liên quan đến tôi` — không vẽ, xem `PHAN_CHUA_DUNG`. */
export const PHAM_VI_TOAN_XA = "Toàn xã";
export const PHAM_VI_GIAO_CHO_TOI = "Giao cho tôi";

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * NHỮNG PHẦN CỦA ĐẶC TẢ **KHÔNG DỰNG ĐƯỢC**, VÀ CHÚNG PHẢI RA TỚI MÀN HÌNH
 *
 * Không giấu trong chú thích, không vẽ một nút chắc chắn hỏng. Cùng khuôn `PHAN_CHUA_DUNG` của màn
 * Thu - Chi ngân sách.
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

export type PhanChuaDung = {
  readonly ten: string;
  readonly viSao: string;
};

export const PHAN_CHUA_DUNG: readonly PhanChuaDung[] = [
  {
    ten: "Ảnh trước / sau khi xử lý (§8.4)",
    viSao:
      "Bảng `anh_phan_anh` KHÔNG TỒN TẠI — migration `0005_duong_xu_ly_phan_anh.sql:29-33` khai " +
      "thẳng rằng nó cố ý chưa được tạo — và hợp đồng không có tuyến tải ảnh nào. Hệ quả nặng hơn " +
      "một ô ảnh trống: quy tắc §14.2 *“không đóng phiếu được khi thiếu ảnh sau xử lý”* hôm nay " +
      "KHÔNG được cưỡng chế ở đâu cả, vì cả bảng ảnh lẫn cờ `bat_buoc_anh_nghiem_thu` của ADR 0008 " +
      "đều chưa có. Nút Đóng phiếu bên dưới vì thế đóng được một phiếu chưa có ảnh nghiệm thu.",
  },
  {
    ten: "Nhật ký xử lý, dòng thời gian (§8.7)",
    viSao:
      "Bảng `nhat_ky_phan_anh` chưa có và không tuyến nào đọc hay ghi nó (cùng migration, dòng " +
      "34-36). Đây là bản ghi NGHIỆP VỤ cán bộ đọc, khác `audit_log` — vết kiểm toán không hiện ra " +
      "cho xã. Dựng ô nhập `Đã làm gì, ai làm, còn vướng gì…` mà không có chỗ lưu là mời cán bộ gõ " +
      "vào một cái hộp rồi mất.",
  },
  {
    ten: "Bốn thẻ KPI (§3), tab Bản đồ nhiệt (§9), tab Báo cáo (§10)",
    viSao:
      "Không có tuyến thống kê, không có tuyến heatmap, và hợp đồng không trả `lat`/`lng`. Bốn con " +
      "số KPI hay một bảng “theo lĩnh vực” dựng bằng cách đếm trang đang xem sẽ là con số của MỘT " +
      "TRANG chứ không của cả xã — và đó là con số lãnh đạo đọc rồi báo cáo lên trên.",
  },
  {
    ten: "Tab phạm vi `Liên quan đến tôi` (§4, phụ lục §5.1)",
    viSao:
      "Máy chủ trả 400 cho `scope=related`: thế nào là “liên quan” và người liên quan được làm gì " +
      "chưa được chốt với khách. Vẽ tab ấy là vẽ một tab biến quyển sổ thành trang lỗi. Hai tab " +
      "`Toàn xã` và `Giao cho tôi` thì đã có.",
  },
  {
    ten: "Lọc `Bị đánh giá thấp`, ghi nhận đánh giá của người dân (§4, §8.6)",
    viSao:
      "Hợp đồng không trả `diem_hai_long` và không có tuyến ghi đánh giá. Cơ chế 1–2 sao tự mở lại " +
      "phiếu do ba cờ cấu hình của ADR 0008 điều khiển, và không bảng nào trong kho này giữ chúng.",
  },
  {
    ten: "Nút `👁 Cho hiện công khai` / `🚫 Ẩn khỏi trang công khai` (§8.3)",
    viSao:
      "Hợp đồng trả `public` để ĐỌC nhưng không có tuyến ghi nó. Ô kiểm duyệt bên dưới vì thế là " +
      "chữ chỉ đọc.",
  },
  {
    ten: "Email của cán bộ trong ô `Đang giao cho` và ô chọn cán bộ (§8.3, §8.5)",
    viSao:
      "Danh bạ chọn người (`GET /api/v1/staff-directory`) cố ý chỉ trả mã, tên, chức vụ và bộ " +
      "phận — không email, không số điện thoại — để mọi cán bộ đăng nhập đọc được nó. Màn hình vì " +
      "thế hiện `Họ tên · Chức danh` thay cho `Họ tên — email · Chức danh`.",
  },
  {
    ten: "Modal `Nhập hộ phản ánh` (§11)",
    viSao:
      "Không có tuyến vào sổ phía cán bộ: `POST /api/v1/citizen-reports` không tồn tại, chỉ có " +
      "`POST /api/v1/my-citizen-reports` của công dân trong Mini App. Ngoài ra câu mô tả của modal " +
      "ấy đã sai từ ADR 0028 và đang chờ khách duyệt câu thay thế (§11).",
  },
  {
    ten: "Câu giải thích trạng thái, tám trong chín (§8.2)",
    viSao:
      "Đặc tả chỉ cho nguyên văn MỘT câu (`Đang phân loại`). Tám câu còn lại là chữ hiện ra cho cán " +
      "bộ của một cơ quan nhà nước; viết thêm ở đây là tự quyết chữ chưa ai duyệt — đúng chỗ §11 " +
      "của chính chương này vừa hỏng.",
  },
  {
    ten: "`⚠ Quá hạn 3 ngày` — số ngày trễ (§8.3, §7)",
    viSao:
      "Hạn đếm bằng **giờ làm việc** của chính xã, cần lịch làm việc, ngày nghỉ lễ và ngày làm bù " +
      "— ba bảng do `identity` sở hữu (ADR 0007). Đếm bằng giờ treo tường ở trình duyệt sẽ ra một " +
      "con số khác con số của máy chủ vào đúng dịp lễ. Màn hình vì thế nói `Quá hạn` kèm mốc hạn " +
      "cuối, không nói mấy ngày.",
  },
];
