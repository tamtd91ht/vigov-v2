import { Check, Pencil, Smartphone, Trash2, X } from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { DATA_TABLE_CLASS } from "@/components/ui/data-table";
import { IconButton } from "@/components/ui/icon-button";
import { nhanBoPhan } from "@/features/cau-hinh/nhan-can-bo";
import { traTen, type BangTraDanhMuc } from "@/features/cau-hinh/tra-danh-muc";
import type { identity_canBoTomTat } from "@/lib/api/schema.gen";
import { cn } from "@/lib/cn";

import {
  CHIP_CHUA_HIEN,
  CHIP_DANG_HIEN,
  COT_MINI_APP,
  DONG_PHU_CO_ZALO,
  ariaRutMiniApp,
  ariaThemMiniApp,
} from "./cong-khai";
import { ariaXoaDong } from "./xoa-dong";

import {
  ariaSua,
  COT_CHUC_VU,
  COT_DI_DONG,
  COT_HO_TEN,
  COT_KHOI,
  COT_MAY_BAN,
  SELECT_PAGE_LABEL,
  selectRowLabel,
} from "./nhan-danh-ba";

/** The selection column's state and handlers. Absent = no selection column (no `content.update`). */
export type RowSelection = {
  readonly selectedIds: ReadonlySet<string>;
  readonly onToggle: (cb: identity_canBoTomTat, on: boolean) => void;
  readonly onTogglePage: (on: boolean) => void;
};

/**
 * Bảng cán bộ của tab **Danh bạ cán bộ** — bản mẫu `StaffDirectoryWorkspace.tsx:264-401`.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * ĐÂY KHÔNG PHẢI BẢN SAO CỦA `features/cau-hinh/danh-ba-can-bo.tsx`: màn `/nguoi-dung` quản lý **tài
 * khoản đăng nhập** (vai trò, khoá, mật khẩu); bảng này quản lý **thông tin liên hệ** — gọi ai, ở
 * khối nào, số nào, có hiện trên Mini App không. Phép tra id bộ phận → tên (`traTen`, `nhanBoPhan`)
 * thì dùng lại thật từ `features/cau-hinh/`, không chép.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 *
 * CỘT CHỌN (chủ đầu tư chốt 09/10/2026, theo bản mẫu) chỉ có khi phiên cầm `content.update`: việc
 * duy nhất người đã chọn dùng tới là thêm / rút khỏi Mini App. Chọn KHÔNG công khai ai — nút
 * "Thêm vào danh bạ Mini App" của thanh chọn mở hộp hỏi ý TỪNG người (#12).
 *
 * MỘT BẢNG Ở MỌI BỀ RỘNG, như bản mẫu: ở 320px khung `overflow-x-auto` cuộn ngang, là một vùng có
 * tên và tới được bằng bàn phím.
 *
 * THUẦN TRÌNH BÀY: không đọc mạng, không giữ state, KHÔNG GỌI HOOK — gọi `BangLienHe` như một hàm vẫn
 * an toàn, và `react-dom/server` kết xuất được nó trong Node.
 */
