import { useEffect, useState } from "react";
import { useAuthStore } from "@/store/auth";
import { saveRefreshToken, clearRefreshToken, loadRefreshToken } from "@/services/api";

/**
 * Runs a silent token refresh when the app boots with isAuthenticated=true but no token
 * (token is not persisted to localStorage for security; must be re-acquired via refresh).
 *
 * Returns { ready: boolean } — false while refresh is in-flight; true once resolved.
 */
export function useBootstrapAuth(): { ready: boolean } {
  const { isAuthenticated, token } = useAuthStore();
  const [hydrated, setHydrated] = useState(() => useAuthStore.persist.hasHydrated());
  const [refreshing, setRefreshing] = useState(false);
  const [done, setDone] = useState(false);

  useEffect(() => {
    if (!hydrated) {
      const unsub = useAuthStore.persist.onFinishHydration(() => setHydrated(true));
      return unsub;
    }
  }, [hydrated]);

  useEffect(() => {
    if (!hydrated) return;
    if (token || !isAuthenticated || refreshing || done) return;

    setRefreshing(true);
    const storedRefresh = loadRefreshToken();
    const headers: Record<string, string> = {};
    if (storedRefresh) headers["X-Refresh-Token"] = storedRefresh;

    fetch(`${import.meta.env.VITE_API_URL || "/api"}/auth/refresh`, {
      method: "POST",
      credentials: "include",
      headers,
    })
      .then(async (res) => {
        if (!res.ok) throw new Error("refresh failed");
        const data = await res.json();
        // Restore the full user (email/id/account_id) from the server — these are
        // intentionally not persisted to localStorage, so the refresh is the source of truth.
        if (data.user) {
          useAuthStore.getState().setAuth(data.user, data.access_token);
        } else {
          useAuthStore.getState().setToken(data.access_token);
        }
        if (data.refresh_token) saveRefreshToken(data.refresh_token);
      })
      .catch(() => {
        clearRefreshToken();
        useAuthStore.getState().logout();
      })
      .finally(() => {
        setRefreshing(false);
        setDone(true);
      });
  }, [hydrated, token, isAuthenticated, refreshing, done]);

  const needsRefresh = hydrated && isAuthenticated && !token && !done;
  return { ready: hydrated && !needsRefresh && !refreshing };
}
