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
