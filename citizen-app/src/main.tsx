import { StrictMode } from "react";
import { createRoot } from "react-dom/client";

import { App } from "./App";
import "./styles.css";

/**
 * Entry point of the Mini App.
 *
 * Nothing here talks to `zmp-sdk`. Phase 1 asks the platform for nothing — no phone number, no
 * location, no user profile — so there is no SDK call to make and no permission for a reviewer
 * to weigh. `zmp-sdk` stays in package.json because phase 2 needs it.
 */
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
