import { useState, useEffect } from "react";
import { BottomNav } from "@/components/BottomNav/BottomNav";
import { PageHeader } from "@/components/PageHeader/PageHeader";
import { RecurringRemindersPanel } from "@/components/RecurringRemindersPanel/RecurringRemindersPanel";
import { cn } from "@/lib/utils";
import { useCashflowQuery, useInvalidateCashflow, CashflowInvoice } from "@/hooks/useCashflow";
import { useDashboardQuery, useInvalidateDashboard } from "@/hooks/useDashboard";
import { CashflowTransaction } from "@/hooks/useCashflow";
import { usePaymentMethods } from "@/hooks/usePaymentMethods";
import { Sheet, SheetContent, SheetHeader, SheetTitle } from "@/components/ui/sheet";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  ArrowDownLeft, ArrowUpRight, ArrowLeftRight, ChevronDown, ChevronUp, HelpCircle,
  ShoppingCart, Car, Heart, Home, Gamepad2, GraduationCap, Shirt, Wrench,
  UtensilsCrossed, ShoppingBasket, Coffee, Wine, Sparkles, Droplets, Apple, Gift,
  Wallet, TrendingUp, PiggyBank, Briefcase, Receipt, Pencil, Trash2,
} from "lucide-react";
import { useCurrencyStore, formatCurrency, maskCurrency } from "@/store/currency";
import { usePrivacyStore } from "@/store/privacy";
import { api } from "@/services/api";
import { getTransactionBadge } from "@/lib/transactionBadge";

const categoryIcons: Record<string, React.ElementType> = {
  salario: Wallet, vr: ShoppingCart, ppr: TrendingUp, freelance: Briefcase,
  investimentos_receita: TrendingUp, aluguel_recebido: Home, outras_entradas: ArrowDownLeft,
  alimentacao: ShoppingCart, transporte: Car, saude: Heart, casa: Home, lazer: Gamepad2,
  educacao: GraduationCap, vestuario: Shirt, presentes: Gift, servicos: Wrench, pets: Heart,
  emprestimos: PiggyBank, impostos: Receipt, beleza: Sparkles, investimentos_despesa: TrendingUp,
  outros: HelpCircle, comer_fora: UtensilsCrossed, comida: ShoppingBasket, frutaria: Apple,
  bebidas_nao_alcoolicas: Coffee, bebidas_alcoolicas: Wine, higiene_pessoal: Sparkles,
  limpeza_da_casa: Droplets, reserva: PiggyBank,
};

function txIcon(category: string, type: string): React.ElementType {
  if (type === "transfer") return ArrowLeftRight;
  if (category === "pagamento_fatura") return Receipt;
  return categoryIcons[category] ?? HelpCircle;
}

function useFmt() {
  const { currency } = useCurrencyStore();
  const hidden = usePrivacyStore((s) => s.valuesHidden);
  return (value: number) =>
    hidden ? maskCurrency(value, currency) : formatCurrency(value, currency);
}

function offsetToYearMonth(offset: number): { year: number; month: number; label: string } {
  const now = new Date();
  const d = new Date(now.getFullYear(), now.getMonth() + offset, 1);
  return {
    year: d.getFullYear(),
    month: d.getMonth() + 1,
    label: d.toLocaleDateString("pt-BR", { month: "short", year: "2-digit" }),
  };
}

const MONTHS_PT = ["Janeiro", "Fevereiro", "Março", "Abril", "Maio", "Junho",
                   "Julho", "Agosto", "Setembro", "Outubro", "Novembro", "Dezembro"];

interface InvoiceCardProps {
  invoice: CashflowInvoice;
  onPay: () => void;
  fmt: (v: number) => string;
}

