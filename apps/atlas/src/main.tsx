import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter } from "react-router-dom";

import App from "./App";
import { ThemeProvider } from "./lib/theme";
import { AuthProvider } from "./lib/auth";
// Vendor stylesheets are imported BEFORE ours on purpose. Leaflet styles
// the map container via its own class on the element we hand it, at the
// same specificity as our `.map-root`; whichever loads last wins. Importing
// vendor CSS after ours previously let it override our positioning and
// collapse the map to zero height, which is a hard bug to see.
import "leaflet/dist/leaflet.css";
import "leaflet.markercluster/dist/MarkerCluster.css";
import "./styles.css";

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <BrowserRouter>
      <ThemeProvider>
        <AuthProvider>
          <App />
        </AuthProvider>
      </ThemeProvider>
    </BrowserRouter>
  </StrictMode>,
);
