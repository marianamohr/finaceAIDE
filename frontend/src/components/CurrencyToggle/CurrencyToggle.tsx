import { useState } from "react";
import { Coins } from "lucide-react";
import { cn } from "@/lib/utils";
import { api } from "@/services/api";
import { useCurrencyStore, Currency } from "@/store/currency";

interface CurrencyToggleProps {
  onCurrencyChange?: (currency: Currency) => void;
}

export function CurrencyToggle({ onCurrencyChange }: CurrencyToggleProps) {
  const { currency: selected, setCurrency } = useCurrencyStore();
  const [status, setStatus] = useState<"idle" | "saving" | "saved">("idle");

  const handleSelect = async (currency: Currency) => {
    if (currency === selected || status === "saving") return;

    setCurrency(currency);
    setStatus("saving");

    try {
      await api.setCurrency(currency);
      setStatus("saved");
      onCurrencyChange?.(currency);
      setTimeout(() => setStatus("idle"), 2000);
    } catch {
      setStatus("idle");
    }
  };

  return (
    <div className="w-full flex items-center gap-3 px-4 py-3">
      <div className="w-9 h-9 rounded-lg bg-secondary flex items-center justify-center shrink-0">
        <Coins className="w-4 h-4 text-muted-foreground" />
      </div>

      <div className="flex-1">
        <p className="text-sm font-medium">Moeda base</p>
        <p className="text-xs text-muted-foreground">
          {status === "saving" ? "Salvando…" : status === "saved" ? "✓ Salvo" : "Moeda para exibição e conversões"}
        </p>
      </div>

      {/* Toggle pill */}
      <div className="flex items-center gap-1 rounded-full bg-secondary p-0.5 shrink-0">
        {(["BRL", "EUR"] as Currency[]).map((currency) => (
          <button
            key={currency}
            onClick={() => handleSelect(currency)}
            className={cn(
              "rounded-full px-3 py-1 text-xs font-medium transition-all",
              selected === currency
                ? "bg-primary text-primary-foreground shadow-sm"
                : "text-muted-foreground hover:text-foreground"
            )}
          >
            {currency === "BRL" ? "BRL 🇧🇷" : "EUR 🇩🇪"}
          </button>
        ))}
      </div>
    </div>
  );
}
