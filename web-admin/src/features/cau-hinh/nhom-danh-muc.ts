/**
 * Bảy danh mục nghiệp vụ → thứ tự các nhóm trên màn hình, và trạng thái của từng nhóm.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * VÌ SAO TÁCH KHỎI `.tsx`, cùng lý do đã ghi ở `ma-tran-quyen.ts` và `tra-danh-muc.ts`: ca
 * đáng lo nhất của màn hình này — ĐỌC ĐƯỢC VÀ KHÔNG CÓ MỤC NÀO — không nhìn thấy bằng mắt mà
 * lại là ca xảy ra Ở MỌI ĐƠN VỊ hôm nay. Nằm lẫn trong một component thì không bài test nào
 * chạm tới nó, và cách nó hỏng là hỏng thành một bảng trống không ai giải thích.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 *
 * BỐN TRẠNG THÁI CỦA MỘT NHÓM, BA Ở TỆP NÀY. Trạng thái thứ tư — `đang tải` — không nằm ở đây
 * và đó là chủ ý, khác với `tra-danh-muc.ts` ngay bên cạnh:
 *
 *   · Ở `tra-danh-muc.ts` pha "chưa đọc xong" PHẢI đi xuống tới từng ô của từng dòng, vì mỗi ô
 *     tra một id khác nhau và component không có chỗ nào rẽ nhánh một lần cho cả bảng.
 *   · Ở đây cả bảy danh mục về trong MỘT lượt (`docDanhMucNghiepVu`), nên toàn màn hình có đúng
 *     một khoảnh khắc "đang tải". Mang nó vào đây nữa là hai chỗ cùng mô tả một sự thật, và
 *     ngày chúng lệch nhau thì màn hình vừa báo đang tải vừa báo chưa có mục (luật 9).
 *
 *   `khongDocDuoc`   401 · 403 · 500 · mạng hỏng. Hiện đúng câu của máy chủ, không diễn giải.
 *   `chuaCoMuc`      đọc được, và danh mục không có mục nào. KHÔNG phải lỗi — xem dưới.
 *   `coMuc`          dựng được bảng.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * `chuaCoMuc` LÀ ĐƯỜNG THÔNG THƯỜNG, KHÔNG PHẢI CA BIÊN — đọc kỹ trước khi sửa nhánh này.
 *
 * Hôm nay BẢY danh mục này rỗng ở MỌI đơn vị, và sẽ còn rỗng: các migration cố ý không gieo
 * mục nào, và bước khởi tạo đơn vị — nơi những mục đầu tiên được lập — chưa tồn tại. Máy chủ
 * nói rõ điều đó ngay tại chỗ dựng phản hồi (`service-petitions/internal/http/loai_nhiem_vu.go`
 * và `muc_uu_tien_nhiem_vu.go`: "[] and never null; an empty scale is today's correct answer for
 * every commune").
 *
 * Nên nhánh này phải NÓI RA THÀNH CÂU. Một vòng quay không bao giờ dừng, một bảng trống, hay
 * một dòng báo lỗi đều mô tả sai một hệ thống đang chạy đúng — và một màn hình trông như hỏng
 * ở một cơ quan nhà nước là một cuộc gọi hỗ trợ, không phải một chi tiết giao diện.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 */

import type { BayDanhMuc, MucDanhMuc } from "@/lib/api/danh-muc-nghiep-vu";
import type { KetQua } from "@/lib/api/goi";

export type TrangThaiNhom =
  | { pha: "khongDocDuoc"; thongBao: string }
  | { pha: "chuaCoMuc" }
  | { pha: "coMuc"; muc: readonly MucDanhMuc[] };

/** Khoá của một nhóm — lấy thẳng từ `BayDanhMuc` để không có bản kê thứ hai của bảy danh mục. */
export type KhoaNhom = keyof BayDanhMuc;

export type NhomDanhMuc = {
  khoa: KhoaNhom;
  /** Tên nhóm theo `docs/ui-ux/14-cau-hinh.md §5`, không tự đặt lại. */
  nhan: string;
  /**
   * Thứ tự các mục trong nhóm này CÓ NGHĨA NGHIỆP VỤ hay chỉ là cách đơn vị sắp xếp.
   *
   * Đúng một nhóm trả lời `true`: mức ưu tiên nhiệm vụ. Xem chú thích trên `BANG_NHOM`.
   */
  thuTuLaThangBac: boolean;
  trangThai: TrangThaiNhom;
};

