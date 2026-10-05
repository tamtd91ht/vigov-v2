import {
  BookOpen,
  Banknote,
  ChartColumn,
  Circle,
  ContactRound,
  LayoutDashboard,
  ListChecks,
  Mail,
  Map as MapIcon,
  Megaphone,
  MessageSquareWarning,
  NotebookPen,
  Settings,
  ShieldCheck,
  Smartphone,
  UserRound,
  Wallet,
  type LucideIcon,
} from "lucide-react";

/**
 * Icon of each menu item — spec §4 "Bảng ánh xạ icon".
 *
 * KEYED BY THE ITEM'S LABEL (`MucMenu.nhan`), not by its route: four items have no route yet
 * (`duong: null`), and the label is the one field every item has. The menu itself — which items,
 * their order, their permission keys — stays in `muc-menu.ts`; this table only decorates it, so
 * nothing about who sees what can change through here.
 *
 * `side-nav.test.tsx` fails the day an item is added to `NHOM_MENU` without a line here,
 * instead of letting it fall back silently to the generic circle below.
 */
export const MENU_ICONS: Readonly<Record<string, LucideIcon>> = {
  "Tổng quan": LayoutDashboard,
  "Nhiệm vụ": ListChecks,
  "Sổ tay lãnh đạo": BookOpen,
  "Biên bản họp": NotebookPen,
  "Văn bản & Đơn thư": Mail,
  "Giải ngân": Banknote,
  "Thu - Chi ngân sách": Wallet,
  "Thông báo": Megaphone,
  "Phản ánh người dân": MessageSquareWarning,
  "Bản đồ kinh tế số": MapIcon,
  "Nội dung Mini App": Smartphone,
  "Danh bạ cán bộ": ContactRound,
  "Báo cáo": ChartColumn,
  // The prototype's child-item icons (`vigov-require/apps/admin/src/lib/navigation.ts`).
  "Người dùng": UserRound,
  "Phân quyền": ShieldCheck,
  "Cấu hình": Settings,
};

export function menuIcon(label: string): LucideIcon {
  return MENU_ICONS[label] ?? Circle;
}
