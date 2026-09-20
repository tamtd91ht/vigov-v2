/**
 * Câu chữ của tab "Thôn / Tổ dân phố" (`docs/ui-ux/14-cau-hinh.md §2`). Hàm thuần, không gọi
 * mạng, không dựng DOM.
 *
 * TỆP RIÊNG, KHÔNG NHẬP CHUNG VỚI `nhan-danh-muc.ts`: một thôn KHÔNG phải một mục danh mục. Hợp
 * đồng nói ra sự khác nhau ấy bằng chính tên trường — mục danh mục có `label` (một nhãn đặt lên
 * một mã), thôn có `name` (một địa bàn CÓ THẬT, có số hộ và nhân khẩu). Đặc tả cũng tách đôi:
 * thôn/tổ dân phố là tab §2, danh mục là tab §5.
 */

// TRẠNG THÁI DÙNG CHUNG MỘT HÀM VỚI TAB DANH MỤC, CÓ CHỦ Ý — và đây là chỗ duy nhất hai tab chạm
// nhau. `active` trên một thôn mang đúng nghĩa `active` trên một mục danh mục, và máy chủ nói ra
// điều đó bằng chính lời của nó (`service-identity/internal/http/thon_to_dan_pho.go`, trên trường
// `Active`: "same reading as the catalogues"). Hai hàm cùng trả về hai chữ ấy là hai bản sao, và
// ngày một bên đổi thành "Ngừng dùng" thì hai bảng cạnh nhau nói hai từ về cùng một sự thật.
export {
  lopTrangThaiMuc as lopTrangThaiDiaBan,
  nhanTrangThaiMuc as nhanTrangThaiDiaBan,
} from "./nhan-danh-muc";

/**
 * Cột "Loại" của một địa bàn — BA CA, và không ca nào là một ô trống.
 *
 * Hợp đồng trả loại về sẵn thành hai trường phẳng (`type_code` + `type_label`), đọc trong cùng
 * một câu truy vấn, nên màn hình này KHÔNG ghép tay với `GET /api/v1/residential-unit-types`.
 * Nhưng hai trường ấy có ba tổ hợp có thật, và máy chủ ghi rõ từng cái:
 *
 *   `chuaPhanLoai`   `type_code` rỗng — địa bàn chưa được phân loại. Máy chủ gọi đây là "a
 *                    legitimate state", không phải dữ liệu hỏng.
 *   `coNhan`         tra được nhãn.
 *   `chiCoMa`        có mã nhưng nhãn rỗng — mục loại đã bị xoá mềm trong khi các thôn vẫn mang
 *                    mã của nó. Máy chủ dặn thẳng: "Fall back to showing the code", và cố ý GIỮ
 *                    dòng thôn lại, vì bỏ nó đi là biến một danh mục lộn xộn thành một thôn biến
 *                    mất khỏi danh sách của chính đơn vị.
 *
 * GỘP CA 1 VÀ CA 3 LÀM MỘT LÀ BÁO SAI: "chưa phân loại" là việc của cán bộ nhập liệu, còn "nhãn
 * không còn trong danh mục" là dữ liệu lệch mà chỉ người quản trị sửa được — hai việc khác nhau
 * thì không được nhìn giống nhau (cùng lý lẽ với `tra-danh-muc.ts`).
 */
export type LoaiDonVi =
  | { loai: "chuaPhanLoai" }
  | { loai: "coNhan"; nhan: string }
  | { loai: "chiCoMa"; ma: string };

export function loaiDonVi(typeCode: string, typeLabel: string): LoaiDonVi {
  if (typeCode === "") return { loai: "chuaPhanLoai" };
  if (typeLabel === "") return { loai: "chiCoMa", ma: typeCode };
  return { loai: "coNhan", nhan: typeLabel };
}

export function nhanLoaiDonVi(l: LoaiDonVi): string {
  switch (l.loai) {
    case "chuaPhanLoai":
      return "Chưa phân loại";
    case "coNhan":
      return l.nhan;
    case "chiCoMa":
      // Hiện MÃ, và nói rõ vì sao chỉ có mã — không để người đọc tưởng "thon" là tên loại.
      return `${l.ma} (nhãn không còn trong danh mục)`;
  }
}

