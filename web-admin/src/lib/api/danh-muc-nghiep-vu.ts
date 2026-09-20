/**
 * Bảy danh mục nghiệp vụ của xã — tab "Danh mục" (`docs/ui-ux/14-cau-hinh.md §5`).
 *
 * KHÁC VỚI `danh-muc.ts` Ở BÊN CẠNH, và hai tệp cố ý không gộp làm một: `danh-muc.ts` đọc bộ
 * phận và vai trò để TRA một id ra tên trên màn danh bạ. Bảy tuyến ở đây là chính nội dung của
 * một màn hình khác. Gộp lại thì mở danh bạ cũng kéo theo bảy lời gọi không ai dùng.
 *
 * KIỂU LẤY TỪ HỢP ĐỒNG, KHÔNG GÕ TAY: mọi hình dạng phản hồi đến từ `schema.gen.ts`, sinh ra từ
 * `kb/20-contracts/openapi.json` (ADR 0014). Không dòng nào ở đây mô tả lại một mục danh mục có
 * những trường gì — đó chính là cách v1 có ba bản của cùng một hình dạng (luật 9).
 *
 * VÌ SAO ĐƯỜNG DẪN TƯƠNG ĐỐI, VÌ SAO KHÔNG ĐỤNG COOKIE, VÌ SAO KHÔNG CÓ `tenant_id`: ba quy tắc
 * ấy đúng cho MỌI tuyến và được nói một lần ở `goi.ts`. Đọc tệp ấy trước khi sửa gì ở đây.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * KHÔNG CÓ BỘ NHỚ ĐỆM Ở MỨC MODULE, VÀ ĐÓ LÀ CHỦ Ý — đọc trước khi "tối ưu". Bảy danh mục này
 * nhỏ và hiếm khi đổi, nên một biến giữ lại kết quả trông rất hợp lý. Nó không hợp lý: danh mục
 * là của MỘT xã, và một tiến trình phục vụ nhiều tên miền. Một bản đệm sống lâu hơn yêu cầu là
 * đúng hình dạng của một lần danh mục xã này hiện trên màn hình xã khác — rò dữ liệu giữa hai
 * cơ quan nhà nước, không phải lỗi hiển thị (luật 1, cấm #1; `skills/load-data-once`, cấm #4).
 *
 * Chỗ đúng để đọc một lần là PHẠM VI MỘT MÀN HÌNH: `docDanhMucNghiepVu()` đọc cả bảy đúng một
 * lượt khi mở màn hình, và không dòng nào trong bảng tự đi hỏi máy chủ.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 *
 * KHÔNG CÓ HÀM GHI NÀO, VÀ KHÔNG ĐỂ SẴN CHỖ CHO MỘT HÀM GHI. Hợp đồng không có tuyến ghi cho
 * bất kỳ danh mục nào trong bảy tuyến này, vì câu hỏi mở #21 chưa được khách chốt: xã được sửa
 * cả DANH SÁCH MÃ hay chỉ được sửa nhãn và thứ tự (`kb/00-foundation/open-questions.json`).
 * Một bề mặt ghi chính là hình dạng trả lời câu ấy, nên nó không mọc ở đây trước.
 */

import { docJSON, type KetQua } from "./goi";
import type {
  comms_danhSachLoaiTaiNguyenRa,
  comms_get_map_asset_types,
  documents_danhSachLoaiVanBanRa,
  documents_get_document_types,
  finance_danhSachHangMucRa,
  finance_get_capital_plan_categories,
  identity_danhSachKhoiNhiemVuRa,
  identity_danhSachLoaiDonViDanCuRa,
  identity_get_residential_unit_types,
  identity_get_task_blocs,
  petitions_danhSachLoaiNhiemVuRa,
  petitions_danhSachMucUuTienRa,
  petitions_get_task_priorities,
  petitions_get_task_types,
} from "./schema.gen";

