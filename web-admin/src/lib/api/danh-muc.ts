/**
 * HAI PHẦN TRONG MỘT TỆP, VÀ RANH GIỚI GIỮA CHÚNG NẰM Ở CHỮ "GHI" — đọc đoạn này trước khi thêm
 * hàm vào đây:
 *
 *   · PHẦN MỘT (ngay dưới) — hai danh mục ĐỌC để tra một id ra tên trên màn danh bạ: bộ phận và
 *     vai trò. Không có đường ghi nào, và không nên có.
 *   · PHẦN HAI (cuối tệp) — ĐƯỜNG GHI của tab "Danh mục" (`14-cau-hinh §5`): năm danh mục nghiệp
 *     vụ mà đơn vị tự sửa được. Phần đọc của năm danh mục ấy đã có chủ ở
 *     `danh-muc-nghiep-vu.ts` và được DÙNG LẠI từ đây chứ không viết lần thứ hai.
 *
 * Hai danh mục của xã: bộ phận (`GET /api/v1/org-units`) và vai trò (`GET /api/v1/roles`).
 *
 * KIỂU LẤY TỪ HỢP ĐỒNG, KHÔNG GÕ TAY: `identity_danhSachBoPhanRa` và `identity_danhSachVaiTroRa`
 * đến từ `schema.gen.ts`. Không tệp nào ở đây mô tả lại một bộ phận hay một vai trò có những
 * trường gì (luật 9).
 *
 * VÌ SAO ĐƯỜNG DẪN TƯƠNG ĐỐI, VÌ SAO KHÔNG ĐỤNG COOKIE, VÌ SAO KHÔNG CÓ `tenant_id`: ba quy tắc
 * ấy đúng cho MỌI tuyến và được nói một lần ở `goi.ts`. Đọc tệp ấy trước khi sửa gì ở đây.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * KHÔNG CÓ BỘ NHỚ ĐỆM Ở TỆP NÀY, VÀ ĐÓ LÀ CHỦ Ý — đọc trước khi "tối ưu".
 *
 * Hai danh mục này là danh sách đóng và nhỏ (khoảng mười bộ phận, tám vai trò), nên một biến
 * ở mức module giữ lại kết quả trông rất hợp lý. Nó không hợp lý: danh mục là của MỘT xã. Một
 * bản đệm sống lâu hơn yêu cầu, trong một tiến trình phục vụ nhiều tên miền, là đúng hình dạng
 * của một lần tên bộ phận của xã này hiện trên màn hình xã khác — một vụ rò dữ liệu giữa hai cơ
 * quan nhà nước, không phải một lỗi hiển thị (luật 1, cấm #1; `skills/load-data-once`, cấm #4).
 *
 * Chỗ đúng để đọc một lần là PHẠM VI MỘT MÀN HÌNH: `docDanhMucDanhBa()` dưới đây đọc cả hai
 * danh mục đúng một lượt khi mở màn hình, rồi màn hình chuyền bảng tra xuống từng dòng. Mỗi
 * dòng tự gọi là mẫu N+1 — ở danh bạ hai mươi dòng thì thành hai mươi lời gọi, và con số ấy
 * chỉ đi lên (`skills/load-data-once`, dạng 1).
 * ─────────────────────────────────────────────────────────────────────────────────────────
 */

