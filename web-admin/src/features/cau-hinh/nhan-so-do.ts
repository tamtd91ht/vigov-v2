/**
 * Câu chữ của tab "Sơ đồ tổ chức" — `docs/ui-ux/14-cau-hinh.md §1`. Hàm thuần, không gọi mạng,
 * không dựng DOM.
 *
 * Cùng khuôn `nhan-danh-muc.ts`: chữ trên màn hình của một cơ quan nhà nước là thứ có người phải
 * trả lời. Nhãn nút và nhãn ô lấy theo đặc tả §1, không tự đặt lại.
 */

export const TIEU_DE_SO_DO = "Sơ đồ tổ chức";

/**
 * Nhãn các nút. CÓ CHỮ BÊN CẠNH BIỂU TƯỢNG, dù đặc tả §1 chỉ vẽ `＋` và `✎`: một nút chỉ có biểu
 * tượng là một nút phải đoán, và trình đọc màn hình đọc `✎` thành tên một ký tự chứ không thành
 * "sửa" (cùng lý lẽ `GHI_CHU_NUT_CO_CHU` ở `nhan-danh-muc.ts`).
 */
export const NUT_THEM_BO_PHAN = "+ Thêm bộ phận";
export const NUT_THEM_CON = "＋ Thêm bộ phận con";
export const NUT_SUA_BO_PHAN = "✎ Sửa";
export const NUT_LUU = "Lưu";
export const NUT_HUY = "Huỷ";

/**
 * Nhãn đọc-được-một-mình của hai nút trên một thẻ — aria-label của đặc tả §1 KÈM TÊN bộ phận.
 * Trình đọc màn hình đọc nút tách khỏi thẻ, và mười nút cùng đọc "Sửa bộ phận" thì không nút nào
 * nói được nó sửa bộ phận nào.
 */
export function nhanNutThemCon(tenBoPhan: string): string {
  return `Thêm bộ phận con của ${tenBoPhan}`;
}

export function nhanNutSua(tenBoPhan: string): string {
  return `Sửa bộ phận ${tenBoPhan}`;
}

/**
 * Số cán bộ của một bộ phận — `staff_count`. Biểu tượng `👥` do thẻ vẽ riêng và ẩn khỏi trình đọc
 * màn hình; câu này là phần mang nghĩa. `0 cán bộ` được in ra, không bỏ trống: một bộ phận chưa có
 * ai là một sự thật cán bộ cần thấy (`bo_phan.go`, `StaffCount`).
 */
export function nhanSoCanBo(soCanBo: number): string {
  return `${soCanBo} cán bộ`;
}

export function tieuDeThemGoc(): string {
  return "Thêm bộ phận";
}

export function tieuDeThemCon(tenCha: string): string {
  return `Thêm bộ phận con của ${tenCha}`;
}

export function tieuDeSua(tenBoPhan: string): string {
  return `Sửa bộ phận ${tenBoPhan}`;
}

export const O_TEN = "Tên bộ phận";
export const O_MA = "Mã";
export const O_CHA = "Bộ phận cha";
export const O_THU_TU = "Thứ tự";

/** Lựa chọn "không có cha" trong ô `Bộ phận cha` — bộ phận đứng ở cấp cao nhất. */
export const CHON_KHONG_CO_CHA = "Không có — bộ phận cấp cao nhất";

/**
 * Chú thích dưới ô `Tên`. NHẮC QUY ƯỚC, KHÔNG ÉP: đặc tả §1 ghi tên "viết HOA", nhưng máy chủ giữ
 * nguyên chữ đã gõ (`bo_phan.go:135`) vì tên đi vào hồ sơ lưu trữ, và một ô tự đổi chữ của người
 * gõ là một ô ghi vào hồ sơ thứ người ấy không gõ.
 */
export const GIAI_THICH_O_TEN =
  "Theo quy ước, tên bộ phận viết hoa toàn bộ — ví dụ: VĂN PHÒNG ĐẢNG ỦY. Hệ thống giữ nguyên " +
  "chữ bạn gõ.";

