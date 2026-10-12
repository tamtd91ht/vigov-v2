/**
 * MẢNH GIAO DIỆN DÙNG CHUNG CỦA APP RIÊNG MỘT XÃ — khung và bộ nút theo prototype khách
 * (`citizen-app/ui-design/prototpye-spec/PROTOTYPE.md` §5, §7; chủ dự án chọn 30/09/2026), lớp CSS `.xa-*`
 * trong `styles.css`.
 *
 * Khác prototype ở chỗ bắt buộc (`skills/accessibility-elderly`, `accessibility.test.ts`) và ở các quyết định
 * của chủ dự án: chữ không dưới 16px (prototype cho 13px), đích chạm không dưới 48px, nút chính màu ĐỎ thương
 * hiệu (prototype: xanh), "đang tải" là CÂU CHỮ HIỆN RÕ kèm khối xám hình nội dung (từ 02/10/2026; trước đó chỉ chữ
 * — câu vẫn mang `role="status"`, khối chỉ trang trí).
 * Ô trái header của bốn tab gốc nay THEO prototype — quốc huy vẽ tay (`NationalEmblem`, `AppHeader.tsx:57-64, 98-142`;
 * bảng lỗi của khách 12/10/2026, dòng 5: "đổi icon ở góc trái thành icon ngôi sao giống prototype"), thay logo của xã
 * mà ADR 0069 đặt vào ô này.
 * Nút lùi nay THEO prototype — chỉ mũi tên, tên "Quay lại" ở `aria-label` (`AppHeader.tsx:48-55`; bảng lỗi của
 * khách 12/10/2026, dòng 3: "dư text 'Quay lại'"); trước đó chủ dự án giữ chữ cạnh mũi tên.
 */
import type { ReactNode } from "react";

import { QUAY_LAI, XA_GIAO_DIEN } from "./noi-dung";
import { BieuTuong, type TenBieuTuong } from "./BieuTuong";
import { EmptyIllustration, type IllustrationKind, LoadingSkeleton, type SkeletonShape } from "./state-visuals";
import { PullToRefresh, type Refresher } from "./pull-to-refresh";

/* ═══════════════════════════════ HEADER ═══════════════════════════════ */

/**
 * Bronze-drum rings in the header's top-right corner (`PROTOTYPE.md` §5.3): 4 circles, 16 rays, a centre.
 * JSX elements, never an SVG string (`BieuTuong.tsx` says why). Coordinates rounded once at module load so
 * the server-rendered markup the tests read is stable.
 */
const DRUM_RINGS = [46, 38, 30, 22] as const;
const DRUM_RAYS = Array.from({ length: 16 }, (_, i) => {
  const a = (i / 16) * Math.PI * 2;
  const at = (r: number) => ({ x: +(50 + Math.cos(a) * r).toFixed(2), y: +(50 + Math.sin(a) * r).toFixed(2) });
  return { from: at(14), to: at(46) };
});

export function DrumPattern() {
  return (
    <svg className="xa-drum" viewBox="0 0 100 100" aria-hidden="true" focusable="false">
      {DRUM_RINGS.map((r) => (
        <circle key={r} cx="50" cy="50" r={r} fill="none" stroke="#ffffff" strokeWidth="1.4" />
      ))}
      {DRUM_RAYS.map((ray, i) => (
        <line key={i} x1={ray.from.x} y1={ray.from.y} x2={ray.to.x} y2={ray.to.y} stroke="#ffffff" strokeWidth="1.2" />
      ))}
      <circle cx="50" cy="50" r="7" fill="#ffffff" />
    </svg>
  );
}

/**
 * The twelve rice-ear strokes of the emblem's ring (prototype `AppHeader.tsx:118-134`), rounded once at module
 * load for the same reason as `DRUM_RAYS`: the server-rendered markup the tests read must be stable.
 */
const EMBLEM_STROKES = Array.from({ length: 12 }, (_, i) => {
  const a = (i / 12) * Math.PI * 2 - Math.PI / 2;
  const at = (r: number) => ({ x: +(16 + Math.cos(a) * r).toFixed(2), y: +(16 + Math.sin(a) * r).toFixed(2) });
  return { from: at(10.4), to: at(12.6) };
});

