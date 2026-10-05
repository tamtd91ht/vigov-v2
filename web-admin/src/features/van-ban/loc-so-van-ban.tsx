"use client";

import { Search } from "lucide-react";
import type { FormEvent } from "react";

import { Field } from "@/components/ui/field";
import { TRANG_DAU, type NganXepConTro } from "@/features/cau-hinh/ngan-xep-con-tro";
import type { ChieuSapXepVanBan, KhoaSapXepVanBan } from "@/lib/api/van-ban";
import { danhSachNam } from "@/lib/nam";

/**
 * Hai điều khiển chung của hai quyển sổ: ô tìm chữ và ô thứ tự — cùng phép "đổi lọc là về trang
 * đầu" mà cả hai dùng.
 */

/**
 * Đổi một bộ lọc là về TRANG ĐẦU: con trỏ cũ thuộc về một truy vấn khác (bộ lọc khác, thứ tự
 * khác), và máy chủ sẽ từ chối nó hoặc — tệ hơn — trả một trang giữa chừng của truy vấn mới.
 */
export function doiLocVeTrangDau(dat: () => void, datNganXep: (n: NganXepConTro) => void): void {
  dat();
  datNganXep(TRANG_DAU);
}

/* ---- thứ tự -------------------------------------------------------------------------------- */

/**
 * Các lựa chọn thứ tự. Khoá và chiều lấy kiểu từ HỢP ĐỒNG (`KhoaSapXepVanBan`): một khoá máy chủ
 * không nhận thì `tsc` đỏ ở đây, không phải 400 ở tay cán bộ.
 *
 * MẶC ĐỊNH LÀ `""` VÀ KHÔNG GỬI GÌ — máy chủ áp số vào sổ giảm dần (`docstore.SapXepVanBanDen`).
 * Lựa chọn mặc định không mang `sort`/`order` để web không giữ bản sao thứ hai của mặc định ấy.
 *
 * KHÔNG CÓ LỰA CHỌN THEO `created_at`, dù hợp đồng nhận nó: số được cấp đúng lúc tạo dòng, dưới
 * một khoá dòng, nên trong một năm của sổ hai thứ tự ấy trùng nhau. Hai lựa chọn cho ra cùng một
 * bảng là một lựa chọn thừa khiến cán bộ đi tìm chỗ khác nhau không có.
 */
type LuaChonThuTu = { nhan: string; sort?: KhoaSapXepVanBan; order?: ChieuSapXepVanBan };

export const THU_TU_SO = {
  "": { nhan: "Số mới nhất trước" },
  "so-tang": { nhan: "Số cũ nhất trước", sort: "number", order: "asc" },
} as const satisfies Record<string, LuaChonThuTu>;

export type MaThuTu = keyof typeof THU_TU_SO;

/** Ô chọn phát ra CHUỖI; chỉ một mã có trong bảng mới thành một thứ tự. Mã lạ → mặc định. */
export function maThuTu(giaTri: string): MaThuTu {
  return Object.hasOwn(THU_TU_SO, giaTri) ? (giaTri as MaThuTu) : "";
}

/** `sort`/`order` gửi lên cho một lựa chọn. Lựa chọn mặc định trả về rỗng — không gửi gì. */
export function sapXepTheoThuTu(ma: MaThuTu): {
  sort?: KhoaSapXepVanBan;
  order?: ChieuSapXepVanBan;
} {
  const tt: LuaChonThuTu = THU_TU_SO[ma];
  return tt.sort === undefined ? {} : { sort: tt.sort, order: tt.order };
}

export function ChonThuTu({
  id,
  thuTu,
  datThuTu,
  disabled = false,
}: {
  id: string;
  thuTu: MaThuTu;
  datThuTu: (ma: MaThuTu) => void;
  disabled?: boolean;
}) {
  // One select of the prototype's filter row (ADR 0068 lần 5): the label is visually hidden — the
  // selected option already says what it does — but stays a real `<label>`.
  return (
    <Field label="Thứ tự" htmlFor={id} kind="select" hideLabel grow="auto">
      <select id={id} value={thuTu} disabled={disabled} onChange={(e) => datThuTu(maThuTu(e.target.value))}>
        {(Object.keys(THU_TU_SO) as MaThuTu[]).map((ma) => (
          <option key={ma} value={ma}>
            {THU_TU_SO[ma].nhan}
          </option>
        ))}
      </select>
    </Field>
  );
}

