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
 * Trạng thái rỗng của tab §2 — nói đúng một điều: đơn vị chưa có địa bàn nào. Không phải lỗi. Việc
 * thêm được hay không là câu của NÚT (có `admin.org` thì có nút, không thì câu `NO_WRITE_PERMISSION`
 * ở `residential-unit-form.ts`), nên câu này không nói thay.
 *
 * "Địa bàn" chứ không phải "bản ghi": người đọc màn hình này là cán bộ, không phải người lập trình.
 */
export const THON_RONG = "Đơn vị chưa có thôn hoặc tổ dân phố nào.";

/**
 * Ghi chú đầu tab §2 (người dùng chốt 29/09/2026, ADR 0059 §2). Nói điều cán bộ cần biết TRƯỚC khi
 * bấm: không có xoá, và vì sao. Một địa bàn là thứ phản ánh và hồ sơ hộ đang trỏ tới; xoá nó là làm
 * những hồ sơ ấy mất tên địa bàn — nên chỉ có "Ngừng dùng", và địa bàn ngừng dùng vẫn ở lại.
 */
// The name predates the write routes; the TEXT changed, the name did not (rule 12 inv 3).
export const GHI_CHU_CHI_XEM_THON = // vi-name-ok: existing export restored, rule 12 inv 3 forbids renaming it
  "Danh sách thôn, tổ dân phố của đơn vị. Không có thao tác xoá: địa bàn không còn dùng thì bấm " +
  "Ngừng dùng. Địa bàn ngừng dùng vẫn nằm trong danh sách và vẫn hiện tên trên mọi hồ sơ, phản ánh " +
  "đã lập; chỉ không còn trong ô chọn địa bàn khi lập hồ sơ mới.";

/**
 * Ô chọn địa bàn cho một hồ sơ MỚI: chỉ địa bàn đang dùng (`active: true`). Máy chủ nói thẳng
 * "pickers filter it out" (`residential_unit_write.go:22`).
 *
 * CHỈ DÀNH CHO Ô CHỌN GHI. Bảng, chỗ hiện tên và BỘ LỌC danh sách giữ nguyên địa bàn đã ngừng dùng —
 * lọc bỏ nó ở bộ lọc là làm phản ánh đã lập ở đó không tìm ra được (xem `residentialUnitFilterLabel`).
 */
export function pickableResidentialUnits<T extends { readonly active: boolean }>(
  units: readonly T[],
): readonly T[] {
  return units.filter((u) => u.active);
}

/**
 * Nhãn của một địa bàn trong BỘ LỌC danh sách: địa bàn đã ngừng dùng vẫn có (hồ sơ cũ vẫn trỏ vào
 * nó) nhưng được nói rõ, để cán bộ không tưởng đó là địa bàn còn nhận hồ sơ.
 */
export function residentialUnitFilterLabel(u: { readonly name: string; readonly active: boolean }): string {
  return u.active ? u.name : `${u.name} (ngừng dùng)`;
}
