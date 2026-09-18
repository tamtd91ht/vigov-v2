import type { ComponentType } from "react";

import { CO_MAN_QUYEN, MAN_QUYEN } from "bien-the/quyen";

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
 */

export type ScreenId = "home" | "solutions" | "about" | "contact" | "quyen";

export type ScreenDefinition = {
  id: ScreenId;
  /** Shown in the bottom tab bar. Short enough not to truncate on a 320px screen. */
  tabLabel: string;
  /** Shown in the app header under the entity name, so the citizen always knows where they are. */
  headerTitle: string;
  component: ComponentType;
};

/** Bốn màn giới thiệu công ty — phần có mặt trong MỌI biến thể bản dựng. */
export const MAN_GIOI_THIEU: readonly ScreenDefinition[] = [
  { id: "home", tabLabel: "Trang chủ", headerTitle: "Trang chủ", component: HomeScreen },
  { id: "solutions", tabLabel: "Giải pháp", headerTitle: "Giải pháp", component: SolutionsScreen },
  { id: "about", tabLabel: "Về ViHAT", headerTitle: "Về ViHAT", component: AboutScreen },
  { id: "contact", tabLabel: "Liên hệ", headerTitle: "Liên hệ", component: ContactScreen },
];

/**
 * Sổ màn hình của BẢN DỰNG NÀY.
 *
 * Ba màn quyền vào app qua đúng một dòng: một tab nữa, lấy từ `bien-the/quyen`. Ở biến thể `goc`
 * cờ `CO_MAN_QUYEN` là `false` và tab ấy không tồn tại — không phải bị ẩn, mà là không có trong
 * danh sách, nên thanh tab, tiêu đề header và nhánh vẽ màn đều không biết tới nó.
 *
 * VÌ SAO KHÔNG VIẾT NHÃN TAB Ở ĐÂY: tệp này không nằm sau alias, nên mọi chữ trong nó đi vào cả
 * bản `goc`. Nhãn nằm trong `features/quyen/index.ts`, sau cửa alias — xem chú thích ở đó.
 */
export const SCREENS: readonly ScreenDefinition[] = CO_MAN_QUYEN
  ? [...MAN_GIOI_THIEU, MAN_QUYEN]
  : MAN_GIOI_THIEU;

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
