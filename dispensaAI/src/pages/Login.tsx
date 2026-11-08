import { useState } from "react";
import { Navigate } from "react-router-dom";
import { ShoppingBasket } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { useAuth } from "@/hooks/useAuth";
import { useAuthStore } from "@/store/auth";
import { useT, LANGUAGES } from "@/i18n";
import { useSettingsStore } from "@/store/settings";

export default function Login() {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const { isAuthenticated, token } = useAuthStore();
  const { login, isLoading, error } = useAuth();
  const t = useT();
  const language = useSettingsStore((s) => s.language);
  const setLanguage = useSettingsStore((s) => s.setLanguage);

  if (isAuthenticated && token) return <Navigate to="/" replace />;

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    login(email, password);
  };

  return (
    <div className="min-h-[var(--app-height)] flex flex-col bg-background safe-top safe-bottom">
      <div className="flex-1 flex flex-col items-center justify-center px-6 py-12">
        <div className="w-16 h-16 rounded-2xl bg-primary/10 flex items-center justify-center mb-6 animate-fade-in">
          <ShoppingBasket className="w-8 h-8 text-primary" />
        </div>

        <h1 className="font-heading text-3xl font-bold tracking-tight text-center mb-2 animate-fade-in">
          {t("app.name")}
        </h1>
        <p className="text-muted-foreground text-center text-sm max-w-[300px] mb-8 animate-fade-in">
          {t("app.tagline")}
        </p>

        <form onSubmit={handleSubmit} className="w-full max-w-sm space-y-3 animate-fade-in">
          <Input
            type="email"
            placeholder={t("login.email")}
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            required
            autoComplete="email"
            className="h-12 rounded-xl bg-secondary border-0 px-4 text-sm"
          />
          <Input
            type="password"
            placeholder={t("login.password")}
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required
            minLength={6}
            autoComplete="current-password"
            className="h-12 rounded-xl bg-secondary border-0 px-4 text-sm"
          />

          {error && <p className="text-destructive text-xs text-center">{error}</p>}

          <Button
            type="submit"
            disabled={isLoading}
            className="w-full h-12 rounded-xl font-heading font-semibold text-sm"
          >
            {isLoading ? t("login.loading") : t("login.submit")}
          </Button>

          <p className="text-xs text-muted-foreground text-center">{t("login.hint")}</p>
        </form>

        <div className="flex gap-2 mt-10">
          {LANGUAGES.map((l) => (
            <button
              key={l.value}
              type="button"
              onClick={() => setLanguage(l.value)}
              className={
                "px-3 py-1.5 rounded-full text-xs font-medium transition-colors " +
                (language === l.value
                  ? "bg-primary text-primary-foreground"
                  : "bg-secondary text-secondary-foreground")
              }
            >
              {l.label}
            </button>
          ))}
        </div>
      </div>
    </div>
  );
}
