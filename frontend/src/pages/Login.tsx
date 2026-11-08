import { useState } from "react"; // eslint-disable-line -- used for email/password state
import { useAuth } from "@/hooks/useAuth";
import { usePasskey } from "@/hooks/usePasskey";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { DollarSign, TrendingUp, Bot, ScanFace } from "lucide-react";
import { Navigate } from "react-router-dom";
import { useAuthStore } from "@/store/auth";

const supportsPasskeys =
  typeof window !== "undefined" && !!window.PublicKeyCredential;

export default function Login() {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const { isAuthenticated, user } = useAuthStore();
  const { login, isLoading, error } = useAuth();
  const { loginWithPasskey, isLoading: passkeyLoading, error: passkeyError } = usePasskey();

  if (isAuthenticated && user) {
    if (user.status === "pending") return <Navigate to="/pending" replace />;
    if (user.role === "admin") return <Navigate to="/admin" replace />;
    return <Navigate to="/chat" replace />;
  }

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    login(email, password);
  };

  return (
    <div className="min-h-[var(--app-height)] flex flex-col bg-background safe-top safe-bottom">
      {/* Hero */}
      <div className="flex-1 flex flex-col items-center justify-center px-6 py-12">
        <div className="w-16 h-16 rounded-2xl bg-primary/10 flex items-center justify-center mb-6 animate-fade-in">
          <DollarSign className="w-8 h-8 text-primary" />
        </div>

        <h1 className="font-heading text-3xl font-bold tracking-tight text-center mb-2 animate-fade-in">
          FinanceAI
        </h1>
        <p className="text-muted-foreground text-center text-sm max-w-[280px] mb-10 animate-fade-in">
          Gerencie suas finanças com inteligência artificial
        </p>

        {/* Features pills */}
        <div className="flex gap-2 mb-10 animate-fade-in">
          {[
            { icon: Bot, label: "Chat IA" },
            { icon: TrendingUp, label: "Análises" },
          ].map(({ icon: Icon, label }) => (
            <span
              key={label}
              className="flex items-center gap-1.5 px-3 py-1.5 rounded-full bg-secondary text-secondary-foreground text-xs font-medium"
            >
              <Icon className="w-3.5 h-3.5" />
              {label}
            </span>
          ))}
        </div>

        {/* Form */}
        <form onSubmit={handleSubmit} className="w-full max-w-sm space-y-3 animate-fade-in">
          <Input
            type="email"
            placeholder="Seu e-mail"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            required
            autoComplete="email"
            className="h-12 rounded-xl bg-secondary border-0 px-4 text-sm"
          />
          <Input
            type="password"
            placeholder="Senha"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required
            minLength={6}
            autoComplete="current-password"
            className="h-12 rounded-xl bg-secondary border-0 px-4 text-sm"
          />

          {(error || passkeyError) && (
            <p className="text-destructive text-xs text-center">{error || passkeyError}</p>
          )}

          <Button
            type="submit"
            disabled={isLoading || passkeyLoading}
            className="w-full h-12 rounded-xl font-heading font-semibold text-sm"
          >
            {isLoading ? "Aguarde..." : "Entrar"}
          </Button>

          {supportsPasskeys && (
            <>
              <div className="relative flex items-center gap-2 py-1">
                <div className="flex-1 h-px bg-border" />
                <span className="text-xs text-muted-foreground">ou</span>
                <div className="flex-1 h-px bg-border" />
              </div>
              <Button
                type="button"
                variant="outline"
                disabled={isLoading || passkeyLoading}
                onClick={() => loginWithPasskey()}
                className="w-full h-12 rounded-xl font-heading font-semibold text-sm gap-2"
              >
                <ScanFace className="w-5 h-5" />
                {passkeyLoading ? "Aguarde..." : "Entrar com Face ID"}
              </Button>
            </>
          )}
        </form>

      </div>
    </div>
  );
}