/**
 * The register's year as one select of the filter row. Same list as `ChonNam` (`danhSachNam`, anchored
 * on the year read ONCE by the screen); the year stays visible as words — `Sổ năm 2026` — because the
 * route requires `year` and the screen must not pick one where nobody sees it (`lib/nam.ts`).
 */
export function RegisterYearSelect({
  id,
  year,
  anchorYear,
  onYear,
}: {
  id: string;
  year: number;
  anchorYear: number;
  onYear: (year: number) => void;
}) {
  return (
    <Field label="Năm của sổ" htmlFor={id} kind="select" hideLabel grow="auto">
      <select id={id} value={year} onChange={(e) => onYear(Number(e.target.value))}>
        {danhSachNam(anchorYear).map((n) => (
          <option key={n} value={n}>
            Sổ năm {n}
          </option>
        ))}
      </select>
    </Field>
  );
}

/* ---- tìm chữ ------------------------------------------------------------------------------- */

/**
 * Ô tìm chữ, gửi bằng SUBMIT chứ không theo từng phím — cùng lý do với sổ nhiệm vụ: mỗi phím là
 * một lời gọi mang chữ cán bộ đang gõ lên chuỗi truy vấn, và chuỗi ấy đi vào log truy cập của máy
 * chủ (luật 3, cấm #4). Chữ tìm KHÔNG đi vào thanh địa chỉ hay lịch sử trình duyệt: màn hình không
 * đẩy nó lên router.
 *
 * `goiY` phải nói ĐÚNG những cột máy chủ tìm — hai quyển sổ tìm trong hai cặp cột khác nhau
 * (`LocVanBanDen.tim`, `LocVanBanDi.tim`). Gợi ý hứa một cột máy chủ không tìm là cán bộ gõ đúng
 * mà nhận "không có văn bản nào", và kết luận văn bản chưa vào sổ.
 *
 * KHÔNG CÓ `maxLength`: máy chủ giới hạn 200 BYTE, còn `maxLength` đếm ký tự — 200 chữ có dấu vượt
 * giới hạn ấy. Một giới hạn ở ô nhập sai đơn vị là một lời hứa sai; câu từ chối của máy chủ hiện
 * nguyên văn thay vào đó.
 */
export function OTimVanBan({
  id,
  goiY,
  tim,
  datTim,
}: {
  id: string;
  goiY: string;
  tim: string;
  datTim: (s: string) => void;
}) {
  function gui(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const giaTri = new FormData(e.currentTarget).get(id);
    // Cắt khoảng trắng ở tầng gọi mạng (`lib/api/van-ban.ts`), không ở đây: một chỗ, một quy tắc.
    datTim(typeof giaTri === "string" ? giaTri : "");
  }

  // The prototype's search box: 256px, a magnifier inside, no visible label and no button —
  // Enter submits (an implicit submission of a one-field form).
  return (
    <form className="m-0 w-64 max-w-full" role="search" onSubmit={gui}>
      <Field label="Tìm văn bản" htmlFor={id} icon={Search} hideLabel grow="auto">
        <input
          id={id}
          name={id}
          type="search"
          defaultValue={tim}
          placeholder={goiY}
          autoComplete="off"
        />
      </Field>
    </form>
  );
}

// The prototype's wording ("Tìm theo trích yếu, cơ quan, số ký hiệu…") minus "cơ quan": the incoming
// route does not search the issuing body, and a hint promising it is a search that silently finds nothing.
export const GOI_Y_TIM_DEN = "Tìm theo trích yếu, số ký hiệu…";
export const GOI_Y_TIM_DI = "Tìm theo trích yếu, nơi nhận…";