/**
 * THE STAR EMBLEM in the root tabs' left slot — the prototype's `NationalEmblem` (`AppHeader.tsx:98-142`), same
 * 32px, same red disc, same gold (customer sheet 12/10/2026, row 5). Drawn inline, never an image file: it must be
 * sharp at every pixel density and cost no network read, and a header that waits on a picture shifts its title.
 *
 * Decoration (`aria-hidden`), not a button: the prototype's button only opened its demo toolbar, which this app
 * does not have, and a tap target that does nothing is the "app is broken" signal for an elderly citizen. The
 * commune's name stands beside it in words, so nothing is lost to a screen reader.
 */
export function NationalEmblem() {
  return (
    <span className="xa-emblem" aria-hidden="true">
      <svg width="32" height="32" viewBox="0 0 32 32" focusable="false">
        <circle cx="16" cy="16" r="15" fill="#b8000c" />
        <circle cx="16" cy="16" r="12.5" fill="none" stroke="#f5c518" strokeWidth="1.6" />
        {EMBLEM_STROKES.map((s, i) => (
          <line key={i} x1={s.from.x} y1={s.from.y} x2={s.to.x} y2={s.to.y} stroke="#f5c518" strokeWidth="1.5" strokeLinecap="round" />
        ))}
        <path d="m16 8.4 2.2 4.6 5 .7-3.6 3.6.9 5-4.5-2.4-4.5 2.4.9-5-3.6-3.6 5-.7L16 8.4Z" fill="#f5c518" />
        <rect x="12" y="23" width="8" height="1.8" rx="0.9" fill="#f5c518" />
      </svg>
    </span>
  );
}

/**
 * The one header shape of the commune app (`PROTOTYPE.md` §5.3): a left slot, the title CENTRED between two
 * slots of equal width, a right slot. The right slot is Zalo's corner and stays empty on every screen: the
 * home bell left it in wave 1 (owner's decision 10, 30/09/2026) — "Thông báo" is a row in Cá nhân › Của tôi.
 */
function HeaderShell(props: { className: string; left: ReactNode; right?: ReactNode; children: ReactNode }) {
  return (
    <header className={`xa-header ${props.className}`}>
      <DrumPattern />
      <div className="xa-header__side">{props.left}</div>
      <div className="xa-header__title">{props.children}</div>
      <div className="xa-header__side xa-header__side--end">{props.right}</div>
    </header>
  );
}

/**
 * Header of the four root tabs: the star emblem on the left (`NationalEmblem`), the title (and an optional line
 * under it — the greeting on home) in the middle. `right`: no screen passes one any more (decision 10); the slot
 * is kept so the title stays centred between two equal sides.
 */
export function RootTabHeader(props: { title: string; subtitle?: string; right?: ReactNode }) {
  return (
    <HeaderShell className="xa-dau-tab" left={<NationalEmblem />} right={props.right}>
      <h1 className="xa-dau-con__tieu-de">{props.title}</h1>
      {props.subtitle !== undefined && props.subtitle !== "" && <p className="xa-header__subtitle">{props.subtitle}</p>}
    </HeaderShell>
  );
}

/**
 * Header của màn con: nút quay lại (mũi tên; tên "Quay lại" cho trình đọc màn hình ở `aria-label`) + tiêu đề ở giữa.
 * Nút Back của Zalo đóng cả app, nên đây là đường về — vẫn là một đích chạm 48×48 dù chỉ còn mũi tên.
 */
export function DauManCon({ tieu_de, onQuayLai }: { tieu_de: string; onQuayLai?: () => void }) {
  return (
    <HeaderShell
      className="xa-dau-con"
      left={
        // No back button only where the prototype has none: the "Đã gửi phản ánh" screen, whose two buttons are
        // the ways on (owner 11/10/2026: theo prototype `NewFeedbackPage.tsx:258`).
        onQuayLai === undefined ? null : (
          <button type="button" className="xa-dau-con__lui" aria-label={QUAY_LAI} onClick={onQuayLai}>
            <BieuTuong ten="chevron-left" co={28} stroke={2.5} />
          </button>
        )
      }
    >
      <h1 className="xa-dau-con__tieu-de">{tieu_de}</h1>
    </HeaderShell>
  );
}

