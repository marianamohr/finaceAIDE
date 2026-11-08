import { useState } from "react"
import { X } from "lucide-react"
import { PaymentMethod } from "@/hooks/usePaymentMethods"
import { Input } from "@/components/ui/input"

function pmIcon(kind: PaymentMethod["kind"]): string {
  switch (kind) {
    case "pix":     return "PIX"
    case "debit":   return "Déb"
    case "cash":    return "Din"
    case "voucher": return "Vou"
    default:        return "Pag"
  }
}

export function PaymentMethodSheet({
  paymentMethods,
  onSelect,
  onClose,
  askAmount = false,
  initialAmount,
  currencySymbol = "R$",
}: {
  paymentMethods: PaymentMethod[]
  onSelect: (pm: PaymentMethod | null, amount?: number) => void
  onClose: () => void
  // When true, the user must inform the real amount paid (variable bills).
  askAmount?: boolean
  initialAmount?: number
  currencySymbol?: string
}) {
  const eligible = paymentMethods.filter((pm) => pm.active && pm.kind !== "credit_card")

  const [amountStr, setAmountStr] = useState(
    initialAmount != null && initialAmount > 0 ? String(initialAmount) : ""
  )
  const amount = parseFloat(amountStr)
  const amountValid = !askAmount || (Number.isFinite(amount) && amount > 0)

  const choose = (pm: PaymentMethod | null) => {
    if (!amountValid) return
    onSelect(pm, askAmount ? amount : undefined)
  }

  return (
    <div className="fixed inset-0 z-50 flex flex-col justify-end">
      <div className="absolute inset-0 bg-black/40" onClick={onClose} />
      <div className="relative bg-background rounded-t-2xl px-4 pb-8 pt-4 space-y-3 max-w-lg mx-auto w-full">
        <div className="flex items-center justify-between mb-1">
          <p className="text-sm font-semibold">Como foi pago?</p>
          <button onClick={onClose} className="text-muted-foreground hover:text-foreground">
            <X className="w-4 h-4" />
          </button>
        </div>
        {askAmount && (
          <div className="space-y-1.5 pb-1">
            <label className="text-sm font-medium">Valor pago ({currencySymbol})</label>
            <Input
              type="number"
              min="0"
              step="0.01"
              inputMode="decimal"
              placeholder="0,00"
              value={amountStr}
              onChange={(e) => setAmountStr(e.target.value)}
              autoFocus
            />
            <p className="text-xs text-muted-foreground">Informe o valor real desta conta no mês.</p>
          </div>
        )}
        {eligible.map((pm) => (
          <button
            key={pm.id}
            onClick={() => choose(pm)}
            disabled={!amountValid}
            className="w-full flex items-center gap-3 p-3 rounded-xl border border-border hover:bg-secondary transition-colors text-left disabled:opacity-40 disabled:pointer-events-none"
          >
            <div
              className="w-9 h-9 rounded-lg flex items-center justify-center shrink-0 text-xs font-bold text-white"
              style={{ backgroundColor: pm.color || "#6b7280" }}
            >
              {pmIcon(pm.kind)}
            </div>
            <span className="text-sm font-medium">{pm.name}</span>
          </button>
        ))}
        <button
          onClick={() => choose(null)}
          disabled={!amountValid}
          className="w-full flex items-center gap-3 p-3 rounded-xl border border-dashed border-border hover:bg-secondary transition-colors text-left disabled:opacity-40 disabled:pointer-events-none"
        >
          <div className="w-9 h-9 rounded-lg bg-secondary flex items-center justify-center shrink-0 text-base">
            💸
          </div>
          <span className="text-sm text-muted-foreground">Sem forma de pagamento</span>
        </button>
      </div>
    </div>
  )
}
