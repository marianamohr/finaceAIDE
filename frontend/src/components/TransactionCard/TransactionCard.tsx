import { cn } from "@/lib/utils";
import { categoryLabel } from "@/lib/categories";
import { formatCurrency, maskCurrency, useCurrencyStore, type Currency } from "@/store/currency";
import { usePrivacyStore } from "@/store/privacy";
import { getTransactionBadge } from "@/lib/transactionBadge";
import {
  ShoppingCart,
  Car,
  Heart,
  Home,
  Gamepad2,
  GraduationCap,
  Shirt,
  Wrench,
  HelpCircle,
  ArrowDownLeft,
  ArrowUpRight,
  ArrowLeftRight,
  UtensilsCrossed,
  ShoppingBasket,
  Coffee,
  Wine,
  Sparkles,
  Droplets,
  Apple,
  Gift,
  Image,
  Wallet,
  TrendingUp,
  PiggyBank,
  Briefcase,
  Receipt,
} from "lucide-react";

const categoryIcons: Record<string, React.ElementType> = {
  salario: Wallet,
  vr: ShoppingCart,
  ppr: TrendingUp,
  freelance: Briefcase,
  investimentos_receita: TrendingUp,
  aluguel_recebido: Home,
  outras_entradas: ArrowDownLeft,
  alimentacao: ShoppingCart,
  transporte: Car,
  saude: Heart,
  casa: Home,
  lazer: Gamepad2,
  educacao: GraduationCap,
  vestuario: Shirt,
  presentes: Gift,
  servicos: Wrench,
  pets: Heart,
  emprestimos: PiggyBank,
  impostos: Receipt,
  beleza: Sparkles,
  investimentos_despesa: TrendingUp,
  outros: HelpCircle,
  comer_fora: UtensilsCrossed,
  comida: ShoppingBasket,
  frutaria: Apple,
  bebidas_nao_alcoolicas: Coffee,
  bebidas_alcoolicas: Wine,
  higiene_pessoal: Sparkles,
  limpeza_da_casa: Droplets,
  reserva: PiggyBank,
};

interface Transaction {
  id: string;
  amount: number;
  category: string;
  merchant: string;
  description: string;
  date: string;
  source: string;
  billing_year?: number;
  installment_number?: number | null;
  type?: string;
  currency?: string;
  image_url?: string;
  onClick?: () => void;
}

export function TransactionCard({ amount, category, merchant, description, date, source, billing_year, installment_number, type, currency, image_url, onClick }: Transaction) {
  const { currency: globalCurrency } = useCurrencyStore();
  const hidden = usePrivacyStore((s) => s.valuesHidden);
  const txCurrency = (currency as Currency) || globalCurrency;
  const isTransfer = type === "transfer";
  const isIncome = !isTransfer && amount > 0;
  const Icon = isTransfer ? ArrowLeftRight : (categoryIcons[category] ?? HelpCircle);
  const label = categoryLabel(category);
  const badge = getTransactionBadge(source, billing_year, installment_number);

  return (
    <div
      className={cn(
        "flex items-center gap-3 px-4 py-3 rounded-xl bg-card hover:bg-secondary/50 transition-colors",
        onClick && "cursor-pointer"
      )}
      onClick={onClick}
    >
      <div className={cn(
        "w-10 h-10 rounded-xl flex items-center justify-center shrink-0",
        isTransfer ? "bg-muted text-muted-foreground" : isIncome ? "bg-primary/10 text-primary" : "bg-destructive/10 text-destructive"
      )}>
        <Icon className="w-5 h-5" />
      </div>

      <div className="flex-1 min-w-0">
        <p className="text-sm font-medium truncate capitalize">
          {merchant ? `${merchant} · ${label}` : (description || label)}
        </p>
        <p className="text-xs text-muted-foreground flex items-center gap-1">
          {new Date(date.slice(0, 10) + "T12:00:00").toLocaleDateString("pt-BR", { day: "2-digit", month: "short" })}
          {source !== "manual" && source !== "recurring" && source !== "installment" && ` · ${source === "chat" ? "chat" : "📸"}`}
          {image_url && <Image className="w-3 h-3 inline-block opacity-50" />}
        </p>
      </div>

      <div className="flex items-center gap-2 shrink-0">
        <span
          className="text-[10px] font-semibold rounded-md px-[7px] py-[2px]"
          style={{ backgroundColor: badge.color + "33", color: badge.color }}
        >
          {badge.icon} {badge.label}
        </span>
        <div className="flex items-center gap-1">
          {isTransfer ? (
            <ArrowLeftRight className="w-3.5 h-3.5 text-muted-foreground" />
          ) : isIncome ? (
            <ArrowDownLeft className="w-3.5 h-3.5 text-primary" />
          ) : (
            <ArrowUpRight className="w-3.5 h-3.5 text-destructive" />
          )}
          <span className={cn(
            "text-sm font-semibold tabular-nums",
            isTransfer ? "text-muted-foreground" : isIncome ? "text-primary" : "text-foreground"
          )}>
            {hidden ? maskCurrency(amount, txCurrency) : formatCurrency(amount, txCurrency)}
          </span>
        </div>
      </div>
    </div>
  );
}