/** Chú thích dưới ô `Mã` của biểu mẫu thêm. Nói TRƯỚC khi lưu rằng mã không sửa lại được. */
export const GIAI_THICH_O_MA_THEM =
  "Có thể để trống: hệ thống tự sinh mã từ tên, ví dụ: van-phong-dang-uy. Mã đã lưu thì không " +
  "sửa được nữa.";

/** Dòng thay cho ô `Mã` ở biểu mẫu sửa — mã hiện ra, nhưng không có ô nhập nào để hứa điều sai. */
export function giaiThichMaKhongSua(ma: string): string {
  return `Mã ${ma}. Mã đã cấp thì không sửa được.`;
}

export const GIAI_THICH_O_THU_TU =
  "Số nguyên. Trong cùng một cấp, bộ phận có số nhỏ hơn đứng trước.";

/**
 * Chú thích dưới ô `Bộ phận cha` của biểu mẫu sửa — vì sao danh sách thiếu vài bộ phận.
 *
 * Danh sách cố ý bỏ chính bộ phận đang sửa và mọi bộ phận con cháu của nó. Không nói ra thì cán bộ
 * đi tìm một bộ phận "bị mất" trong ô chọn.
 */
export const GIAI_THICH_O_CHA_SUA =
  "Danh sách không có chính bộ phận này và các bộ phận trực thuộc nó: không dời một bộ phận vào " +
  "dưới chính nó được.";

/** Lỗi tại chỗ khi ô `Thứ tự` không phải một số nguyên. Máy chủ không bao giờ thấy thân ấy. */
export const LOI_THU_TU = "Thứ tự phải là một số nguyên, ví dụ: 3.";

/** Biểu mẫu sửa được gửi mà không đổi gì — không gửi yêu cầu nào. */
export const CHUA_CO_THAY_DOI = "Chưa có thay đổi nào để lưu.";

/** Câu báo sau khi ghi xong. Nói rõ ĐÃ LÀM GÌ, với bộ phận nào. */
export function daThem(ten: string): string {
  return `Đã thêm bộ phận ${ten}.`;
}

export function daLuu(ten: string): string {
  return `Đã lưu thay đổi của bộ phận ${ten}.`;
}

export const DANG_TAI = "Đang tải sơ đồ tổ chức của đơn vị…";

/**
 * Trạng thái rỗng — một đơn vị mới nhận hệ thống chưa có bộ phận nào. Hai câu theo hai ca, vì
 * người thiếu quyền mà được bảo "bấm Thêm bộ phận" sẽ đi tìm một nút không có.
 */
export function nhanCayRong(themDuoc: boolean): string {
  const dau = "Đơn vị chưa có bộ phận nào trong sơ đồ tổ chức.";
  return themDuoc
    ? `${dau} Bấm ${NUT_THEM_BO_PHAN} ở trên để lập bộ phận đầu tiên.`
    : `${dau} Tài khoản của bạn không có quyền thêm bộ phận.`;
}

/** Câu hiện thay cho các nút ghi khi tài khoản không có `admin.org`. */
export const CAU_THIEU_QUYEN_GHI =
  "Tài khoản của bạn chỉ xem được sơ đồ tổ chức. Việc thêm và sửa bộ phận cần quyền Quản lý sơ " +
  "đồ tổ chức.";

/**
 * Vì sao thẻ không có nút `🗑 Xoá bộ phận` của đặc tả — nói ngay trong tab, một lần. Lý do đầy đủ
 * nằm ở mục "xoá bộ phận" của `PHAN_CHUA_DUNG` (`nhan-cau-hinh.ts`); câu này không chép lại lý do.
 */
export const GHI_CHU_CHUA_XOA =
  "Chưa xoá được bộ phận từ màn hình này. Lý do ghi ở phần chưa dựng cuối trang.";
