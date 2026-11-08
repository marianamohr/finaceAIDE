import { useState } from "react";
import { useSearchParams, useNavigate } from "react-router-dom";
import { useAuthStore } from "@/store/auth";
import { api } from "@/services/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { DollarSign } from "lucide-react";

export default function Invite() {
  const [searchParams] = useSearchParams();
  const token = searchParams.get("token") ?? "";
  const navigate = useNavigate();
  const setAuth = useAuthStore((s) => s.setAuth);

  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  if (!token) {
    return (
      <div className="h-[var(--app-height)] flex items-center justify-center bg-background px-6 text-center">
        <p className="text-sm text-muted-foreground">Link de convite inválido.</p>
      </div>
    );
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (password !== confirm) {
      setError("As senhas não coincidem.");
      return;
    }
    setIsLoading(true);
    setError(null);
    try {
      const data = await api.post<{ user: any; access_token: string }>("/auth/invite/accept", { token, password });
      setAuth(data.user, data.access_token);
      navigate("/chat");
    } catch (e: any) {
      setError(e.message || "Link inválido ou expirado.");
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <div className="min-h-[var(--app-height)] flex flex-col items-center justify-center bg-background px-6 py-12">
      <div className="w-16 h-16 rounded-2xl bg-primary/10 flex items-center justify-center mb-6">
        <DollarSign className="w-8 h-8 text-primary" />
      </div>

      <h1 className="font-heading text-2xl font-bold mb-1">Criar sua senha</h1>
      <p className="text-sm text-muted-foreground mb-8">Você foi convidado para o FinanceAI.</p>

      <form onSubmit={handleSubmit} className="w-full max-w-sm space-y-3">
        <Input
          type="password"
          placeholder="Nova senha (mín. 8 caracteres)"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          required
          minLength={8}
          autoComplete="new-password"
          className="h-12 rounded-xl bg-secondary border-0 px-4 text-sm"
        />
        <Input
          type="password"
          placeholder="Confirmar senha"
          value={confirm}
          onChange={(e) => setConfirm(e.target.value)}
          required
          minLength={8}
          autoComplete="new-password"
          className="h-12 rounded-xl bg-secondary border-0 px-4 text-sm"
        />

        {error && <p className="text-destructive text-xs text-center">{error}</p>}

        <Button
          type="submit"
          disabled={isLoading}
          className="w-full h-12 rounded-xl font-heading font-semibold text-sm"
        >
          {isLoading ? "Aguarde..." : "Criar conta"}
        </Button>
      </form>
    </div>
  );
}