/** Lớp CSS đi theo LOẠI kết quả, không theo câu chữ — xem `lopNhanDanhMuc` ở tab Người dùng. */
export function lopLoaiDonVi(l: LoaiDonVi): string | undefined {
  switch (l.loai) {
    case "coNhan":
      return undefined;
    case "chiCoMa":
      return "nhan-lech";
    case "chuaPhanLoai":
      return "nhan-trong";
  }
}

const DINH_DANG_SO = new Intl.NumberFormat("vi-VN");

/**
 * Cột "Số hộ" và "Nhân khẩu" — `household_count` · `population_count`, cả hai là `number | null`.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * `null` KHÔNG PHẢI `0`, VÀ KHÔNG ĐƯỢC HIỆN THÀNH `0`. Máy chủ giữ hai giá trị ấy tách nhau qua
 * cả ba tầng và nói vì sao (`service-identity/internal/http/thon_to_dan_pho.go`): `0` là một
 * KHẲNG ĐỊNH về địa bàn ("không có hộ nào"), `null` là sự VẮNG MẶT của một khẳng định ("chưa
 * nhập"). Một đơn vị nhập bảng tính thiếu cột sẽ công bố những số không nó chưa bao giờ khẳng
 * định — và một số không đi tiếp vào báo cáo gửi lên trên như một con số thật.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 *
 * Định dạng `1.132` theo `vi-VN`, đúng ví dụ của đặc tả §2 — và ghim `vi-VN` chứ không theo cài
 * đặt của máy, vì dấu phân cách hàng nghìn khác nhau giữa các miền địa phương làm `1.132` đọc
 * thành một phẩy một ba hai.
 */
export function nhanSoDem(so: number | null): string {
  if (so === null) return "Chưa nhập";
  // Ca thứ ba — có giá trị nhưng không phải một con số hữu hạn — là hỏng hợp đồng, không phải một
  // trạng thái nghiệp vụ, nên nó nói ra bằng một câu KHÁC thay vì hiện chữ "NaN".
  if (!Number.isFinite(so)) return "Không đọc được";
  return DINH_DANG_SO.format(so);
}

/**
 * Trạng thái rỗng của tab §2 — cùng hai điều phải nói như `nhanNhomRong` của tab Danh mục, và
 * cùng một lý do: hôm nay bảng này rỗng ở mọi đơn vị. Migration 0005 tạo bảng và CỐ Ý không gieo
 * bản ghi nào (`service-identity/internal/http/thon_to_dan_pho.go`: "That is every commune today").
 *
 * "Địa bàn" chứ không phải "bản ghi": người đọc màn hình này là cán bộ, không phải người lập trình.
 */
export const THON_RONG =
  "Đơn vị chưa có thôn hoặc tổ dân phố nào. Màn hình này chỉ xem, không thêm được địa bàn mới.";

/**
 * Ghi chú đầu tab §2 — vì sao không có nút thêm, sửa, xoá hay nhập Excel nào.
 *
 * LÝ DO KHÁC HẲN TAB DANH MỤC, nên câu chữ cũng khác: ở đây không phải câu hỏi mở #21 mà là câu
 * máy chủ ghi ngay trên tuyến — lập một thôn, nhập hai thôn làm một, hay cho một thôn ngừng hoạt
 * động là những hành vi hành chính tác động lên một bản ghi mà phản ánh và hồ sơ hộ đang trỏ tới;
 * ai được làm, và hồ sơ đang trỏ vào một địa bàn biến mất thì ra sao, chưa ai trả lời.
 */
export const GHI_CHU_CHI_XEM_THON =
  "Màn hình hiện chỉ xem. Thêm, sửa, xoá địa bàn và nhập từ tệp Excel chưa mở vì chưa có quy " +
  "định cho việc nhập, tách hay ngừng dùng một địa bàn mà hồ sơ của đơn vị đang trỏ tới.";
