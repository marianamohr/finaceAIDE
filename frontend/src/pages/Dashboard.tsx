import { useState, useEffect } from "react";
import { BottomNav } from "@/components/BottomNav/BottomNav";
import { PageHeader } from "@/components/PageHeader/PageHeader";
import { TransactionCard } from "@/components/TransactionCard/TransactionCard";
import { ArrowDownLeft, ArrowUpRight, Wallet, Trash2, ChevronDown, ChevronUp, Pencil } from "lucide-react";
import { RecurringRemindersPanel } from "@/components/RecurringRemindersPanel/RecurringRemindersPanel";
import { cn } from "@/lib/utils";
import { useDashboardQuery, useInvalidateDashboard, usePrefetchNextMonth, Transaction, PaymentMode } from "@/hooks/useDashboard";
import { Sheet, SheetContent, SheetHeader, SheetTitle } from "@/components/ui/sheet";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { api } from "@/services/api";
import {
  Pagination,
  PaginationContent,
  PaginationEllipsis,
  PaginationItem,
  PaginationLink,
  PaginationNext,
  PaginationPrevious,
} from "@/components/ui/pagination";
import { useCurrencyStore, formatCurrency, maskCurrency } from "@/store/currency";
import { usePrivacyStore } from "@/store/privacy";

const CATEGORY_COLORS = [
  "bg-primary",
  "bg-accent",
  "bg-warning",
  "bg-destructive",
  "bg-muted-foreground",
  "bg-secondary-foreground",
];

const PAGE_SIZE = 10;

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

function buildPageNumbers(current: number, total: number): (number | "ellipsis")[] {
  if (total <= 7) return Array.from({ length: total }, (_, i) => i + 1);
  const pages: (number | "ellipsis")[] = [];
  pages.push(1);
  if (current > 3) pages.push("ellipsis");
  for (let p = Math.max(2, current - 1); p <= Math.min(total - 1, current + 1); p++) {
    pages.push(p);
  }
  if (current < total - 2) pages.push("ellipsis");
  pages.push(total);
  return pages;
}

