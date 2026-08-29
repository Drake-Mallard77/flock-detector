import { Link } from "react-router-dom";

import { API_BASE } from "../lib/api";

export default function MethodologyPage() {
  return (
    <div className="page">
      <h1>Methodology</h1>
      <p className="lede">
        What this atlas publishes, where the data comes from, and what it deliberately does not
        claim.
      </p>

      <div className="prose">
        <h2>Two different kinds of data</h2>
        <p>
          <strong>Deployment records</strong> are agency-level: a named agency, a city, and what
          public records show about its use of ALPR cameras. These are shown at{" "}
          <strong>city-level precision only</strong> and every published record links to the
          documents it rests on.
        </p>
        <p>
          <strong>Camera locations</strong> on the map are a separate layer, drawn from
          OpenStreetMap and from sightings submitted here. A pin means someone documented a
          camera at that spot — it is not a claim about which agency operates it.
        </p>

        <h2>Review status</h2>
        <ul>
          <li>
            <strong>Confirmed</strong> — a moderator verified the record against its sources.
          </li>
          <li>
            <strong>Contract found</strong> — a contract or agreement exists, but details such
            as camera counts may still be unverified.
          </li>
          <li>
            <strong>OSM documented</strong> — mapped by OpenStreetMap contributors and
            attributed to this operator, but <em>not</em> checked against public records. It
            means someone recorded these cameras in the field, not that an agency's paperwork
            confirms them. Most records on this site currently carry this status.
          </li>
          <li>
            <strong>Under review</strong> — submitted, not yet verified. Not treated as
            established fact.
          </li>
          <li>
            <strong>Disputed</strong> — someone contested the record and it is being re-checked.
          </li>
        </ul>

        <h2>Why "OSM documented" is its own status</h2>
        <p>
          Camera locations come from OpenStreetMap, and where contributors recorded an{" "}
          <code>operator</code> tag, this atlas groups those cameras into an agency-level
          record. That grouping is mechanical: it reflects what volunteers mapped, not a
          document anyone filed.
        </p>
        <p>
          Calling those records "confirmed" would claim exactly the verification this project
          exists to do. So they publish under their own status instead, and become{" "}
          <strong>Confirmed</strong> only once a moderator has checked them against a council
          report, contract, invoice, or FOIA response.
        </p>

        <h2>What we don't know, we mark</h2>
        <p>
          Where records don't establish a number, the atlas shows{" "}
          <span className="unknown">?</span> rather than an estimate. An absent record is not
          evidence of absence, and a published record is not a claim that the deployment is
          current — check the "last reviewed" date on each record.
        </p>

        <h2>Data sources and licensing</h2>
        <p>
          Camera location data includes contributions from{" "}
          <a href="https://www.openstreetmap.org/copyright" target="_blank" rel="noopener noreferrer">
            OpenStreetMap
          </a>
          , specifically nodes tagged as ALPR surveillance cameras manufactured by Flock Safety.
          That data is © OpenStreetMap contributors and is made available under the{" "}
          <a href="https://opendatacommons.org/licenses/odbl/" target="_blank" rel="noopener noreferrer">
            Open Database License (ODbL)
          </a>
          . Any redistribution of the underlying database must carry the same attribution and
          license terms.
        </p>
        <p>
          Map tiles are served by OpenStreetMap and are also © OpenStreetMap contributors.
        </p>

        {/* Placed directly under the licensing section on purpose: CSV has
            no comment convention every parser tolerates, so the ODbL terms
            can't travel inside the file the way they do in the GeoJSON.
            Stating them where the download link lives is the honest
            substitute. */}
        <h2>Download the data</h2>
        <p>
          The published dataset is available as a file. Analysing it shouldn't
          require scraping the map.
        </p>
        <ul className="downloads">
          <li>
            <a href={`${API_BASE}/export/deployments.csv`}>
              Agency records (CSV)
            </a>
            <span>
              Every published record — agency, location, documented units,
              evidence type, and a link back to the record page. Excludes
              candidates still under review.
            </span>
          </li>
          <li>
            <a href={`${API_BASE}/export/cameras.csv`}>Camera locations (CSV)</a>
            <span>
              All mapped camera locations, with direction, manufacturer and
              operator where OpenStreetMap records them.
            </span>
          </li>
          <li>
            <a href={`${API_BASE}/export/cameras.geojson`}>
              Camera locations (GeoJSON)
            </a>
            <span>
              The same points, ready to open in QGIS or any mapping tool.
              Carries its ODbL attribution inside the file.
            </span>
          </li>
        </ul>
        <p>
          These files are the database, not a rendering of it, so the ODbL
          share-alike terms above apply to anything derived from them: keep
          the attribution, and license any redistributed database the same
          way.
        </p>

        <h2>Corrections</h2>
        <p>
          If a record is wrong, out of date, or missing context, please{" "}
          <Link to="/submit">submit the documentation</Link> that shows it. Corrections backed by
          sources are handled the same way as new records.
        </p>
      </div>
    </div>
  );
}
