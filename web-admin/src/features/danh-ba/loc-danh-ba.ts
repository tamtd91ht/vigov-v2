/**
 * Bộ lọc của màn **Danh bạ cán bộ** — ô tìm, ô khối / đơn vị, ô trạng thái hiển thị
 * (`docs/ui-ux/12-danh-ba-can-bo.md §3`) — và phép "đổi lọc là về trang đầu".
 *
 * MODULE THUẦN, KHÔNG JSX: mọi quyết định ở đây kiểm được bằng một phép so giá trị, không phải
 * dựng cả cây component. Cùng khuôn với `nhan-danh-ba.ts`.
 *
 * CHỮ TÌM SỐNG TRONG STATE CỦA REACT, VÀ CHỈ Ở ĐÓ. Không đẩy lên router, không vào `searchParams`,
 * không vào `history.state`: thanh địa chỉ và lịch sử trình duyệt của một máy dùng chung là đúng
 * chỗ một họ tên hay một số điện thoại không được nằm lại (luật 3, cấm #4). Đi tiếp các trang của
 * một lần tìm dùng con trỏ trong THÂN `POST /api/v1/staff/searches` (`lib/api/can-bo.ts`).
 */

import { TRANG_DAU, type NganXepConTro } from "@/features/cau-hinh/ngan-xep-con-tro";
import { TU_KHOA_TIM_TOI_DA, chuanHoaTuKhoaTim, type TuKhoaHopLe } from "@/lib/api/can-bo";

/* ---- câu chữ -------------------------------------------------------------------------------- */

/** Nhãn và gợi ý của ô tìm — gợi ý là nguyên văn đặc tả §3. */
export const NHAN_O_TIM = "Tìm cán bộ";
export const GOI_Y_O_TIM = "Tìm theo tên, chức vụ, số điện thoại…";
export const NUT_TIM = "Tìm";

/**
 * Câu từ chối chữ tìm quá dài — cùng câu máy chủ trả (`http/can_bo_tim.go`), để cán bộ không thấy
 * hai cách nói cho một quy tắc. KHÔNG nhắc lại chữ đã gõ: nó thường là họ tên hoặc số điện thoại.
 */
export const CAU_TU_KHOA_QUA_DAI = `Từ khoá tìm kiếm quá dài (tối đa ${TU_KHOA_TIM_TOI_DA} ký tự).`;

export const NHAN_LOC_KHOI = "Khối / đơn vị";
export const TAT_CA_KHOI = "Tất cả khối / đơn vị";
export const NHAN_LOC_HIEN_THI = "Trạng thái hiển thị";

/* ---- trạng thái hiển thị trên Mini App ------------------------------------------------------ */

/**
 * Ba lựa chọn của ô trạng thái hiển thị — nhãn và giá trị `1` / `0` là nguyên văn đặc tả §3.
 *
 * GIÁ TRỊ CỦA Ô CHỌN KHÔNG PHẢI GIÁ TRỊ GỬI LÊN. Ô chọn phát ra chuỗi; `congKhaiTheoMa` đổi nó thành
 * `true` / `false` / `null`, và `lib/api/can-bo.ts` đổi tiếp thành đúng hai chữ máy chủ nhận.
 */
export const LUA_CHON_HIEN_THI = {
  "": { nhan: "Hiện và chưa hiện", congKhai: null },
  "1": { nhan: "Đang hiện trên Mini App", congKhai: true },
  "0": { nhan: "Chưa hiện", congKhai: false },
} as const satisfies Record<string, { nhan: string; congKhai: boolean | null }>;

export type MaHienThi = keyof typeof LUA_CHON_HIEN_THI;

/**
 * Thứ tự vẽ ba lựa chọn — một MẢNG, không `Object.keys(LUA_CHON_HIEN_THI)`: JavaScript xếp khoá
 * dạng số nguyên (`"0"`, `"1"`) lên TRƯỚC mọi khoá khác, nên `Object.keys` cho ra "Chưa hiện" đứng
 * đầu và lựa chọn mặc định đứng cuối.
 */
export const THU_TU_HIEN_THI: readonly MaHienThi[] = ["", "1", "0"];

/** Ô chọn phát ra CHUỖI; chỉ một mã có trong bảng mới thành bộ lọc. Mã lạ → "cả hai". */
export function maHienThi(giaTri: string): MaHienThi {
  return Object.hasOwn(LUA_CHON_HIEN_THI, giaTri) ? (giaTri as MaHienThi) : "";
}

