/**
 * Thanh tab của màn Cấu hình (`docs/ui-ux/14-cau-hinh.md §0`) — phần quyết định, tách khỏi phần dựng.
 *
 * QUYẾT ĐỊNH CỦA NGƯỜI DÙNG (26/09): tài khoản mở được TỪ HAI tab trở lên thì hiện thanh tab; chỉ mở
 * được MỘT tab thì KHÔNG hiện thanh, nội dung tab ấy hiện thẳng. Một nút tab đứng một mình không
 * chọn được gì, chỉ chiếm chỗ.
 *
 * Also drives the tab bar of `/he-thong` (`SYSTEM_TABS`, owner 12/10/2026): same two decision functions.
 *
 * TAB NÀO HIỆN ĐI ĐÚNG THEO CỔNG ĐÃ CÓ, không theo một luật mới đặt ở đây. Các tab có cổng —
 * Trường bản đồ (`asset.read`, khoá tuyến ĐỌC
 * `GET /map-field-schemas`), Lời hệ thống (`admin.message`), Máy chủ thư (`admin.mail`) và
 * Kênh Zalo (`admin.zalo`) — mỗi khoá khoá mọi tuyến của tab ấy, cả tuyến đọc (chủ dự án
 * 15/10/2026, trước đó cả ba đi theo `admin.lookup`), Tự động hoá (`admin.sla`, khoá cả ba tuyến `automation-jobs`) và Nhật ký
 * hệ thống (`admin.audit`, khoá năm tuyến `*-audit-entries`), Nhận diện xã (`admin.org`), Thời hạn
 * xử lý (`admin.sla`, khoá của `GET /sla` — ADR 0079 D1) — gọi đúng hàm của `quyen-tab.ts` mà chính
 * tab ấy gọi, để chỉ có MỘT nguồn quyết định. Bốn tab kia (Sơ đồ tổ chức, Thôn / Tổ dân phố, Danh
 * mục, Lịch làm việc) KHÔNG có cổng (`cong: null`) vì tuyến đọc của chúng mở cho mọi tài khoản đã
 * đăng nhập (Lịch làm việc nay ở `/he-thong`); khoá ghi của Thôn / Tổ dân phố (`admin.village`)
 * chỉ quyết nút ghi trong tab, không ẩn tab. Thêm cổng cho bốn tab này là để GIAO DIỆN từ chối điều máy chủ không từ chối — luật 5,
 * cấm #1.
 *
 * Đây vẫn là TIỆN DỤNG, không phải biện pháp: máy chủ kiểm quyền trên từng yêu cầu.
 */

import type { KetQua } from "@/lib/api/goi";
import type { identity_phienHienTaiRa } from "@/lib/api/schema.gen";

import {
  auditLogTabDecision,
  automationTabDecision,
  brandingTabDecision,
  mailServerTabDecision,
  mapFieldTabDecision,
  slaTabDecision,
  systemMessagesTabDecision,
  zaloChannelTabDecision,
  type QuyetDinhTab,
} from "./quyen-tab";

export type MaTabCauHinh =
  | "so-do-to-chuc"
  | "thon-to-dan-pho"
  | "danh-muc"
  | "truong-ban-do"
  | "loi-he-thong"
  | "thoi-han-xu-ly"
  | "tu-dong-hoa"
  | "may-chu-thu"
  | "kenh-zalo";

/** Tabs of `/he-thong` — the three that left `/cau-hinh` on 12/10/2026 (see `SYSTEM_TABS`). */
export type SystemTabId = "lich-lam-viec" | "nhat-ky-he-thong" | "nhan-dien-xa";

/** `null` là chưa đọc xong phiên — cùng ba trạng thái với `PhienDaDoc` của `PhienProvider`. */
export type PhienDoc = KetQua<identity_phienHienTaiRa> | null;

export type MoTaTab<M extends string = MaTabCauHinh> = {
  readonly ma: M;
  readonly nhan: string;
  /** `null` = tab không có cổng ẩn cả tab (tuyến đọc `any-authenticated`). */
  readonly cong: ((phien: KetQua<identity_phienHienTaiRa>) => QuyetDinhTab) | null;
};

/**
 * Nine tabs — the prototype's. Đúng thứ tự và đúng nhãn của §0, trừ Người dùng và Phân quyền: hai tab ấy
 * rời màn này ngày 05/10/2026 sang `/nguoi-dung` và `/nguoi-dung/phan-quyen`, theo bản mẫu chủ dự án
 * duyệt (`moved-tabs.ts`; §0 của `docs/ui-ux/14` chưa ghi việc này). Tự động hoá có cổng `admin.sla`
 * vì cả ba tuyến của nó — kể cả tuyến đọc — khai khoá ấy (`automationTabDecision`).
 *
 * "Kênh Zalo" (ADR 0074 #6, 05/10/2026) is not in §0; it sits right after "Máy chủ thư" — the other
 * outbound channel of the commune. Since 15/10/2026 each of the two has its own key
 * (`admin.mail`, `admin.zalo`) instead of sharing `admin.lookup`.
 *
 * OWNER DECISION 08/10/2026 (ADR 0079 D1): "Thời hạn xử lý" is gated on `admin.sla` like the prototype,
 * and its three calendar tables moved to a tab of their own, "Lịch làm việc".
 *
 * USER DECISION 09/10/2026 put the prototype's nine tabs first, in its order, and the three it lacks —
 * "Lịch làm việc", "Nhật ký hệ thống", "Nhận diện xã" — after them on this bar.
 *
 * OWNER DECISION 12/10/2026 (customer bug sheet S27, reopened "Chưa bỏ 3 tab dư") REPLACES that: this
 * bar shows EXACTLY the prototype's nine (`vigov-require/apps/admin/src/components/admin/
 * ConfigWorkspace.tsx`). The three extra tabs are NOT deleted — they are the only screens for the working
 * calendar and holidays (deadline input, ADR 0007), the audit trail reader (ADR 0054) and the commune's
 * logo and banner (ADR 0069). They moved, gates unchanged, to a menu item of their own, "Hệ thống"
 * (`/he-thong`, `SYSTEM_TABS` below). An old `/cau-hinh?tab=<one of them>` link redirects there
 * (`moved-tabs.ts`).
 */
