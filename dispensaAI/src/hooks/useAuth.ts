import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { useQueryClient } from "@tanstack/react-query";
import { useAuthStore } from "@/store/auth";
import { useSettingsStore } from "@/store/settings";
import { api, saveRefreshToken, clearRefreshToken } from "@/services/api";

export function useAuth() {
  const { isAuthenticated, user, logout: storeLogout, setAuth } = useAuthStore();
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const navigate = useNavigate();
  const queryClient = useQueryClient();

  const login = async (email: string, password: string) => {
    setIsLoading(true);
    setError(null);
    try {
      const data = await api.login(email, password);
      setAuth(data.user, data.access_token);
      if (data.refresh_token) saveRefreshToken(data.refresh_token);
      navigate("/");
    } catch (e) {
      setError(e instanceof Error ? e.message : "Erro ao entrar");
    } finally {
      setIsLoading(false);
    }
  };

  const logout = async () => {
    await api.logout();
    clearRefreshToken();
    useAuthStore.persist.clearStorage();
    useSettingsStore.getState().reset();
    queryClient.clear();
    storeLogout();
    navigate("/login");
  };

  return { isAuthenticated, user, login, logout, isLoading, error };
}
