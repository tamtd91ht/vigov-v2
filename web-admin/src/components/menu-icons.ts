import {
  BellRing,
  BookOpen,
  ChartColumn,
  Circle,
  ClipboardCheck,
  ClipboardList,
  Contact,
  FileText,
  LayoutDashboard,
  Map as MapIcon,
  MessageSquareWarning,
  NotebookPen,
  Send,
  Settings,
  ShieldCheck,
  Smartphone,
  UserRound,
  UsersRound,
  Wallet,
  type LucideIcon,
} from "lucide-react";
import { createElement } from "react";

/**
 * Icon of each menu item — the prototype's (`vigov-require` `apps/admin/src/lib/navigation.ts`, ADR 0068
 * lần 5). "Báo cáo" is its `BarChart3` under the current lucide name `ChartColumn`. "Danh bạ cán bộ" is
 * a tab of "Nội dung Mini App" since 06/10/2026, as in the prototype, so it has no row and no icon here.
 *
 * KEYED BY THE ITEM'S LABEL (`nhan`), not by its route: placeholder items and parent rows have no route,
 * and the label is the one field every row has. The menu itself — which items, their order, their
 * permission keys — stays in `muc-menu.ts`; this table only decorates it, so nothing about who sees what
 * can change through here.
 *
 * `side-nav.test.tsx` fails the day a row is added to `NHOM_MENU` without a line here, instead of letting
 * it fall back silently to the generic circle below.
 */
export const MENU_ICONS: Readonly<Record<string, LucideIcon>> = {
  "Tổng quan": LayoutDashboard,
  "Nhiệm vụ": ClipboardList,
  "Sổ tay lãnh đạo": NotebookPen,
  "Biên bản họp": ClipboardCheck,
  "Văn bản & Đơn thư": FileText,
  "Giải ngân": Wallet,
  "Thu - Chi ngân sách": Wallet,
  "Thông báo nội bộ": BellRing,
  "Danh bạ người dân": Contact,
  "Gửi tin ZNS / SMS": Send,
  "Phản ánh người dân": MessageSquareWarning,
  "Bản đồ kinh tế số": MapIcon,
  "Nội dung Mini App": Smartphone,
  "Báo cáo": ChartColumn,
  "Người dùng & Phân quyền": UsersRound,
  "Người dùng": UserRound,
  "Phân quyền": ShieldCheck,
  "Hướng dẫn sử dụng": BookOpen,
  "Cấu hình": Settings,
};

export function menuIcon(label: string): LucideIcon {
  return MENU_ICONS[label] ?? Circle;
}

/**
 * The row's icon, decorative (the row's word names it). `createElement` on the looked-up component, not
 * `const Icon = menuIcon(…); <Icon/>` inside a component — the latter reads to React's lint as a component
 * created during render.
 */
export function MenuIcon({ label }: { label: string }) {
  return createElement(menuIcon(label), { "aria-hidden": true, focusable: "false", strokeWidth: 1.8 });
}
