import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter } from "react-router-dom";

import App from "./App";
import { AuthProvider } from "./lib/auth";
// MapLibre's stylesheet is imported BEFORE ours on purpose. It styles the
// map container via `.maplibregl-map`, which it adds to whichever element
// we hand it — the same specificity as our own `.map-root`. Whichever loads
// last wins, so importing it after ours let it override our positioning and
// collapse the map to zero height.
import "maplibre-gl/dist/maplibre-gl.css";
import "./styles.css";

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <BrowserRouter>
      <AuthProvider>
        <App />
      </AuthProvider>
    </BrowserRouter>
  </StrictMode>,
);
