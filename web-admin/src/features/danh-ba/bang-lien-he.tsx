import {
  Building2,
  CircleCheck,
  EyeOff,
  Pencil,
  Phone,
  Smartphone,
  Trash2,
} from "lucide-react";
import type { ReactNode } from "react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { ActionMenu, type ActionMenuItem } from "@/components/ui/action-menu";
import { IconButton } from "@/components/ui/icon-button";
import { PendingCell, PendingColumnHeader } from "@/components/ui/pending-feature";
import { nhanBoPhan } from "@/features/cau-hinh/nhan-can-bo";
import { traTen, type BangTraDanhMuc, type KetTra } from "@/features/cau-hinh/tra-danh-muc";
import type { identity_canBoTomTat } from "@/lib/api/schema.gen";
import { cn } from "@/lib/cn";

import {
  CHIP_CHUA_HIEN,
  CHIP_DANG_HIEN,
  COT_MINI_APP,
  DONG_PHU_CO_ZALO,
  NUT_RUT_MINI_APP,
  NUT_THEM_MINI_APP,
  ariaRutMiniApp,
  ariaThemMiniApp,
  dongYGhiLuc,
} from "./cong-khai";
import { DELETE_SHORT, NHAN_XOA_DONG, ariaXoaDong } from "./xoa-dong";

import {
  ariaMoreActions,
  ariaSua,
  COT_CHUC_VU,
  COT_DI_DONG,
  COT_HO_TEN,
  COT_KHOI,
  COT_MAY_BAN,
  EDIT_SHORT,
  NUT_SUA_THONG_TIN,
  pendingPart,
} from "./nhan-danh-ba";

/**
 * Bảng cán bộ của màn **Danh bạ** — `docs/ui-ux/12-danh-ba-can-bo.md §4`.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * ĐÂY KHÔNG PHẢI BẢN SAO CỦA `features/cau-hinh/danh-ba-can-bo.tsx`, VÀ SỰ KHÁC NHAU LÀ SỰ KHÁC
 * NHAU MÀ CHÍNH ĐẶC TẢ ĐẶT RA (§1): màn `/nguoi-dung` (trước 05/10/2026 là tab `Cấu hình → Người dùng`) quản lý **tài khoản đăng nhập** —
 * mã cán bộ, vai trò, trạng thái khoá, đăng nhập gần nhất, cấp tài khoản, đặt lại mật khẩu. Màn
 * này quản lý **thông tin liên hệ**: gọi ai, ở khối nào, số nào.
 *
 * Vì vậy bảng ở đây KHÔNG dựng lại `BangCanBo` của tab kia: mười một cột của nó mang đúng những
 * thứ màn liên hệ không nói tới, và sáu hành động của nó gồm khoá tài khoản với đặt lại mật khẩu —
 * hai việc có hậu quả nặng, không thuộc về một màn danh bạ. Bảy dòng dữ liệu chung thì lấy từ
 * CHÍNH `identity.canBoTomTat` của hợp đồng, không từ một kiểu chép lại.
 *
 * PHẦN DÙNG LẠI THÌ DÙNG LẠI THẬT, KHÔNG CHÉP: phép tra id bộ phận → tên (`traTen`) và năm câu
 * chữ cho năm ca tra (`nhanBoPhan`) đến thẳng từ `features/cau-hinh/`. Chép chúng sang đây là hai
 * bản sẽ trôi (luật 9, cấm #2) — và bản trôi sẽ là bản nói sai về một bộ phận đã bị xoá mềm.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 *
 * KHÔNG CÓ CỘT CHECKBOX TRONG BẢNG, và sự vắng mặt ấy không phải quên. Công khai nhiều người một lần
 * (người dùng chốt 30/09/2026) sống trong khung riêng `bulk-publication-panel.tsx`, nơi MỖI người
 * được chọn mang ô "Đã hỏi ý người này" của riêng mình — #12 vẫn hỏi ý TỪNG người. Một ô chọn
 * trong bảng này, cách xa ô xác nhận đồng ý, là chỗ để chọn mà không hỏi. Hai nút
 * `Thêm vào / Rút khỏi danh bạ Mini App` vẫn nằm trên TỪNG DÒNG cho thao tác một người.
 *
 * HAI CÁCH VẼ CÙNG MỘT DANH SÁCH (ADR 0068, đặc tả giao diện §9): từ 768px là BẢNG — ngữ nghĩa bảng
 * còn nguyên cho trình đọc màn hình; dưới 768px là DANH SÁCH THẺ. Không làm bằng cách đổi `display`
 * của `table`/`tr`/`td` (cách ấy làm mất ngữ nghĩa bảng); mỗi bản ẩn hẳn (`display: none`) ở bề rộng
 * của bản kia, nên trình đọc màn hình chỉ gặp một. Cả hai đọc CÙNG các hàm nhãn và CÙNG các handler
 * — mỗi nút ở bản thẻ là đúng nút ở bản bảng, cùng `aria-label`, cùng lời gọi.
 *
 * THUẦN TRÌNH BÀY: không đọc mạng, không giữ state, KHÔNG GỌI HOOK — cột "?" là phần tử con
 * (`PendingColumnHeader`), nên gọi `BangLienHe` như một hàm vẫn an toàn. Nhờ vậy kết xuất được bằng `react-dom/server`
 * trong Node và bài kiểm hỏi thẳng được "dòng này có RA TỚI TRANG không".
 */
