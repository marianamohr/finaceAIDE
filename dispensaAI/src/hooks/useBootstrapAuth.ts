import { useEffect, useState } from "react";
import { useAuthStore } from "@/store/auth";
import { saveRefreshToken, clearRefreshToken, loadRefreshToken } from "@/services/api";

/**
 * Faz um refresh silencioso quando o app abre com isAuthenticated=true mas sem token
 * (o access token não é persistido por segurança; é reobtido via /auth/refresh).
 * Devolve { ready: false } enquanto o refresh está em andamento.
 */
export function useBootstrapAuth(): { ready: boolean } {
  const { isAuthenticated, token } = useAuthStore();
  const [hydrated, setHydrated] = useState(() => useAuthStore.persist.hasHydrated());
  const [refreshing, setRefreshing] = useState(false);
  const [done, setDone] = useState(false);

  useEffect(() => {
    if (!hydrated) {
      return useAuthStore.persist.onFinishHydration(() => setHydrated(true));
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
