import { useState } from "react";
import { useAuthStore, User } from "@/store/auth";
import { useSettingsStore } from "@/store/settings";
import { useCurrencyStore } from "@/store/currency";
import { api, saveRefreshToken, clearRefreshToken } from "@/services/api";
import { useNavigate } from "react-router-dom";

type AuthResponse = { user: User; access_token: string; refresh_token?: string };

export function useAuth() {
  const { isAuthenticated, user, logout: storeLogout, setAuth } = useAuthStore();
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const navigate = useNavigate();

  const login = async (email: string, password: string) => {
    setIsLoading(true);
    setError(null);
    try {
      const data = await api.post<AuthResponse>("/auth/login", { email, password });
      setAuth(data.user, data.access_token);
      if (data.refresh_token) saveRefreshToken(data.refresh_token);
      if (data.user.status === "pending") {
        navigate("/pending");
      } else if (data.user.role === "admin") {
        navigate("/admin");
      } else {
        navigate("/chat");
      }
    } catch (e: any) {
      setError(e.message || "Erro ao entrar");
    } finally {
      setIsLoading(false);
    }
  };

  const register = async (email: string, password: string) => {
    setIsLoading(true);
    setError(null);
    try {
      const data = await api.post<AuthResponse>("/auth/register", { email, password });
      setAuth(data.user, data.access_token);
      if (data.refresh_token) saveRefreshToken(data.refresh_token);
      if (data.user.status === "pending") {
        navigate("/pending");
      } else if (data.user.role === "admin") {
        navigate("/admin");
      } else {
        navigate("/chat");
      }
    } catch (e: any) {
      setError(e.message || "Erro ao criar conta");
    } finally {
      setIsLoading(false);
    }
  };

  const logout = () => {
    // Clear storage first, before storeLogout() triggers Zustand persist re-write
    clearRefreshToken();
    useAuthStore.persist.clearStorage();
    useSettingsStore.persist.clearStorage();
    useCurrencyStore.persist.clearStorage();
    localStorage.removeItem("financeai-auth");
    localStorage.removeItem("financeai-currency");
    localStorage.removeItem("financeai-refresh");
    localStorage.removeItem("financeai-settings");
    storeLogout();
    navigate("/login");
  };

  return { isAuthenticated, user, login, register, logout, isLoading, error };
}
