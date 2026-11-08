import { useState } from "react"
import { BottomNav } from "@/components/BottomNav/BottomNav"
import { PageHeader } from "@/components/PageHeader/PageHeader"
import { useRecurringReminders, RecurringExpenseWithStatus } from "@/hooks/useRecurringExpenses"
import { usePaymentMethods, PaymentMethod } from "@/hooks/usePaymentMethods"
import { useCurrencyStore, formatCurrency, maskCurrency, currencySymbol } from "@/store/currency"
import { usePrivacyStore } from "@/store/privacy"
import { categoryEmoji } from "@/lib/categories"
import { CalendarDays, Check } from "lucide-react"
import { cn } from "@/lib/utils"
import { toast } from "sonner"
import { PaymentMethodSheet } from "@/components/PaymentMethodSheet/PaymentMethodSheet"

function useFmt() {
  const { currency } = useCurrencyStore()
  const hidden = usePrivacyStore((s) => s.valuesHidden)
  return (v: number) =>
    hidden ? maskCurrency(v, currency) : formatCurrency(v, currency)
}

function ReminderItem({
  item,
  fmt,
  onPay,
  onUnpay,
}: {
  item: RecurringExpenseWithStatus
  fmt: (v: number) => string
  onPay: () => void
  onUnpay: () => void
}) {
  return (
    <div
      className={cn(
        "flex items-center gap-3 p-3.5 rounded-xl border bg-card",
        item.paid ? "border-border opacity-60" : "border-border"
      )}
    >
      <div className="w-9 h-9 rounded-lg bg-secondary flex items-center justify-center shrink-0 text-base">
        {item.category_slug ? categoryEmoji(item.category_slug) : "💸"}
      </div>
      <div className="flex-1 min-w-0">
        <p className={cn("text-sm font-semibold truncate", item.paid && "line-through text-muted-foreground")}>
          {item.name}
        </p>
        <div className="flex items-center gap-2 mt-0.5">
          <span className="text-xs text-muted-foreground">
            {fmt(item.paid && item.paid_amount != null ? item.paid_amount : item.amount)}
          </span>
          {item.is_variable && !item.paid && (
            <span className="text-[10px] font-medium text-muted-foreground bg-secondary rounded px-1.5 py-0.5">estimado</span>
          )}
          {item.due_day != null && (
            <span className="flex items-center gap-0.5 text-xs text-muted-foreground">
              <CalendarDays className="w-3 h-3" />
              vence dia {item.due_day}
            </span>
          )}
        </div>
      </div>
      {item.paid ? (
        <button
          onClick={onUnpay}
          className="flex items-center gap-1 text-xs text-muted-foreground hover:text-destructive transition-colors shrink-0"
        >
          <Check className="w-4 h-4 text-primary" />
          <span>Desfazer</span>
        </button>
      ) : (
        <button
          onClick={onPay}
          className="shrink-0 rounded-lg bg-primary text-primary-foreground text-xs font-semibold px-3 py-1.5 hover:bg-primary/90 transition-colors"
        >
          Pagar
        </button>
      )}
    </div>
  )
}

export default function GastosFixosLembretes() {
  const fmt = useFmt()
  const { currency } = useCurrencyStore()
  const symbol = currencySymbol(currency)
  const now = new Date()
  const year = now.getFullYear()
  const month = now.getMonth() + 1
  const { reminders, isLoading, pendingCount, markPaid, unmarkPaid } = useRecurringReminders(year, month)
  const { paymentMethods } = usePaymentMethods()

  const [pendingItem, setPendingItem] = useState<RecurringExpenseWithStatus | null>(null)

  const monthLabel = now.toLocaleDateString("pt-BR", { month: "long", year: "numeric" })

  const handlePayClick = (item: RecurringExpenseWithStatus) => {
    setPendingItem(item)
  }

  const handlePayConfirm = async (pm: PaymentMethod | null, amount?: number) => {
    if (!pendingItem) return
    const item = pendingItem
    setPendingItem(null)
    try {
      await markPaid(item.id, pm?.id, amount)
      toast.success(`${item.name} marcado como pago`)
    } catch {
      toast.error("Erro ao marcar como pago")
    }
  }

  const handleUnpay = async (item: RecurringExpenseWithStatus) => {
    try {
      await unmarkPaid(item.id)
      toast.success(`${item.name} desmarcado`)
    } catch {
      toast.error("Erro ao desmarcar pagamento")
    }
  }

  return (
    <div className="min-h-screen bg-background flex flex-col">
      <PageHeader title="Contas do Mês" />

      <main className="flex-1 overflow-y-auto px-4 pb-24 pt-4 space-y-3 max-w-lg mx-auto w-full">
        <p className="text-xs text-muted-foreground capitalize px-0.5">{monthLabel}</p>

        {isLoading ? (
          <div className="flex justify-center py-12">
            <div className="w-6 h-6 border-2 border-primary border-t-transparent rounded-full animate-spin" />
          </div>
        ) : reminders.length === 0 ? (
          <div className="text-center py-12 text-muted-foreground text-sm">
            <p className="text-2xl mb-2">📋</p>
            <p>Nenhum gasto fixo cadastrado.</p>
            <p className="mt-1 text-xs">Acesse Perfil → Gastos Fixos para adicionar.</p>
          </div>
        ) : (
          <>
            {pendingCount === 0 ? (
              <div className="flex items-center gap-2 rounded-xl bg-primary/10 border border-primary/20 px-4 py-3">
                <Check className="w-4 h-4 text-primary shrink-0" />
                <p className="text-sm text-primary font-medium">Todos os gastos fixos pagos este mês! 🎉</p>
              </div>
            ) : (
              <div className="flex items-center gap-2 rounded-xl bg-warning/10 border border-warning/20 px-4 py-3">
                <p className="text-sm text-warning font-medium">
                  {pendingCount} {pendingCount === 1 ? "conta pendente" : "contas pendentes"}
                </p>
              </div>
            )}

            {reminders.map((item) => (
              <ReminderItem
                key={item.id}
                item={item}
                fmt={fmt}
                onPay={() => handlePayClick(item)}
                onUnpay={() => handleUnpay(item)}
              />
            ))}
          </>
        )}
      </main>

      {pendingItem && (
        <PaymentMethodSheet
          paymentMethods={paymentMethods}
          onSelect={handlePayConfirm}
          onClose={() => setPendingItem(null)}
          askAmount={pendingItem.is_variable}
          initialAmount={pendingItem.amount}
          currencySymbol={symbol}
        />
      )}

      <BottomNav />
    </div>
  )
}