/**
 * Bảy nhóm, THEO ĐÚNG THỨ TỰ BẢNG §5 CỦA ĐẶC TẢ — không theo vần chữ cái, không theo tên dịch vụ.
 *
 * BA NHÓM CỦA ĐẶC TẢ KHÔNG CÓ Ở ĐÂY, và chúng vắng vì không có tuyến chứ không vì ai bỏ sót:
 * `Lĩnh vực phản ánh`, `Loại đơn thư`, `Trạng thái nhiệm vụ` chưa có tuyến nào trong hợp đồng REST
 * (`kb/20-contracts/openapi.json`). Riêng `Trạng thái nhiệm vụ` còn là tâm điểm của câu hỏi mở #21
 * — nó không mọc ra ở đây trước khi khách trả lời.
 *
 * VÌ SAO CÓ CỜ `thuTuLaThangBac` THAY VÌ MỘT NHÁNH `if (khoa === "mucUuTienNhiemVu")` rải trong
 * component: thứ tự của `items` ở mức ưu tiên LÀ thang bậc của đơn vị, không phải sở thích trình
 * bày (`service-petitions/internal/store/muc_uu_tien_nhiem_vu.go`: "THE ORDER IS THE MEANING OF
 * THIS LIST"). Một chỗ nào đó sắp lại mảng ấy theo vần không đổi cách trình bày — nó đổi mức việc
 * mà đơn vị coi là gấp nhất, và màn hình vẫn trông bình thường. Cờ ở đây chỉ bật một câu giải
 * thích cho người đọc; không hàm nào trong tệp này sắp xếp lại mảng nào.
 */
const BANG_NHOM: readonly { khoa: KhoaNhom; nhan: string; thuTuLaThangBac: boolean }[] = [
  { khoa: "loaiTaiNguyenBanDo", nhan: "Loại tài nguyên bản đồ", thuTuLaThangBac: false },
  { khoa: "hangMucKeHoachVon", nhan: "Hạng mục kế hoạch vốn", thuTuLaThangBac: false },
  { khoa: "loaiVanBan", nhan: "Loại văn bản", thuTuLaThangBac: false },
  { khoa: "loaiDonViDanCu", nhan: "Loại đơn vị dân cư", thuTuLaThangBac: false },
  { khoa: "khoiNhiemVu", nhan: "Khối nhiệm vụ", thuTuLaThangBac: false },
  { khoa: "loaiNhiemVu", nhan: "Loại nhiệm vụ", thuTuLaThangBac: false },
  { khoa: "mucUuTienNhiemVu", nhan: "Mức ưu tiên nhiệm vụ", thuTuLaThangBac: true },
];

/**
 * Bảy kết quả đọc → bảy nhóm dựng được, giữ nguyên thứ tự `BANG_NHOM`.
 *
 * BẢY NHÓM LUÔN ĐỦ BẢY, kể cả khi một tuyến hỏng: năm dịch vụ khác nhau đứng sau bảy tuyến này,
 * nên một dịch vụ đang khởi động lại là chuyện có thật. Bỏ nhóm ấy khỏi màn hình là để người dùng
 * kết luận đơn vị không có danh mục đó — một câu sai. Nhóm vẫn hiện, kèm đúng câu máy chủ trả về.
 */
export function nhomDanhMuc(bay: BayDanhMuc): readonly NhomDanhMuc[] {
  return BANG_NHOM.map((n) => ({ ...n, trangThai: trangThaiNhom(bay[n.khoa]) }));
}

/**
 * Một kết quả đọc → trạng thái của một nhóm.
 *
 * Tham số cố ý chỉ đòi `{ items }` với phần tử là `MucDanhMuc`: bảy tuyến do năm dịch vụ phục vụ
 * và kiểu phản hồi của chúng là BẢY kiểu sinh ra khác nhau, nên một chữ ký nhắc tên bảy kiểu ấy
 * sẽ phải sửa mỗi lần thêm một danh mục. `MucDanhMuc` là hợp của bảy kiểu SINH RA từ hợp đồng
 * (`lib/api/danh-muc-nghiep-vu.ts`) — không dòng nào ở đây gõ lại một mục danh mục có trường gì.
 *
 * `items` RỖNG KHÔNG BAO GIỜ ĐƯỢC ĐỌC THÀNH LỖI, và cũng không bao giờ được đọc thành `null`:
 * máy chủ trả `[]` chứ không trả `null` (mọi tuyến đều `make(..., 0, len(ds))`).
 */
export function trangThaiNhom(kq: KetQua<{ items: readonly MucDanhMuc[] }>): TrangThaiNhom {
  if (!kq.ok) return { pha: "khongDocDuoc", thongBao: kq.thongBao };
  if (kq.duLieu.items.length === 0) return { pha: "chuaCoMuc" };
  return { pha: "coMuc", muc: kq.duLieu.items };
}
