import { useState } from "react";

import { TabBar } from "./components/TabBar";
import { COMPANY } from "./content/company-profile";
import { DEFAULT_SCREEN_ID, findScreen, type ScreenId } from "./features/company-intro/screens";

/**
 * Phase 1 shell: four static screens, no navigation library, no state beyond the current tab.
 *
 * WHY THE HEADER ALWAYS NAMES THE OWNER:
 *
 *   In phase 2 this header is where the COMMUNE name lives, on every screen, because a citizen
 *   who cannot see which commune they are addressing can submit to the wrong one (README
 *   §Non-negotiables #2). Phase 1 has no commune, so the slot holds the entity that published
 *   the app. The slot exists from the first screen so phase 2 fills it rather than inventing it.
 *
 *   It has already been filled once, for real: the discovery layer built at commit `25591e8`
 *   put a confirmed commune name in this exact slot, on every screen. That work was removed
 *   from the submitted bundle on purpose, not abandoned — README §"Giai đoạn 2 bắt đầu từ đâu".
 *
 * WHY THIS FILE READS NO LAUNCH PARAMETER ANY MORE:
 *
 *   Reading `t` / `src` means importing `zmp-sdk`, and that single import costs 256 kB raw /
 *   64 kB gzip — it doubles the bundle. Nothing in the introduction app needs a parameter, so
 *   the app that gets submitted carries none of it. Phase 2 brings the SDK back for
 *   `getPhoneNumber` (ADR 0020), and the deep-link reading comes back with it.
 *
 * WHY NOT A ROUTER:
 *
 *   Four sibling screens with no deep-linkable state do not need history. When phase 2 brings
 *   deep links (`https://zalo.me/s/<APP_ID>/?t=...`), routing becomes a real requirement and
 *   gets decided then, with that requirement in hand.
 */
export function App() {
  const [currentId, setCurrentId] = useState<ScreenId>(DEFAULT_SCREEN_ID);
  const screen = findScreen(currentId);
  const Screen = screen.component;

  return (
    <div className="app">
      <header className="app-header">
        <p className="app-header__owner">{COMPANY.name}</p>
        <p className="app-header__screen">{screen.headerTitle}</p>
      </header>

      <main className="app-main" id="main">
        <Screen />
      </main>

      <TabBar current={currentId} onSelect={setCurrentId} />
    </div>
  );
}
