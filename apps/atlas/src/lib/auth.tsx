import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";

import { getMe, googleLogin, setAuthToken, type Me } from "./api";

/**
 * Session state for the Review Desk.
 *
 * The token is kept in localStorage so a reload doesn't force a re-login,
 * but the *role* is never read from storage — it comes from GET /auth/me on
 * every load, and the server re-checks it on every privileged action anyway.
 * Anything client-side is a convenience for rendering, never the access
 * control itself: a user who edits localStorage gets a nicer-looking menu
 * and nothing more.
 */
const TOKEN_KEY = "flockwatch.token";

interface AuthState {
  me: Me | null;
  loading: boolean;
  signIn: (googleCredential: string) => Promise<void>;
  signOut: () => void;
  isModerator: boolean;
}

const AuthContext = createContext<AuthState | null>(null);

function readStoredToken(): string | null {
  try {
    return localStorage.getItem(TOKEN_KEY);
  } catch {
    // Private mode / blocked storage — sign-in still works for this tab.
    return null;
  }
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [me, setMe] = useState<Me | null>(null);
  const [loading, setLoading] = useState(true);

  // Restore a session on load by asking the server who we are, rather than
  // trusting anything cached locally.
  useEffect(() => {
    const token = readStoredToken();
    if (!token) {
      setLoading(false);
      return;
    }

    setAuthToken(token);
    getMe()
      .then(setMe)
      .catch(() => {
        // Expired or revoked — clear it rather than leaving a dead token.
        setAuthToken(null);
        try {
          localStorage.removeItem(TOKEN_KEY);
        } catch {
          /* ignore */
        }
      })
      .finally(() => setLoading(false));
  }, []);

  const signIn = useCallback(async (googleCredential: string) => {
    const res = await googleLogin(googleCredential);
    setAuthToken(res.token);
    try {
      localStorage.setItem(TOKEN_KEY, res.token);
    } catch {
      /* ignore */
    }
    setMe(await getMe());
  }, []);

  const signOut = useCallback(() => {
    setAuthToken(null);
    try {
      localStorage.removeItem(TOKEN_KEY);
    } catch {
      /* ignore */
    }
    setMe(null);
  }, []);

  const value = useMemo<AuthState>(
    () => ({
      me,
      loading,
      signIn,
      signOut,
      isModerator: me?.role === "moderator" || me?.role === "admin",
    }),
    [me, loading, signIn, signOut],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthState {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth must be used inside <AuthProvider>");
  return ctx;
}
