import { Link } from "react-router-dom";

export default function NotFoundPage() {
  return (
    <div className="page">
      <h1>Page not found</h1>
      <p className="lede">
        That page doesn't exist. Try the <Link to="/">map</Link> or browse{" "}
        <Link to="/deployments">deployments</Link>.
      </p>
    </div>
  );
}
