/**
 * MẢNH GIAO DIỆN DÙNG CHUNG CỦA APP RIÊNG MỘT XÃ — khung và bộ nút theo prototype khách
 * (`citizen-app/ui-design/prototpye-spec/PROTOTYPE.md` §5, §7; chủ dự án chọn 30/09/2026), lớp CSS `.xa-*`
 * trong `styles.css`.
 *
 * Khác prototype ở chỗ bắt buộc (`skills/accessibility-elderly`, `accessibility.test.ts`) và ở các quyết định
 * của chủ dự án: chữ không dưới 16px (prototype cho 13px), đích chạm không dưới 48px, nút chính màu ĐỎ thương
 * hiệu (prototype: xanh), "đang tải" là CHỮ (prototype: khối xám nhấp nháy), nút lùi giữ chữ "Quay lại" cạnh
 * mũi tên (prototype: chỉ mũi tên), và ô trái header là LOGO CỦA XÃ (prototype: quốc huy vẽ tay).
 */
import { type ReactNode, useState } from "react";

import { QUAY_LAI, XA_GIAO_DIEN } from "./noi-dung";
import { BieuTuong, type TenBieuTuong } from "./BieuTuong";

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
 * The file `deploy.mjs --vao-thang` copies from `scripts/logo-xa/<domain>.png` into the build (ADR 0047:246).
 * KEPT AS A FALLBACK until every commune has uploaded its own logo (ADR 0069 Hệ quả, same reasoning as ADR 0067
 * B11 for the banner): removing it early turns the header of a commune that has not uploaded yet from its logo
 * into the building icon.
 */
export const BUNDLED_LOGO_SRC = "./logo-xa.png";

/**
 * The pictures the header tries, in order: the logo the commune uploaded (`/commune-profiles` `logo_url`, read
 * at runtime — ADR 0069 #4, #8), then the bundled file. `""` (none uploaded, or the profile not read) skips
 * the first. After the list, the building icon (ADR 0069 #7). PURE.
 */
export function logoSources(runtimeLogoUrl: string): readonly string[] {
  return runtimeLogoUrl === "" ? [BUNDLED_LOGO_SRC] : [runtimeLogoUrl, BUNDLED_LOGO_SRC];
}

/**
 * What the header slot draws, given which sources already failed to load: the first source not failed, or the
 * building icon when none is left. Hookless so tests can call it and fire `onError` directly.
 *
 * `alt=""`: decoration — the commune's name stands right beside it in words (the header title), so a screen
 * reader reading "logo Xã …" before "Xã …" says the name twice. The box is the same 40×40 for picture and icon
 * (`.xa-hero__logo` / `.xa-hero__dai-dien`), so a swap or a failure never moves the title.
 */
export function CommuneLogoView(props: {
  sources: readonly string[];
  failed: readonly string[];
  onFail: (src: string) => void;
}) {
  const src = props.sources.find((s) => !props.failed.includes(s));
  if (src === undefined) {
    return (
      <span className="xa-hero__dai-dien" aria-hidden="true">
        <BieuTuong ten="build" co={22} />
      </span>
    );
  }
  // `key`: a new element per source, so a late `onError` of the previous picture cannot be read as this one's.
  return <img key={src} className="xa-hero__logo" src={src} alt="" decoding="async" onError={() => props.onFail(src)} />;
}

/**
 * LOGO XÃ ở ô trái header của bốn tab gốc: logo xã tự tải (đọc lúc chạy) → ảnh chép lúc dựng → biểu tượng tòa
 * nhà; ảnh tải hỏng thì xuống bước sau. Chuyển từ `TrangXa.tsx` (30/09/2026) để mọi header gốc dùng chung một ô.
 *
 * Failures are remembered BY URL, not as one flag: the profile arrives after the first paint, so the slot first
 * shows the bundled file and then the uploaded logo — a bundled file that already failed must stay skipped, and
 * an uploaded logo that fails falls to the bundled one, not straight to the icon.
 */
// vi-name-ok: existing component moved from TrangXa.tsx, not renamed (rule 12 #3)
export function LogoXa({ logoUrl = "" }: { logoUrl?: string }) {
  const [failed, setFailed] = useState<readonly string[]>([]);
  return <CommuneLogoView sources={logoSources(logoUrl)} failed={failed} onFail={(src) => setFailed((f) => markLogoFailed(f, src))} />;
}

/** `failed` with `src` added once. PURE — the state step `LogoXa` takes on an image error, exported for tests. */
export function markLogoFailed(failed: readonly string[], src: string): readonly string[] {
  return failed.includes(src) ? failed : [...failed, src];
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
 * Header of the four root tabs: the commune's logo on the left, the title (and an optional line under it —
 * the greeting on home) in the middle. `right`: no screen passes one any more (decision 10); the slot is
 * kept so the title stays centred between two equal sides. `logoUrl`: the uploaded logo from the profile
 * read (`""` or absent = none — see `LogoXa`).
 */
export function RootTabHeader(props: { title: string; subtitle?: string; right?: ReactNode; logoUrl?: string }) {
  return (
    <HeaderShell className="xa-dau-tab" left={<LogoXa logoUrl={props.logoUrl} />} right={props.right}>
      <h1 className="xa-dau-con__tieu-de">{props.title}</h1>
      {props.subtitle !== undefined && props.subtitle !== "" && <p className="xa-header__subtitle">{props.subtitle}</p>}
    </HeaderShell>
  );
}

/**
 * Header của màn con: nút quay lại (mũi tên + chữ "Quay lại") + tiêu đề ở giữa. Nút Back của Zalo đóng cả app,
 * nên đây là đường về.
 */
export function DauManCon({ tieu_de, onQuayLai }: { tieu_de: string; onQuayLai: () => void }) {
  return (
    <HeaderShell
      className="xa-dau-con"
      left={
        <button type="button" className="xa-dau-con__lui" onClick={onQuayLai}>
          <BieuTuong ten="chevron-left" co={24} stroke={2.4} />
          <span>{QUAY_LAI}</span>
        </button>
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
 * Khối trạng thái (`PROTOTYPE.md` §7 `ErrorState` / `EmptyState`), luôn là chữ đọc được, không chỉ biểu tượng:
 *   · đang tải — CHỮ, không vòng quay, không khối nhấp nháy (quyết định của chủ dự án);
 *   · lỗi     — vòng 80px đỏ nhạt + biểu tượng, câu nói việc cần làm, nút phụ (thường là "Thử lại");
 *   · trống   — vòng 80px xanh nhạt + biểu tượng, câu làm tiêu đề, `hint` (tuỳ chọn) nói bước tiếp theo.
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
}) {
  if (props.dang_tai && !props.loi) {
    return (
      <div className="xa-trang-thai xa-trang-thai--tai">
        <p role="status">{props.cau}</p>
      </div>
    );
  }
  return (
    <div className={`xa-trang-thai${props.loi ? " xa-trang-thai--loi" : ""}`}>
      <span className="xa-status-circle" aria-hidden="true">
        <BieuTuong ten={props.bieu_tuong} co={36} />
      </span>
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

/** The body of a child screen — the ONE part that scrolls; its header above and any footer below stay put. */
export function TrangCon({ children }: { children: ReactNode }) {
  return <div className="xa-trang xa-trang--con">{children}</div>;
}
