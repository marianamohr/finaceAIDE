import { useState, useEffect } from "react"
import { BottomNav } from "@/components/BottomNav/BottomNav"
import { PageHeader } from "@/components/PageHeader/PageHeader"
import { useRecurringExpenses, RecurringExpense } from "@/hooks/useRecurringExpenses"
import { usePaymentMethods, PaymentMethod } from "@/hooks/usePaymentMethods"
import { useCurrencyStore, formatCurrency, currencySymbol, maskCurrency } from "@/store/currency"
import { usePrivacyStore } from "@/store/privacy"
import { api } from "@/services/api"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Sheet, SheetContent, SheetHeader, SheetTitle } from "@/components/ui/sheet"
import { categoryLabel, categoryEmoji, EXPENSE_SLUGS_STANDARD } from "@/lib/categories"
import { Pencil, Trash2, CalendarDays, Plus, CreditCard, Ban } from "lucide-react"
import { toast } from "sonner"

interface Category {
  id: string
  slug: string
  label: string
  type: string
}

function useFmt() {
  const { currency } = useCurrencyStore()
  const hidden = usePrivacyStore((s) => s.valuesHidden)
  return (v: number) =>
    hidden ? maskCurrency(v, currency) : formatCurrency(v, currency)
}

// ─── Formulário: Gasto fixo (sem cartão) ────────────────────────────────────

interface FixedFormState {
  name: string
  amount: string
  categoryId: string
  dueDay: string
  startMonth: string
  endMonth: string
  isVariable: boolean
}

const EMPTY_FIXED: FixedFormState = { name: "", amount: "", categoryId: "", dueDay: "", startMonth: "", endMonth: "", isVariable: false }

function parseMonthInput(value: string): { month: number; year: number } | null {
  if (!value) return null
  const [year, month] = value.split("-").map(Number)
  if (!year || !month) return null
  return { month, year }
}

function FixedExpenseForm({
  initial,
  categories,
  onSave,
  onCancel,
  saving,
}: {
  initial: FixedFormState
  categories: Category[]
  onSave: (form: FixedFormState) => void
  onCancel: () => void
  saving: boolean
}) {
  const [form, setForm] = useState<FixedFormState>(initial)
  const { currency } = useCurrencyStore()
  const symbol = currencySymbol(currency)
  const set = (k: keyof FixedFormState) => (e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement>) =>
    setForm((f) => ({ ...f, [k]: e.target.value }))
  const expenseCategories = categories.filter((c) => c.type === "expense" && EXPENSE_SLUGS_STANDARD.includes(c.slug))

  return (
    <div className="space-y-4 px-1 pb-8">
      <div className="space-y-1.5">
        <label className="text-sm font-medium">Nome</label>
        <Input placeholder="Ex: Aluguel, Internet..." value={form.name} onChange={set("name")} />
      </div>
      <div className="space-y-1.5">
        <label className="text-sm font-medium">{form.isVariable ? "Valor estimado" : "Valor esperado"} ({symbol})</label>
        <Input type="number" min="0" step="0.01" placeholder="0,00" value={form.amount} onChange={set("amount")} />
      </div>
      <label className="flex items-start gap-2.5 rounded-lg border border-border bg-card p-3 cursor-pointer">
        <input
          type="checkbox"
          className="mt-0.5 h-4 w-4 shrink-0 accent-primary"
          checked={form.isVariable}
          onChange={(e) => setForm((f) => ({ ...f, isVariable: e.target.checked }))}
        />
        <span className="min-w-0">
          <span className="block text-sm font-medium">Valor variável</span>
          <span className="block text-xs text-muted-foreground">Para contas que mudam todo mês (água, luz, gás). O valor real é informado na hora de pagar.</span>
        </span>
      </label>
      <div className="space-y-1.5">
        <label className="text-sm font-medium">Categoria</label>
        <select value={form.categoryId} onChange={set("categoryId")} className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm shadow-sm focus:outline-none focus:ring-1 focus:ring-ring">
          <option value="">Selecionar...</option>
          {expenseCategories.map((c) => (
            <option key={c.id} value={c.id}>{categoryEmoji(c.slug)} {categoryLabel(c.slug)}</option>
          ))}
        </select>
      </div>
      <div className="space-y-1.5">
        <label className="text-sm font-medium">Dia de vencimento <span className="text-muted-foreground font-normal">(opcional)</span></label>
        <Input type="number" min="1" max="28" placeholder="Ex: 5" value={form.dueDay} onChange={set("dueDay")} />
        <p className="text-xs text-muted-foreground">Dia do mês entre 1 e 28</p>
      </div>
      <div className="space-y-1.5">
        <label className="text-sm font-medium">Início <span className="text-muted-foreground font-normal">(opcional)</span></label>
        <Input type="month" value={form.startMonth} onChange={set("startMonth")} />
      </div>
      <div className="space-y-1.5">
        <label className="text-sm font-medium">Fim <span className="text-muted-foreground font-normal">(opcional)</span></label>
        <Input type="month" value={form.endMonth} onChange={set("endMonth")} />
      </div>
      <div className="flex gap-2 pt-2">
        <Button variant="outline" className="flex-1" onClick={onCancel} disabled={saving}>Cancelar</Button>
        <Button className="flex-1" onClick={() => onSave(form)} disabled={saving || !form.name || !form.amount}>
          {saving ? "Salvando..." : "Salvar"}
        </Button>
      </div>
    </div>
  )
}