export default function Dashboard() {
  const fmt = useFmt();
  const [selectedOffset, setSelectedOffset] = useState(0);
  const [minOffset, setMinOffset] = useState(-12);
  const [maxOffset, setMaxOffset] = useState(2);

  const { year: selectedYear, month: selectedMonth } = offsetToYearMonth(selectedOffset);

  const [selectedTx, setSelectedTx] = useState<Transaction | null>(null);
  const [selectedCategories, setSelectedCategories] = useState<string[]>([]);
  const [deleting, setDeleting] = useState(false);
  const [isEditing, setIsEditing] = useState(false);
  const [editForm, setEditForm] = useState<Partial<Transaction>>({});
  const [saving, setSaving] = useState(false);
  const [allCategories, setAllCategories] = useState<{ id: string; slug: string; label: string; type: string }[]>([]);

  useEffect(() => {
    api.get<{ id: string; slug: string; label: string; type: string }[]>("/categories")
      .then(setAllCategories).catch(() => {});
  }, []);

  const [paymentModes, setPaymentModes] = useState<PaymentMode[]>([]);

  useEffect(() => {
    api.get<PaymentMode[]>("/payment-modes")
      .then(setPaymentModes).catch(() => {});
  }, []);
  const [showAllCategories, setShowAllCategories] = useState(false);
  const [currentPage, setCurrentPage] = useState(1);
  const [confirmingTransfer, setConfirmingTransfer] = useState(false);

  const invalidateDashboard = useInvalidateDashboard();
  const { data: dashData, isLoading: loading } = useDashboardQuery(selectedYear, selectedMonth);

  const summary = dashData?.summary ?? null;
  const transactions = dashData?.transactions ?? [];
  const balanceTransfer = dashData?.balanceTransfer?.pending ? dashData.balanceTransfer : null;

  const { year: maxYear, month: maxMonth } = offsetToYearMonth(maxOffset);
  const hasNextMonth = usePrefetchNextMonth(maxYear, maxMonth);

  // Reset filters when month changes
  useEffect(() => {
    setSelectedCategories([]);
    setShowAllCategories(false);
  }, [selectedOffset]);

  // Reset pagination when filter or month changes
  useEffect(() => {
    setCurrentPage(1);
  }, [selectedCategories, selectedOffset]);

  const rawCategories = summary ? Object.entries(summary.categories) : [];
  const hasNegative = rawCategories.some(([, v]) => v < 0);
  const allExpenseCategories = rawCategories
    .filter(([, v]) => hasNegative ? v < 0 : true)
    .sort((a, b) => Math.abs(b[1]) - Math.abs(a[1]));

  const expenseCategories = showAllCategories
    ? allExpenseCategories
    : allExpenseCategories.slice(0, 5);

  const totalExpenses = allExpenseCategories.reduce((acc, [, v]) => acc + Math.abs(v), 0);
  const hasMoreCategories = allExpenseCategories.length > 5;

  const filteredTransactions = selectedCategories.length > 0
    ? transactions.filter((tx) => selectedCategories.includes(tx.category))
    : transactions;

  const selectedTotal = selectedCategories.length > 0
    ? allExpenseCategories
        .filter(([cat]) => selectedCategories.includes(cat))
        .reduce((acc, [, v]) => acc + Math.abs(v), 0)
    : null;

  const totalPages = Math.ceil(filteredTransactions.length / PAGE_SIZE);
  const paginatedTransactions = filteredTransactions.slice(
    (currentPage - 1) * PAGE_SIZE,
    currentPage * PAGE_SIZE
  );

  const windowOffsets = [
    selectedOffset - 2,
    selectedOffset - 1,
    selectedOffset,
    selectedOffset + 1,
    selectedOffset + 2,
  ];

  const summaryCards = [
    { label: "Receitas", value: summary?.income ?? 0, icon: ArrowDownLeft, variant: "income" as const },
    { label: "Despesas", value: Math.abs(summary?.expenses ?? 0), icon: ArrowUpRight, variant: "expense" as const },
    { label: "Saldo", value: summary?.balance ?? 0, icon: Wallet, variant: "balance" as const },
  ];

  const deleteTransaction = async (id: string) => {
    await api.delete(`/transactions/${id}`);
    invalidateDashboard();
  };

  const updateTransaction = async (id: string, data: Partial<Omit<Transaction, "id" | "source" | "image_url">>) => {
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
  };

  const handleConfirmTransfer = async () => {
    if (!balanceTransfer) return;
    setConfirmingTransfer(true);
    try {
      await api.confirmBalanceTransfer(balanceTransfer.amount, selectedYear, selectedMonth);
      invalidateDashboard();
    } finally {
      setConfirmingTransfer(false);
    }
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
    // Check category spending limit before saving an expense
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
          // Non-critical: proceed with the transaction if the check fails
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

  return (
    <div className="h-[var(--app-height)] lg:h-full flex flex-col bg-background">
      <PageHeader title="Dashboard" />

      {/* Month pills */}
      <div className="flex gap-1.5 px-4 pb-2">
        {windowOffsets.map((offset) => {
          const { year, month, label } = offsetToYearMonth(offset);
          const isSelected = offset === selectedOffset;
          const isToday = offset === 0;
          const isLeftEdge = offset === windowOffsets[0];
          const isRightEdge = offset === windowOffsets[4];
          const isDisabled = isRightEdge && offset >= maxOffset && !hasNextMonth;

          const handleClick = () => {
            if (isDisabled) return;
            if (isLeftEdge && offset <= minOffset) {
              setMinOffset((prev) => prev - 1);
            }
            if (isRightEdge && offset >= maxOffset) {
              setMaxOffset((prev) => prev + 1);
            }
            setSelectedOffset(offset);
          };

          return (
            <button
              key={`${year}-${month}`}
              onClick={handleClick}
              disabled={isDisabled}
              className={cn(
                "flex-1 flex flex-col items-center py-1 rounded-full text-xs font-medium transition-colors",
                isSelected
                  ? "bg-primary text-primary-foreground"
                  : "bg-secondary text-secondary-foreground hover:bg-secondary/80",
                isDisabled && "opacity-40 pointer-events-none"
              )}
            >
              <span>{label}</span>
              {isToday && !isSelected && (
                <span className="w-1 h-1 rounded-full bg-primary mt-0.5" />
              )}
            </button>
          );
        })}
      </div>

      {/* Scrollable content */}
      <div className="flex-1 overflow-y-auto px-4 pb-4 space-y-4">
        {/* Summary Cards */}
        <div className="grid grid-cols-3 gap-2">
          {summaryCards.map(({ label, value, icon: Icon, variant }) => (
            <div key={label} className="rounded-xl bg-card p-3 space-y-1">
              <div className={cn(
                "w-7 h-7 rounded-lg flex items-center justify-center",
                variant === "income" && "bg-primary/10 text-primary",
                variant === "expense" && "bg-destructive/10 text-destructive",
                variant === "balance" && "bg-secondary text-foreground",
              )}>
                <Icon className="w-3.5 h-3.5" />
              </div>
              <p className="text-[10px] text-muted-foreground">{label}</p>
              <p className="text-sm font-semibold tabular-nums font-heading">
                {fmt(value)}
              </p>
            </div>
          ))}
        </div>

        {/* Balance transfer banner */}
        {balanceTransfer && (
          <div className="rounded-xl bg-primary/10 border border-primary/20 p-4 flex items-center justify-between gap-3">
            <div className="min-w-0">
              <p className="text-sm font-medium">Sobrou {fmt(balanceTransfer.amount)} no mês anterior</p>
              <p className="text-xs text-muted-foreground mt-0.5">Trazer para o saldo deste mês?</p>
            </div>
            <div className="flex gap-2 shrink-0">
              <Button
                size="sm"
                variant="ghost"
                className="h-8 text-xs"
                onClick={() => setBalanceTransfer(null)}
              >
                Ignorar
              </Button>
              <Button
                size="sm"
                className="h-8 text-xs"
                onClick={handleConfirmTransfer}
                disabled={confirmingTransfer}
              >
                {confirmingTransfer ? "..." : "Sim"}
              </Button>
            </div>
          </div>
        )}

        {/* Recurring expenses reminders — current month only */}
        {selectedOffset === 0 && (
          <RecurringRemindersPanel year={selectedYear} month={selectedMonth} initialData={dashData?.recurringReminders} />
        )}

        {/* Category breakdown */}
        <div className="rounded-xl bg-card p-4">
          <div className="flex items-center justify-between mb-3">
            <h2 className="font-heading text-sm font-semibold">Gastos por categoria</h2>
            {selectedCategories.length > 0 && (
              <button
                onClick={() => setSelectedCategories([])}
                className="text-xs text-muted-foreground underline"
              >
                Limpar filtro
              </button>
            )}
          </div>
          {selectedTotal !== null && (
            <p className="text-xs text-muted-foreground mb-2">
              Total selecionado: <span className="font-semibold text-foreground">{fmt(selectedTotal)}</span>
            </p>
          )}
          {loading && <p className="text-xs text-muted-foreground">Carregando...</p>}
          {!loading && allExpenseCategories.length === 0 && (
            <p className="text-xs text-muted-foreground">Nenhum gasto registrado neste mês.</p>
          )}
          <div className="space-y-2.5">
            {expenseCategories.map(([cat, total], i) => {
              const pct = totalExpenses > 0
                ? Math.round((Math.abs(total) / totalExpenses) * 100)
                : 0;
              const isSelected = selectedCategories.includes(cat);
              return (
                <button
                  key={cat}
                  onClick={() => setSelectedCategories((prev) =>
                    prev.includes(cat) ? prev.filter((c) => c !== cat) : [...prev, cat]
                  )}
                  className={cn(
                    "w-full text-left rounded-lg px-2 py-1.5 transition-colors",
                    isSelected ? "bg-secondary" : "hover:bg-secondary/50"
                  )}
                >
                  <div className="flex justify-between text-xs mb-1">
                    <span className={cn("capitalize font-medium", isSelected && "text-foreground")}>{cat}</span>
                    <span className={cn("tabular-nums", isSelected ? "text-foreground font-semibold" : "text-muted-foreground")}>
                      {fmt(total)} · {pct}%
                    </span>
                  </div>
                  <div className="h-1.5 rounded-full bg-secondary overflow-hidden">
                    <div
                      className={cn("h-full rounded-full transition-all", CATEGORY_COLORS[i % CATEGORY_COLORS.length])}
                      style={{ width: `${pct}%` }}
                    />
                  </div>
                </button>
              );
            })}
          </div>

          {hasMoreCategories && (
            <button
              onClick={() => setShowAllCategories((v) => !v)}
              className="mt-3 flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground transition-colors mx-auto"
            >
              {showAllCategories ? (
                <><ChevronUp className="w-3.5 h-3.5" /> Ver menos</>
              ) : (
                <><ChevronDown className="w-3.5 h-3.5" /> Ver todas as categorias ({allExpenseCategories.length})</>
              )}
            </button>
          )}
        </div>

        {/* Transactions */}
        <div>
          <h2 className="font-heading text-sm font-semibold mb-2">
            {selectedCategories.length === 1 ? (
              <span className="capitalize">{selectedCategories[0]}</span>
            ) : selectedCategories.length > 1 ? (
              `${selectedCategories.length} categorias`
            ) : "Transações"}{" "}
            {!loading && filteredTransactions.length > 0 && (
              <span className="text-muted-foreground font-normal">({filteredTransactions.length})</span>
            )}
          </h2>
          {loading && <p className="text-xs text-muted-foreground">Carregando...</p>}
          {!loading && filteredTransactions.length === 0 && (
            <p className="text-xs text-muted-foreground">
              {selectedCategories.length > 0 ? "Nenhuma transação nestas categorias." : "Nenhuma transação neste mês."}
            </p>
          )}
          <div className="space-y-1.5">
            {paginatedTransactions.map((tx) => (
              <TransactionCard
                key={tx.id}
                {...tx}
                onClick={() => setSelectedTx(tx)}
              />
            ))}
          </div>

          {totalPages > 1 && (
            <div className="mt-4">
              <Pagination>
                <PaginationContent>
                  <PaginationItem>
                    <PaginationPrevious
                      onClick={() => setCurrentPage((p) => Math.max(1, p - 1))}
                      className={currentPage === 1 ? "pointer-events-none opacity-40" : "cursor-pointer"}
                    />
                  </PaginationItem>

                  {buildPageNumbers(currentPage, totalPages).map((p, i) =>
                    p === "ellipsis" ? (
                      <PaginationItem key={`ellipsis-${i}`}>
                        <PaginationEllipsis />
                      </PaginationItem>
                    ) : (
                      <PaginationItem key={p}>
                        <PaginationLink
                          isActive={p === currentPage}
                          onClick={() => setCurrentPage(p)}
                          className="cursor-pointer"
                        >
                          {p}
                        </PaginationLink>
                      </PaginationItem>
                    )
                  )}

                  <PaginationItem>
                    <PaginationNext
                      onClick={() => setCurrentPage((p) => Math.min(totalPages, p + 1))}
                      className={currentPage === totalPages ? "pointer-events-none opacity-40" : "cursor-pointer"}
                    />
                  </PaginationItem>
                </PaginationContent>
              </Pagination>
            </div>
          )}
        </div>
      </div>

      <BottomNav />

      {/* Transaction detail sheet */}
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
                  {selectedTx && selectedTx.amount < 0 && selectedTx.type !== "transfer" && (
                    <div className="space-y-1">
                      <label className="text-xs text-muted-foreground">Forma de pagamento</label>
                      <select
                        value={editForm.payment_method_id ?? ""}
                        onChange={(e) => setEditForm((f) => ({ ...f, payment_method_id: e.target.value || undefined }))}
                        className="w-full h-9 rounded-md border border-input bg-background px-3 text-sm ring-offset-background focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2"
                      >
                        <option value="">Selecionar</option>
                        {paymentModes.map((m) => (
                          <option key={m.id} value={m.id}>{m.label}</option>
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
                        {paymentModes.find((m) => m.id === selectedTx.payment_method_id)?.label ?? "—"}
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
    </div>
  );
}
