import { useEffect, useRef } from "react";

const CLIENT_ID = import.meta.env.VITE_GOOGLE_CLIENT_ID ?? "";

// Minimal shape of the Google Identity Services global we actually use.
declare global {
  interface Window {
    google?: {
      accounts: {
        id: {
          initialize: (config: {
            client_id: string;
            callback: (response: { credential: string }) => void;
          }) => void;
          renderButton: (parent: HTMLElement, options: Record<string, unknown>) => void;
        };
      };
    };
  }
}

/**
 * Renders Google's own sign-in button.
 *
 * The script is loaded on demand here rather than in index.html so visitors
 * who never sign in don't get a third-party script — this is a surveillance
 * transparency site, and quietly loading Google on every page view would be
 * at odds with that.
 */
export default function GoogleSignIn({
  onCredential,
}: {
  onCredential: (credential: string) => void;
}) {
  const target = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!CLIENT_ID || !target.current) return;

    function render() {
      if (!window.google || !target.current) return;
      window.google.accounts.id.initialize({
        client_id: CLIENT_ID,
        callback: (response) => onCredential(response.credential),
      });
      window.google.accounts.id.renderButton(target.current, {
        theme: "outline",
        size: "large",
        text: "signin_with",
      });
    }

    if (window.google) {
      render();
      return;
    }

    const script = document.createElement("script");
    script.src = "https://accounts.google.com/gsi/client";
    script.async = true;
    script.onload = render;
    document.head.appendChild(script);
  }, [onCredential]);

  if (!CLIENT_ID) {
    return (
      <div className="notice error">
        Google sign-in isn't configured for this build (<code>VITE_GOOGLE_CLIENT_ID</code> is
        unset).
      </div>
    );
  }

  return <div ref={target} />;
}