// ─── Formulário: Assinatura de cartão ───────────────────────────────────────

interface SubFormState {
  name: string
  amount: string
  categoryId: string
  paymentMethodId: string
  startMonth: string
  endMonth: string
}

const EMPTY_SUB: SubFormState = { name: "", amount: "", categoryId: "", paymentMethodId: "", startMonth: "", endMonth: "" }

function SubscriptionForm({
  initial,
  categories,
  creditCards,
  onSave,
  onCancel,
  saving,
}: {
  initial: SubFormState
  categories: Category[]
  creditCards: PaymentMethod[]
  onSave: (form: SubFormState) => void
  onCancel: () => void
  saving: boolean
}) {
  const [form, setForm] = useState<SubFormState>(initial)
  const { currency } = useCurrencyStore()
  const symbol = currencySymbol(currency)
  const set = (k: keyof SubFormState) => (e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement>) =>
    setForm((f) => ({ ...f, [k]: e.target.value }))
  const expenseCategories = categories.filter((c) => c.type === "expense" && EXPENSE_SLUGS_STANDARD.includes(c.slug))

  return (
    <div className="space-y-4 px-1 pb-8">
      <p className="text-xs text-muted-foreground">Assinaturas de cartão aparecem automaticamente na fatura — sem precisar marcar como pago.</p>
      <div className="space-y-1.5">
        <label className="text-sm font-medium">Nome</label>
        <Input placeholder="Ex: Netflix, Spotify..." value={form.name} onChange={set("name")} />
      </div>
      <div className="space-y-1.5">
        <label className="text-sm font-medium">Valor ({symbol})</label>
        <Input type="number" min="0" step="0.01" placeholder="0,00" value={form.amount} onChange={set("amount")} />
      </div>
      <div className="space-y-1.5">
        <label className="text-sm font-medium">Cartão de crédito</label>
        <select value={form.paymentMethodId} onChange={set("paymentMethodId")} className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm shadow-sm focus:outline-none focus:ring-1 focus:ring-ring">
          <option value="">Selecionar cartão...</option>
          {creditCards.map((c) => (
            <option key={c.id} value={c.id}>{c.name}</option>
          ))}
        </select>
      </div>
      <div className="space-y-1.5">
        <label className="text-sm font-medium">Categoria</label>
        <select value={form.categoryId} onChange={set("categoryId")} className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm shadow-sm focus:outline-none focus:ring-1 focus:ring-ring">
          <option value="">Sem categoria</option>
          {expenseCategories.map((c) => (
            <option key={c.id} value={c.id}>{categoryEmoji(c.slug)} {categoryLabel(c.slug)}</option>
          ))}
        </select>
      </div>
      <div className="space-y-1.5">
        <label className="text-sm font-medium">Início <span className="text-muted-foreground font-normal">(opcional)</span></label>
        <Input type="month" value={form.startMonth} onChange={set("startMonth")} />
      </div>
      <div className="space-y-1.5">
        <label className="text-sm font-medium">Fim <span className="text-muted-foreground font-normal">(opcional)</span></label>
        <Input type="month" value={form.endMonth} onChange={set("endMonth")} />
      </div>
      <div className="flex gap-2 pt-2">
        <Button variant="outline" className="flex-1" onClick={onCancel} disabled={saving}>Cancelar</Button>
        <Button className="flex-1" onClick={() => onSave(form)} disabled={saving || !form.name || !form.amount || !form.paymentMethodId}>
          {saving ? "Salvando..." : "Salvar"}
        </Button>
      </div>
    </div>
  )
}