/* ---- trạng thái truy vấn -------------------------------------------------------------------- */

/**
 * Bộ lọc đang ÁP DỤNG — không phải thứ đang gõ dở trong ô tìm. Ô tìm chỉ thành bộ lọc khi bấm Tìm
 * hoặc Enter; gõ từng phím không gọi mạng.
 */
export type LocDanhBa = {
  /** `null` = không tìm theo chữ → `GET /api/v1/staff`. */
  readonly tuKhoa: TuKhoaHopLe | null;
  /** id bộ phận; `""` = tất cả. */
  readonly boPhan: string;
  readonly hienThi: MaHienThi;
};

export type TruyVanDanhBa = { readonly loc: LocDanhBa; readonly nganXep: NganXepConTro };

export const TRUY_VAN_DAU: TruyVanDanhBa = {
  loc: { tuKhoa: null, boPhan: "", hienThi: "" },
  nganXep: TRANG_DAU,
};

/**
 * Đổi một hoặc vài bộ lọc — và LUÔN về trang đầu.
 *
 * Con trỏ của trang 3 thuộc về truy vấn cũ: gửi nó kèm bộ lọc mới thì máy chủ hoặc từ chối, hoặc
 * — tệ hơn — trả một trang giữa chừng của truy vấn mới, và cán bộ không thấy những người đứng
 * trước con trỏ ấy. Một hàm duy nhất trả về CẢ bộ lọc lẫn ngăn xếp, nên không có cách nào đổi cái
 * này mà quên cái kia.
 */
export function apLoc(truyVan: TruyVanDanhBa, doi: Partial<LocDanhBa>): TruyVanDanhBa {
  return { loc: { ...truyVan.loc, ...doi }, nganXep: TRANG_DAU };
}

/** Tham số trang cho `docTrangDanhBa`, dựng từ trạng thái truy vấn. */
export function thamSoDoc(truyVan: TruyVanDanhBa): {
  boPhan: string;
  congKhai: boolean | null;
  cursor: string | null;
} {
  return {
    boPhan: truyVan.loc.boPhan,
    congKhai: LUA_CHON_HIEN_THI[truyVan.loc.hienThi].congKhai,
    cursor: truyVan.nganXep.hienTai,
  };
}

/**
 * Có bộ lọc nào đang áp không — quyết định câu cho một danh sách rỗng. "Xã chưa có cán bộ nào" và
 * "không ai khớp điều kiện" là hai câu khẳng định khác hẳn nhau về một cơ quan.
 */
export function dangLoc(loc: LocDanhBa): boolean {
  return loc.tuKhoa !== null || loc.boPhan !== "" || loc.hienThi !== "";
}

/**
 * Bấm Tìm (hoặc Enter) với chữ `oTim` trong ô: hoặc một câu từ chối, hoặc một thay đổi bộ lọc.
 *
 *   quá dài → câu từ chối, KHÔNG đổi bộ lọc đang áp (danh sách đang hiện vẫn đúng với điều kiện cũ)
 *             và KHÔNG gọi mạng — chữ không rời trình duyệt.
 *   rỗng    → `tuKhoa: null`: bỏ tìm, quay về `GET /api/v1/staff`.
 *   hợp lệ  → `tuKhoa` đã chuẩn hoá: `POST /api/v1/staff/searches`.
 */
export function ketQuaGuiTim(
  oTim: string,
): { readonly loi: string } | { readonly doi: Pick<LocDanhBa, "tuKhoa"> } {
  const tu = chuanHoaTuKhoaTim(oTim);
  if (tu.loai === "quaDai") return { loi: CAU_TU_KHOA_QUA_DAI };
  return { doi: { tuKhoa: tu.loai === "rong" ? null : tu } };
}

/**
 * Giá trị ô chọn khối → bộ lọc. Chỉ nhận id có trong danh mục đã đọc; giá trị lạ (sửa bằng
 * DevTools, hay danh mục vừa đổi) → "tất cả", không gửi một id không ai chọn.
 */
export function maBoPhanLoc(giaTri: string, danhMuc: readonly { readonly id: string }[]): string {
  return danhMuc.some((m) => m.id === giaTri) ? giaTri : "";
}
