"use client";

import { Search } from "lucide-react";
import type { ChangeEvent, FormEvent, ReactNode } from "react";

import { cn } from "@/lib/cn";
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

/* ---- the prototype's filter-row select ------------------------------------------------------ */

/**
 * The prototype's native select of a filter row (`DocumentWorkspace.tsx:39-40`, `SELECT_CLASS`): 36px,
 * `rounded-md`, `line` hairline, white, 12.5px. `pr-9` leaves the right end to the chevron the global
 * select frame draws (`globals.css`, `:where(select…)`); `min-w-0`/`min-h-0` lift that frame's 12rem
 * floor and its 32px minimum, which would otherwise widen every select of the row.
 */
export const FILTER_SELECT_CLASS = cn(
  "h-9 min-h-0 min-w-0 cursor-pointer rounded-md border border-solid border-line bg-white pr-9 pl-3",
  "[font-family:inherit] text-[12.5px] text-ink outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50",
);

/**
 * One select of a filter row. The prototype names each select only by its first option and an
 * `aria-label`; this keeps a REAL `<label>` (visually hidden) so the name is also a click target.
 * Wrapped in a `<span>`: a `<div>` whose children are `label + select` is turned into a column by a
 * global legacy rule (`globals.css`, "LABEL ABOVE").
 */
export function FilterSelect({
  id,
  label,
  value,
  onChange,
  disabled = false,
  children,
}: {
  id: string;
  label: string;
  value: string | number;
  onChange: (value: string) => void;
  disabled?: boolean;
  children: ReactNode;
}) {
  return (
    <span className="inline-flex min-w-0">
      <label htmlFor={id} className="an-thi-giac">
        {label}
      </label>
      <select
        id={id}
        value={value}
        disabled={disabled}
        className={FILTER_SELECT_CLASS}
        onChange={(e: ChangeEvent<HTMLSelectElement>) => onChange(e.target.value)}
      >
        {children}
      </select>
    </span>
  );
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
  return (
    <FilterSelect id={id} label="Thứ tự" value={thuTu} disabled={disabled} onChange={(v) => datThuTu(maThuTu(v))}>
      {(Object.keys(THU_TU_SO) as MaThuTu[]).map((ma) => (
        <option key={ma} value={ma}>
          {THU_TU_SO[ma].nhan}
        </option>
      ))}
    </FilterSelect>
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
    <FilterSelect id={id} label="Năm của sổ" value={year} onChange={(v) => onYear(Number(v))}>
      {danhSachNam(anchorYear).map((n) => (
        <option key={n} value={n}>
          Sổ năm {n}
        </option>
      ))}
    </FilterSelect>
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

  // The prototype's search box (`DocumentWorkspace.tsx:176-183`): 256px, the magnifier inside at 12px,
  // a 36px shadcn Input with 36px left padding and 12.5px words, no visible label and no button —
  // Enter submits (an implicit submission of a one-field form). The label stays, visually hidden.
  return (
    <form className="relative m-0 w-64 max-w-full" role="search" onSubmit={gui}>
      <label htmlFor={id} className="an-thi-giac">
        Tìm văn bản
      </label>
      <Search
        aria-hidden="true"
        focusable="false"
        strokeWidth={1.8}
        className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-ink-muted"
      />
      <input
        id={id}
        name={id}
        type="search"
        defaultValue={tim}
        placeholder={goiY}
        autoComplete="off"
        className={cn(
          "box-border h-9 w-full min-w-0 rounded-lg border border-solid border-input bg-transparent py-1 pr-2.5 pl-9",
          "[font-family:inherit] text-[12.5px] text-navy outline-none placeholder:text-muted-foreground",
          "focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50",
        )}
      />
    </form>
  );
}

// The prototype's wording, verbatim (`DocumentWorkspace.tsx:179`) — owner decision 08/10/2026 (Q6): the
// incoming route is getting issuing-body search in a parallel backend card.
export const GOI_Y_TIM_DEN = "Tìm theo trích yếu, cơ quan, số ký hiệu…";
export const GOI_Y_TIM_DI = "Tìm theo trích yếu, nơi nhận…";
