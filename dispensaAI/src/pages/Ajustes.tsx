import { useEffect, useState } from "react";
import { LogOut } from "lucide-react";
import { useNavigate } from "react-router-dom";
import { toast } from "sonner";
import { PageHeader } from "@/components/PageHeader";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { usePantrySettings } from "@/hooks/usePantrySettings";
import { usePantryAIUsage } from "@/hooks/usePantryAIUsage";
import { useAuth } from "@/hooks/useAuth";
import { useAuthStore } from "@/store/auth";
import { useT, LANGUAGES } from "@/i18n";
import type { PantryCountry, PantryCurrency, PantryLanguage } from "@/services/api";

interface FormState {
  language: PantryLanguage;
  country: PantryCountry;
  currency: PantryCurrency;
  shopping_interval_days: number;
  ask_produce_units: boolean;
}

export default function Ajustes() {
  const t = useT();
  const { settings, isLoading, update } = usePantrySettings();
  const { usage } = usePantryAIUsage();
  const { logout } = useAuth();
  const navigate = useNavigate();
  const user = useAuthStore((s) => s.user);
  const [form, setForm] = useState<FormState | null>(null);

  useEffect(() => {
    if (settings && !form) {
      setForm({
        language: settings.language,
        country: settings.country,
        currency: settings.currency,
        shopping_interval_days: settings.shopping_interval_days,
        ask_produce_units: settings.ask_produce_units,
      });
    }
  }, [settings, form]);

  const dirty =
    !!form &&
    !!settings &&
    (form.language !== settings.language ||
      form.country !== settings.country ||
      form.currency !== settings.currency ||
      form.shopping_interval_days !== settings.shopping_interval_days ||
      form.ask_produce_units !== settings.ask_produce_units);

  const handleSave = () => {
    if (!form) return;
    update.mutate(form, {
      onSuccess: () => toast.success(t("common.saved")),
      onError: (e) => toast.error(e instanceof Error ? e.message : t("common.error")),
    });
  };

  return (
    <div className="flex-1 min-h-0 flex flex-col">
      <PageHeader title={t("settings.title")} />

      <div className="flex-1 overflow-y-auto px-4 py-4 space-y-6">
        {isLoading || !form ? (
          <p className="text-sm text-muted-foreground">{t("common.loading")}</p>
        ) : (
          <>
            <section className="space-y-2">
              <Label>{t("settings.language")}</Label>
              <Select
                value={form.language}
                onValueChange={(v) => setForm({ ...form, language: v as PantryLanguage })}
              >
                <SelectTrigger className="h-11 rounded-xl"><SelectValue /></SelectTrigger>
                <SelectContent>
                  {LANGUAGES.map((l) => (
                    <SelectItem key={l.value} value={l.value}>{l.label}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <p className="text-xs text-muted-foreground">{t("settings.language.help")}</p>
            </section>

            <section className="space-y-2">
              <Label>{t("settings.country")}</Label>
              <Select
                value={form.country}
                onValueChange={(v) => {
                  const country = v as PantryCountry;
                  setForm({ ...form, country, currency: country === "DE" ? "EUR" : "BRL" });
                }}
              >
                <SelectTrigger className="h-11 rounded-xl"><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="BR">{t("settings.country.BR")}</SelectItem>
                  <SelectItem value="DE">{t("settings.country.DE")}</SelectItem>
                </SelectContent>
              </Select>
              <p className="text-xs text-muted-foreground">{t("settings.country.help")}</p>
            </section>

            <section className="space-y-2">
              <Label>{t("settings.currency")}</Label>
              <Select
                value={form.currency}
                onValueChange={(v) => setForm({ ...form, currency: v as PantryCurrency })}
              >
                <SelectTrigger className="h-11 rounded-xl"><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="BRL">R$ (BRL)</SelectItem>
                  <SelectItem value="EUR">€ (EUR)</SelectItem>
                </SelectContent>
              </Select>
            </section>

            <section className="space-y-2">
              <Label htmlFor="interval">{t("settings.interval")}</Label>
              <div className="flex items-center gap-2">
                <Input
                  id="interval"
                  type="number"
                  inputMode="numeric"
                  min={1}
                  max={60}
                  value={form.shopping_interval_days}
                  onChange={(e) =>
                    setForm({ ...form, shopping_interval_days: Math.max(1, Math.min(60, Number(e.target.value) || 1)) })
                  }
                  className="h-11 rounded-xl w-24"
                />
                <span className="text-sm text-muted-foreground">{t("settings.interval.days")}</span>
              </div>
              <p className="text-xs text-muted-foreground">{t("settings.interval.help")}</p>
            </section>

            <section className="flex items-start justify-between gap-4">
              <div className="space-y-1">
                <Label htmlFor="askProduce">{t("settings.askProduce")}</Label>
                <p className="text-xs text-muted-foreground">{t("settings.askProduce.help")}</p>
              </div>
              <Switch
                id="askProduce"
                checked={form.ask_produce_units}
                onCheckedChange={(v) => setForm({ ...form, ask_produce_units: v })}
              />
            </section>

            <Button
              type="button"
              onClick={handleSave}
              disabled={!dirty || update.isPending}
              className="w-full h-12 rounded-xl font-heading font-semibold"
            >
              {update.isPending ? t("common.saving") : t("common.save")}
            </Button>

            <Button
              type="button"
              variant="outline"
              onClick={() => navigate("/inventario")}
              className="w-full h-11 rounded-xl"
            >
              {t("settings.doInventory")}
            </Button>

            {usage && (
              <section className="pt-4 border-t border-border space-y-1">
                <Label>{t("settings.aiUsage.title")}</Label>
                <p className="text-xs text-muted-foreground">
                  {t("settings.aiUsage.tokens", {
                    input: usage.total_input_tokens.toLocaleString(),
                    output: usage.total_output_tokens.toLocaleString(),
                  })}
                </p>
                <p className="text-xs text-muted-foreground">
                  {t("settings.aiUsage.cost", { cost: usage.estimated_cost_usd.toFixed(2) })}
                </p>
              </section>
            )}

            <section className="pt-4 border-t border-border space-y-3">
              <p className="text-xs text-muted-foreground">
                {t("settings.account")}: {user?.email ?? "—"}
              </p>
              <Button
                type="button"
                variant="outline"
                onClick={logout}
                className="w-full h-11 rounded-xl text-destructive gap-2"
              >
                <LogOut className="w-4 h-4" />
                {t("common.logout")}
              </Button>
              <p className="text-[11px] text-muted-foreground text-center">
                {t("settings.version")} {__APP_VERSION__}
              </p>
            </section>
          </>
        )}
      </div>
    </div>
  );
}
