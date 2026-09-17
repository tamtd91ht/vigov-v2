import type { ComponentType } from "react";

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

export type ScreenId = "home" | "solutions" | "about" | "contact";

export type ScreenDefinition = {
  id: ScreenId;
  /** Shown in the bottom tab bar. Short enough not to truncate on a 320px screen. */
  tabLabel: string;
  /** Shown in the app header under the entity name, so the citizen always knows where they are. */
  headerTitle: string;
  component: ComponentType;
};

export const SCREENS: readonly ScreenDefinition[] = [
  { id: "home", tabLabel: "Trang chủ", headerTitle: "Trang chủ", component: HomeScreen },
  { id: "solutions", tabLabel: "Giải pháp", headerTitle: "Giải pháp", component: SolutionsScreen },
  { id: "about", tabLabel: "Về ViHAT", headerTitle: "Về ViHAT", component: AboutScreen },
  { id: "contact", tabLabel: "Liên hệ", headerTitle: "Liên hệ", component: ContactScreen },
];

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