export const TAB_CAU_HINH: readonly MoTaTab<MaTabCauHinh>[] = [
  { ma: "so-do-to-chuc", nhan: "Sơ đồ tổ chức", cong: null },
  { ma: "thon-to-dan-pho", nhan: "Thôn / Tổ dân phố", cong: null },
  { ma: "danh-muc", nhan: "Danh mục", cong: null },
  { ma: "truong-ban-do", nhan: "Trường bản đồ", cong: mapFieldTabDecision },
  { ma: "loi-he-thong", nhan: "Lời hệ thống", cong: systemMessagesTabDecision },
  { ma: "thoi-han-xu-ly", nhan: "Thời hạn xử lý", cong: slaTabDecision },
  { ma: "tu-dong-hoa", nhan: "Tự động hoá", cong: automationTabDecision },
  { ma: "may-chu-thu", nhan: "Máy chủ thư", cong: mailServerTabDecision },
  { ma: "kenh-zalo", nhan: "Kênh Zalo", cong: zaloChannelTabDecision },
];

/**
 * The tabs of `/he-thong` (owner, 12/10/2026): the three `/cau-hinh` tabs the prototype lacks, in the
 * order they had there. Each keeps the gate it had — the move changes WHERE a tab is, never WHO sees it:
 *
 *   | Tab              | Gate                | Why                                                          |
 *   |------------------|---------------------|--------------------------------------------------------------|
 *   | Lịch làm việc    | none (`cong: null`) | the three calendar reads are any-authenticated; its write    |
 *   |                  |                     | buttons still need `admin.sla` (`quyetDinhGhiThoiHan`)        |
 *   | Nhật ký hệ thống | `admin.audit`       | the key of all five `*-audit-entries` reads (ADR 0054 §1)    |
 *   | Nhận diện xã     | `admin.org`         | the key of all seven `/commune-branding` routes (ADR 0069 #2) |
 *
 * The ids are the ones these tabs had on `/cau-hinh`, so `moved-tabs.ts` forwards `?tab=<id>` unchanged.
 */
export const SYSTEM_TABS: readonly MoTaTab<SystemTabId>[] = [
  { ma: "lich-lam-viec", nhan: "Lịch làm việc", cong: null },
  { ma: "nhat-ky-he-thong", nhan: "Nhật ký hệ thống", cong: auditLogTabDecision },
  { ma: "nhan-dien-xa", nhan: "Nhận diện xã", cong: brandingTabDecision },
];

/**
 * The `/he-thong` tab a `?tab=` value names, or `null`. Matched against `SYSTEM_TABS` by exact string,
 * never looked up on an object (`?tab=constructor` names nothing); a repeated `?tab=` is ambiguous and
 * names nothing. It only picks the starting tab: the gate still decides whether that tab shows.
 */
export function systemTabFromQuery(tab: string | string[] | undefined): SystemTabId | null {
  if (typeof tab !== "string") return null;
  return SYSTEM_TABS.find((t) => t.ma === tab)?.ma ?? null;
}

/**
 * Các tab được hiện, giữ nguyên thứ tự của `ds`.
 *
 * FAIL CLOSED với tab có cổng: chưa đọc xong phiên (`null`) hoặc đọc hỏng thì tab ấy KHÔNG hiện —
 * "chưa rõ" không được hành xử như "có". Tab không cổng thì hiện ở mọi trạng thái, vì chúng không
 * phụ thuộc phiên.
 */
export function cacTabHien<M extends string>(
  ds: readonly MoTaTab<M>[],
  phien: PhienDoc,
): readonly MoTaTab<M>[] {
  return ds.filter((t) => t.cong === null || (phien !== null && t.cong(phien).hien));
}

/**
 * Có dựng thanh tab hay không.
 *
 * CHƯA ĐỌC XONG PHIÊN THÌ CHƯA DỰNG THANH: lúc ấy chưa biết tài khoản mở được mấy tab, và dựng thanh
 * rồi gỡ đi (hoặc dựng bốn nút rồi mọc thêm hai) là màn hình nhảy dưới tay cán bộ. Trong lúc chờ, tab
 * đầu tiên hiện thẳng; đọc xong thì thanh chỉ có thể XUẤT HIỆN, không bao giờ biến mất.
 */
export function coThanhTab(phien: PhienDoc, soTabHien: number): boolean {
  return phien !== null && soTabHien >= 2;
}

/**
 * Chỉ số tab kế tiếp theo phím, theo mẫu tabs của WAI-ARIA: ← / → đi vòng, Home về đầu, End về
 * cuối. Phím khác trả `null` để trình duyệt xử lý như thường (Tab vẫn rời khỏi thanh).
 */
export function tabKeTheoPhim(phim: string, hienTai: number, soTab: number): number | null {
  if (soTab <= 0) return null;
  switch (phim) {
    case "ArrowRight":
      return (hienTai + 1) % soTab;
    case "ArrowLeft":
      return (hienTai - 1 + soTab) % soTab;
    case "Home":
      return 0;
    case "End":
      return soTab - 1;
    default:
      return null;
  }
}
