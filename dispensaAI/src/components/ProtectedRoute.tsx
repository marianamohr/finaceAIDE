import { Navigate } from "react-router-dom";
import { useAuthStore } from "@/store/auth";
import { useBootstrapAuth } from "@/hooks/useBootstrapAuth";

export function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const { isAuthenticated, user, token } = useAuthStore();
  const { ready } = useBootstrapAuth();

  if (!ready) return null;

  if (!isAuthenticated || !token) {
    return <Navigate to="/login" replace />;
  }

  if (user?.status === "pending" || user?.status === "suspended") {
    return (
      <div className="h-[var(--app-height)] flex items-center justify-center bg-background px-6">
        <div className="text-center space-y-2">
          <p className="text-lg font-semibold">
            {user.status === "pending" ? "Conta aguardando aprovação" : "Conta suspensa"}
          </p>
          <p className="text-sm text-muted-foreground">Fale com o administrador do FinanceAI.</p>
        </div>
      </div>
    );
  }

  return <>{children}</>;
}
