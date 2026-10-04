import type { PendingFeatureInfo } from "@/components/ui/pending-feature";

/**
 * Wording and fixed lists of `/ban-do` (`docs/ui-ux/10-ban-do-kinh-te-so.md`). One copy of each
 * sentence: the screen and its tests read them from here (rule 9, forbidden #2).
 */

/** `asset.update` — "Cập nhật bản đồ tài nguyên" (`service-identity/migrations/0001_init.sql:288`). Write key of every asset route. */
export const ASSET_UPDATE_PERMISSION = "asset.update";

export const PAGE_TITLE = "Bản đồ phát triển kinh tế số";
export const PAGE_SUBTITLE = "Vị trí doanh nghiệp, hộ kinh doanh, hợp tác xã, trường học, cơ sở y tế và công trình trên địa bàn.";
export const NO_ASSET_READ =
  "Tài khoản của bạn không có quyền xem bản đồ tài nguyên (asset.read), nên phần này không hiển thị. Liên hệ quản trị viên của đơn vị nếu bạn cần quyền này.";

/* ---- frame + basemap -------------------------------------------------------------------------- */

export const NO_BASEMAP = "Chưa cấu hình bản đồ nền.";
export const NO_BASEMAP_DETAIL = "Sổ địa điểm vẫn dùng được. Người vận hành hệ thống cần đặt địa chỉ bản đồ nền.";
export const FRAME_NOT_SET = "Xã chưa đặt khung bản đồ.";
export const FRAME_ASK_ADMIN = "Liên hệ quản trị viên của xã để đặt khung.";
export const FRAME_LOAD_FAILED = "Chưa tải được khung bản đồ của xã";
export const FRAME_FORM_TITLE = "Đặt khung bản đồ của xã";
export const FRAME_CHANGE_BUTTON = "Đổi khung bản đồ";
export const FRAME_FROM_POINTS_BUTTON = "Lấy tâm từ các đối tượng đã có";
export const FRAME_FORM_NOTE =
  "Bản đồ chỉ hiện trong khung này: không kéo hay thu nhỏ ra ngoài được. Tâm phải nằm trên đất liền Việt Nam.";
export const OUTSIDE_FRAME = "Đối tượng nằm ngoài khung bản đồ của xã";
export const PIN_OUTSIDE_FRAME = "Vị trí nằm ngoài khung bản đồ của xã. Hãy chọn một điểm trong khung.";
export const MY_LOCATION_OUTSIDE = "Vị trí của bạn nằm ngoài khung bản đồ của xã, nên không đặt ghim theo vị trí này.";
export const MY_LOCATION_FAILED = "Không lấy được vị trí của bạn. Hãy cho phép trình duyệt dùng vị trí, hoặc bấm vào bản đồ.";
export const MAP_LOAD_FAILED = "Không thể tải bản đồ. Vui lòng thử lại.";

/**
 * Required attribution (ADR 0072 H1). EXACTLY the string the OpenFreeMap TileJSON
 * (`https://tiles.openfreemap.org/planet`, read 04/10/2026) returns, so MapLibre's attribution control
 * treats the source's entry and this one as the same and draws it once — and still draws it if a
 * changed style stops carrying it. Rendered by MapLibre's own sanitiser; no user data in it.
 */
export const OPENFREEMAP_ATTRIBUTION =
  '<a href="https://openfreemap.org" target="_blank">OpenFreeMap</a> <a href="https://www.openmaptiles.org/" target="_blank">&copy; OpenMapTiles</a> Data from <a href="https://www.openstreetmap.org/copyright" target="_blank">OpenStreetMap</a>';

/* ---- catalogue -------------------------------------------------------------------------------- */

export const NO_GROUPS = "Xã chưa có nhóm tài nguyên bản đồ.";
export const SEED_DEFAULTS_BUTTON = "Nạp 11 nhóm mặc định";
export const NO_GROUPS_ASK_ADMIN = "Quản trị viên của xã nạp được 11 nhóm mặc định từ màn này.";

/* ---- statuses (ADR 0072 §5, spec §10) --------------------------------------------------------- */

export const STATUS_OPTIONS: readonly { readonly value: string; readonly label: string }[] = [
  { value: "dang-hoat-dong", label: "Đang hoạt động" },
  { value: "tam-ngung", label: "Tạm ngừng" },
  { value: "da-giai-the", label: "Đã giải thể" },
];
export const DEFAULT_STATUS = "dang-hoat-dong";

/** A status the server sent that is not one of the three is shown AS SENT, never relabelled. */
export function statusLabel(value: string): string {
  return STATUS_OPTIONS.find((s) => s.value === value)?.label ?? value;
}

/* ---- industries — VSIC level 2, the 20 codes of spec §5 --------------------------------------- */

export const INDUSTRIES: readonly { readonly code: string; readonly label: string }[] = [
  { code: "01", label: "Nông nghiệp, trồng trọt, chăn nuôi" },
  { code: "03", label: "Khai thác, nuôi trồng thuỷ sản" },
  { code: "10", label: "Chế biến thực phẩm" },
  { code: "13", label: "Dệt" },
  { code: "14", label: "May mặc" },
  { code: "16", label: "Chế biến gỗ" },
  { code: "22", label: "Sản phẩm cao su, nhựa" },
  { code: "23", label: "Vật liệu xây dựng" },
  { code: "25", label: "Sản phẩm kim loại, cơ khí" },
  { code: "41", label: "Xây dựng nhà" },
  { code: "42", label: "Xây dựng công trình" },
  { code: "45", label: "Bán, sửa chữa ô tô, xe máy" },
  { code: "46", label: "Bán buôn" },
  { code: "47", label: "Bán lẻ" },
  { code: "49", label: "Vận tải đường bộ" },
  { code: "55", label: "Lưu trú" },
  { code: "56", label: "Ăn uống" },
  { code: "85", label: "Giáo dục và đào tạo" },
  { code: "86", label: "Y tế" },
  { code: "96", label: "Dịch vụ khác" },
];

