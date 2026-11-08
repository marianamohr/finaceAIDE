import { create } from "zustand";
import { persist } from "zustand/middleware";

export interface User {
  id?: string;
  email?: string;
  role: string;       // "user" | "admin"
  status: string;     // "pending" | "active" | "suspended"
  account_id?: string;
}

interface AuthState {
  user: User | null;
  token: string | null;
  isAuthenticated: boolean;
  setAuth: (user: User, token: string) => void;
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
      name: "fai-a",
      partialize: (state) => ({
        // token intentionally excluded — re-acquired via /auth/refresh on load
        isAuthenticated: state.isAuthenticated,
        user: state.user ? { role: state.user.role, status: state.user.status } : null,
      }),
    }
  )
);
