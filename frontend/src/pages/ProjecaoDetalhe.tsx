import { useState } from "react"
import { PageHeader } from "@/components/PageHeader/PageHeader"
import { BottomNav } from "@/components/BottomNav/BottomNav"
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs"
import { useExpenseProjectionDetail, ProjectionItem } from "@/hooks/useAnalytics"
import { useCurrencyStore, formatCurrency, maskCurrency } from "@/store/currency"
import { usePrivacyStore } from "@/store/privacy"

function useFmt() {
  const { currency } = useCurrencyStore()
  const valuesHidden = usePrivacyStore((s) => s.valuesHidden)
  return (value: number) =>
    valuesHidden ? maskCurrency(value, currency) : formatCurrency(value, currency)
}

const CATEGORY_ICONS: Record<string, string> = {
  casa: "🏠",
  saude: "❤️",
  lazer: "🎮",
  impostos: "📋",
  transporte: "🚗",
  educacao: "📚",
  servicos: "⚙️",
  outros: "📦",
}

const CATEGORY_LABELS: Record<string, string> = {
  casa: "Casa",
  saude: "Saúde",
  lazer: "Lazer",
  impostos: "Impostos",
  transporte: "Transporte",
  educacao: "Educação",
  servicos: "Serviços",
  outros: "Outros",
}

type GroupByField = "category_label" | "card_name"

function AccordionGroup({
  items,
  groupBy,
  color,
}: {
  items: ProjectionItem[]
  groupBy: GroupByField
  color: string
}) {
  const fmt = useFmt()
  const isCard = groupBy === "card_name"

  const groups = items.reduce<Record<string, ProjectionItem[]>>((acc, item) => {
    const key = (item[groupBy] ?? "outros") || "outros"
    acc[key] = [...(acc[key] ?? []), item]
    return acc
  }, {})

  const [open, setOpen] = useState<Record<string, boolean>>({})
  const toggle = (key: string) =>
    setOpen((prev) => ({ ...prev, [key]: !prev[key] }))

  if (Object.keys(groups).length === 0) return null

  return (
    <div className="space-y-2">
      {Object.entries(groups).map(([key, groupItems]) => {
        const subtotal = groupItems.reduce((s, i) => s + i.amount, 0)
        const expanded = open[key] ?? false
        const icon = isCard ? "💳" : (CATEGORY_ICONS[key] ?? "📦")
        const label = isCard ? key : (CATEGORY_LABELS[key] ?? key)

        return (
          <div key={key} className="rounded-lg overflow-hidden border border-border">
            <button
              className="w-full flex items-center justify-between px-4 py-3 bg-card hover:bg-muted/50 transition-colors"
              onClick={() => toggle(key)}
            >
              <span className="text-sm font-medium text-foreground">
                {icon} {label}
              </span>
              <div className="flex items-center gap-2">
                <span className="text-sm font-bold" style={{ color }}>
                  {fmt(subtotal)}
                </span>
                <span className="text-xs text-muted-foreground">
                  {expanded ? "▼" : "▶"}
                </span>
              </div>
            </button>
            {expanded && (
              <div>
                {groupItems.map((item, i) => (
                  <div
                    key={i}
                    className="flex items-center justify-between px-4 py-2 pl-8 bg-card border-t border-border"
                  >
                    <div>
                      <p className="text-sm text-foreground">{item.name}</p>
                      {item.installment_info && (
                        <p className="text-xs text-muted-foreground">
                          parcela {item.installment_info}
                        </p>
                      )}
                    </div>
                    <span className="text-sm font-semibold" style={{ color }}>
                      {fmt(item.amount)}
                    </span>
                  </div>
                ))}
              </div>
            )}
          </div>
        )
      })}
    </div>
  )
}

const COLOR_FIXED = "#6ee7b7"
const COLOR_CARD = "#67e8f9"

