import { create } from "zustand";
import { persist } from "zustand/middleware";
import type { AuthUser } from "@/services/api";

interface AuthState {
  user: AuthUser | null;
  token: string | null;
  isAuthenticated: boolean;
  setAuth: (user: AuthUser, token: string) => void;
  setToken: (token: string) => void;
  logout: () => void;
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set) => ({
      user: null,
      token: null,
      isAuthenticated: false,
      setAuth: (user, token) => set({ user, token, isAuthenticated: true }),
      setToken: (token) => set({ token, isAuthenticated: true }),
      logout: () => set({ user: null, token: null, isAuthenticated: false }),
    }),
    {
      name: "dai-a",
      partialize: (state) => ({
        // O access token não é persistido: é reobtido via /auth/refresh no boot.
        isAuthenticated: state.isAuthenticated,
        user: state.user ? { role: state.user.role, status: state.user.status } : null,
      }),
    },
  ),
);