export function BangLienHe({
  danhSach,
  traBoPhan,
  onSua,
  congKhai,
  onXoa,
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
   * Mở hộp xoá dòng nhập trùng. `undefined` = phiên KHÔNG có `admin.user.delete` (hoặc chưa đọc xong)
   * → không vẽ nút. Dòng có tài khoản VẪN có nút: hộp mở ra để nói vì sao không xoá được và chỉ sang
   * việc khoá — người bấm đang đi tìm lối, không nên gặp một nút mờ không lời giải thích.
   */
  onXoa?: (cb: identity_canBoTomTat) => void;
}) {
  return (
    <>
      {/* `role="region"` + `tabIndex` để vùng cuộn tới được bằng bàn phím. Khung thẻ là của `Card`
          bao ngoài, nên vùng cuộn bỏ viền và bóng riêng của `.bang-cuon`.

          TIÊU ĐỀ CỘT DÍNH (đặc tả v2 §8.1) CẦN VÙNG CUỘN DỌC CỦA CHÍNH BẢNG: `overflow-x: auto` biến
          khung này thành vùng cuộn theo cả hai chiều, nên `position: sticky` của `th` bám vào KHUNG,
          không bám vào trang. Vì vậy khung có chiều cao trần (vừa màn hình trừ đầu trang và thanh
          lọc) và tự cuộn dọc; trang ngắn hơn trần thì không có gì khác. Dòng 48px: đệm dọc 8px quanh
          ảnh đại diện 32px. */}
      <div
        className={cn(
          "bang-cuon hidden rounded-none border-0 bg-transparent shadow-none md:block",
          "overflow-y-auto md:max-h-[calc(100dvh-16rem)] md:min-h-48",
          "[&_thead_th]:sticky [&_thead_th]:top-0 [&_thead_th]:z-[1] [&_thead_th]:shadow-[inset_0_-1px_0_var(--line)]",
          "[&_tbody_td]:py-2",
        )}
        role="region"
        aria-label="Danh bạ cán bộ của đơn vị"
        tabIndex={0}
      >
        <table className="bang-can-bo">
          <caption className="an-thi-giac">
            Danh bạ cán bộ của đơn vị: họ tên, chức vụ, khối/đơn vị, số liên hệ và trạng thái trên
            Zalo Mini App.
          </caption>
          <thead>
            <tr>
              {/* Spec §5/§7 "Ảnh đại diện": a column the data does not have yet, drawn with the "?" of
                  ADR 0068 §14 (`PHAN_CHUA_DUNG`). Desktop table only — the phone card keeps the initials. */}
              <PendingColumnHeader info={pendingPart("Ảnh đại diện")} />
              <th scope="col">{COT_HO_TEN}</th>
              <th scope="col">{COT_CHUC_VU}</th>
              <th scope="col">{COT_KHOI}</th>
              {/* HAI CỘT SỐ, KHÔNG MỘT — xem `COT_MAY_BAN` / `COT_DI_DONG` trong `nhan-danh-ba.ts`
                  (#16). Đặc tả §4 chỉ vẽ một cột `Di động`; gộp lại dưới nhãn ấy là đặt số công vụ
                  và dữ liệu cá nhân dưới cùng một cái tên. */}
              <th scope="col">{COT_MAY_BAN}</th>
              <th scope="col">{COT_DI_DONG}</th>
              <th scope="col">{COT_MINI_APP}</th>
              <th scope="col">
                <span className="an-thi-giac">Hành động</span>
              </th>
            </tr>
          </thead>
          <tbody>
            {danhSach.map((cb) => (
              <tr key={cb.id}>
                <PendingCell />
                <td>
                  {/* Tên đậm + dòng phụ thư điện tử — đúng §4. Tên KHÔNG phải nút mở chi tiết như
                      đặc tả vẽ: màn này không có khối chi tiết riêng, vì mọi trường `canBoTomTat`
                      mang nghĩa liên hệ đều đã nằm ngay trên dòng. Một nút mở ra đúng thứ đang hiện
                      là một nút không làm gì. */}
                  <span className="flex items-center gap-3">
                    <Avatar fullName={cb.full_name} size="sm" />
                    <span className="min-w-0 leading-tight">
                      <span className="ten-can-bo text-ink-900">{cb.full_name}</span>
                      <span className="dong-phu leading-tight text-ink-500">{cb.email}</span>
                    </span>
                  </span>
                </td>
                <td className="text-ink-700">{cb.position || <Dash />}</td>
                <td>
                  <OKhoi ket={traTen(traBoPhan, cb.department_id)} />
                </td>
                {/* HIỆN NGUYÊN VĂN THỨ MÁY CHỦ TRẢ, KHÔNG CHE VÀ KHÔNG ĐỊNH DẠNG LẠI. Câu mở #11
                    chốt 22/09/2026: không che trong nội bộ xã. Máy chủ trả số nguyên vẹn, và việc
                    che còn nguyên ở bản xuất Excel cùng mọi đường ra ngoài cơ quan — hai bề mặt
                    chưa tồn tại. Định dạng lại thành `0900 000 001` như ví dụ đặc tả thì số copy ra
                    không gọi được và không tìm lại được. Icon chỉ đứng CẠNH số, không chen vào chuỗi. */}
                <td>
                  <DeskPhone cb={cb} />
                </td>
                <td>
                  <MobilePhone cb={cb} />
                </td>
                <td>
                  <OMiniApp cb={cb} />
                </td>
                <td>
                  {/* ONE icon button for the everyday action, the rest in "⋯" with WORDS (spec v2 §7):
                      publishing a personal number and deleting a row are never icon-only. The menu
                      items are built by `rowActions` from the SAME handlers and the SAME permission
                      props as the phone cards below. */}
                  <span className="flex items-center justify-end gap-1">
                    <IconButton
                      type="button"
                      label={ariaSua(cb.full_name)}
                      onClick={() => onSua(cb)}
                    >
                      <Pencil aria-hidden="true" />
                    </IconButton>
                    <ActionMenu
                      label={ariaMoreActions(cb.full_name)}
                      items={rowActions(cb, onSua, congKhai, onXoa)}
                    />
                  </span>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {/* Dưới 768px: mỗi cán bộ một thẻ. Nút cao 44px — vùng chạm cho cán bộ lớn tuổi bấm trên điện
          thoại (`skills/accessibility-elderly`). */}
      <ul className="m-0 flex list-none flex-col gap-3 p-0 md:hidden" aria-label="Danh bạ cán bộ của đơn vị">
        {danhSach.map((cb) => (
          <li key={cb.id} className="min-w-0 rounded-card border border-line bg-surface p-4 shadow-sm">
            <div className="flex items-start gap-3">
              <Avatar fullName={cb.full_name} />
              <div className="min-w-0 flex-1">
                <span className="ten-can-bo break-words text-ink-900">{cb.full_name}</span>
                <span className="dong-phu break-words text-ink-500">{cb.position || <Dash />}</span>
              </div>
              <OMiniApp cb={cb} />
            </div>
            <div className="mt-3 flex min-w-0 flex-col items-start gap-1.5 text-sm text-ink-700">
              <span className="flex min-w-0 items-center gap-2">
                <Building2 aria-hidden="true" focusable="false" className={ICON_CLASS} />
                <span className={lopKhoi(traTen(traBoPhan, cb.department_id))}>
                  {nhanBoPhan(traTen(traBoPhan, cb.department_id))}
                </span>
              </span>
              {cb.email !== "" && <span className="max-w-full break-all text-ink-500">{cb.email}</span>}
              <DeskPhone cb={cb} />
              <MobilePhone cb={cb} />
            </div>
            {/* The Mini App action takes its own full row — its words are long and must stay whole at
                320px — then edit and delete share the next row. Same order on screen and for Tab. */}
            <div className="mt-3 flex flex-wrap gap-2 border-t border-line pt-3">
              {congKhai !== undefined &&
                (cb.published ? (
                  <Button
                    type="button"
                    variant="secondary"
                    className={MOBILE_WIDE}
                    icon={<EyeOff aria-hidden="true" />}
                    aria-label={ariaRutMiniApp(cb.full_name)}
                    onClick={() => congKhai.onRut(cb)}
                  >
                    {NUT_RUT_MINI_APP}
                  </Button>
                ) : (
                  <Button
                    type="button"
                    variant="secondary"
                    className={MOBILE_WIDE}
                    icon={<Smartphone aria-hidden="true" />}
                    aria-label={ariaThemMiniApp(cb.full_name)}
                    onClick={() => congKhai.onThem(cb)}
                  >
                    {NUT_THEM_MINI_APP}
                  </Button>
                ))}
              <Button
                type="button"
                variant="secondary"
                className={MOBILE_HALF}
                icon={<Pencil aria-hidden="true" />}
                aria-label={ariaSua(cb.full_name)}
                onClick={() => onSua(cb)}
              >
                {EDIT_SHORT}
              </Button>
              {/* Text, not an icon alone: deleting is never icon-only (spec v2 §7). */}
              {onXoa !== undefined && (
                <Button
                  type="button"
                  variant="danger"
                  className={MOBILE_HALF}
                  icon={<Trash2 aria-hidden="true" />}
                  aria-label={ariaXoaDong(cb.full_name)}
                  onClick={() => onXoa(cb)}
                >
                  {DELETE_SHORT}
                </Button>
              )}
            </div>
          </li>
        ))}
      </ul>
    </>
  );
}

/**
 * The "⋯" items of one row (spec v2 §8.1): edit, then publish OR withdraw, then — after a separator,
 * red, last — delete. An absent permission prop removes its item, exactly as it removes the button on
 * the phone card: `congKhai` undefined → no Mini App item; `onXoa` undefined → no delete item and
 * no dangling separator.
 */
export function rowActions(
  cb: identity_canBoTomTat,
  onSua: (cb: identity_canBoTomTat) => void,
  congKhai: { onThem: (cb: identity_canBoTomTat) => void; onRut: (cb: identity_canBoTomTat) => void } | undefined,
  onXoa: ((cb: identity_canBoTomTat) => void) | undefined,
): ActionMenuItem[] {
  const items: ActionMenuItem[] = [
    { kind: "item", id: "sua", label: NUT_SUA_THONG_TIN, icon: Pencil, onSelect: () => onSua(cb) },
  ];
  if (congKhai !== undefined) {
    items.push(
      cb.published
        ? { kind: "item", id: "rut", label: NUT_RUT_MINI_APP, icon: EyeOff, onSelect: () => congKhai.onRut(cb) }
        : { kind: "item", id: "them", label: NUT_THEM_MINI_APP, icon: Smartphone, onSelect: () => congKhai.onThem(cb) },
    );
  }
  if (onXoa !== undefined) {
    items.push(
      { kind: "separator", id: "tach-xoa" },
      { kind: "item", id: "xoa", label: NHAN_XOA_DONG, icon: Trash2, tone: "danger", onSelect: () => onXoa(cb) },
    );
  }
  return items;
}

const ICON_CLASS = "size-4 shrink-0 text-ink-500";

/** Phone-card buttons: 44px targets; the wide one may wrap its words rather than overflow. */
const MOBILE_WIDE = "h-auto min-h-11 basis-full py-2 text-center leading-tight whitespace-normal";
const MOBILE_HALF = "h-11 min-w-0 flex-[1_1_0%]";

/**
 * Initials for the avatar — spec §7: two letters, "họ cuối + tên" (the last two words: "Nguyễn Văn
 * An" → "VA"). One word gives one letter. NFC first, so a decomposed "Ấ" stays one letter instead of
 * splitting into a bare "A" plus a combining mark.
 */
export function initialsOf(fullName: string): string {
  const words = fullName.normalize("NFC").trim().split(/\s+/).filter((w) => w !== "");
  return words
    .slice(-2)
    .map((w) => Array.from(w)[0] ?? "")
    .join("")
    .toLocaleUpperCase("vi-VN");
}

/** Decorative: the name next to it is the accessible text. */
function Avatar({ fullName, size = "md" }: { fullName: string; size?: "sm" | "md" }) {
  return (
    <span
      aria-hidden="true"
      className={cn(
        "grid shrink-0 place-items-center rounded-full bg-brand-100 font-bold text-brand-700",
        // 32px in a 48px table row (spec v2 §8.1), 36px on a phone card (spec §7).
        size === "sm" ? "size-8 text-xs" : "size-9 text-[13px]",
      )}
    >
      {initialsOf(fullName)}
    </span>
  );
}

/** An empty field reads "—", never a blank cell (spec §7). `--ink-500`: `--ink-400` is not for text. */
function Dash() {
  return <span className="text-ink-500">—</span>;
}

function PhoneLine({ icon, children }: { icon: ReactNode; children: ReactNode }) {
  return (
    <span className="inline-flex items-center gap-1.5 whitespace-nowrap tabular-nums">
      {icon}
      {children}
    </span>
  );
}

/** The office landline — official contact information (#16). */
function DeskPhone({ cb }: { cb: identity_canBoTomTat }) {
  if (cb.phone === "") return <Dash />;
  return (
    <PhoneLine icon={<Phone aria-hidden="true" focusable="false" className={ICON_CLASS} />}>
      {cb.phone}
    </PhoneLine>
  );
}

/**
 * Personal mobile + the "Có Zalo" sub-line. A two-column grid, not a flex row: the number (a text
 * node, so an anonymous grid item) sits beside the icon and `.dong-phu` drops UNDER the number,
 * aligned with it — while in the markup the number is still followed directly by that sub-line.
 */
function MobilePhone({ cb }: { cb: identity_canBoTomTat }) {
  if (cb.mobile === "" && !cb.has_zalo) return <Dash />;
  return (
    <span className="inline-grid grid-cols-[auto_auto] items-center gap-x-1.5 whitespace-nowrap tabular-nums [&>.dong-phu]:col-start-2">
      <Smartphone aria-hidden="true" focusable="false" className={ICON_CLASS} />
      {cb.mobile}
      {cb.has_zalo && <span className="dong-phu">{DONG_PHU_CO_ZALO}</span>}
    </span>
  );
}

/**
 * Ô cột "Trên Mini App": huy hiệu trạng thái (icon + chữ, không bao giờ chỉ màu), và với người đang
 * hiện thì thêm thời điểm ghi nhận đồng ý — bằng chứng #12 đòi, hiện ngay cạnh trạng thái nó bảo
 * chứng.
 */
function OMiniApp({ cb }: { cb: identity_canBoTomTat }) {
  if (!cb.published) {
    return (
      <Badge tone="neutral" icon={EyeOff} className="shrink-0">
        {CHIP_CHUA_HIEN}
      </Badge>
    );
  }
  const dongY = dongYGhiLuc(cb.consent_recorded_at);
  return (
    <span className="inline-flex shrink-0 flex-col items-start gap-1">
      <Badge tone="success" icon={CircleCheck}>
        {CHIP_DANG_HIEN}
      </Badge>
      {dongY !== null && <span className="dong-phu">{dongY}</span>}
    </span>
  );
}

/**
 * Ô cột "Khối / đơn vị".
 *
 * KHÔNG BAO GIỜ DỰNG RA MỘT Ô TRỐNG: `nhanBoPhan` luôn trả một câu, kể cả khi không tra được. Hình
 * đi theo LOẠI kết quả chứ không theo câu chữ, để "chưa phân bộ phận" (một trạng thái bình thường,
 * chữ nghiêng xám) và "không tra được" (một dòng dữ liệu lệch, chỉ người quản trị sửa được, huy
 * hiệu đỏ) không trông giống nhau. Tên khối tra được là huy hiệu xanh.
 */
function OKhoi({ ket }: { ket: KetTra }) {
  if (ket.loai === "coTen") {
    return (
      <Badge tone="info" icon={Building2}>
        {nhanBoPhan(ket)}
      </Badge>
    );
  }
  if (ket.loai === "khongTraDuoc") {
    return (
      <Badge tone="danger" className="nhan-lech">
        {nhanBoPhan(ket)}
      </Badge>
    );
  }
  return <span className="nhan-trong">{nhanBoPhan(ket)}</span>;
}

function lopKhoi(ket: KetTra): string | undefined {
  switch (ket.loai) {
    case "coTen":
      return undefined;
    case "khongTraDuoc":
      return "nhan-lech";
    default:
      return "nhan-trong";
  }
}
