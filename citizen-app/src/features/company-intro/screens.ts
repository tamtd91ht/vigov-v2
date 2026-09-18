import type { ComponentType } from "react";

import { MAN_DANH_THIEP } from "../tinh-nang/index";

import { AboutScreen } from "./AboutScreen";
import { ContactScreen } from "./ContactScreen";
import { HomeScreen } from "./HomeScreen";
import { SolutionsScreen } from "./SolutionsScreen";

/**
 * The screen registry — the ONE place a screen is declared.
 *
 * Adding a screen means adding a row here; the tab bar, the header title and the routing
 * switch all read from this list. A second list would drift from this one, and the symptom of
 * that drift is a tab that leads nowhere.
 *
 * NĂM TAB, MỖI TAB MỘT VIỆC. Tab "Danh thiếp" là một TÍNH NĂNG, không phải một màn giới thiệu:
 * nó nằm giữa thanh tab vì đó là thứ người dùng mở app để làm, và vì người duyệt của Zalo phải
 * thấy ngay `scanQRCode` được dùng vào việc gì. Hai tính năng còn lại — tìm văn phòng và đăng ký
 * nhận tư vấn — nằm trên tab Liên hệ, đúng chỗ người ta đã định liên hệ.
 */

export type ScreenId = "home" | "solutions" | "danh-thiep" | "about" | "contact";

export type ScreenDefinition = {
  id: ScreenId;
  /** Shown in the bottom tab bar. Short enough not to truncate on a 320px screen. */
  tabLabel: string;
  /** Shown in the app header under the entity name, so the citizen always knows where they are. */
  headerTitle: string;
  component: ComponentType;
};

/** Sổ màn hình — giống nhau ở mọi biến thể bản dựng. */
export const SCREENS: readonly ScreenDefinition[] = [
  { id: "home", tabLabel: "Trang chủ", headerTitle: "Trang chủ", component: HomeScreen },
  { id: "solutions", tabLabel: "Giải pháp", headerTitle: "Giải pháp", component: SolutionsScreen },
  MAN_DANH_THIEP,
  { id: "about", tabLabel: "Về ViHAT", headerTitle: "Về ViHAT", component: AboutScreen },
  { id: "contact", tabLabel: "Liên hệ", headerTitle: "Liên hệ", component: ContactScreen },
];

/**
 * Giữ lại cái tên cũ cho những chỗ chỉ quan tâm tới bốn màn giới thiệu công ty.
 *
 * `bundle-for-zalo.test.ts` dùng nó để khẳng định bản `goc` vẫn là một app có nội dung, chứ
 * không phải một bản rỗng — và phép kiểm ấy không nên đỏ chỉ vì thứ tự tab đổi.
 */
export const MAN_GIOI_THIEU: readonly ScreenDefinition[] = SCREENS.filter(
  (man) => man.id !== "danh-thiep",
);

export const DEFAULT_SCREEN_ID: ScreenId = "home";

export function findScreen(id: ScreenId): ScreenDefinition {
  const screen = SCREENS.find((candidate) => candidate.id === id);
  // Fail closed rather than render a blank page: an unknown id is a programming error, and a
  // silently empty screen is the kind of defect that reaches a review submission.
  if (!screen) {
    throw new Error(`Unknown screen id: ${id}`);
  }
  return screen;
}
