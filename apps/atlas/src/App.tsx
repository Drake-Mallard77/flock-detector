import { Suspense, lazy } from "react";
import { NavLink, Route, Routes } from "react-router-dom";

import DeploymentDetailPage from "./pages/DeploymentDetailPage";
import DeploymentsPage from "./pages/DeploymentsPage";
import MethodologyPage from "./pages/MethodologyPage";
import NotFoundPage from "./pages/NotFoundPage";
import SubmitPage from "./pages/SubmitPage";

// MapLibre is ~1MB of the bundle on its own. Lazy-loading it keeps the
// text pages (deployments, methodology, submit) fast for people who never
// open the map.
const MapPage = lazy(() => import("./pages/MapPage"));

export default function App() {
  return (
    <div className="layout">
      <header className="site-header">
        <NavLink to="/" className="brand">
          FlockWatch
          <small>Public Records Atlas</small>
        </NavLink>
        <nav className="site-nav">
          <NavLink to="/" end>
            Map
          </NavLink>
          <NavLink to="/deployments">Deployments</NavLink>
          <NavLink to="/methodology">Methodology</NavLink>
          <NavLink to="/submit" className="cta">
            + Submit a sighting
          </NavLink>
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
          <Route path="/methodology" element={<MethodologyPage />} />
          <Route path="/submit" element={<SubmitPage />} />
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
          . See <NavLink to="/methodology">Methodology</NavLink>.
        </p>
      </footer>
    </div>
  );
}