/* ═══════════════════════════════ SECTION HEADER ═══════════════════════════════ */

/** The accent bars of `PROTOTYPE.md` §6.1 — decoration beside the words, never the only signal. */
export type SectionAccent = "brand" | "cam" | "luc" | "xanh";

/**
 * A section title with its 4×20 accent bar and, optionally, "Xem tất cả ›" (`PROTOTYPE.md` §6.1). The link is a
 * full 48px tap target in muted bold — the prototype's 40px exemption is not taken.
 */
export function SectionHeader(props: {
  title: string;
  accent?: SectionAccent;
  id?: string;
  more?: { label: string; onPress: () => void };
}) {
  return (
    <div className={`xa-dau-nhom xa-dau-nhom--${props.accent ?? "brand"}`}>
      <h2 className="xa-dau-khoi__tieu-de" id={props.id}>
        {props.title}
      </h2>
      {props.more && (
        <button type="button" className="xa-dau-khoi__them" onClick={props.more.onPress}>
          {props.more.label}
          <BieuTuong ten="chevron-right" co={18} />
        </button>
      )}
    </div>
  );
}

/** Tiêu đề một khối trên trang chủ + "Xem tất cả". */
export function DauKhoi({ tieu_de, onXemTatCa }: { tieu_de: string; onXemTatCa?: () => void }) {
  return (
    <div className="xa-dau-khoi">
      <h2 className="xa-dau-khoi__tieu-de">{tieu_de}</h2>
      {onXemTatCa && (
        <button type="button" className="xa-dau-khoi__them" onClick={onXemTatCa}>
          {XA_GIAO_DIEN.xem_tat_ca}
        </button>
      )}
    </div>
  );
}

/* ═══════════════════════════════ TONES · BUTTONS ═══════════════════════════════ */

/**
 * The measured tone classes (`xa-mau--*` in `styles.css`, the prototype's six tones §4.4 plus navy as the
 * neutral). Pink is gone (owner, 30/09/2026): the prototype's red tone took its place.
 */
export type Tone = "red" | "xanh" | "luc" | "cam" | "cyan" | "tim" | "navy";

/** Ô biểu tượng tròn nền nhạt; màu là tên một lớp `xa-mau--*` đã đo trong `styles.css`. */
export function OBieuTuong({ ten, mau }: { ten: TenBieuTuong; mau: Tone }) {
  return (
    <span className={`xa-o-bt xa-mau--${mau}`}>
      <BieuTuong ten={ten} co={24} />
    </span>
  );
}

export type ButtonVariant = "primary" | "secondary" | "quiet" | "danger";

/**
 * The four button shapes of `PROTOTYPE.md` §7, as class lists — so a screen that writes `<button>` itself and
 * one that uses `CommuneButton` look the same. Primary is the BRAND RED (owner, 30/09/2026), not the
 * prototype's blue; danger is the error red, and always carries words that say what will be lost.
 */
export const BUTTON_CLASS: Readonly<Record<ButtonVariant, string>> = {
  primary: "xa-nut",
  secondary: "xa-nut xa-nut--phu",
  quiet: "xa-nut xa-nut--quiet",
  danger: "xa-nut xa-nut--danger",
};

/** A full-width action, 52px tall, 17px bold. `icon` sits before the words; it never replaces them. */
export function CommuneButton(props: {
  variant?: ButtonVariant;
  icon?: TenBieuTuong;
  disabled?: boolean;
  onClick: () => void;
  children: ReactNode;
}) {
  return (
    <button type="button" className={BUTTON_CLASS[props.variant ?? "primary"]} disabled={props.disabled} onClick={props.onClick}>
      {props.icon && <BieuTuong ten={props.icon} co={20} />}
      {props.children}
    </button>
  );
}

/* ═══════════════════════════════ STATES ═══════════════════════════════ */

/**
 * The drawing an EMPTY state shows, from the icon the caller already names — so every empty list of a kind looks
 * alike without each call site choosing. Unknown icons get the generic tray. PURE, exported for tests.
 */
export function illustrationFor(icon: TenBieuTuong): IllustrationKind {
  if (icon === "news" || icon === "newspaper") return "news";
  if (icon === "chat" || icon === "message-square-plus" || icon === "message-square-warning") return "petition";
  if (icon === "users" || icon === "contact-book") return "directory";
  return "generic";
}