/**
 * MỘT MỤC DANH MỤC, SUY RA TỪ HỢP ĐỒNG CHỨ KHÔNG GÕ LẠI.
 *
 * Bảy tuyến do năm dịch vụ khác nhau phục vụ, và hôm nay bảy hình dạng ấy trùng nhau. Viết ra
 * một `type MucDanhMuc = { id, code, label, is_default, active }` ở đây là dựng đúng bản sao
 * thứ hai của một hình dạng đã có chủ — và bản sao ấy vẫn biên dịch được vào ngày một dịch vụ
 * đổi hình dạng của nó, chỉ là màn hình bắt đầu nói sai (luật 9).
 *
 * Hợp là phép nối bảy kiểu SINH RA. Ngày một dịch vụ bỏ `label` đi, hợp này hẹp lại và `tsc`
 * đỏ ngay tại chỗ dùng — tức là chỗ bảng đang hiển thị nhãn ấy.
 */
export type MucDanhMuc =
  | comms_danhSachLoaiTaiNguyenRa["items"][number]
  | documents_danhSachLoaiVanBanRa["items"][number]
  | finance_danhSachHangMucRa["items"][number]
  | identity_danhSachKhoiNhiemVuRa["items"][number]
  | identity_danhSachLoaiDonViDanCuRa["items"][number]
  | petitions_danhSachLoaiNhiemVuRa["items"][number]
  | petitions_danhSachMucUuTienRa["items"][number];

/**
 * GET /api/v1/map-asset-types — danh mục loại tài nguyên bản đồ của xã.
 *
 * KHÔNG PHÂN TRANG, và hợp đồng không có tham số nào để phân trang: tuyến trả NGUYÊN danh sách
 * (`items`) hoặc hỏng. Máy chủ TỪ CHỐI thay vì cắt bớt khi danh mục vượt trần, đúng để không có
 * một danh sách ngắn đi một cách lặng lẽ (`service-comms/internal/http/loai_tai_nguyen_ban_do.go`).
 *
 * Tuyến là `any-authenticated`: mọi tài khoản đã đăng nhập CỦA CHÍNH XÃ ĐÓ đọc được, vì nhãn
 * danh mục xuất hiện ở ô chọn và bộ lọc của gần như mọi màn hình. Không chéo xã — máy chủ buộc
 * `tenant_id` ở tầng kho.
 */
export function layLoaiTaiNguyenBanDo(): Promise<KetQua<comms_danhSachLoaiTaiNguyenRa>> {
  const duongDan: comms_get_map_asset_types["duongDan"] = "/api/v1/map-asset-types";
  return docJSON<comms_danhSachLoaiTaiNguyenRa>(duongDan);
}

/** GET /api/v1/capital-plan-categories — hạng mục kế hoạch vốn. Xem chú thích hàm trên. */
export function layHangMucKeHoachVon(): Promise<KetQua<finance_danhSachHangMucRa>> {
  const duongDan: finance_get_capital_plan_categories["duongDan"] =
    "/api/v1/capital-plan-categories";
  return docJSON<finance_danhSachHangMucRa>(duongDan);
}

/** GET /api/v1/document-types — loại văn bản. Xem chú thích `layLoaiTaiNguyenBanDo`. */
export function layLoaiVanBan(): Promise<KetQua<documents_danhSachLoaiVanBanRa>> {
  const duongDan: documents_get_document_types["duongDan"] = "/api/v1/document-types";
  return docJSON<documents_danhSachLoaiVanBanRa>(duongDan);
}

/** GET /api/v1/residential-unit-types — loại đơn vị dân cư (thôn / tổ dân phố). */
export function layLoaiDonViDanCu(): Promise<KetQua<identity_danhSachLoaiDonViDanCuRa>> {
  const duongDan: identity_get_residential_unit_types["duongDan"] =
    "/api/v1/residential-unit-types";
  return docJSON<identity_danhSachLoaiDonViDanCuRa>(duongDan);
}

/** GET /api/v1/task-blocs — khối nhiệm vụ (Khối Uỷ ban / Khối Đảng / Khác). */
export function layKhoiNhiemVu(): Promise<KetQua<identity_danhSachKhoiNhiemVuRa>> {
  const duongDan: identity_get_task_blocs["duongDan"] = "/api/v1/task-blocs";
  return docJSON<identity_danhSachKhoiNhiemVuRa>(duongDan);
}

