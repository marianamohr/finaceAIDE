import { Navigate } from "react-router-dom";
import { useAuthStore } from "@/store/auth";
import { useBootstrapAuth } from "@/hooks/useBootstrapAuth";

export function AdminRoute({ children }: { children: React.ReactNode }) {
  const { isAuthenticated, user, token } = useAuthStore();
  const { ready } = useBootstrapAuth();

  if (!ready) return null;

  if (!isAuthenticated || !token) {
    return <Navigate to="/login" replace />;
  }

  if (user?.role !== "admin") {
    return <Navigate to="/chat" replace />;
  }

  return <>{children}</>;
}