// ─── Card de item ────────────────────────────────────────────────────────────

function ExpenseCard({
  item,
  fmt,
  categories,
  cardName,
  inactive,
  onEdit,
  onDelete,
  onTerminate,
}: {
  item: RecurringExpense
  fmt: (v: number) => string
  categories: Category[]
  cardName?: string
  inactive?: boolean
  onEdit: () => void
  onDelete?: () => void
  onTerminate?: () => void
}) {
  const slug = item.category_id ? (categories.find((c) => c.id === item.category_id)?.slug ?? "") : ""

  return (
    <div className={`flex items-center gap-3 p-3.5 rounded-xl border border-border bg-card${inactive ? " opacity-50" : ""}`}>
      <div className="w-9 h-9 rounded-lg bg-secondary flex items-center justify-center shrink-0 text-base">
        {slug ? categoryEmoji(slug) : "💸"}
      </div>
      <div className="flex-1 min-w-0">
        <p className="text-sm font-semibold truncate">{item.name}</p>
        <div className="flex items-center gap-2 mt-0.5 flex-wrap">
          <span className="text-xs text-muted-foreground">{fmt(item.amount)}</span>
          {item.is_variable && (
            <span className="text-[10px] font-medium text-muted-foreground bg-secondary rounded px-1.5 py-0.5">estimado</span>
          )}
          {item.due_day != null && (
            <span className="flex items-center gap-0.5 text-xs text-muted-foreground">
              <CalendarDays className="w-3 h-3" />
              dia {item.due_day}
            </span>
          )}
          {cardName && (
            <span className="flex items-center gap-0.5 text-xs text-muted-foreground">
              <CreditCard className="w-3 h-3" />
              {cardName}
            </span>
          )}
        </div>
        {(item.start_year || item.end_year) && (
          <p className="text-xs text-muted-foreground mt-0.5">
            {item.start_year && item.start_month ? `a partir de ${String(item.start_month).padStart(2, "0")}/${item.start_year}` : ""}
            {item.start_year && item.end_year ? " · " : ""}
            {item.end_year && item.end_month ? `até ${String(item.end_month).padStart(2, "0")}/${item.end_year}` : ""}
          </p>
        )}
      </div>
      <div className="flex items-center gap-1 shrink-0">
        <button onClick={onEdit} className="p-1.5 text-muted-foreground hover:text-primary transition-colors" aria-label="Editar">
          <Pencil className="w-3.5 h-3.5" />
        </button>
        {onTerminate && (
          <button onClick={onTerminate} className="p-1.5 text-muted-foreground hover:text-destructive transition-colors" aria-label="Encerrar assinatura">
            <Ban className="w-3.5 h-3.5" />
          </button>
        )}
        {onDelete && !onTerminate && (
          <button onClick={onDelete} className="p-1.5 text-muted-foreground hover:text-destructive transition-colors" aria-label="Remover">
            <Trash2 className="w-3.5 h-3.5" />
          </button>
        )}
      </div>
    </div>
  )
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

function isInactive(item: RecurringExpense): boolean {
  if (!item.end_year || !item.end_month) return false
  const now = new Date()
  const currentYear = now.getFullYear()
  const currentMonth = now.getMonth() + 1
  return item.end_year < currentYear ||
    (item.end_year === currentYear && item.end_month < currentMonth)
}

// ─── Página principal ────────────────────────────────────────────────────────

type SheetMode = "fixed" | "subscription" | null

export default function GastosFixos() {
  const fmt = useFmt()
  const { items, isLoading, create, update, remove } = useRecurringExpenses()
  const { paymentMethods } = usePaymentMethods()
  const [categories, setCategories] = useState<Category[]>([])
  const [sheetMode, setSheetMode] = useState<SheetMode>(null)
  const [editingItem, setEditingItem] = useState<RecurringExpense | null>(null)
  const [fixedForm, setFixedForm] = useState<FixedFormState>(EMPTY_FIXED)
  const [subForm, setSubForm] = useState<SubFormState>(EMPTY_SUB)
  const [saving, setSaving] = useState(false)
  const [terminatingItem, setTerminatingItem] = useState<RecurringExpense | null>(null)
  const [terminateMonth, setTerminateMonth] = useState("")

  useEffect(() => {
    api.get<Category[]>("/categories").then(setCategories).catch(() => {})
  }, [])

  const creditCards = paymentMethods.filter((pm) => pm.active && pm.kind === "credit_card")
  const allCardItems = items.filter((item) => !!item.payment_method_id)
  const activeCardItems = allCardItems.filter((item) => !isInactive(item))
  const inactiveCardItems = allCardItems.filter((item) => isInactive(item))
  const fixedItems = items.filter((item) => !item.payment_method_id)

  const cardById = (id: string) => paymentMethods.find((pm) => pm.id === id)

  const openCreateFixed = () => {
    setEditingItem(null)
    setFixedForm(EMPTY_FIXED)
    setSheetMode("fixed")
  }

  const openCreateSub = () => {
    setEditingItem(null)
    setSubForm(EMPTY_SUB)
    setSheetMode("subscription")
  }

  const openEdit = (item: RecurringExpense) => {
    setEditingItem(item)
    if (item.payment_method_id) {
      setSubForm({
        name: item.name,
        amount: String(item.amount),
        categoryId: item.category_id ?? "",
        paymentMethodId: item.payment_method_id,
        startMonth: item.start_year && item.start_month ? `${item.start_year}-${String(item.start_month).padStart(2, "0")}` : "",
        endMonth: item.end_year && item.end_month ? `${item.end_year}-${String(item.end_month).padStart(2, "0")}` : "",
      })
      setSheetMode("subscription")
    } else {
      setFixedForm({
        name: item.name,
        amount: String(item.amount),
        categoryId: item.category_id ?? "",
        dueDay: item.due_day != null ? String(item.due_day) : "",
        startMonth: item.start_year && item.start_month ? `${item.start_year}-${String(item.start_month).padStart(2, "0")}` : "",
        endMonth: item.end_year && item.end_month ? `${item.end_year}-${String(item.end_month).padStart(2, "0")}` : "",
        isVariable: item.is_variable,
      })
      setSheetMode("fixed")
    }
  }

  const handleSaveFixed = async (f: FixedFormState) => {
    const dueDayNum = f.dueDay ? parseInt(f.dueDay, 10) : undefined
    if (dueDayNum !== undefined && (dueDayNum < 1 || dueDayNum > 28)) {
      toast.error("Dia de vencimento deve ser entre 1 e 28")
      return
    }
    const start = parseMonthInput(f.startMonth)
    const end = parseMonthInput(f.endMonth)
    if (start && end && (end.year < start.year || (end.year === start.year && end.month < start.month))) {
      toast.error("A data de fim deve ser igual ou posterior à data de início")
      return
    }
    setSaving(true)
    try {
      if (editingItem) {
        await update(editingItem.id, {
          name: f.name,
          amount: parseFloat(f.amount),
          category_id: f.categoryId || null,
          payment_method_id: null,
          due_day: dueDayNum ?? null,
          start_month: start?.month ?? null,
          start_year: start?.year ?? null,
          end_month: end?.month ?? null,
          end_year: end?.year ?? null,
          is_variable: f.isVariable,
        })
        toast.success("Gasto fixo atualizado")
      } else {
        await create({
          name: f.name,
          amount: parseFloat(f.amount),
          category_id: f.categoryId || undefined,
          due_day: dueDayNum,
          start_month: start?.month,
          start_year: start?.year,
          end_month: end?.month,
          end_year: end?.year,
          is_variable: f.isVariable,
        })
        toast.success("Gasto fixo criado")
      }
      setSheetMode(null)
    } catch {
      toast.error("Erro ao salvar gasto fixo")
    } finally {
      setSaving(false)
    }
  }

  const handleSaveSub = async (f: SubFormState) => {
    const start = parseMonthInput(f.startMonth)
    const end = parseMonthInput(f.endMonth)
    if (start && end && (end.year < start.year || (end.year === start.year && end.month < start.month))) {
      toast.error("A data de fim deve ser igual ou posterior à data de início")
      return
    }
    setSaving(true)
    try {
      if (editingItem) {
        await update(editingItem.id, {
          name: f.name,
          amount: parseFloat(f.amount),
          category_id: f.categoryId || null,
          payment_method_id: f.paymentMethodId,
          start_month: start?.month ?? null,
          start_year: start?.year ?? null,
          end_month: end?.month ?? null,
          end_year: end?.year ?? null,
        })
        toast.success("Assinatura atualizada")
      } else {
        await create({
          name: f.name,
          amount: parseFloat(f.amount),
          category_id: f.categoryId || undefined,
          payment_method_id: f.paymentMethodId,
          start_month: start?.month,
          start_year: start?.year,
          end_month: end?.month,
          end_year: end?.year,
        })
        toast.success("Assinatura criada")
      }
      setSheetMode(null)
    } catch {
      toast.error("Erro ao salvar assinatura")
    } finally {
      setSaving(false)
    }
  }

  const handleDelete = async (id: string) => {
    try {
      await remove(id)
      toast.success("Removido")
    } catch {
      toast.error("Erro ao remover")
    }
  }

  const handleTerminate = async () => {
    if (!terminatingItem || !terminateMonth) return
    const [year, month] = terminateMonth.split("-").map(Number)
    try {
      await update(terminatingItem.id, { end_month: month, end_year: year })
      toast.success("Assinatura encerrada")
      setTerminatingItem(null)
    } catch {
      toast.error("Erro ao encerrar assinatura")
    }
  }

  const isEmpty = activeCardItems.length === 0 && fixedItems.length === 0 && inactiveCardItems.length === 0

  const [cardExpanded, setCardExpanded] = useState(false)
  const [fixedExpanded, setFixedExpanded] = useState(false)
  const [inactiveExpanded, setInactiveExpanded] = useState(false)

  const PREVIEW = 3

  return (
    <div className="min-h-screen bg-background flex flex-col">
      <PageHeader title="Gastos Fixos" />

      <main className="flex-1 overflow-y-auto px-4 pb-24 pt-4 space-y-3 max-w-lg mx-auto w-full">
        <div className="flex gap-2">
          <Button onClick={openCreateFixed} className="flex-1" variant="outline">
            <Plus className="w-4 h-4 mr-2" />
            Gasto fixo
          </Button>
          <Button onClick={openCreateSub} className="flex-1" variant="outline">
            <CreditCard className="w-4 h-4 mr-2" />
            Assinatura
          </Button>
        </div>

        {isLoading ? (
          <div className="flex justify-center py-12">
            <div className="w-6 h-6 border-2 border-primary border-t-transparent rounded-full animate-spin" />
          </div>
        ) : isEmpty ? (
          <div className="text-center py-12 text-muted-foreground text-sm">
            <p className="text-2xl mb-2">📋</p>
            <p>Nenhum gasto fixo cadastrado ainda.</p>
            <p className="mt-1">Adicione contas que se repetem todo mês.</p>
          </div>
        ) : (
          <>
            {activeCardItems.length > 0 && (
              <div className="space-y-2">
                <p className="text-xs font-semibold text-muted-foreground uppercase tracking-wide px-0.5">Assinaturas de cartão</p>
                {(cardExpanded ? activeCardItems : activeCardItems.slice(0, PREVIEW)).map((item) => (
                  <ExpenseCard
                    key={item.id}
                    item={item}
                    fmt={fmt}
                    categories={categories}
                    cardName={cardById(item.payment_method_id!)?.name}
                    onEdit={() => openEdit(item)}
                    onTerminate={() => {
                      const n = new Date()
                      setTerminateMonth(`${n.getFullYear()}-${String(n.getMonth() + 1).padStart(2, "0")}`)
                      setTerminatingItem(item)
                    }}
                  />
                ))}
                {activeCardItems.length > PREVIEW && (
                  <button onClick={() => setCardExpanded((v) => !v)} className="w-full text-xs text-muted-foreground hover:text-foreground transition-colors py-1">
                    {cardExpanded ? "Ver menos" : `Ver mais ${activeCardItems.length - PREVIEW}`}
                  </button>
                )}
              </div>
            )}

            {fixedItems.length > 0 && (
              <div className="space-y-2">
                <p className="text-xs font-semibold text-muted-foreground uppercase tracking-wide px-0.5">Gastos fixos</p>
                {(fixedExpanded ? fixedItems : fixedItems.slice(0, PREVIEW)).map((item) => (
                  <ExpenseCard
                    key={item.id}
                    item={item}
                    fmt={fmt}
                    categories={categories}
                    onEdit={() => openEdit(item)}
                    onDelete={() => handleDelete(item.id)}
                  />
                ))}
                {fixedItems.length > PREVIEW && (
                  <button onClick={() => setFixedExpanded((v) => !v)} className="w-full text-xs text-muted-foreground hover:text-foreground transition-colors py-1">
                    {fixedExpanded ? "Ver menos" : `Ver mais ${fixedItems.length - PREVIEW}`}
                  </button>
                )}
              </div>
            )}

            {inactiveCardItems.length > 0 && (
              <div className="space-y-2 mt-2">
                <div className="flex items-center gap-2">
                  <div className="flex-1 h-px bg-border" />
                  <p className="text-xs font-semibold text-muted-foreground uppercase tracking-wide px-1">Encerradas</p>
                  <div className="flex-1 h-px bg-border" />
                </div>
                {(inactiveExpanded ? inactiveCardItems : inactiveCardItems.slice(0, PREVIEW)).map((item) => (
                  <ExpenseCard
                    key={item.id}
                    item={item}
                    fmt={fmt}
                    categories={categories}
                    cardName={cardById(item.payment_method_id!)?.name}
                    inactive
                    onEdit={() => openEdit(item)}
                  />
                ))}
                {inactiveCardItems.length > PREVIEW && (
                  <button onClick={() => setInactiveExpanded((v) => !v)} className="w-full text-xs text-muted-foreground hover:text-foreground transition-colors py-1">
                    {inactiveExpanded ? "Ver menos" : `Ver mais ${inactiveCardItems.length - PREVIEW}`}
                  </button>
                )}
              </div>
            )}
          </>
        )}
      </main>

      {/* Sheet: Gasto fixo */}
      <Sheet open={sheetMode === "fixed"} onOpenChange={(open) => { if (!open) setSheetMode(null) }}>
        <SheetContent side="bottom" className="max-h-[90dvh] overflow-y-auto rounded-t-2xl">
          <SheetHeader className="pb-4">
            <SheetTitle>{editingItem ? "Editar gasto fixo" : "Novo gasto fixo"}</SheetTitle>
          </SheetHeader>
          <FixedExpenseForm
            initial={fixedForm}
            categories={categories}
            onSave={handleSaveFixed}
            onCancel={() => setSheetMode(null)}
            saving={saving}
          />
        </SheetContent>
      </Sheet>

      {/* Sheet: Assinatura de cartão */}
      <Sheet open={sheetMode === "subscription"} onOpenChange={(open) => { if (!open) setSheetMode(null) }}>
        <SheetContent side="bottom" className="max-h-[90dvh] overflow-y-auto rounded-t-2xl">
          <SheetHeader className="pb-4">
            <SheetTitle>{editingItem ? "Editar assinatura" : "Nova assinatura de cartão"}</SheetTitle>
          </SheetHeader>
          <SubscriptionForm
            initial={subForm}
            categories={categories}
            creditCards={creditCards}
            onSave={handleSaveSub}
            onCancel={() => setSheetMode(null)}
            saving={saving}
          />
        </SheetContent>
      </Sheet>

      <Sheet open={!!terminatingItem} onOpenChange={(open) => { if (!open) setTerminatingItem(null) }}>
        <SheetContent side="bottom" className="max-h-[90dvh] overflow-y-auto rounded-t-2xl">
          <SheetHeader className="pb-4">
            <SheetTitle>Encerrar assinatura</SheetTitle>
          </SheetHeader>
          <div className="space-y-4 px-1 pb-8">
            <p className="text-sm text-muted-foreground">Informe o mês em que a assinatura foi cancelada.</p>
            <div className="space-y-1.5">
              <label className="text-sm font-medium">Mês de encerramento</label>
              <Input
                type="month"
                value={terminateMonth}
                onChange={(e) => setTerminateMonth(e.target.value)}
              />
            </div>
            <div className="flex gap-2 pt-2">
              <Button variant="outline" className="flex-1" onClick={() => setTerminatingItem(null)}>Cancelar</Button>
              <Button variant="destructive" className="flex-1" onClick={handleTerminate} disabled={!terminateMonth}>
                Encerrar
              </Button>
            </div>
          </div>
        </SheetContent>
      </Sheet>

      <BottomNav />
    </div>
  )
}