export function BangLienHe({
  danhSach,
  traBoPhan,
  onSua,
  congKhai,
  onXoa,
  selection,
}: {
  danhSach: readonly identity_canBoTomTat[];
  /** Bảng tra đã dựng sẵn, đi XUỐNG như tham số — không dòng nào tự đi hỏi máy chủ. */
  traBoPhan: BangTraDanhMuc;
  onSua: (cb: identity_canBoTomTat) => void;
  /**
   * Hai hành động Mini App. `undefined` = phiên KHÔNG có `content.update` (hoặc chưa đọc xong
   * phiên) → không vẽ nút nào. Ẩn là tiện dụng: máy chủ kiểm khoá ấy trên từng lời gọi.
   */
  congKhai?: {
    onThem: (cb: identity_canBoTomTat) => void;
    onRut: (cb: identity_canBoTomTat) => void;
  };
  /**
   * Mở hộp xoá dòng nhập trùng (lý do bắt buộc). `undefined` = phiên KHÔNG có `admin.user.delete`
   * (hoặc chưa đọc xong) → không vẽ nút. Dòng có tài khoản VẪN có nút: hộp mở ra để nói vì sao.
   */
  onXoa?: (cb: identity_canBoTomTat) => void;
  selection?: RowSelection;
}) {
  const allOnPage = danhSach.length > 0 && danhSach.every((cb) => selection?.selectedIds.has(cb.id) === true);
  return (
    <table className={cn(DATA_TABLE_CLASS, "w-full text-sm")}>
      <caption className="an-thi-giac">
        Danh bạ cán bộ của đơn vị: họ tên, chức vụ, khối/đơn vị, số liên hệ và trạng thái trên Zalo Mini
        App.
      </caption>
      <thead>
        <tr>
          {selection !== undefined && (
            <th scope="col" className="w-10">
              <input
                type="checkbox"
                checked={allOnPage}
                aria-label={SELECT_PAGE_LABEL}
                onChange={(e) => selection.onTogglePage(e.target.checked)}
                className="accent-brand m-0 size-3.5"
              />
            </th>
          )}
          <th scope="col" className="min-w-52">
            {COT_HO_TEN}
          </th>
          <th scope="col" className="min-w-56">
            {COT_CHUC_VU}
          </th>
          <th scope="col" className="min-w-56">
            {COT_KHOI}
          </th>
          {/* HAI CỘT SỐ, KHÔNG MỘT (#16): bản mẫu vẽ một cột `Di động`; gộp lại là đặt số công vụ và
              dữ liệu cá nhân dưới cùng một cái tên. */}
          <th scope="col" className="w-32">
            {COT_MAY_BAN}
          </th>
          <th scope="col" className="w-32">
            {COT_DI_DONG}
          </th>
          <th scope="col" className="w-36 text-center">
            {COT_MINI_APP}
          </th>
          {/* Always drawn: every row has at least its edit button. */}
          <th scope="col" className="w-32">
            <span className="an-thi-giac">Hành động</span>
          </th>
        </tr>
      </thead>
      <tbody>
        {danhSach.map((cb) => (
          <tr key={cb.id} className={cn(cb.published && "bg-leaf/4")}>
            {selection !== undefined && (
              <td>
                <input
                  type="checkbox"
                  checked={selection.selectedIds.has(cb.id)}
                  aria-label={selectRowLabel(cb.full_name)}
                  onChange={(e) => selection.onToggle(cb, e.target.checked)}
                  className="accent-brand m-0 size-3.5"
                />
              </td>
            )}
            <td className="text-navy font-semibold">
              {/* The prototype's name button: opens the edit dialog, as the pencil does. */}
              <button
                type="button"
                className="text-navy m-0 cursor-pointer border-0 bg-transparent p-0 text-left [font-family:inherit] text-sm font-semibold hover:underline"
                onClick={() => onSua(cb)}
              >
                {cb.full_name}
              </button>
              {cb.email !== "" && <div className="text-ink-muted text-[11px] font-normal">{cb.email}</div>}
            </td>
            <td>{cb.position || "—"}</td>
            {/* `nhanBoPhan` never returns an empty string: an unknown id says so in words. */}
            <td className="text-ink-muted text-[12px]">{nhanBoPhan(traTen(traBoPhan, cb.department_id))}</td>
            {/* SHOWN AS THE SERVER SENT IT, UNMASKED (#11, 22/09/2026: no masking inside the commune) and
                not reformatted — a reformatted number cannot be copied into a phone. */}
            <td className="tabular-nums">{cb.phone || "—"}</td>
            <td className="tabular-nums">
              {cb.mobile || "—"}
              {cb.has_zalo && <div className="text-brand text-[11px]">{DONG_PHU_CO_ZALO}</div>}
            </td>
            <td className="text-center">
              {cb.published ? (
                <Badge tone="success" icon={Check}>
                  {CHIP_DANG_HIEN}
                </Badge>
              ) : (
                <span className="text-ink-muted text-[12px]">{CHIP_CHUA_HIEN}</span>
              )}
            </td>
            <td>
              {/* The prototype's three outline icon buttons, in its order: Mini App (add or withdraw), edit,
                  delete. Publishing only OPENS the consent box (#12); delete only opens the reason box. */}
              <span className="flex justify-end gap-1">
                {congKhai !== undefined &&
                  (cb.published ? (
                    <IconButton
                      type="button"
                      variant="secondary"
                      label={ariaRutMiniApp(cb.full_name)}
                      onClick={() => congKhai.onRut(cb)}
                    >
                      <X aria-hidden="true" className="size-3.5" />
                    </IconButton>
                  ) : (
                    <IconButton
                      type="button"
                      variant="secondary"
                      label={ariaThemMiniApp(cb.full_name)}
                      onClick={() => congKhai.onThem(cb)}
                    >
                      <Smartphone aria-hidden="true" className="size-3.5" />
                    </IconButton>
                  ))}
                <IconButton type="button" variant="secondary" label={ariaSua(cb.full_name)} onClick={() => onSua(cb)}>
                  <Pencil aria-hidden="true" className="size-3.5" />
                </IconButton>
                {onXoa !== undefined && (
                  <IconButton
                    type="button"
                    variant="secondary"
                    className="text-danger hover:not-disabled:text-danger"
                    label={ariaXoaDong(cb.full_name)}
                    onClick={() => onXoa(cb)}
                  >
                    <Trash2 aria-hidden="true" className="size-3.5" />
                  </IconButton>
                )}
              </span>
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}