/**
 * Khối trạng thái (`PROTOTYPE.md` §7 `ErrorState` / `EmptyState`), luôn là chữ đọc được, không chỉ biểu tượng:
 *   · đang tải — CÂU CHỮ (`role="status"`, luôn hiện) + khối xám hình nội dung sắp tới (`shape`), không vòng quay.
 *     Quyết định của chủ dự án 02/10/2026 (UI-1), thay quyết định cũ "chỉ chữ, không khối nhấp nháy": khối chỉ
 *     nhấp nháy bằng độ mờ, và đứng yên hẳn khi máy xin giảm chuyển động (`styles.css`);
 *   · lỗi     — vòng 80px đỏ nhạt + biểu tượng, câu nói việc cần làm, nút phụ (thường là "Thử lại");
 *   · trống   — hình minh hoạ nhỏ (`illustrationFor`), câu làm tiêu đề, `hint` (tuỳ chọn) nói bước tiếp theo.
 */
export function KhoiTrangThai(props: {
  bieu_tuong: TenBieuTuong;
  cau: string;
  loi?: boolean;
  dang_tai?: boolean;
  nut?: { nhan: string; onBam: () => void };
  /** A secondary line under `cau` ("Mã hỗ trợ: <mã>"), muted rather than error-coloured. `null`/absent = none. */
  support_code?: string | null;
  /** Empty state: one muted sentence under the title saying what the citizen can do next. */
  hint?: string | null;
  /** Loading: the shape of what is coming (`state-visuals.tsx`). Default `list`; `none` = the sentence alone. */
  shape?: SkeletonShape;
  /** Loading, list shapes: how many rows to draw (default 3). */
  rows?: number;
}) {
  if (props.dang_tai && !props.loi) {
    return (
      <div className="xa-trang-thai xa-trang-thai--tai">
        <p role="status">{props.cau}</p>
        <LoadingSkeleton prefix="xa" shape={props.shape ?? "list"} rows={props.rows} />
      </div>
    );
  }
  return (
    <div className={`xa-trang-thai${props.loi ? " xa-trang-thai--loi" : ""}`}>
      {props.loi ? (
        <span className="xa-status-circle" aria-hidden="true">
          <BieuTuong ten={props.bieu_tuong} co={36} />
        </span>
      ) : (
        <EmptyIllustration kind={illustrationFor(props.bieu_tuong)} className="xa-illus" />
      )}
      <p className="xa-status-title" role={props.loi ? "alert" : undefined}>
        {props.cau}
      </p>
      {props.hint != null && props.hint !== "" && <p className="xa-status-hint">{props.hint}</p>}
      {props.support_code != null && <p className="xa-phu">{props.support_code}</p>}
      {props.nut && (
        <button type="button" className={BUTTON_CLASS.secondary} onClick={props.nut.onBam}>
          {props.nut.nhan}
        </button>
      )}
    </div>
  );
}

/**
 * Chỗ của một việc cần đăng nhập (gửi, xem, tra cứu phản ánh) khi app riêng chưa có phiên. Đường đăng
 * nhập theo App ID của app xã CHƯA DỰNG (ADR 0047 §6); nói thật điều ấy, và chỉ người dân tới xã.
 */
export function ChuaDangNhap({ cau }: { cau: string }) {
  return (
    <div className="xa-the xa-the--dem xa-ghi-chu">
      <BieuTuong ten="info" co={22} />
      <div>
        <p className="xa-ghi-chu__tieu-de">{XA_GIAO_DIEN.chua_dang_nhap_tieu_de}</p>
        <p>{cau}</p>
      </div>
    </div>
  );
}

/**
 * The body of a child screen — the ONE part that scrolls; its header above and any footer below stay put. A pull at
 * its top re-runs the screen's loaders (`pull-to-refresh.tsx`): `onRefresh` for those the screen holds itself, plus
 * any list below that registered its own. Neither → the pull does nothing.
 */
export function TrangCon({ children, onRefresh }: { children: ReactNode; onRefresh?: Refresher }) {
  return (
    <PullToRefresh className="xa-trang xa-trang--con" onRefresh={onRefresh}>
      {children}
    </PullToRefresh>
  );
}
