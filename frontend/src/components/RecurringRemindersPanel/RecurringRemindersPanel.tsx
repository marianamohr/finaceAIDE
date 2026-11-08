import { useState } from "react"
import { useNavigate } from "react-router-dom"
import { useRecurringReminders, RecurringExpenseWithStatus } from "@/hooks/useRecurringExpenses"
import { usePaymentMethods, PaymentMethod } from "@/hooks/usePaymentMethods"
import { useInvalidateDashboard } from "@/hooks/useDashboard"
import { useCurrencyStore, formatCurrency, maskCurrency, currencySymbol } from "@/store/currency"
import { usePrivacyStore } from "@/store/privacy"
import { categoryEmoji } from "@/lib/categories"
import { CalendarCheck, ChevronDown, ChevronUp, CalendarDays, Check } from "lucide-react"
import { cn } from "@/lib/utils"
import { toast } from "sonner"
import { PaymentMethodSheet } from "@/components/PaymentMethodSheet/PaymentMethodSheet"

function useFmt() {
  const { currency } = useCurrencyStore()
  const hidden = usePrivacyStore((s) => s.valuesHidden)
  return (v: number) =>
    hidden ? maskCurrency(v, currency) : formatCurrency(v, currency)
}

function ReminderRow({
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
    <div className="flex items-center gap-3 py-2.5 border-t border-border first:border-t-0">
      <span className="text-base shrink-0">
        {item.category_slug ? categoryEmoji(item.category_slug) : "💸"}
      </span>
      <div className="flex-1 min-w-0">
        <p className={cn("text-sm truncate", item.paid && "line-through text-muted-foreground")}>
          {item.name}
        </p>
        <div className="flex items-center gap-2">
          <span className="text-xs text-muted-foreground">
            {fmt(item.paid && item.paid_amount != null ? item.paid_amount : item.amount)}
          </span>
          {item.is_variable && !item.paid && (
            <span className="text-[10px] font-medium text-muted-foreground bg-secondary rounded px-1.5 py-0.5">estimado</span>
          )}
          {item.due_day != null && (
            <span className="flex items-center gap-0.5 text-xs text-muted-foreground">
              <CalendarDays className="w-3 h-3" />
              dia {item.due_day}
            </span>
          )}
        </div>
      </div>
      {item.paid ? (
        <button
          onClick={onUnpay}
          className="flex items-center gap-1 text-xs text-muted-foreground hover:text-destructive transition-colors shrink-0"
          aria-label="Desfazer pagamento"
        >
          <Check className="w-4 h-4 text-primary" />
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

interface RecurringRemindersPanelProps {
  year: number
  month: number
  initialData?: Array<{
    id: string
    name: string
    amount: number
    category_id?: string
    category_slug?: string
    due_day?: number
    is_variable?: boolean
    paid: boolean
    transaction_id?: string
    paid_amount?: number
  }>
}

export function RecurringRemindersPanel({ year, month, initialData }: RecurringRemindersPanelProps) {
  const fmt = useFmt()
  const { currency } = useCurrencyStore()
  const symbol = currencySymbol(currency)
  const navigate = useNavigate()
  const invalidateDashboard = useInvalidateDashboard()
  const { reminders, isLoading, pendingCount, markPaid, unmarkPaid } = useRecurringReminders(year, month)
  const { paymentMethods } = usePaymentMethods()
  const hasPending = initialData
    ? initialData.some((r) => !r.paid)
    : reminders.some((r) => !r.paid)
  const [expanded, setExpanded] = useState(hasPending)
  const [pendingItem, setPendingItem] = useState<RecurringExpenseWithStatus | null>(null)

  // Use initialData from the dashboard query while the hook's own fetch is in-flight
  const displayReminders: RecurringExpenseWithStatus[] =
    isLoading && initialData
      ? (initialData as RecurringExpenseWithStatus[])
      : reminders

  if ((isLoading && !initialData) || displayReminders.length === 0) return null

  const handlePayClick = (item: RecurringExpenseWithStatus) => {
    setPendingItem(item)
  }

  const handlePayConfirm = async (pm: PaymentMethod | null, amount?: number) => {
    if (!pendingItem) return
    const item = pendingItem
    setPendingItem(null)
    try {
      await markPaid(item.id, pm?.id, amount)
      invalidateDashboard()
      toast.success(`${item.name} marcado como pago`)
    } catch {
      toast.error("Erro ao marcar como pago")
    }
  }

  const handleUnpay = async (item: RecurringExpenseWithStatus) => {
    try {
      await unmarkPaid(item.id)
      invalidateDashboard()
      toast.success(`${item.name} desmarcado`)
    } catch {
      toast.error("Erro ao desmarcar pagamento")
    }
  }

  const displayPendingCount = displayReminders.filter((r) => !r.paid).length

  return (
    <>
      <div className="rounded-xl border border-border bg-card overflow-hidden">
        <button
          onClick={() => setExpanded((v) => !v)}
          className="w-full flex items-center gap-2 px-4 py-3 text-left hover:bg-secondary/30 transition-colors"
        >
          <CalendarCheck className={cn("w-4 h-4 shrink-0", displayPendingCount > 0 ? "text-warning" : "text-primary")} />
          <span className="flex-1 text-sm font-semibold">
            {displayPendingCount > 0 ? (
              <>
                <span className="inline-flex items-center justify-center min-w-[20px] h-5 rounded-full bg-warning/20 text-warning text-xs font-bold px-1.5 mr-1.5">
                  {displayPendingCount}
                </span>
                {displayPendingCount === 1 ? "gasto fixo pendente" : "gastos fixos pendentes"}
              </>
            ) : (
              <span className="text-primary">Gastos fixos — todos pagos ✓</span>
            )}
          </span>
          <span className="text-muted-foreground shrink-0">
            {expanded ? <ChevronUp className="w-4 h-4" /> : <ChevronDown className="w-4 h-4" />}
          </span>
        </button>

        {expanded && (
          <div className="px-4 pb-3">
            {displayReminders.map((item) => (
              <ReminderRow
                key={item.id}
                item={item}
                fmt={fmt}
                onPay={() => handlePayClick(item)}
                onUnpay={() => handleUnpay(item)}
              />
            ))}
            <button
              onClick={() => navigate("/gastos-fixos/lembretes")}
              className="mt-2 text-xs text-muted-foreground hover:text-foreground underline"
            >
              Ver tudo →
            </button>
          </div>
        )}
      </div>

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
    </>
  )
}