import {
  layHangMucKeHoachVon,
  layLoaiNhiemVu,
  layLoaiTaiNguyenBanDo,
  layLoaiVanBan,
  layMucUuTienNhiemVu,
} from "./danh-muc-nghiep-vu";
import { LOI_KHONG_RO, docJSON, goiGhi, type KetQua } from "./goi";
import type {
  comms_delete_map_asset_types_by_id,
  comms_loaiTaiNguyenRa,
  comms_patch_map_asset_types_by_id,
  comms_post_map_asset_types,
  comms_suaLoaiTaiNguyenVao,
  comms_themLoaiTaiNguyenVao,
  comms_xoaLoaiTaiNguyenVao,
  documents_delete_document_types_by_id,
  documents_loaiVanBanRa,
  documents_patch_document_types_by_id,
  documents_post_document_types,
  documents_suaLoaiVanBanVao,
  documents_themLoaiVanBanVao,
  documents_xoaLoaiVanBanVao,
  finance_delete_capital_plan_categories_by_id,
  finance_hangMucRa,
  finance_patch_capital_plan_categories_by_id,
  finance_post_capital_plan_categories,
  finance_suaHangMucVao,
  finance_themHangMucVao,
  finance_xoaHangMucVao,
  identity_danhSachBoPhanRa,
  identity_danhSachVaiTroRa,
  identity_get_org_units,
  identity_get_roles,
  petitions_delete_task_priorities_by_id,
  petitions_delete_task_types_by_id,
  petitions_loaiNhiemVuRa,
  petitions_mucUuTienRa,
  petitions_patch_task_priorities_by_id,
  petitions_patch_task_types_by_id,
  petitions_post_task_priorities,
  petitions_post_task_types,
  petitions_suaLoaiNhiemVuVao,
  petitions_suaMucUuTienVao,
  petitions_themLoaiNhiemVuVao,
  petitions_themMucUuTienVao,
  petitions_xoaLoaiNhiemVuVao,
  petitions_xoaMucUuTienVao,
} from "./schema.gen";

/**
 * GET /api/v1/org-units — cây bộ phận của xã.
 *
 * KHÔNG PHÂN TRANG, và hợp đồng không có tham số nào để phân trang: tuyến trả NGUYÊN danh
 * sách (`items`). Đó là điều kiện để một bảng tra id → tên là đầy đủ; một danh mục trả về từng
 * trang sẽ cho ra những id "không tra được" chỉ vì chúng nằm ở trang sau.
 *
 * Tuyến là `any-authenticated`: mọi tài khoản đã đăng nhập CỦA CHÍNH XÃ ĐÓ đọc được, vì tên bộ
 * phận xuất hiện ở ô phân công, luồng văn bản và mọi bộ lọc. Không chéo xã — máy chủ buộc
 * `tenant_id` ở tầng kho.
 */
export function layDanhMucBoPhan(): Promise<KetQua<identity_danhSachBoPhanRa>> {
  const duongDan: identity_get_org_units["duongDan"] = "/api/v1/org-units";
  return docJSON<identity_danhSachBoPhanRa>(duongDan);
}

/** GET /api/v1/roles — danh mục vai trò của xã. Cũng trả nguyên danh sách; xem hàm trên. */
export function layDanhMucVaiTro(): Promise<KetQua<identity_danhSachVaiTroRa>> {
  const duongDan: identity_get_roles["duongDan"] = "/api/v1/roles";
  return docJSON<identity_danhSachVaiTroRa>(duongDan);
}

/** Hai danh mục mà một màn hình danh bạ cần, đọc trong cùng một lượt. */
export type DanhMucDanhBa = {
  boPhan: KetQua<identity_danhSachBoPhanRa>;
  vaiTro: KetQua<identity_danhSachVaiTroRa>;
};

/**
 * Đọc cả hai danh mục — ĐÚNG HAI lời gọi, cho cả màn hình, dù danh bạ có bao nhiêu dòng.
 *
 * HAI KẾT QUẢ RỜI NHAU, KHÔNG GỘP THÀNH MỘT. Danh mục vai trò hỏng không có lý do gì làm cột
 * Bộ phận trống theo: hai cột, hai câu trả lời, và mỗi cột nói đúng chuyện của nó. Gộp lại là
 * biến một sự cố của một tuyến thành hai cột cùng im lặng.
 *
 * `Promise.all` chứ không phải hai lần `await` nối tiếp: hai tuyến không phụ thuộc nhau, nên
 * chờ tuần tự chỉ cộng thêm một vòng mạng vào thời gian mở màn hình. Cả hai hàm đều không ném
 * (`docJSON` gói lỗi vào `KetQua`), nên `Promise.all` ở đây không có nhánh `reject` nào.
 */
