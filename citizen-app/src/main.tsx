import { StrictMode } from "react";
import { createRoot } from "react-dom/client";

import { App, COMMUNE_HEADER_TOP_COLOR } from "./App";
import { clearSharedActionBar } from "./features/tinh-nang/zalo-api";
import { thamSoMoApp, thamSoXa } from "./lib/launch-params";
import { XA_CO_DINH } from "./lib/xa-co-dinh";
import "./styles.css";

/**
 * Entry point of the Mini App.
 *
 * ONE PLATFORM CALL BEFORE THE FIRST RENDER, AND ONLY ON THE SHARED APP'S COMMUNE QR PATH (owner, 12/10/2026):
 * Zalo opens the shared App ID with "ViHAT Group" on its own top bar (`app-config.json` is static per App ID).
 * The same branch `App.tsx` `AppChung` takes — a commune's own build never, a trusted `?d=&src=qr|zns` link
 * always — clears that bar now instead of after the commune lookup (`clearSharedActionBar`). Fire and forget:
 * outside Zalo it answers `ngoai-zalo` and nothing changes; React does not wait for it.
 */
if (XA_CO_DINH === null && thamSoXa(thamSoMoApp()) !== null) {
  document.title = "";
  void clearSharedActionBar(COMMUNE_HEADER_TOP_COLOR);
}

const container = document.getElementById("app");
if (!container) {
  // Fail loudly at start-up rather than rendering nothing: a blank Mini App is indistinguishable
  // from a crashed one, and that is exactly what gets reported as "the app does not open".
  throw new Error("Root element #app is missing from index.html");
}

createRoot(container).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