export default function ProjecaoDetalhe() {
  const fmt = useFmt()
  const { data, loading, error } = useExpenseProjectionDetail(6)

  if (loading) {
    return (
      <div className="h-[var(--app-height)] lg:h-full flex flex-col bg-background">
        <PageHeader title="Projeção de Gastos" showBack />
        <div className="flex-1 flex items-center justify-center">
          <p className="text-sm text-muted-foreground">Carregando...</p>
        </div>
        <BottomNav />
      </div>
    )
  }

  if (error) {
    return (
      <div className="h-[var(--app-height)] lg:h-full flex flex-col bg-background">
        <PageHeader title="Projeção de Gastos" showBack />
        <div className="flex-1 flex items-center justify-center">
          <p className="text-sm text-muted-foreground">Erro ao carregar projeção.</p>
        </div>
        <BottomNav />
      </div>
    )
  }

  if (data.length === 0) {
    return (
      <div className="h-[var(--app-height)] lg:h-full flex flex-col bg-background">
        <PageHeader title="Projeção de Gastos" showBack />
        <div className="flex-1 flex items-center justify-center">
          <p className="text-sm text-muted-foreground">Sem dados de projeção.</p>
        </div>
        <BottomNav />
      </div>
    )
  }

  return (
    <div className="h-[var(--app-height)] lg:h-full flex flex-col bg-background">
      <PageHeader title="Projeção de Gastos" showBack />

      <div className="flex-1 overflow-y-auto">
        <Tabs defaultValue={data[0].label} className="w-full">
          <div className="sticky top-0 z-10 bg-background border-b border-border">
            <TabsList className="w-full h-auto justify-start rounded-none bg-transparent p-0 overflow-x-auto flex">
              {data.map((m) => (
                <TabsTrigger
                  key={m.label}
                  value={m.label}
                  className="shrink-0 rounded-none border-b-2 border-transparent data-[state=active]:border-primary data-[state=active]:bg-transparent px-4 py-3 text-sm"
                >
                  {m.label}
                </TabsTrigger>
              ))}
            </TabsList>
          </div>

          {data.map((m) => (
            <TabsContent key={m.label} value={m.label} className="mt-0 p-4 space-y-4">
              {/* Summary cards */}
              <div className="grid grid-cols-2 gap-2">
                <div className="bg-card rounded-lg p-3 text-center border border-border">
                  <p className="text-xs text-muted-foreground uppercase mb-1">Fixos</p>
                  <p className="text-sm font-bold" style={{ color: COLOR_FIXED }}>{fmt(m.fixed)}</p>
                </div>
                <div className="bg-card rounded-lg p-3 text-center border border-border">
                  <p className="text-xs text-muted-foreground uppercase mb-1">Cartão</p>
                  <p className="text-sm font-bold" style={{ color: COLOR_CARD }}>{fmt(m.installments + m.card)}</p>
                </div>
              </div>

              {/* Total row */}
              <div className="bg-card rounded-lg px-4 py-3 flex justify-between items-center border border-border">
                <p className="text-sm text-muted-foreground">Total projetado</p>
                <p className="text-base font-bold">{fmt(m.total)}</p>
              </div>

              {/* Fixed items */}
              {m.items.fixed.length > 0 && (
                <div className="space-y-2">
                  <p className="text-xs font-bold uppercase tracking-wide" style={{ color: COLOR_FIXED }}>
                    Fixos
                  </p>
                  <AccordionGroup items={m.items.fixed} groupBy="category_label" color={COLOR_FIXED} />
                </div>
              )}

              {/* Card + installment items merged */}
              {(m.items.card.length > 0 || m.items.installments.length > 0) && (
                <div className="space-y-2">
                  <p className="text-xs font-bold uppercase tracking-wide" style={{ color: COLOR_CARD }}>
                    Cartão
                  </p>
                  <AccordionGroup
                    items={[...m.items.installments, ...m.items.card]}
                    groupBy="card_name"
                    color={COLOR_CARD}
                  />
                </div>
              )}
            </TabsContent>
          ))}
        </Tabs>
      </div>

      <BottomNav />
    </div>
  )
}
