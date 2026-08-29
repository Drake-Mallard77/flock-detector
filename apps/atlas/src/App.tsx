import { Suspense, lazy, useEffect, useRef, useState } from "react";
import { NavLink, Route, Routes, useLocation } from "react-router-dom";

import ThemeToggle from "./components/ThemeToggle";
import { useTheme } from "./lib/theme";

import DeploymentDetailPage from "./pages/DeploymentDetailPage";
import DeploymentsPage from "./pages/DeploymentsPage";
import ChangesPage from "./pages/ChangesPage";
import CoveragePage from "./pages/CoveragePage";
import MethodologyPage from "./pages/MethodologyPage";
import NotFoundPage from "./pages/NotFoundPage";
import StatePage from "./pages/StatePage";
import StatesPage from "./pages/StatesPage";
import ReviewDeskPage from "./pages/ReviewDeskPage";
import SubmitPage from "./pages/SubmitPage";

// MapLibre is ~1MB of the bundle on its own. Lazy-loading it keeps the
// text pages (deployments, methodology, submit) fast for people who never
// open the map.
const MapPage = lazy(() => import("./pages/MapPage"));

export default function App() {
  const { preference, setPreference } = useTheme();

  // Collapsed navigation, phones only.
  //
  // The header was two rows and 144px on a 390px screen — a sixth of the
  // viewport spent on chrome before any content, and on the map page that
  // is 144px of map. CSS hides this button above the breakpoint, so the
  // desktop header keeps every link visible: hiding navigation behind a
  // click costs discoverability, and only earns it back where space is
  // genuinely short.
  const [menuOpen, setMenuOpen] = useState(false);
  const navRef = useRef<HTMLElement>(null);
  const toggleRef = useRef<HTMLButtonElement>(null);
  const { pathname } = useLocation();

  // Navigating closes the menu. Without this it stays open over the page
  // you just asked for, which reads as the link having failed.
  useEffect(() => setMenuOpen(false), [pathname]);

  useEffect(() => {
    if (!menuOpen) return;

    function onKey(e: KeyboardEvent) {
      // Escape is what people try first when a panel traps them, and
      // returning focus to the button keeps keyboard users where they were.
      if (e.key === "Escape") {
        setMenuOpen(false);
        toggleRef.current?.focus();
      }
    }
    function onPointer(e: PointerEvent) {
      const t = e.target as Node;
      if (!navRef.current?.contains(t) && !toggleRef.current?.contains(t)) {
        setMenuOpen(false);
      }
    }

    document.addEventListener("keydown", onKey);
    document.addEventListener("pointerdown", onPointer);
    return () => {
      document.removeEventListener("keydown", onKey);
      document.removeEventListener("pointerdown", onPointer);
    };
  }, [menuOpen]);

  return (
    <div className="layout">
      <header className="site-header">
        <NavLink to="/" className="brand">
          FlockWatch
          <small>Public Records Atlas</small>
        </NavLink>
        <button
          ref={toggleRef}
          type="button"
          className="nav-toggle"
          aria-expanded={menuOpen}
          aria-controls="site-nav"
          aria-label={menuOpen ? "Close menu" : "Open menu"}
          onClick={() => setMenuOpen((v) => !v)}
        >
          {/* Three bars drawn as spans rather than an icon font or an SVG
              sprite: it is three rectangles, and a dependency for that
              would be silly. aria-hidden because the button already has a
              label — without it a screen reader announces nothing useful. */}
          <span aria-hidden="true" />
          <span aria-hidden="true" />
          <span aria-hidden="true" />
        </button>

        <nav
          id="site-nav"
          ref={navRef}
          className={menuOpen ? "site-nav is-open" : "site-nav"}
        >
          <NavLink to="/" end>
            Map
          </NavLink>
          <NavLink to="/deployments">Deployments</NavLink>
          <NavLink to="/states">States</NavLink>
          {/* Methodology is intentionally not in the top nav — it's
              reference material, not a primary destination, and it was
              crowding Map/Deployments. Still linked from the footer and
              from the record pages that depend on it. */}
          <NavLink to="/review">Review desk</NavLink>
          <NavLink to="/submit" className="cta">
            + Submit a sighting
          </NavLink>
          <ThemeToggle preference={preference} onChange={setPreference} />
        </nav>
      </header>

      <main>
        <Routes>
          <Route
            path="/"
            element={
              <Suspense fallback={<div className="page state">Loading map…</div>}>
                <MapPage />
              </Suspense>
            }
          />
          <Route path="/deployments" element={<DeploymentsPage />} />
          <Route path="/deployments/:id" element={<DeploymentDetailPage />} />
          {/* Readable form. The UUID route above stays permanently — it is
              already published and indexed — and redirects here. */}
          <Route path="/state/:code/:slug" element={<DeploymentDetailPage />} />
          <Route path="/states" element={<StatesPage />} />
          <Route path="/state/:code" element={<StatePage />} />
          <Route path="/changes" element={<ChangesPage />} />
          <Route path="/coverage" element={<CoveragePage />} />
          <Route path="/methodology" element={<MethodologyPage />} />
          <Route path="/submit" element={<SubmitPage />} />
          {/* Not route-guarded: the page renders sign-in or an access
              notice itself, and the API rejects unauthorized actions
              regardless. Hiding the route would only obscure it. */}
          <Route path="/review" element={<ReviewDeskPage />} />
          <Route path="*" element={<NotFoundPage />} />
        </Routes>
      </main>

      <footer className="site-footer">
        <p style={{ margin: 0 }}>
          An independent public-records project. Every published record links to its sources.
        </p>
        {/* ODbL requires attribution wherever OSM-derived data is shown. This
            is a licensing obligation, not a courtesy — see docs/ARCHITECTURE.md. */}
        <p style={{ margin: "0.35rem 0 0" }}>
          Camera locations include data from{" "}
          <a href="https://www.openstreetmap.org/copyright" target="_blank" rel="noopener noreferrer">
            © OpenStreetMap contributors
          </a>
          , available under the{" "}
          <a href="https://opendatacommons.org/licenses/odbl/" target="_blank" rel="noopener noreferrer">
            Open Database License (ODbL)
          </a>
          . See <NavLink to="/methodology">Methodology</NavLink> and{" "}
          <NavLink to="/coverage">Coverage</NavLink>, and{" "}
          <NavLink to="/changes">what has changed recently</NavLink>.
        </p>
      </footer>
    </div>
  );
}