export function docDanhMucDanhBa(): Promise<DanhMucDanhBa> {
  return Promise.all([layDanhMucBoPhan(), layDanhMucVaiTro()]).then(([boPhan, vaiTro]) => ({
    boPhan,
    vaiTro,
  }));
}

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * PHẦN HAI — ĐƯỜNG GHI CỦA TAB "DANH MỤC" (`docs/ui-ux/14-cau-hinh.md §5`)
 *
 * Năm danh mục nghiệp vụ mà đơn vị tự sửa được, mỗi danh mục ba thao tác ghi: thêm · sửa · xoá
 * mềm. Phần ĐỌC của đúng năm danh mục ấy đã có chủ ở `danh-muc-nghiep-vu.ts` và được dùng lại
 * ở đây — một hàm đọc thứ hai là một bản sao sẽ trôi (luật 9, cấm #2).
 *
 * ──────────────────────────────────────────────────────────────────────────────────────────
 * BA TẦNG, VÀ VÌ SAO TẦNG KHÔNG BAO GIỜ ĐI LÊN TRONG THÂN YÊU CẦU (ADR 0024).
 *
 *   tầng 1 · `source: "don-vi"`   — đơn vị tự thêm: xoá mềm CÓ · tắt CÓ · đổi nhãn CÓ
 *   tầng 2 · `source: "he-thong"` — phần mềm cấp:   xoá mềm KHÔNG · tắt CÓ · đổi nhãn CÓ
 *   tầng 3 · `he-thong` + mã nguồn rẽ nhánh theo mã: xoá KHÔNG · tắt KHÔNG · đổi nhãn CÓ
 *
 * `source` và `tier` CHỈ ĐI XUỐNG, KHÔNG BAO GIỜ ĐI LÊN. Máy chủ từ chối 400 ngay khi thân yêu
 * cầu NHẮC TỚI một trong hai, kể cả khi giá trị gửi lên đúng bằng giá trị nó vừa trả về
 * (`service-documents/internal/http/loai_van_ban.go:247` và `:284`; bốn dịch vụ kia y hệt).
 * Đó không phải sự khắt khe thừa: `nguon` quyết định tầng, nên một client đặt được `nguon` là
 * một client tự xếp dòng của mình vào tầng 2 rồi đi vòng qua mọi rào phía dưới
 * (`service-documents/internal/domain/danh_muc_ba_tang.go:32-36`).
 *
 * Hệ quả cho tệp này, và là lý do thân yêu cầu được DỰNG TỪNG TRƯỜNG chứ không trải từ dòng
 * đang có: màn hình đọc một dòng rồi gửi lại chính nó là lối viết tự nhiên nhất, và nó gửi kèm
 * `source` với `tier` — 400 cho một thao tác không có gì sai. Kiểu `Omit<...>` chặn lúc biên
 * dịch, phép dựng từng trường chặn lúc chạy; cần cả hai, vì kiểu không sống tới lúc chạy.
 *
 * `code` CŨNG KHÔNG ĐI LÊN TRONG PATCH. Mã đã cấp thì không đổi (luật 7, bất biến 3): hồ sơ
 * nghiệp vụ giữ mã ẤY LÀM GIÁ TRỊ và không gì viết lại chúng. Máy chủ trả 400 `ErrMaBatBien`
 * (`loai_van_ban.go:288`), và trigger CSDL từ chối lần nữa
 * (`service-documents/migrations/0003_danh_muc_loai_van_ban.sql:149`).
 * ══════════════════════════════════════════════════════════════════════════════════════════
 */

/**
 * Một mục của năm danh mục có đường ghi — hợp của năm kiểu SINH RA, không gõ lại.
 *
 * KHÁC `MucDanhMuc` Ở `danh-muc-nghiep-vu.ts`: hợp ấy có BẢY nhánh, gồm hai danh mục của
 * identity không có `order`, `source`, `tier` và cũng không có tuyến ghi nào. Dùng hợp bảy
 * nhánh ở đây thì `m.tier` không biên dịch được — và cách "sửa" gần nhất là ép kiểu, tức là
 * màn hình tự khẳng định một trường mà hợp đồng không hứa.
 */
export type MucDanhMucGhi =
  | comms_loaiTaiNguyenRa
  | documents_loaiVanBanRa
  | finance_hangMucRa
  | petitions_loaiNhiemVuRa
  | petitions_mucUuTienRa;

/** Khoá năm nhóm của tab. Đây là cách MÀN HÌNH gom nhóm, không phải dữ liệu của hợp đồng. */
export type KhoaDanhMucGhi =
  | "loaiTaiNguyenBanDo"
  | "hangMucKeHoachVon"
  | "loaiVanBan"
  | "loaiNhiemVu"
  | "mucUuTienNhiemVu";

/** Một lượt đọc trọn một danh mục. Kiểu chung để bảng đọc được `tier` của mọi nhóm. */
export type DocDanhMucGhi = () => Promise<KetQua<{ items: readonly MucDanhMucGhi[] }>>;

/**
 * Một nhóm danh mục: hai đường ghi của nó, và cách đọc nó.
 *
 * KHÔNG MANG TÊN HIỂN THỊ. Tên nhóm là chữ của màn hình và đã có chủ ở
 * `features/cau-hinh/nhom-danh-muc.ts`; một bản thứ hai ở tầng API là bản sẽ trôi, và bản trôi
 * là bản hiện trên màn hình (luật 9, cấm #2).
 *
 * MÔ TẢ ĐƯỢC CHUYỀN THẲNG VÀO HÀM GHI, không có bảng tra khoá → đường dẫn ở giữa. Một bảng tra
 * đẻ ra một nhánh "không tìm thấy khoá" mà không lời gọi hợp lệ nào chạm tới, và nhánh ấy phải
 * chọn giữa ném lỗi hay trả về một mặc định — mặc định ở đây nghĩa là ghi vào nhầm danh mục.
 */
export type MoTaDanhMucGhi = {
  readonly khoa: KhoaDanhMucGhi;
  /** POST vào đây. */
  readonly gocThem: string;
  /** PATCH và DELETE vào đây, `{id}` thay bằng id của mục. */
  readonly mauMuc: string;
  readonly doc: DocDanhMucGhi;
};

/**
 * Năm nhóm, THEO ĐÚNG THỨ TỰ BẢNG §5 — không theo vần chữ cái, không theo tên dịch vụ.
 *
 * ĐƯỜNG DẪN KHÔNG PHẢI CHUỖI GÕ TAY: mỗi chuỗi mang `satisfies <kiểu sinh ra>["duongDan"]`, nên
 * ngày một tuyến đổi đường dẫn trong hợp đồng thì `tsc` đỏ ngay tại dòng này — chứ không phải
 * lúc chạy, bằng một 404 mà cán bộ đọc thành "không lưu được".
 *
 * NĂM NHÓM NÀY KHÔNG PHẢI MƯỜI NHÓM CỦA ĐẶC TẢ. Năm nhóm còn lại vắng vì KHÔNG CÓ TUYẾN GHI chứ
 * không vì ai bỏ sót: `Loại đơn vị dân cư` và `Khối nhiệm vụ` mới chỉ có tuyến đọc, còn
 * `Lĩnh vực phản ánh`, `Loại đơn thư` và `Trạng thái nhiệm vụ` chưa có tuyến nào. Riêng
 * `Trạng thái nhiệm vụ` là tâm điểm câu hỏi mở #21 và không được mọc ra ở đây trước khi khách
 * trả lời (`kb/00-foundation/open-questions.json`).
 */
export const NAM_DANH_MUC_GHI: readonly MoTaDanhMucGhi[] = [
  {
    khoa: "loaiTaiNguyenBanDo",
    gocThem: "/api/v1/map-asset-types" satisfies comms_post_map_asset_types["duongDan"],
    mauMuc: "/api/v1/map-asset-types/{id}" satisfies comms_patch_map_asset_types_by_id["duongDan"] &
      comms_delete_map_asset_types_by_id["duongDan"],
    doc: layLoaiTaiNguyenBanDo,
  },
  {
    khoa: "hangMucKeHoachVon",
    gocThem:
      "/api/v1/capital-plan-categories" satisfies finance_post_capital_plan_categories["duongDan"],
    mauMuc:
      "/api/v1/capital-plan-categories/{id}" satisfies finance_patch_capital_plan_categories_by_id["duongDan"] &
        finance_delete_capital_plan_categories_by_id["duongDan"],
    doc: layHangMucKeHoachVon,
  },
  {
    khoa: "loaiVanBan",
    gocThem: "/api/v1/document-types" satisfies documents_post_document_types["duongDan"],
    mauMuc:
      "/api/v1/document-types/{id}" satisfies documents_patch_document_types_by_id["duongDan"] &
        documents_delete_document_types_by_id["duongDan"],
    doc: layLoaiVanBan,
  },
  {
    khoa: "loaiNhiemVu",
    gocThem: "/api/v1/task-types" satisfies petitions_post_task_types["duongDan"],
    mauMuc: "/api/v1/task-types/{id}" satisfies petitions_patch_task_types_by_id["duongDan"] &
      petitions_delete_task_types_by_id["duongDan"],
    doc: layLoaiNhiemVu,
  },
  {
    khoa: "mucUuTienNhiemVu",
    gocThem: "/api/v1/task-priorities" satisfies petitions_post_task_priorities["duongDan"],
    mauMuc:
      "/api/v1/task-priorities/{id}" satisfies petitions_patch_task_priorities_by_id["duongDan"] &
        petitions_delete_task_priorities_by_id["duongDan"],
    doc: layMucUuTienNhiemVu,
  },
];

/**
 * Thân POST mà màn hình được phép gửi — thân của hợp đồng, TRỪ `source` và `tier`.
 *
 * GIAO CỦA NĂM KIỂU SINH RA, KHÔNG PHẢI MỘT KIỂU CHỌN LÀM ĐẠI DIỆN. Năm dịch vụ phát ra năm kiểu
 * riêng, và hôm nay chúng trùng nhau. Chọn một kiểu làm chuẩn thì ngày một dịch vụ đổi hình dạng,
 * bốn lời gọi kia vẫn biên dịch. Phép giao thì không: một trường bắt buộc mới ở bất kỳ dịch vụ
 * nào cũng hiện ra ở đây và làm đỏ mọi chỗ dựng thân yêu cầu.
 */
export type ThemMucVao = Omit<comms_themLoaiTaiNguyenVao, "source" | "tier"> &
  Omit<documents_themLoaiVanBanVao, "source" | "tier"> &
  Omit<finance_themHangMucVao, "source" | "tier"> &
  Omit<petitions_themLoaiNhiemVuVao, "source" | "tier"> &
  Omit<petitions_themMucUuTienVao, "source" | "tier">;

/**
 * Thân PATCH — trừ `source`, `tier` VÀ `code`.
 *
 * `code` bị trừ ở đây chứ không phải bị bỏ quên: mã đã cấp thì không đổi được (luật 7, bất biến
 * 3), và một ô nhập mã trong biểu mẫu sửa là một ô hứa điều máy chủ sẽ từ chối. Đổi cách gọi một
 * mục là đổi `label`; thay hẳn khái niệm là thêm dòng mới rồi tắt dòng cũ.
 */
export type SuaMucVao = Omit<comms_suaLoaiTaiNguyenVao, "source" | "tier" | "code"> &
  Omit<documents_suaLoaiVanBanVao, "source" | "tier" | "code"> &
  Omit<finance_suaHangMucVao, "source" | "tier" | "code"> &
  Omit<petitions_suaLoaiNhiemVuVao, "source" | "tier" | "code"> &
  Omit<petitions_suaMucUuTienVao, "source" | "tier" | "code">;

/** Thân DELETE. `reason` BẮT BUỘC — luật 7, bất biến 1: `deleted_at` · `deleted_by` · lý do. */
export type XoaMucVao = comms_xoaLoaiTaiNguyenVao &
  documents_xoaLoaiVanBanVao &
  finance_xoaHangMucVao &
  petitions_xoaLoaiNhiemVuVao &
  petitions_xoaMucUuTienVao;

/** Đường dẫn của một mục cụ thể. `encodeURIComponent` vì id đi vào ĐƯỜNG DẪN, không vào thân. */
function duongDanMuc(mauMuc: string, id: string): string {
  return mauMuc.replace("{id}", encodeURIComponent(id));
}

/** Đọc thân JSON của một phản hồi đã thành công. Thân hỏng là "không đọc được", không phải 200. */
async function docThanRa(phanHoi: Response): Promise<KetQua<MucDanhMucGhi>> {
  try {
    return { ok: true, duLieu: (await phanHoi.json()) as MucDanhMucGhi };
  } catch {
    return { ok: false, thongBao: LOI_KHONG_RO };
  }
}

/**
 * POST — thêm một mục của riêng đơn vị. Mục thêm từ đây LUÔN là tầng 1; máy chủ ghi
 * `source: "don-vi"` làm hằng và không đọc trường ấy từ yêu cầu.
 *
 * `khoaChongTrung` LÀ THAM SỐ, KHÔNG PHẢI SINH TẠI CHỖ. Hợp đồng đòi `Idempotency-Key` cho tuyến
 * này; sinh khoá bên trong hàm thì mỗi lần bấm lại sau một lỗi mạng là một khoá mới — tức đúng
 * cái mà khoá chống trùng sinh ra để chặn, vì lần gửi đầu CÓ THỂ đã tới máy chủ. Khoá vì vậy do
 * biểu mẫu giữ, sống bằng đời một lần gửi, và lần bấm lại dùng lại chính nó.
 */
export async function themMuc(
  mo: MoTaDanhMucGhi,
  than: ThemMucVao,
  khoaChongTrung: string,
): Promise<KetQua<MucDanhMucGhi>> {
  // DỰNG TỪNG TRƯỜNG, KHÔNG TRẢI TỪ ĐỐI TƯỢNG NGUỒN — xem đầu phần hai. Một `...than` ở đây là
  // đường để `source`/`tier` đi lên vào ngày ai đó truyền vào một dòng đọc được từ máy chủ.
  const thanGui = {
    code: than.code,
    label: than.label,
    order: than.order,
    is_default: than.is_default,
  };

  const kq = await goiGhi(mo.gocThem, "POST", thanGui, 201, {
    "Idempotency-Key": khoaChongTrung,
  });
  return kq.ok ? docThanRa(kq.duLieu) : kq;
}

/**
 * PATCH — sửa nhãn, thứ tự, trạng thái dùng, hoặc đặt mặc định.
 *
 * Trường nào `undefined` thì `JSON.stringify` bỏ hẳn khỏi thân, và máy chủ đọc "không nhắc tới"
 * là "không đổi" (`suaLoaiVanBanVao` toàn con trỏ). Đó là lý do biểu mẫu sửa chỉ gửi những
 * trường nó thật sự đổi, và là lý do nút `Tắt` gửi đúng một trường `active`.
 */
export async function suaMuc(
  mo: MoTaDanhMucGhi,
  id: string,
  than: SuaMucVao,
): Promise<KetQua<MucDanhMucGhi>> {
  const thanGui = {
    label: than.label,
    order: than.order,
    active: than.active,
    is_default: than.is_default,
  };

  const kq = await goiGhi(duongDanMuc(mo.mauMuc, id), "PATCH", thanGui, 200);
  return kq.ok ? docThanRa(kq.duLieu) : kq;
}

/**
 * DELETE — xoá **mềm**, kèm lý do bắt buộc. 204, không thân.
 *
 * XOÁ Ở ĐÂY KHÔNG PHẢI XOÁ. Dòng vẫn nằm trong CSDL mang `deleted_at`, `deleted_by`,
 * `delete_reason`, và MÃ CỦA NÓ VẪN BỊ CHIẾM vĩnh viễn — thêm lại đúng mã ấy trả 409 kèm câu
 * giải thích của máy chủ (luật 7, bất biến 3). Không có `Idempotency-Key`: hợp đồng không đòi,
 * và xoá mềm hai lần cùng một dòng không sinh thêm dòng nào.
 */
export async function xoaMuc(mo: MoTaDanhMucGhi, id: string, lyDo: string): Promise<KetQua<null>> {
  const thanGui: XoaMucVao = { reason: lyDo };

  const kq = await goiGhi(duongDanMuc(mo.mauMuc, id), "DELETE", thanGui, 204);
  return kq.ok ? { ok: true, duLieu: null } : kq;
}

/** Một nhóm đã đọc xong: mô tả của nó, và kết quả đọc. */
export type NhomDaDoc = {
  readonly mo: MoTaDanhMucGhi;
  readonly kq: KetQua<{ items: readonly MucDanhMucGhi[] }>;
};

/**
 * Đọc cả năm danh mục — ĐÚNG NĂM lời gọi cho cả màn hình, dù mỗi danh mục có bao nhiêu mục.
 *
 * NĂM KẾT QUẢ RỜI NHAU, KHÔNG GỘP: bốn dịch vụ đứng sau năm tuyến này, nên một dịch vụ đang khởi
 * động lại là chuyện có thật và chỉ nên làm im lặng ĐÚNG khối của nó.
 */
export function docNamDanhMucGhi(): Promise<readonly NhomDaDoc[]> {
  return Promise.all(NAM_DANH_MUC_GHI.map((mo) => mo.doc().then((kq) => ({ mo, kq }))));
}