function InvoiceCard({ invoice, onPay, fmt }: InvoiceCardProps) {
  const [expanded, setExpanded] = useState(false);
  const today = new Date();

  const closingDate = invoice.closing_day
    ? new Date(invoice.billing_year, invoice.billing_month - 1, invoice.closing_day)
    : null;
  const isClosed = closingDate ? today >= closingDate : true;

  const absTotal = Math.abs(invoice.total);

  return (
    <div>
      <div
        className="flex items-start gap-2 py-2 border-b border-border cursor-pointer"
        onClick={() => setExpanded((e) => !e)}
      >
        <div
          className="w-7 h-7 rounded-lg flex items-center justify-center shrink-0 mt-0.5"
          style={{ background: `${invoice.color}22` }}
        >
          <div className="w-3 h-3 rounded-full" style={{ background: invoice.color }} />
        </div>
        <div className="flex-1 min-w-0">
          <p className="text-xs font-semibold">
            {invoice.name} — Fatura {MONTHS_PT[invoice.billing_month - 1]}/{invoice.billing_year}
          </p>
          <p className="text-[10px] text-muted-foreground mt-0.5">
            {invoice.due_day
              ? <span className={cn(isClosed && !invoice.is_paid ? "text-destructive" : "text-warning")}>
                  Vence dia {invoice.due_day}
                </span>
              : null}
            {" · toque para ver compras"}
          </p>
          {!invoice.is_paid && !isClosed && (
            <span className="inline-flex items-center gap-1 text-[9px] text-warning bg-warning/10 rounded px-1.5 py-0.5 mt-1">
              📅 Estimado · fatura ainda aberta
            </span>
          )}
          {!invoice.is_paid && isClosed && (
            <span className="inline-flex items-center gap-1 text-[9px] text-destructive bg-destructive/10 rounded px-1.5 py-0.5 mt-1">
              Aguardando pagamento
            </span>
          )}
        </div>
        <div className="flex flex-col items-end gap-1 shrink-0">
          <span className={cn(
            "text-xs font-bold tabular-nums",
            invoice.is_paid ? "text-foreground" : isClosed ? "text-destructive" : "text-warning"
          )}>
            {invoice.is_paid ? "−" : ""}{fmt(absTotal)}
          </span>
          {invoice.is_paid ? (
            <span className="text-[9px] text-primary font-semibold">Pago ✓</span>
          ) : isClosed ? (
            <button
              onClick={(e) => { e.stopPropagation(); onPay(); }}
              className="text-[9px] bg-primary text-primary-foreground rounded px-2 py-0.5 font-semibold"
            >
              Pagar
            </button>
          ) : (
            <span className="text-[9px] text-warning bg-warning/10 rounded px-1.5 py-0.5">A vencer</span>
          )}
          {expanded ? <ChevronUp className="w-3 h-3 text-muted-foreground" /> : <ChevronDown className="w-3 h-3 text-muted-foreground" />}
        </div>
      </div>

      {expanded && (
        <div className="ml-9 bg-secondary rounded-lg px-2 py-1 mb-1">
          {invoice.transactions.map((tx) => (
            <div key={tx.id} className="flex justify-between items-center py-1 border-b border-border last:border-0">
              <div className="min-w-0">
                <p className="text-[11px] truncate">{tx.merchant || tx.description || tx.category}</p>
                <p className="text-[10px] text-muted-foreground capitalize">{tx.category}</p>
              </div>
              <div className="flex items-center gap-2 shrink-0">
                {(() => {
                  const badge = getTransactionBadge(tx.source, invoice.billing_year);
                  return (
                    <span
                      className="text-[10px] font-semibold rounded-md px-[7px] py-[2px]"
                      style={{ backgroundColor: badge.color + "33", color: badge.color }}
                    >
                      {badge.icon} {badge.label}
                    </span>
                  );
                })()}
                <span className="text-[11px] font-semibold tabular-nums ml-2">
                  {fmt(Math.abs(tx.amount))}
                </span>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

export default function Caixa() {
  const fmt = useFmt();
  const [selectedOffset, setSelectedOffset] = useState(0);
  const [minOffset, setMinOffset] = useState(-12);
  const [maxOffset, setMaxOffset] = useState(2);

  const { year: selectedYear, month: selectedMonth } = offsetToYearMonth(selectedOffset);
  const { data, isLoading } = useCashflowQuery(selectedYear, selectedMonth);
  const invalidate = useInvalidateCashflow();
  const { paymentMethods, payInvoice } = usePaymentMethods();

  const [payingInvoice, setPayingInvoice] = useState<CashflowInvoice | null>(null);
  const [invoicePayAmount, setInvoicePayAmount] = useState<number>(0);
  const [invoiceDebitMethodId, setInvoiceDebitMethodId] = useState<string | null>(null);
  const [accountBalance, setAccountBalance] = useState<number | null>(null);
  const [transferDismissed, setTransferDismissed] = useState(false);
  const [confirmingTransfer, setConfirmingTransfer] = useState(false);
  const [voucherDismissed, setVoucherDismissed] = useState<Record<string, boolean>>({});
  const [confirmingVoucher, setConfirmingVoucher] = useState<string | null>(null);
  const [voucherAmounts, setVoucherAmounts] = useState<Record<string, string>>({});

  const { data: dashData } = useDashboardQuery(selectedYear, selectedMonth);
  const invalidateDashboard = useInvalidateDashboard();
  const balanceTransfer = !transferDismissed && dashData?.balanceTransfer?.pending ? dashData.balanceTransfer : null;
  const voucherSummary = dashData?.voucherSummary ?? [];
  const voucherPending = (dashData?.voucherPending ?? []).filter(
    (v) => !voucherDismissed[v.payment_method_id]
  );

  const [selectedTx, setSelectedTx] = useState<CashflowTransaction | null>(null);
  const [isEditing, setIsEditing] = useState(false);
  const [editForm, setEditForm] = useState<Partial<CashflowTransaction>>({});
  const [saving, setSaving] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [allCategories, setAllCategories] = useState<{ id: string; slug: string; label: string; type: string }[]>([]);

  useEffect(() => {
    api.get<{ id: string; slug: string; label: string; type: string }[]>("/categories")
      .then(setAllCategories).catch(() => {});
  }, []);

useEffect(() => {
    setTransferDismissed(false);
  }, [selectedOffset]);

  useEffect(() => {
    if (!dashData?.voucherPending) return;
    setVoucherAmounts((prev) => {
      const next = { ...prev };
      for (const v of dashData.voucherPending) {
        if (!(v.payment_method_id in next)) {
          next[v.payment_method_id] = v.last_deposit > 0 ? String(v.last_deposit) : "";
        }
      }
      return next;
    });
  }, [dashData?.voucherPending]);

  useEffect(() => {
    api.get<{ balance: number }>(`/transactions/summary?year=${selectedYear}&month=${selectedMonth}`)
      .then((s) => setAccountBalance(s.balance))
      .catch(() => setAccountBalance(null));
  }, [selectedYear, selectedMonth]);

  const handleConfirmVoucher = async (paymentMethodId: string) => {
    const amount = parseFloat(voucherAmounts[paymentMethodId] ?? "0");
    if (!amount || amount <= 0) return;
    setConfirmingVoucher(paymentMethodId);
    try {
      await api.confirmVoucherDeposit(paymentMethodId, amount, selectedYear, selectedMonth);
      invalidateDashboard();
    } finally {
      setConfirmingVoucher(null);
    }
  };

  const handleConfirmTransfer = async () => {
    if (!balanceTransfer) return;
    setConfirmingTransfer(true);
    try {
      await api.confirmBalanceTransfer(balanceTransfer.amount, selectedYear, selectedMonth);
      invalidateDashboard();
      invalidate();
    } finally {
      setConfirmingTransfer(false);
    }
  };

  const summary = data?.summary;
  const realTxs = data?.real_transactions ?? [];
  const invoices = data?.invoices ?? [];

  const windowOffsets = [
    selectedOffset - 2, selectedOffset - 1, selectedOffset,
    selectedOffset + 1, selectedOffset + 2,
  ];

  const deleteTransaction = async (id: string) => {
    await api.delete(`/transactions/${id}`);
    invalidateDashboard();
    invalidate();
  };

  const updateTransaction = async (id: string, data: Partial<Omit<CashflowTransaction, "id" | "kind" | "type">>) => {
    await api.put(`/transactions/${id}`, {
      amount: data.amount,
      category: data.category,
      merchant: data.merchant,
      description: data.description,
      date: data.date ? data.date.slice(0, 10) : undefined,
      source: "manual",
      payment_method_id: data.payment_method_id ?? null,
    });
    invalidateDashboard();
    invalidate();
  };

  const handleDelete = async () => {
    if (!selectedTx) return;
    setDeleting(true);
    try {
      await deleteTransaction(selectedTx.id);
      setSelectedTx(null);
    } finally {
      setDeleting(false);
    }
  };

  const handleEditStart = () => {
    if (!selectedTx) return;
    setEditForm({
      amount: selectedTx.amount,
      category: selectedTx.category,
      merchant: selectedTx.merchant,
      description: selectedTx.description,
      date: selectedTx.date.slice(0, 10),
      payment_method_id: selectedTx.payment_method_id,
    });
    setIsEditing(true);
  };

  const handleEditSave = async () => {
    if (!selectedTx) return;
    const amount = editForm.amount ?? selectedTx.amount;
    const categorySlug = editForm.category ?? selectedTx.category;
    if (amount < 0 && categorySlug) {
      const cat = allCategories.find((c) => c.slug === categorySlug);
      if (cat) {
        try {
          const check = await api.get<{ exceeds: boolean; current: number; limit: number }>(
            `/account/limits/check?category_id=${cat.id}&amount=${Math.abs(amount)}`
          );
          if (check.exceeds) {
            const confirmed = window.confirm(
              `Você vai ultrapassar o limite de R$ ${check.limit.toFixed(2).replace(".", ",")} nessa categoria. Confirmar mesmo assim?`
            );
            if (!confirmed) return;
          }
        } catch {
          // Non-critical: proceed if the check fails
        }
      }
    }
    setSaving(true);
    try {
      await updateTransaction(selectedTx.id, editForm);
      setIsEditing(false);
      setSelectedTx(null);
    } finally {
      setSaving(false);
    }
  };

  const handlePayInvoice = async () => {
    if (!payingInvoice || !invoiceDebitMethodId) return;
    await payInvoice(
      payingInvoice.payment_method_id,
      payingInvoice.billing_year,
      payingInvoice.billing_month,
      invoicePayAmount,
      invoiceDebitMethodId,
    );
    setPayingInvoice(null);
    invalidate();
  };

  const debitOptions = payingInvoice
    ? paymentMethods.filter((pm) => pm.active && pm.id !== payingInvoice.payment_method_id)
    : [];

  return (
    <div className="h-[var(--app-height)] lg:h-full flex flex-col bg-background">
      <PageHeader title="Caixa" />

      {/* Saldo disponível */}
      {accountBalance !== null && (
        <div className="mx-4 mb-2 rounded-xl bg-card px-4 py-3 flex items-center justify-between">
          <div>
            <p className="text-[10px] text-muted-foreground uppercase tracking-wide font-semibold">Saldo disponível em conta</p>
            <p className={cn("text-xl font-bold tabular-nums font-heading mt-0.5", accountBalance >= 0 ? "text-blue-500" : "text-destructive")}>
              {fmt(accountBalance)}
            </p>
          </div>
          <Wallet className="w-5 h-5 text-muted-foreground" />
        </div>
      )}

      {/* Balance transfer banner */}
      {balanceTransfer && (
        <div className="mx-4 mb-2 rounded-xl bg-primary/10 border border-primary/20 p-4 flex items-center justify-between gap-3">
          <div className="min-w-0">
            <p className="text-sm font-medium">Sobrou {fmt(balanceTransfer.amount)} no mês anterior</p>
            <p className="text-xs text-muted-foreground mt-0.5">Trazer para o saldo deste mês?</p>
          </div>
          <div className="flex gap-2 shrink-0">
            <Button size="sm" variant="ghost" className="h-8 text-xs" onClick={() => setTransferDismissed(true)}>
              Ignorar
            </Button>
            <Button size="sm" className="h-8 text-xs" onClick={handleConfirmTransfer} disabled={confirmingTransfer}>
              {confirmingTransfer ? "..." : "Sim"}
            </Button>
          </div>
        </div>
      )}

      {/* Voucher deposit banners */}
      {voucherPending.map((v) => (
        <div key={v.payment_method_id} className="mx-4 mb-2 rounded-xl bg-primary/10 border border-primary/20 p-4 space-y-2">
          <p className="text-sm font-medium">Depósito do {v.name}</p>
          <p className="text-xs text-muted-foreground">Qual foi o valor recebido em {MONTHS_PT[selectedMonth - 1]}?</p>
          <div className="flex gap-2 items-center">
            <Input
              className="h-8 text-xs flex-1"
              type="number"
              value={voucherAmounts[v.payment_method_id] ?? ""}
              onChange={(e) =>
                setVoucherAmounts((prev) => ({ ...prev, [v.payment_method_id]: e.target.value }))
              }
              placeholder="0,00"
            />
            <Button size="sm" variant="ghost" className="h-8 text-xs shrink-0"
              onClick={() => setVoucherDismissed((prev) => ({ ...prev, [v.payment_method_id]: true }))}>
              Ignorar
            </Button>
            <Button size="sm" className="h-8 text-xs shrink-0"
              onClick={() => handleConfirmVoucher(v.payment_method_id)}
              disabled={confirmingVoucher === v.payment_method_id}>
              {confirmingVoucher === v.payment_method_id ? "..." : "Confirmar"}
            </Button>
          </div>
        </div>
      ))}

      {/* Recurring expenses reminders — current month only */}
      {selectedOffset === 0 && (
        <div className="mx-4 mb-2">
          <RecurringRemindersPanel year={selectedYear} month={selectedMonth} />
        </div>
      )}

      {/* Month pills */}
      <div className="flex gap-1.5 px-4 pb-2">
        {windowOffsets.map((offset) => {
          const { year, month, label } = offsetToYearMonth(offset);
          const isSelected = offset === selectedOffset;
          const isLeftEdge = offset === windowOffsets[0];
          const isRightEdge = offset === windowOffsets[4];

          return (
            <button
              key={`${year}-${month}`}
              onClick={() => {
                if (isLeftEdge && offset <= minOffset) setMinOffset((p) => p - 1);
                if (isRightEdge && offset >= maxOffset) setMaxOffset((p) => p + 1);
                setSelectedOffset(offset);
              }}
              className={cn(
                "flex-1 flex flex-col items-center py-1 rounded-full text-xs font-medium transition-colors",
                isSelected
                  ? "bg-primary text-primary-foreground"
                  : "bg-secondary text-secondary-foreground hover:bg-secondary/80",
              )}
            >
              <span>{label}</span>
              {offset === 0 && !isSelected && (
                <span className="w-1 h-1 rounded-full bg-primary mt-0.5" />
              )}
            </button>
          );
        })}
      </div>

      <div className="flex-1 overflow-y-auto px-4 pb-4 space-y-4">
        {/* Summary cards */}
        <div className={cn("grid gap-2", (summary?.reserve_transfers ?? 0) !== 0 ? "grid-cols-3" : "grid-cols-2")}>
          <div className="rounded-xl bg-card p-3 space-y-1">
            <div className="w-7 h-7 rounded-lg bg-primary/10 text-primary flex items-center justify-center">
              <ArrowDownLeft className="w-3.5 h-3.5" />
            </div>
            <p className="text-[10px] text-muted-foreground">Entradas</p>
            <p className="text-sm font-semibold tabular-nums font-heading text-primary">
              {fmt(summary?.income ?? 0)}
            </p>
          </div>
          <div className="rounded-xl bg-card p-3 space-y-1">
            <div className="w-7 h-7 rounded-lg bg-destructive/10 text-destructive flex items-center justify-center">
              <ArrowUpRight className="w-3.5 h-3.5" />
            </div>
            <p className="text-[10px] text-muted-foreground">Saídas reais</p>
            <p className="text-sm font-semibold tabular-nums font-heading text-destructive">
              {fmt(Math.abs(summary?.real_outflow ?? 0))}
            </p>
          </div>
          {(summary?.reserve_transfers ?? 0) !== 0 && (
            <div className="rounded-xl bg-card p-3 space-y-1">
              <div className="w-7 h-7 rounded-lg bg-amber-500/10 text-amber-500 flex items-center justify-center">
                <PiggyBank className="w-3.5 h-3.5" />
              </div>
              <p className="text-[10px] text-muted-foreground">Reservas</p>
              <p className="text-sm font-semibold tabular-nums font-heading text-amber-500">
                {fmt(Math.abs(summary?.reserve_transfers ?? 0))}
              </p>
            </div>
          )}
        </div>

        {/* VR balance cards — inline detail */}
        {voucherSummary.map((v) => (
          <div key={v.payment_method_id} className="rounded-xl bg-card p-3 space-y-2">
            <div className="flex items-center gap-2">
              <div className="w-7 h-7 rounded-lg flex items-center justify-center shrink-0" style={{ background: `${v.color}22` }}>
                <ShoppingCart className="w-3.5 h-3.5" style={{ color: v.color }} />
              </div>
              <p className="text-[10px] font-semibold text-muted-foreground uppercase tracking-wide">{v.name}</p>
            </div>
            <div className="grid grid-cols-2 gap-x-4 gap-y-1">
              <div>
                <p className="text-[9px] text-muted-foreground">Saldo anterior</p>
                <p className="text-xs font-semibold tabular-nums">{fmt(v.prev_balance)}</p>
              </div>
              <div>
                <p className="text-[9px] text-muted-foreground">Crédito {MONTHS_PT[selectedMonth - 1]}</p>
                <p className="text-xs font-semibold tabular-nums text-primary">{fmt(v.month_deposit)}</p>
              </div>
              <div>
                <p className="text-[9px] text-muted-foreground">Gastos {MONTHS_PT[selectedMonth - 1]}</p>
                <p className="text-xs font-semibold tabular-nums text-destructive">−{fmt(v.month_spent)}</p>
              </div>
              <div>
                <p className="text-[9px] text-muted-foreground">Saldo atual</p>
                <p className="text-xs font-bold tabular-nums" style={{ color: v.color }}>{fmt(v.prev_balance + v.month_deposit - v.month_spent)}</p>
              </div>
            </div>
          </div>
        ))}

        {/* Movimentações */}
        <div className="rounded-xl bg-card px-3 py-2">
          <p className="text-[10px] font-semibold text-muted-foreground uppercase tracking-wide mb-2">
            Movimentações
          </p>

          {isLoading && (
            <p className="text-xs text-muted-foreground text-center py-4">Carregando...</p>
          )}

          {/* Real transactions (pix/débito/receitas/invoice_payment/reserve) */}
          {realTxs.map((tx) => {
            const isReserve = tx.type === "transfer";
            const Icon = txIcon(tx.category, tx.type);
            const isIncome = tx.amount > 0;
            return (
              <div
                key={tx.id}
                className="flex items-start gap-2 py-2 border-b border-border last:border-0 cursor-pointer hover:bg-secondary/40 rounded-lg px-1 -mx-1 transition-colors"
                onClick={() => setSelectedTx(tx)}
              >
                <div className={cn(
                  "w-7 h-7 rounded-lg flex items-center justify-center shrink-0",
                  isReserve ? "bg-amber-500/10 text-amber-500"
                    : isIncome ? "bg-primary/10 text-primary"
                    : "bg-destructive/10 text-destructive",
                )}>
                  <Icon className="w-4 h-4" />
                </div>
                <div className="flex-1 min-w-0">
                  <p className="text-xs font-semibold">{tx.merchant || tx.description || (tx.category === "pagamento_fatura" ? "Pagamento de Fatura" : tx.category)}</p>
                  <p className="text-[10px] text-muted-foreground mt-0.5 capitalize">
                    {new Date(tx.date).toLocaleDateString("pt-BR", { day: "2-digit", month: "short" })}
                    {isReserve ? " · reserva" : tx.kind ? ` · ${tx.kind}` : ""}
                  </p>
                </div>
                {(() => {
                  const badge = getTransactionBadge(tx.source);
                  return (
                    <span
                      className="text-[10px] font-semibold rounded-md px-[7px] py-[2px] shrink-0"
                      style={{ backgroundColor: badge.color + "33", color: badge.color }}
                    >
                      {badge.icon} {badge.label}
                    </span>
                  );
                })()}
                <span className={cn(
                  "text-xs font-bold tabular-nums shrink-0",
                  isReserve ? "text-amber-500"
                    : tx.amount > 0 ? "text-primary"
                    : "text-destructive"
                )}>
                  {tx.amount > 0 ? "+" : "−"}{fmt(Math.abs(tx.amount))}
                </span>
              </div>
            );
          })}

          {/* Divider antes das faturas */}
          {invoices.length > 0 && realTxs.length > 0 && (
            <div className="my-2 border-t border-dashed border-border" />
          )}

          {/* Faturas de crédito */}
          {invoices.map((inv) => (
            <InvoiceCard
              key={`${inv.payment_method_id}-${inv.billing_year}-${inv.billing_month}`}
              invoice={inv}
              onPay={() => {
                setInvoicePayAmount(Math.abs(inv.total));
                setInvoiceDebitMethodId(null);
                setPayingInvoice(inv);
              }}
              fmt={fmt}
            />
          ))}

          {!isLoading && realTxs.length === 0 && invoices.length === 0 && (
            <p className="text-xs text-muted-foreground text-center py-4">
              Nenhuma movimentação neste mês.
            </p>
          )}
        </div>
      </div>

      {/* Modal de pagamento de fatura */}
      {payingInvoice && (
        <Sheet open onOpenChange={(open) => { if (!open) setPayingInvoice(null); }}>
          <SheetContent side="bottom" className="rounded-t-2xl pb-8">
            <SheetHeader className="mb-4">
              <SheetTitle>Pagar fatura {payingInvoice.name}</SheetTitle>
            </SheetHeader>
            <p className="text-xs text-muted-foreground mb-3">
              Fatura {MONTHS_PT[payingInvoice.billing_month - 1]}/{payingInvoice.billing_year} —
              total no app {fmt(Math.abs(payingInvoice.total))}
            </p>
            <div className="space-y-1 mb-3">
              <label className="text-xs text-muted-foreground">Valor pago</label>
              <Input
                type="number"
                step="0.01"
                min="0.01"
                value={invoicePayAmount}
                onChange={(e) => setInvoicePayAmount(parseFloat(e.target.value) || 0)}
                className="h-9"
              />
            </div>
            {invoicePayAmount > Math.abs(payingInvoice.total) && (
              <p className="text-xs text-muted-foreground mb-3">
                {fmt(invoicePayAmount - Math.abs(payingInvoice.total))} serão lançados como compras anteriores no cartão.
              </p>
            )}
            <div className="space-y-1 mb-3">
              <label className="text-xs text-muted-foreground">Pagar com</label>
              <select
                className="w-full h-9 rounded-md border border-input bg-background px-3 text-sm"
                value={invoiceDebitMethodId ?? ""}
                onChange={(e) => setInvoiceDebitMethodId(e.target.value || null)}
              >
                <option value="">Selecionar...</option>
                {debitOptions.map((pm) => (
                  <option key={pm.id} value={pm.id}>{pm.name}</option>
                ))}
              </select>
            </div>
            <div className="flex gap-2">
              <Button variant="outline" className="flex-1" onClick={() => setPayingInvoice(null)}>
                Cancelar
              </Button>
              <Button
                className="flex-1"
                onClick={handlePayInvoice}
                disabled={invoicePayAmount <= 0 || invoiceDebitMethodId === null}
              >
                Confirmar pagamento
              </Button>
            </div>
          </SheetContent>
        </Sheet>
      )}

      {/* Sheet de detalhe/edição de transação */}
      <Sheet open={!!selectedTx} onOpenChange={(open) => { if (!open) { setSelectedTx(null); setIsEditing(false); } }}>
        <SheetContent side="bottom" className="rounded-t-2xl pb-8">
          {selectedTx && (
            <>
              <SheetHeader className="mb-4">
                <SheetTitle className="text-base capitalize">
                  {isEditing ? "Editar transação" : (selectedTx.merchant
                    ? `${selectedTx.merchant} · ${selectedTx.category}`
                    : selectedTx.category)}
                </SheetTitle>
              </SheetHeader>

              {isEditing ? (
                <div className="space-y-3">
                  <div className="space-y-1">
                    <label className="text-xs text-muted-foreground">Valor</label>
                    <Input
                      type="number"
                      step="0.01"
                      value={editForm.amount ?? ""}
                      onChange={(e) => setEditForm((f) => ({ ...f, amount: parseFloat(e.target.value) }))}
                      className="h-9"
                    />
                  </div>
                  <div className="space-y-1">
                    <label className="text-xs text-muted-foreground">Categoria</label>
                    <select
                      value={editForm.category ?? ""}
                      onChange={(e) => setEditForm((f) => ({ ...f, category: e.target.value }))}
                      className="w-full h-9 rounded-md border border-input bg-background px-3 text-sm ring-offset-background focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2"
                    >
                      <option value="">Selecionar categoria</option>
                      <optgroup label="Receitas">
                        {allCategories.filter(c => c.type === "income").map(c => (
                          <option key={c.slug} value={c.slug}>{c.label}</option>
                        ))}
                      </optgroup>
                      <optgroup label="Despesas">
                        {allCategories.filter(c => c.type === "expense").map(c => (
                          <option key={c.slug} value={c.slug}>{c.label}</option>
                        ))}
                      </optgroup>
                    </select>
                  </div>
                  <div className="space-y-1">
                    <label className="text-xs text-muted-foreground">Estabelecimento</label>
                    <Input
                      value={editForm.merchant ?? ""}
                      onChange={(e) => setEditForm((f) => ({ ...f, merchant: e.target.value }))}
                      className="h-9"
                    />
                  </div>
                  <div className="space-y-1">
                    <label className="text-xs text-muted-foreground">Descrição</label>
                    <Input
                      value={editForm.description ?? ""}
                      onChange={(e) => setEditForm((f) => ({ ...f, description: e.target.value }))}
                      className="h-9"
                    />
                  </div>
                  <div className="space-y-1">
                    <label className="text-xs text-muted-foreground">Data</label>
                    <Input
                      type="date"
                      value={editForm.date ?? ""}
                      onChange={(e) => setEditForm((f) => ({ ...f, date: e.target.value }))}
                      className="h-9"
                    />
                  </div>
                  {selectedTx.amount < 0 && selectedTx.type !== "transfer" && (
                    <div className="space-y-1">
                      <label className="text-xs text-muted-foreground">Forma de pagamento</label>
                      <select
                        value={editForm.payment_method_id ?? ""}
                        onChange={(e) => setEditForm((f) => ({ ...f, payment_method_id: e.target.value || undefined }))}
                        className="w-full h-9 rounded-md border border-input bg-background px-3 text-sm ring-offset-background focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2"
                      >
                        <option value="">Selecionar</option>
                        {paymentMethods.map((m) => (
                          <option key={m.id} value={m.id}>{m.name}</option>
                        ))}
                      </select>
                    </div>
                  )}
                  <div className="flex gap-2 mt-2">
                    <Button
                      variant="outline"
                      className="flex-1"
                      onClick={() => setIsEditing(false)}
                      disabled={saving}
                    >
                      Cancelar
                    </Button>
                    <Button
                      className="flex-1"
                      onClick={handleEditSave}
                      disabled={saving}
                    >
                      {saving ? "Salvando..." : "Salvar"}
                    </Button>
                  </div>
                </div>
              ) : (
                <div className="space-y-3">
                  <div className="flex justify-between text-sm">
                    <span className="text-muted-foreground">Categoria</span>
                    <span className="font-medium capitalize">{selectedTx.category}</span>
                  </div>
                  {selectedTx.payment_method_id && (
                    <div className="flex justify-between text-sm">
                      <span className="text-muted-foreground">Forma de pagamento</span>
                      <span className="font-medium capitalize">
                        {paymentMethods.find((m) => m.id === selectedTx.payment_method_id)?.name ?? "—"}
                      </span>
                    </div>
                  )}
                  <div className="flex justify-between text-sm">
                    <span className="text-muted-foreground">Valor</span>
                    <span className={cn(
                      "font-semibold tabular-nums",
                      selectedTx.amount > 0 ? "text-primary" : "text-foreground"
                    )}>
                      {selectedTx.amount > 0 ? "+" : ""}R$ {Math.abs(selectedTx.amount).toFixed(2).replace(".", ",")}
                    </span>
                  </div>
                  <div className="flex justify-between text-sm">
                    <span className="text-muted-foreground">Data</span>
                    <span>{new Date(selectedTx.date.slice(0, 10) + "T12:00:00").toLocaleDateString("pt-BR")}</span>
                  </div>
                  {selectedTx.description && (
                    <div className="flex justify-between text-sm">
                      <span className="text-muted-foreground">Descrição</span>
                      <span className="text-right max-w-[60%]">{selectedTx.description}</span>
                    </div>
                  )}
                  {selectedTx.image_url && (
                    <a
                      href={selectedTx.image_url}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="block mt-2"
                    >
                      <img
                        src={selectedTx.image_url}
                        alt="Nota fiscal"
                        className="w-full rounded-xl object-cover max-h-48 border border-border"
                      />
                      <p className="text-xs text-muted-foreground text-center mt-1">Toque para abrir em tamanho real</p>
                    </a>
                  )}
                  <div className="flex gap-2 mt-2">
                    <Button
                      variant="outline"
                      className="flex-1"
                      onClick={handleEditStart}
                    >
                      <Pencil className="w-4 h-4 mr-2" />
                      Editar
                    </Button>
                    <Button
                      variant="destructive"
                      className="flex-1"
                      onClick={handleDelete}
                      disabled={deleting}
                    >
                      <Trash2 className="w-4 h-4 mr-2" />
                      {deleting ? "Excluindo..." : "Excluir"}
                    </Button>
                  </div>
                </div>
              )}
            </>
          )}
        </SheetContent>
      </Sheet>

      <BottomNav />
    </div>
  );
}
