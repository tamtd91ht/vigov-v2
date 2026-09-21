/**
 * Câu chữ của màn tra cứu phiếu phản ánh (`docs/ui-ux/09-phan-anh-nguoi-dan.md §8`). Hàm thuần:
 * không gọi mạng, không dựng DOM. Thời điểm hiện tại được TRUYỀN VÀO, không đọc từ đồng hồ ở
 * đây — một hàm tự đọc đồng hồ là một hàm không kiểm được ở đúng lúc quan trọng nhất: ngay trước
 * và ngay sau hạn.
 */

import type { petitions_phieuPhanAnhRa } from "@/lib/api/schema.gen";

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
 * `reporter_name` về dạng `Nguyễn V. A.` và `reporter_phone` về dạng `09****5678`: máy chủ che ở
 * đường ra vì hôm nay KHÔNG có khoá quyền nào mở xem đầy đủ. Hệ quả có thật và được nói ra chứ
 * không giấu: cán bộ KHÔNG gọi lại được cho người phản ánh từ màn hình này. Khoá quyền nào mở số
 * đầy đủ là câu hỏi của khách (luật 3, điều kiện dừng #1), và che là câu trả lời đóng-khi-chưa-rõ
 * trong lúc chờ.
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

/** Câu dẫn của ô nhập mã. Nói rõ màn này tra MỘT phiếu, không phải danh sách. */
export const HUONG_DAN_TRA_CUU =
  "Nhập mã tra cứu đã trả cho người dân để mở đúng một phiếu. Màn hình này chưa có danh sách " +
  "phiếu: hợp đồng chưa có tuyến trả về danh sách phản ánh.";

/** Chưa gõ mã nào. Trạng thái BÌNH THƯỜNG lúc mở màn, không phải lỗi. */
export const CHUA_TRA_CUU = "Chưa tra phiếu nào. Nhập mã tra cứu rồi bấm Tra cứu.";
