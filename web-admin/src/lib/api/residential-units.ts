/**
 * Danh sách thôn / tổ dân phố của xã — `GET /api/v1/residential-units` — và hai tuyến GHI của nó.
 *
 * TỆP RIÊNG, KHÔNG NẰM CHUNG VỚI BẢY DANH MỤC Ở `danh-muc-nghiep-vu.ts`, vì đây KHÔNG phải một
 * danh mục. Hợp đồng nói ra sự khác nhau ấy bằng chính tên trường: một mục danh mục có `label`
 * — một nhãn đặt lên một mã — còn một thôn có `name`, vì nó là một thứ CÓ TÊN, một địa bàn có
 * thật với số hộ và nhân khẩu (`service-identity/internal/http/thon_to_dan_pho.go`). Đặc tả
 * cũng tách đôi: thôn/tổ dân phố là tab §2, danh mục là tab §5.
 *
 * KIỂU LẤY TỪ HỢP ĐỒNG, KHÔNG GÕ TAY. Quy tắc chung cho mọi lời gọi nằm ở `goi.ts`; quy tắc
 * "không đệm ở mức module" và lý do của nó nằm ở `danh-muc-nghiep-vu.ts`.
 *
 * GHI (người dùng chốt 29/09/2026, ADR 0059 §2): thêm (`POST`) và sửa (`PATCH`), cả hai `admin.org`.
 * KHÔNG CÓ XOÁ, và đó là quyết định chứ không phải chỗ trống: "Ngừng dùng" là `active: false` trên
 * PATCH — thôn vẫn ở trong danh sách, ô chọn lọc nó ra, và mọi hồ sơ đang trỏ vào nó vẫn in được
 * tên (`service-identity/internal/http/residential_unit_write.go:20-24`). Không có nhập, không có tách
 * (luật 1, điều kiện dừng #3). Nhập từ Excel ở `residential-unit-import.ts`.
 */

import { docJSON, docThanLoiGoi, goiGhi, type KetQua } from "./request";
import type {
  identity_createResidentialUnitIn,
  identity_danhSachThonToDanPhoRa,
  identity_get_residential_units,
  identity_patch_residential_units_by_id,
  identity_post_residential_units,
  identity_thonToDanPhoRa,
  identity_updateResidentialUnitIn,
} from "./schema.gen";

/**
 * GET /api/v1/residential-units — nguyên danh sách địa bàn của xã, kèm nhãn loại đơn vị.
 *
 * KHÔNG PHÂN TRANG: tuyến trả cả danh sách hoặc hỏng. Khi danh sách vượt trần, máy chủ TỪ CHỐI
 * thay vì cắt bớt — một danh sách ngắn đi lặng lẽ sẽ khiến phản ánh bị lập cho sai địa bàn
 * trong khi màn hình trông hoàn toàn bình thường.
 *
 * NHÃN LOẠI VỀ SẴN TRONG CÙNG PHẢN HỒI (`type_code` + `type_label`), nên màn hình này KHÔNG
 * ghép tay với `GET /api/v1/residential-unit-types`: máy chủ đã nối trong một câu truy vấn. Ghép
 * lại ở đây là dựng câu trả lời thứ hai cho cùng một câu hỏi, và hai câu ấy sẽ lệch nhau.
 *
 * TRẢ CẢ ĐỊA BÀN ĐÃ NGỪNG DÙNG (`active: false`). Ô chọn nào cần lọc thì lọc bằng
 * `pickableResidentialUnits` (`features/cau-hinh/nhan-thon.ts`); bảng và bộ lọc giữ nguyên.
 */
export function layDanhSachThonToDanPho(): Promise<KetQua<identity_danhSachThonToDanPhoRa>> {
  const duongDan: identity_get_residential_units["duongDan"] = "/api/v1/residential-units";
  return docJSON<identity_danhSachThonToDanPhoRa>(duongDan);
}

/**
 * POST body — the contract's type WITHOUT `active`: the server refuses `active` on create with 400
 * `active_not_settable` (a new unit is in use; retiring it is a PATCH with its own trail).
 */
export type CreateResidentialUnitBody = Omit<identity_createResidentialUnitIn, "active">;

/**
 * PATCH body — the contract's type WITHOUT `code` (400 `code_not_editable`: an issued code never
 * changes, rule 7 invariant 3), and with the two counts retyped.
 *
 * WHY THE COUNTS ARE RETYPED: the generator renders the server's `optionalCountIn` as a REQUIRED
 * `Record<string, never>` — it cannot see the hand-written `UnmarshalJSON`
 * (`residential_unit_write.go:71-95`). The wire meaning has three states, and all three are needed:
 * absent = leave alone, `null` = CLEAR ("chưa nhập", never 0), a number = set it. Same gap, same
 * treatment as `identity_optionalHoursIn` in `thoi-han-xu-ly.ts`.
 */
export type UpdateResidentialUnitBody = Omit<
  identity_updateResidentialUnitIn,
  "code" | "household_count" | "population_count"
> & {
  household_count?: identity_thonToDanPhoRa["household_count"];
  population_count?: identity_thonToDanPhoRa["population_count"];
};

const CREATE_PATH = "/api/v1/residential-units" satisfies identity_post_residential_units["duongDan"];
const UPDATE_PATH = "/api/v1/residential-units/{id}" satisfies identity_patch_residential_units_by_id["duongDan"];

/**
 * POST /api/v1/residential-units — add one unit. 201 with the unit.
 *
 * `idempotencyKey` IS A PARAMETER: minted when the form OPENS and kept across retries of that one
 * attempt — a key minted here would turn a retry after a network error into a second unit with the
 * same name (the first send may have reached the server). Same rule as `themBoPhan`.
 *
 * BUILT FIELD BY FIELD, so nothing the caller did not mean to send goes up (an `active` spread in
 * from a row read back would be a 400).
 */
export function createResidentialUnit(
  body: CreateResidentialUnitBody,
  idempotencyKey: string,
): Promise<KetQua<identity_thonToDanPhoRa>> {
  const sent: CreateResidentialUnitBody = {
    name: body.name,
    code: body.code,
    type_code: body.type_code,
    head_staff_code: body.head_staff_code,
    household_count: body.household_count,
    population_count: body.population_count,
    order: body.order,
  };
  return docThanLoiGoi<identity_thonToDanPhoRa>(
    goiGhi(CREATE_PATH, "POST", sent, 201, { "Idempotency-Key": idempotencyKey }),
  );
}

/**
 * PATCH /api/v1/residential-units/{id} — partial edit, including `active` (false = Ngừng dùng,
 * true = Dùng lại). No `Idempotency-Key`: the contract declares none, and resending one edit lands
 * on the same state.
 *
 * FIELD BY FIELD, and `undefined` stays ABSENT (`JSON.stringify` drops it) while `null` goes up —
 * that difference IS "leave alone" vs "clear" for the two counts. `code` cannot be sent: the type
 * forbids it at compile time, this copy at run time.
 */
export function updateResidentialUnit(
  id: string,
  body: UpdateResidentialUnitBody,
): Promise<KetQua<identity_thonToDanPhoRa>> {
  const sent: UpdateResidentialUnitBody = {
    name: body.name,
    type_code: body.type_code,
    head_staff_code: body.head_staff_code,
    household_count: body.household_count,
    population_count: body.population_count,
    order: body.order,
    active: body.active,
  };
  return docThanLoiGoi<identity_thonToDanPhoRa>(
    goiGhi(UPDATE_PATH.replace("{id}", encodeURIComponent(id)), "PATCH", sent, 200),
  );
}
