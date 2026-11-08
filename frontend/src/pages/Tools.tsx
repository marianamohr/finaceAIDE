import { useEffect, useState } from "react";
import { BottomNav } from "@/components/BottomNav/BottomNav";
import { PageHeader } from "@/components/PageHeader/PageHeader";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";
import { api } from "@/services/api";

const DEFAULT_BR_WAGE = 1518;
const DEFAULT_DE_WAGE = 2250;

function formatSalaries(value: number): string {
  return value.toLocaleString("pt-BR", { minimumFractionDigits: 2, maximumFractionDigits: 2 });
}

function toWorkTime(salaries: number): string {
  if (!isFinite(salaries) || salaries <= 0) return "";
  const totalDays = salaries * 30;
  const months = Math.floor(totalDays / 30);
  const days = Math.round(totalDays % 30);
  if (months === 0) return `${days} dia${days !== 1 ? "s" : ""}`;
  if (days === 0) return `${months} mês${months !== 1 ? "es" : ""}`;
  return `${months} mês${months !== 1 ? "es" : ""} e ${days} dia${days !== 1 ? "s" : ""}`;
}

export default function Tools() {
  const [brWage, setBrWage] = useState(String(DEFAULT_BR_WAGE));
  const [deWage, setDeWage] = useState(String(DEFAULT_DE_WAGE));
  const [itemName, setItemName] = useState("");
  const [priceBRL, setPriceBRL] = useState("");
  const [priceEUR, setPriceEUR] = useState("");
  const [exchangeRate, setExchangeRate] = useState<{ rate: number; fetchedAt: Date } | null>(null);

  useEffect(() => {
    api.getExchangeRate("BRL", "EUR")
      .then((data) => setExchangeRate({ rate: data.rate, fetchedAt: new Date() }))
      .catch(() => {/* silently ignore */});
  }, []);

  const brWageNum = parseFloat(brWage.replace(",", ".")) || 0;
  const deWageNum = parseFloat(deWage.replace(",", ".")) || 0;
  const brlNum = parseFloat(priceBRL.replace(",", ".")) || 0;
  const eurNum = parseFloat(priceEUR.replace(",", ".")) || 0;

  const brSalaries = brWageNum > 0 && brlNum > 0 ? brlNum / brWageNum : null;
  const deSalaries = deWageNum > 0 && eurNum > 0 ? eurNum / deWageNum : null;

  const hasResult = brSalaries !== null || deSalaries !== null;
  const maxSalaries = Math.max(brSalaries ?? 0, deSalaries ?? 0);

  return (
    <div className="h-[var(--app-height)] lg:h-full flex flex-col bg-background">
      <PageHeader title="Tools" />

      <div className="flex-1 overflow-y-auto px-4 pb-4 space-y-4">
        {/* Calculator card */}
        <div className="rounded-xl bg-card p-4 space-y-4">
          <h2 className="font-heading text-sm font-semibold">Calculadora de Poder de Compra</h2>
          <p className="text-xs text-muted-foreground">
            Quantos salários mínimos você precisa para comprar um item no Brasil vs Alemanha?
          </p>

          {exchangeRate && (
            <div className="rounded-lg bg-secondary px-3 py-2 flex items-center justify-between">
              <span className="text-xs text-muted-foreground">Câmbio atual</span>
              <span className="text-xs font-medium tabular-nums">
                1 BRL = {exchangeRate.rate.toFixed(4)} EUR
              </span>
            </div>
          )}

          {/* Minimum wages */}
          <div>
            <p className="text-xs text-muted-foreground mb-2">Salário mínimo (editável)</p>
            <div className="grid grid-cols-2 gap-2">
              <div className="space-y-1">
                <label className="text-xs font-medium flex items-center gap-1">
                  🇧🇷 Brasil (R$)
                </label>
                <Input
                  type="number"
                  value={brWage}
                  onChange={(e) => setBrWage(e.target.value)}
                  className="h-9 text-sm"
                  placeholder="1621"
                />
              </div>
              <div className="space-y-1">
                <label className="text-xs font-medium flex items-center gap-1">
                  🇩🇪 Alemanha (€)
                </label>
                <Input
                  type="number"
                  value={deWage}
                  onChange={(e) => setDeWage(e.target.value)}
                  className="h-9 text-sm"
                  placeholder="2122"
                />
              </div>
            </div>
          </div>

          {/* Item */}
          <div className="space-y-2">
            <p className="text-xs text-muted-foreground">Preço do item</p>
            <Input
              type="text"
              value={itemName}
              onChange={(e) => setItemName(e.target.value)}
              placeholder="Nome do item (ex: iPhone 17)"
              className="h-9 text-sm"
            />
            <div className="grid grid-cols-2 gap-2">
              <div className="space-y-1">
                <label className="text-xs font-medium">🇧🇷 Preço em R$</label>
                <Input
                  type="number"
                  value={priceBRL}
                  onChange={(e) => setPriceBRL(e.target.value)}
                  placeholder="7999"
                  className="h-9 text-sm"
                />
              </div>
              <div className="space-y-1">
                <label className="text-xs font-medium">🇩🇪 Preço em €</label>
                <Input
                  type="number"
                  value={priceEUR}
                  onChange={(e) => setPriceEUR(e.target.value)}
                  placeholder="999"
                  className="h-9 text-sm"
                />
              </div>
            </div>
          </div>

          {/* Result */}
          {hasResult && (
            <div className="space-y-3 pt-2 border-t border-border">
              {itemName && (
                <p className="text-sm font-medium text-center">{itemName}</p>
              )}

              <div className="grid grid-cols-2 gap-3">
                {brSalaries !== null && (
                  <div className={cn(
                    "rounded-xl p-3 text-center space-y-1",
                    brSalaries > (deSalaries ?? 0) ? "bg-destructive/10" : "bg-primary/10"
                  )}>
                    <p className="text-2xl font-bold font-heading tabular-nums">
                      {formatSalaries(brSalaries)}
                    </p>
                    <p className="text-xs text-muted-foreground">🇧🇷 salários mínimos</p>
                    <p className="text-[10px] text-muted-foreground">{toWorkTime(brSalaries)} de trabalho</p>
                  </div>
                )}
                {deSalaries !== null && (
                  <div className={cn(
                    "rounded-xl p-3 text-center space-y-1",
                    deSalaries > (brSalaries ?? 0) ? "bg-destructive/10" : "bg-primary/10"
                  )}>
                    <p className="text-2xl font-bold font-heading tabular-nums">
                      {formatSalaries(deSalaries)}
                    </p>
                    <p className="text-xs text-muted-foreground">🇩🇪 salários mínimos</p>
                    <p className="text-[10px] text-muted-foreground">{toWorkTime(deSalaries)} de trabalho</p>
                  </div>
                )}
              </div>

              {/* Comparison bar */}
              {brSalaries !== null && deSalaries !== null && maxSalaries > 0 && (
                <div className="space-y-1.5">
                  <div className="space-y-1">
                    <div className="flex justify-between text-xs">
                      <span>🇧🇷 Brasil</span>
                      <span className="text-muted-foreground">{formatSalaries(brSalaries)} sal.</span>
                    </div>
                    <div className="h-2 rounded-full bg-secondary overflow-hidden">
                      <div
                        className={cn("h-full rounded-full transition-all", brSalaries > deSalaries ? "bg-destructive" : "bg-primary")}
                        style={{ width: `${Math.min((brSalaries / maxSalaries) * 100, 100)}%` }}
                      />
                    </div>
                  </div>
                  <div className="space-y-1">
                    <div className="flex justify-between text-xs">
                      <span>🇩🇪 Alemanha</span>
                      <span className="text-muted-foreground">{formatSalaries(deSalaries)} sal.</span>
                    </div>
                    <div className="h-2 rounded-full bg-secondary overflow-hidden">
                      <div
                        className={cn("h-full rounded-full transition-all", deSalaries > brSalaries ? "bg-destructive" : "bg-primary")}
                        style={{ width: `${Math.min((deSalaries / maxSalaries) * 100, 100)}%` }}
                      />
                    </div>
                  </div>
                  <p className="text-xs text-center text-muted-foreground pt-1">
                    {brSalaries > deSalaries
                      ? `No Brasil custa ${formatSalaries(brSalaries / deSalaries)}× mais em salários`
                      : deSalaries > brSalaries
                      ? `Na Alemanha custa ${formatSalaries(deSalaries / brSalaries)}× mais em salários`
                      : "Custo equivalente em salários mínimos"}
                  </p>
                </div>
              )}
            </div>
          )}
        </div>
      </div>

      <BottomNav />
    </div>
  );
}