/** GET /api/v1/task-types — loại nhiệm vụ. */
export function layLoaiNhiemVu(): Promise<KetQua<petitions_danhSachLoaiNhiemVuRa>> {
  const duongDan: petitions_get_task_types["duongDan"] = "/api/v1/task-types";
  return docJSON<petitions_danhSachLoaiNhiemVuRa>(duongDan);
}

/**
 * GET /api/v1/task-priorities — thang mức ưu tiên nhiệm vụ.
 *
 * THỨ TỰ CỦA `items` LÀ THANG BẬC, không phải sở thích trình bày. Máy chủ ghi rõ điều này
 * (`service-petitions/internal/http/muc_uu_tien_nhiem_vu.go`): "khẩn" chỉ là khẩn khi đặt cạnh
 * những mức xung quanh nó. Một chỗ nào đó sắp xếp lại mảng này theo vần chữ cái không phải là
 * đổi cách trình bày — nó đổi mức việc mà xã coi là gấp nhất, và màn hình vẫn trông bình thường.
 * Vì vậy không hàm nào trong chuỗi từ đây tới bảng được sắp xếp lại mảng.
 */
export function layMucUuTienNhiemVu(): Promise<KetQua<petitions_danhSachMucUuTienRa>> {
  const duongDan: petitions_get_task_priorities["duongDan"] = "/api/v1/task-priorities";
  return docJSON<petitions_danhSachMucUuTienRa>(duongDan);
}

/** Bảy danh mục mà tab "Danh mục" cần, đọc trong cùng một lượt. */
export type BayDanhMuc = {
  loaiTaiNguyenBanDo: KetQua<comms_danhSachLoaiTaiNguyenRa>;
  hangMucKeHoachVon: KetQua<finance_danhSachHangMucRa>;
  loaiVanBan: KetQua<documents_danhSachLoaiVanBanRa>;
  loaiDonViDanCu: KetQua<identity_danhSachLoaiDonViDanCuRa>;
  khoiNhiemVu: KetQua<identity_danhSachKhoiNhiemVuRa>;
  loaiNhiemVu: KetQua<petitions_danhSachLoaiNhiemVuRa>;
  mucUuTienNhiemVu: KetQua<petitions_danhSachMucUuTienRa>;
};

/**
 * Đọc cả bảy danh mục — ĐÚNG BẢY lời gọi cho cả màn hình, dù mỗi danh mục có bao nhiêu mục.
 *
 * BẢY KẾT QUẢ RỜI NHAU, KHÔNG GỘP THÀNH MỘT. Năm dịch vụ khác nhau đứng sau bảy tuyến này, nên
 * một dịch vụ chạm trần hay đang khởi động lại là chuyện có thật và chỉ nên làm im lặng ĐÚNG
 * khối của nó. Gộp lại là biến một sự cố của một dịch vụ thành bảy danh mục cùng không hiện.
 *
 * `Promise.all` chứ không phải bảy lần `await` nối tiếp: bảy tuyến không phụ thuộc nhau, nên
 * chờ tuần tự chỉ cộng bảy vòng mạng vào thời gian mở màn hình. Mọi hàm đều không ném
 * (`docJSON` gói lỗi vào `KetQua`), nên `Promise.all` ở đây không có nhánh `reject` nào.
 */
export function docDanhMucNghiepVu(): Promise<BayDanhMuc> {
  return Promise.all([
    layLoaiTaiNguyenBanDo(),
    layHangMucKeHoachVon(),
    layLoaiVanBan(),
    layLoaiDonViDanCu(),
    layKhoiNhiemVu(),
    layLoaiNhiemVu(),
    layMucUuTienNhiemVu(),
  ]).then(
    ([
      loaiTaiNguyenBanDo,
      hangMucKeHoachVon,
      loaiVanBan,
      loaiDonViDanCu,
      khoiNhiemVu,
      loaiNhiemVu,
      mucUuTienNhiemVu,
    ]) => ({
      loaiTaiNguyenBanDo,
      hangMucKeHoachVon,
      loaiVanBan,
      loaiDonViDanCu,
      khoiNhiemVu,
      loaiNhiemVu,
      mucUuTienNhiemVu,
    }),
  );
}