export function industryLabel(code: string): string {
  return INDUSTRIES.find((i) => i.code === code)?.label ?? code;
}

/* ---- the add / edit form (spec §8) ------------------------------------------------------------ */

export const ADD_BUTTON = "Thêm đối tượng";
export const ADD_TITLE = "Thêm đối tượng lên bản đồ";
export const ADD_DESCRIPTION = "Bắt buộc: nhóm, tên và vị trí. Phần còn lại điền được lúc nào cũng được.";
export const ADD_SUBMIT = "Thêm vào bản đồ";
export const EDIT_SUBMIT = "Lưu thay đổi";
export const PIN_HINT = "Bấm vào bản đồ để đặt ghim, rồi kéo ghim để chỉnh cho đúng.";
export const MY_LOCATION_BUTTON = "Vị trí của tôi";
export const REQUIRED_GROUP = "Hãy chọn nhóm.";
export const REQUIRED_NAME = "Hãy nhập tên.";
export const REQUIRED_POSITION = "Hãy đặt vị trí trên bản đồ hoặc nhập vĩ độ và kinh độ.";
export const BAD_COORDINATE = "Vĩ độ phải từ -90 đến 90 và kinh độ từ -180 đến 180, tối đa 6 chữ số thập phân.";
export const MASKED_NOTE =
  "Người đại diện, điện thoại và mã số thuế đang được che, nên không sửa được ở đây.";
export const NO_CHANGE = "Chưa có thay đổi nào để lưu.";

/* ---- detail + delete -------------------------------------------------------------------------- */

export const VERIFY_BUTTON = "Xác minh";
export const UNVERIFY_BUTTON = "Bỏ xác minh";
export const EDIT_BUTTON = "Sửa";
export const DELETE_BUTTON = "Xoá";
export const DELETE_EXPLANATION =
  "Đối tượng sẽ rời khỏi bản đồ, sổ địa điểm và số liệu. Hồ sơ vẫn được lưu giữ kèm lý do xoá.";
export const DELETE_REASON_REQUIRED = "Vui lòng nhập lý do xoá. Lý do được lưu cùng hồ sơ đã xoá.";
export const VERIFIED_CHIP = "Đã xác minh";
export const UNVERIFIED_CHIP = "Chưa xác minh";

export function deleteTitle(name: string): string {
  return `Xoá ${name} khỏi bản đồ`;
}

/* ---- views ------------------------------------------------------------------------------------ */

export const VIEW_MAP = "Bản đồ";
export const VIEW_REGISTER = "Sổ địa điểm";
export const SEARCH_PLACEHOLDER = "Tên, địa chỉ, mã số thuế…";
export const LAYERS_TITLE = "LỚP BẢN ĐỒ";
export const FILTERS_TITLE = "BỘ LỌC";
export const HIDE_ALL = "Ẩn hết";
export const SHOW_ALL = "Hiện hết";
export const UNVERIFIED_LEGEND = "Chấm nhạt: chưa xác minh.";

/* ---- parts of the spec that are not built (ADR 0068 §14) -------------------------------------- */

/**
 * Each reason is ADR 0072's (§4 scope, H2). Shown behind the "?" of a disabled control at the spec's
 * position; read by `tools/tien_do_san_pham.py` to count what this screen still lacks.
 */
// vi-name-ok: the product-progress tool (`tools/tien_do_san_pham.py`) counts every `PHAN_CHUA_DUNG*` block by this exact name
export const PHAN_CHUA_DUNG: readonly PendingFeatureInfo[] = [
  {
    ten: "Mẫu Excel",
    viSao:
      "Tải tệp Excel mẫu theo nhóm đang chọn, cột gồm trường chung và trường tuỳ biến của nhóm. Hoãn theo phạm vi đã chốt (ADR 0072 §4): nhập và mẫu Excel làm sau bản đồ.",
  },
  {
    ten: "Nhập Excel",
    viSao:
      "Nạp hàng loạt đối tượng từ tệp Excel, trùng mã số thuế thì cập nhật. Hoãn theo phạm vi đã chốt (ADR 0072 §4).",
  },
  {
    ten: "Trình chiếu",
    viSao: "Chế độ phòng họp: bản đồ toàn màn hình, chữ lớn. Chưa dựng.",
  },
  {
    ten: "Quy mô lao động",
    viSao:
      "Lọc theo số lao động của cơ sở. Máy chủ chưa có bộ lọc này, nên ô lọc chưa dùng được.",
  },
  {
    ten: "Bản đồ nhiệt phản ánh",
    viSao:
      "Phủ mật độ phiếu phản ánh của người dân lên bản đồ. Chưa làm: cần hợp đồng đọc từ phân hệ phản ánh, và việc vẽ toạ độ phản ánh của công dân lên nền bản đồ bên ngoài chưa được quyết (ADR 0072 §4, H2).",
  },
  {
    ten: "Mật độ theo thôn",
    viSao:
      "Bảng số cơ sở và số phản ánh theo từng thôn. Chưa làm: cần đọc số phản ánh từ phân hệ phản ánh, hợp đồng hiện tại chưa có (ADR 0072 §4).",
  },
];

export function pendingPart(name: string): PendingFeatureInfo {
  const p = PHAN_CHUA_DUNG.find((x) => x.ten === name);
  if (p === undefined) throw new Error(`pendingPart: không có mục "${name}"`);
  return p;
}
