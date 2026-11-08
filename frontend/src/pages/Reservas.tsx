import { useState, useEffect } from "react"
import { BottomNav } from "@/components/BottomNav/BottomNav"
import { PageHeader } from "@/components/PageHeader/PageHeader"
import { useReservas, Goal, GoalTransaction, AddTransactionPayload, CreateGoalPayload } from "@/hooks/useReservas"
import { useCurrencyStore, formatCurrency, maskCurrency } from "@/store/currency"
import { usePrivacyStore } from "@/store/privacy"
import { useMediaQuery } from "@/hooks/useMediaQuery"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Sheet, SheetContent, SheetHeader, SheetTitle } from "@/components/ui/sheet"
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { PiggyBank, Plus, TrendingUp, TrendingDown, CircleDollarSign } from "lucide-react"
import { cn } from "@/lib/utils"

function useFmt() {
  const { currency } = useCurrencyStore()
  const hidden = usePrivacyStore((s) => s.valuesHidden)
  return (value: number) =>
    hidden ? maskCurrency(value, currency) : formatCurrency(value, currency)
}

function sourceLabel(source: string): string {
  switch (source) {
    case "initial": return "Saldo inicial"
    case "yield": return "Rendimento"
    case "withdrawal": return "Saque"
    case "chat": return "Chat"
    default: return "Aporte"
  }
}

function sourceIcon(source: string) {
  switch (source) {
    case "withdrawal": return <TrendingDown className="w-3.5 h-3.5 text-red-500" />
    case "yield": return <TrendingUp className="w-3.5 h-3.5 text-blue-500" />
    default: return <TrendingUp className="w-3.5 h-3.5 text-green-500" />
  }
}


// ---------- CircularProgress ----------

function CircularProgress({ pct, size }: { pct: number; size: number }) {
  const clamped = Math.min(100, Math.max(0, pct))
  const strokeWidth = size * 0.085
  const r = size / 2 - strokeWidth / 2
  const circumference = 2 * Math.PI * r
  const offset = circumference * (1 - clamped / 100)

  return (
    <div className="relative inline-flex items-center justify-center" style={{ width: size, height: size }}>
      <svg
        width={size}
        height={size}
        style={{ transform: "rotate(-90deg)" }}
      >
        <circle
          cx={size / 2}
          cy={size / 2}
          r={r}
          fill="none"
          stroke="hsl(var(--border))"
          strokeWidth={strokeWidth}
          strokeLinecap="round"
        />
        <circle
          cx={size / 2}
          cy={size / 2}
          r={r}
          fill="none"
          stroke="hsl(var(--primary))"
          strokeWidth={strokeWidth}
          strokeLinecap="round"
          strokeDasharray={circumference}
          strokeDashoffset={offset}
          style={{ transition: "stroke-dashoffset 0.6s ease" }}
        />
      </svg>
      <div
        className="absolute inset-0 flex flex-col items-center justify-center"
        style={{ lineHeight: 1.1 }}
      >
        <span className="font-bold leading-none" style={{ fontSize: size * 0.18 }}>
          {Math.round(clamped)}%
        </span>
        <span className="text-muted-foreground leading-none mt-0.5" style={{ fontSize: size * 0.11 }}>
          meta
        </span>
      </div>
    </div>
  )
}

// ---------- Cover image library ----------

const COVER_BY_TYPE: Record<string, string> = {
  travel: "https://images.unsplash.com/photo-1483729558449-99ef09a8c325?w=600&q=80",
  car: "https://images.unsplash.com/photo-1494976388531-d1058494cdd8?w=600&q=80",
  house: "https://images.unsplash.com/photo-1568605114967-8130f3a36994?w=600&q=80",
  emergency: "https://images.unsplash.com/photo-1554224155-6726b3ff858f?w=600&q=80",
  emergency_global: "https://images.unsplash.com/photo-1554224155-6726b3ff858f?w=600&q=80",
  other: "https://images.unsplash.com/photo-1579621970563-ebec7560ff3e?w=600&q=80",
}

function coverUrl(goal: Goal): string {
  return goal.cover_image_url ?? COVER_BY_TYPE[goal.goal_type] ?? COVER_BY_TYPE.other
}

// ---------- Transaction list (shared between card and detail sheet) ----------

function TransactionList({ txs, loadingTxs, fmt }: { txs: GoalTransaction[]; loadingTxs: boolean; fmt: (v: number) => string }) {
  return (
    <div className="space-y-1.5 pt-1 border-t border-border">
      {loadingTxs && <p className="text-xs text-muted-foreground">Carregando...</p>}
      {!loadingTxs && txs.length === 0 && (
        <p className="text-xs text-muted-foreground">Nenhuma movimentação ainda.</p>
      )}
      {txs.map((tx) => (
        <div key={tx.id} className="flex items-center justify-between text-xs">
          <div className="flex items-center gap-1.5 min-w-0">
            {sourceIcon(tx.source)}
            <span className="text-muted-foreground shrink-0">{sourceLabel(tx.source)}</span>
            {tx.description && (
              <span className="truncate text-muted-foreground">· {tx.description}</span>
            )}
          </div>
          <span className={cn("shrink-0 ml-2 font-medium", tx.amount >= 0 ? "text-green-600" : "text-red-500")}>
            {tx.amount >= 0 ? "+" : ""}{fmt(tx.amount)}
          </span>
        </div>
      ))}
    </div>
  )
}

// ---------- GoalCard ----------

interface GoalCardProps {
  goal: Goal
  onAddTransaction: (goal: Goal, initialSource?: AddTransactionPayload["source"]) => void
  onCardClick: (goal: Goal) => void
}

function GoalCard({ goal, onAddTransaction: _onAddTransaction, onCardClick }: GoalCardProps) {
  const fmt = useFmt()
  const pct = goal.target_amount > 0 ? (goal.current_amount / goal.target_amount) * 100 : 0
  const img = coverUrl(goal)
  const deadlineLabel = goal.deadline
    ? new Date(goal.deadline).toLocaleDateString("pt-BR", { month: "short", year: "numeric" })
    : null

  return (
    <div className="bg-card border border-border rounded-xl overflow-hidden flex flex-col">
      {/* Image header */}
      <div className="relative h-[130px] shrink-0 overflow-hidden">
        <img src={img} alt={goal.name} className="w-full h-full object-cover" />
        <div className="absolute inset-0 bg-gradient-to-b from-transparent to-black/60" />
        <p className="absolute bottom-2.5 left-3.5 right-3.5 text-white font-semibold text-sm truncate drop-shadow">
          {goal.name}
        </p>
      </div>

      {/* Body */}
      <div className="flex flex-col p-4 gap-3 flex-1">
        {/* Progress ring */}
        <div className="flex justify-center">
          <CircularProgress pct={pct} size={80} />
        </div>

        {/* Fixed info grid — same fields on every card */}
        <div className="grid grid-cols-2 gap-x-3 gap-y-2 text-xs">
          <div>
            <p className="uppercase text-muted-foreground tracking-wide" style={{ fontSize: 10 }}>Reservado</p>
            <p className="font-semibold" style={{ fontSize: 13 }}>{fmt(goal.current_amount)}</p>
          </div>
          <div>
            <p className="uppercase text-muted-foreground tracking-wide" style={{ fontSize: 10 }}>Meta</p>
            <p className="font-semibold" style={{ fontSize: 13 }}>{fmt(goal.target_amount)}</p>
          </div>
          <div>
            <p className="uppercase text-muted-foreground tracking-wide" style={{ fontSize: 10 }}>Prazo</p>
            <p className="font-semibold" style={{ fontSize: 13 }}>{deadlineLabel ?? "—"}</p>
          </div>
          <div>
            <p className="uppercase text-muted-foreground tracking-wide" style={{ fontSize: 10 }}>Concluído</p>
            <p className="font-semibold" style={{ fontSize: 13 }}>{pct.toFixed(0)}%</p>
          </div>
        </div>

        {/* Actions — pushed to bottom */}
        <div className="mt-auto pt-2 border-t border-border">
          <Button size="sm" variant="outline" className="w-full" onClick={() => onCardClick(goal)}>
            Ver detalhes
          </Button>
        </div>
      </div>
    </div>
  )
}

// ---------- GoalDetailSheet ----------

interface GoalDetailSheetProps {
  goal: Goal | null
  open: boolean
  onClose: () => void
  onAddTransaction: (goal: Goal, initialSource?: AddTransactionPayload["source"]) => void
  onEditGoal: (goal: Goal) => void
  listTransactions: (id: string) => Promise<GoalTransaction[]>
}

function GoalDetailSheet({ goal, open, onClose, onAddTransaction, onEditGoal, listTransactions }: GoalDetailSheetProps) {
  const fmt = useFmt()
  const isDesktop = useMediaQuery("(min-width: 1024px)")
  const [txs, setTxs] = useState<GoalTransaction[]>([])
  const [loadingTxs, setLoadingTxs] = useState(false)

  useEffect(() => {
    if (!open || !goal) return
    setLoadingTxs(true)
    listTransactions(goal.id)
      .then(setTxs)
      .finally(() => setLoadingTxs(false))
  }, [open, goal?.id, listTransactions])

  if (!goal) return null

  const pct = goal.target_amount > 0 ? (goal.current_amount / goal.target_amount) * 100 : 0
  const img = coverUrl(goal)
  const deadlineLabel = goal.deadline
    ? new Date(goal.deadline).toLocaleDateString("pt-BR", { month: "short", year: "numeric" })
    : null
  const instLine = [goal.institution, goal.account_type].filter(Boolean).join(" · ")

  const content = (
    <>
      {/* Hero */}
      <div className="relative h-[150px] overflow-hidden rounded-t-2xl">
        <img src={img} alt={goal.name} className="w-full h-full object-cover" />
        <div className="absolute inset-0 bg-gradient-to-b from-transparent to-black/60" />
        <p className="absolute bottom-3 left-4 right-24 text-white font-bold text-base truncate drop-shadow">
          {goal.name}
        </p>
        <div className="absolute bottom-2 right-3">
          <CircularProgress pct={pct} size={96} />
        </div>
      </div>

      {/* Info */}
      <div className="p-4 space-y-4">
        <div className="grid grid-cols-3 gap-3">
          <div className="text-center">
            <p className="text-xs text-muted-foreground uppercase tracking-wide" style={{ fontSize: 10 }}>Reservado</p>
            <p className="font-bold text-sm">{fmt(goal.current_amount)}</p>
          </div>
          <div className="text-center">
            <p className="text-xs text-muted-foreground uppercase tracking-wide" style={{ fontSize: 10 }}>Meta</p>
            <p className="font-bold text-sm">{fmt(goal.target_amount)}</p>
          </div>
          <div className="text-center">
            <p className="text-xs text-muted-foreground uppercase tracking-wide" style={{ fontSize: 10 }}>% Concluído</p>
            <p className="font-bold text-sm">{pct.toFixed(0)}%</p>
          </div>
        </div>

        <div className="grid grid-cols-2 gap-3 text-xs">
          {instLine && (
            <div className="col-span-2 flex items-center gap-1 text-foreground font-medium">
              <CircleDollarSign className="w-3.5 h-3.5 shrink-0" />
              <span className="truncate">{instLine}</span>
            </div>
          )}
          {deadlineLabel && (
            <div>
              <p className="text-muted-foreground uppercase tracking-wide" style={{ fontSize: 10 }}>Prazo</p>
              <p className="font-medium">{deadlineLabel}</p>
            </div>
          )}
          {goal.annual_rate > 0 && (
            <div>
              <p className="text-muted-foreground uppercase tracking-wide" style={{ fontSize: 10 }}>Rendimento</p>
              <p className="font-medium">{(goal.annual_rate * 100).toFixed(1)}% a.a.</p>
            </div>
          )}
          {goal.monthly_contribution > 0 && (
            <div>
              <p className="text-muted-foreground uppercase tracking-wide" style={{ fontSize: 10 }}>Contribuição</p>
              <p className="font-medium">{fmt(goal.monthly_contribution)}/mês</p>
            </div>
          )}
        </div>

        <div className="flex gap-2">
          <Button className="flex-1" onClick={() => { onClose(); onAddTransaction(goal, "manual") }}>
            <Plus className="w-4 h-4 mr-1" />
            Movimentar
          </Button>
          {goal.goal_type !== "emergency_global" && (
            <Button variant="outline" className="flex-1" onClick={() => { onClose(); onEditGoal(goal) }}>
              Alterar meta
            </Button>
          )}
        </div>

        <div className="space-y-2">
          <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide">Histórico de movimentações</p>
          <TransactionList txs={txs} loadingTxs={loadingTxs} fmt={fmt} />
        </div>
      </div>
    </>
  )

  if (isDesktop) {
    return (
      <Dialog open={open} onOpenChange={(v) => { if (!v) onClose() }}>
        <DialogContent className="sm:max-w-md p-0 overflow-hidden overflow-y-auto max-h-[90vh]">
          {content}
        </DialogContent>
      </Dialog>
    )
  }

  return (
    <Sheet open={open} onOpenChange={(v) => { if (!v) onClose() }}>
      <SheetContent side="bottom" className="pb-safe rounded-t-3xl max-h-[95vh] overflow-y-auto p-0">
        {content}
      </SheetContent>
    </Sheet>
  )
}

// ---------- EditGoalSheet ----------

interface EditGoalSheetProps {
  goal: Goal | null
  open: boolean
  onClose: () => void
  onSubmit: (id: string, payload: Partial<CreateGoalPayload>) => Promise<Goal>
}

function EditGoalSheet({ goal, open, onClose, onSubmit }: EditGoalSheetProps) {
  const isDesktop = useMediaQuery("(min-width: 1024px)")
  const [target, setTarget] = useState("")
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    if (open && goal) setTarget(String(goal.target_amount))
  }, [open, goal])

  async function handleSubmit() {
    if (!goal || !target) return
    const parsed = parseFloat(target.replace(",", "."))
    if (isNaN(parsed) || parsed <= 0) return
    setSaving(true)
    try {
      await onSubmit(goal.id, { target_amount: parsed })
      onClose()
    } finally {
      setSaving(false)
    }
  }

  const formContent = (
    <div className="space-y-3">
      <p className="text-sm text-muted-foreground">Nova meta para <span className="font-medium text-foreground">{goal?.name}</span></p>
      <Input
        type="number"
        placeholder="Valor da meta"
        value={target}
        onChange={(e) => setTarget(e.target.value)}
        inputMode="decimal"
        autoFocus
      />
      <Button onClick={handleSubmit} disabled={!target || saving} className="w-full">
        {saving ? "Salvando..." : "Salvar meta"}
      </Button>
    </div>
  )

  if (isDesktop) {
    return (
      <Dialog open={open} onOpenChange={(v) => { if (!v) onClose() }}>
        <DialogContent className="sm:max-w-sm">
          <DialogHeader className="mb-4">
            <DialogTitle>Alterar meta</DialogTitle>
          </DialogHeader>
          {formContent}
        </DialogContent>
      </Dialog>
    )
  }

  return (
    <Sheet open={open} onOpenChange={(v) => { if (!v) onClose() }}>
      <SheetContent side="bottom" className="pb-safe rounded-t-2xl">
        <SheetHeader className="mb-4">
          <SheetTitle>Alterar meta</SheetTitle>
        </SheetHeader>
        {formContent}
      </SheetContent>
    </Sheet>
  )
}

// ---------- AddTransactionSheet (responsive) ----------

interface AddTransactionSheetProps {
  goal: Goal | null
  open: boolean
  onClose: () => void
  onSubmit: (goalId: string, payload: AddTransactionPayload) => Promise<void>
  initialSource?: AddTransactionPayload["source"]
}

function AddTransactionSheet({ goal, open, onClose, onSubmit, initialSource }: AddTransactionSheetProps) {
  const isDesktop = useMediaQuery("(min-width: 1024px)")
  const [amount, setAmount] = useState("")
  const [description, setDescription] = useState("")
  const [source, setSource] = useState<AddTransactionPayload["source"]>(initialSource ?? "manual")
  const [expenseCat, setExpenseCat] = useState("")
  const [saving, setSaving] = useState(false)

  function reset() {
    setAmount("")
    setDescription("")
    setSource(initialSource ?? "manual")
    setExpenseCat("")
  }

  async function handleSubmit() {
    if (!goal || !amount) return
    const parsed = parseFloat(amount.replace(",", "."))
    if (isNaN(parsed)) return
    setSaving(true)
    try {
      const finalAmount = source === "withdrawal" ? -Math.abs(parsed) : Math.abs(parsed)
      await onSubmit(goal.id, {
        amount: finalAmount,
        description,
        source,
        expense_category: expenseCat || undefined,
      })
      reset()
      onClose()
    } finally {
      setSaving(false)
    }
  }

  const formContent = (
    <div className="space-y-3">
      <div className="grid grid-cols-2 gap-2">
        {(["manual", "yield", "withdrawal"] as const).map((s) => (
          <button
            key={s}
            aria-pressed={source === s}
            onClick={() => setSource(s)}
            className={cn(
              "text-xs py-2 px-3 rounded-lg border transition-colors text-left",
              source === s
                ? "border-primary bg-primary/10 text-primary font-medium"
                : "border-border text-muted-foreground hover:text-foreground"
            )}
          >
            {sourceLabel(s)}
          </button>
        ))}
      </div>
      <Input
        type="number"
        placeholder="Valor (ex: 500)"
        value={amount}
        onChange={(e) => setAmount(e.target.value)}
        inputMode="decimal"
      />
      <Input
        placeholder="Descrição (opcional)"
        value={description}
        onChange={(e) => setDescription(e.target.value)}
      />
      {source === "withdrawal" && (
        <select
          value={expenseCat}
          onChange={(e) => setExpenseCat(e.target.value)}
          className="w-full h-10 rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2"
        >
          <option value="">Categoria da despesa</option>
          <option value="saude">Saúde</option>
          <option value="casa">Casa</option>
          <option value="lazer">Lazer</option>
          <option value="educacao">Educação</option>
          <option value="vestuario">Vestuário</option>
          <option value="presentes">Presentes</option>
          <option value="servicos">Serviços</option>
          <option value="outros">Outros</option>
        </select>
      )}
      <Button onClick={handleSubmit} disabled={!amount || saving} className="w-full">
        {saving ? "Salvando..." : "Confirmar"}
      </Button>
    </div>
  )

  const title = `${goal?.name ?? ""} — Registrar movimento`

  if (isDesktop) {
    return (
      <Dialog open={open} onOpenChange={(v) => { if (!v) { reset(); onClose() } }}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader className="mb-4">
            <DialogTitle className="text-base">{title}</DialogTitle>
          </DialogHeader>
          {formContent}
        </DialogContent>
      </Dialog>
    )
  }

  return (
    <Sheet open={open} onOpenChange={(v) => { if (!v) { reset(); onClose() } }}>
      <SheetContent side="bottom" className="pb-safe rounded-t-2xl">
        <SheetHeader className="mb-4">
          <SheetTitle className="text-base">{title}</SheetTitle>
        </SheetHeader>
        {formContent}
      </SheetContent>
    </Sheet>
  )
}

// ---------- CreateGoalSheet ----------

interface CreateGoalSheetProps {
  open: boolean
  onClose: () => void
  onSubmit: (payload: CreateGoalPayload) => Promise<Goal>
}

function CreateGoalSheet({ open, onClose, onSubmit }: CreateGoalSheetProps) {
  const [name, setName] = useState("")
  const [target, setTarget] = useState("")
  const [initial, setInitial] = useState("")
  const [allocPct, setAllocPct] = useState("")
  const [institution, setInstitution] = useState("")
  const [accountType, setAccountType] = useState("")
  const [deadline, setDeadline] = useState("")
  const [goalType, setGoalType] = useState<NonNullable<CreateGoalPayload["goal_type"]>>("other")
  const [coverImageUrl, setCoverImageUrl] = useState("")
  const [imageTab, setImageTab] = useState<"library" | "url">("library")
  const [saving, setSaving] = useState(false)

  function reset() {
    setName("")
    setTarget("")
    setInitial("")
    setAllocPct("")
    setInstitution("")
    setAccountType("")
    setDeadline("")
    setGoalType("other")
    setCoverImageUrl("")
    setImageTab("library")
  }

  async function handleSubmit() {
    if (!name || !target) return
    setSaving(true)
    try {
      await onSubmit({
        name,
        target_amount: parseFloat(target.replace(",", ".")),
        initial_amount: initial ? parseFloat(initial.replace(",", ".")) : 0,
        allocation_pct: allocPct ? parseFloat(allocPct.replace(",", ".")) / 100 : 0,
        institution: institution || undefined,
        account_type: accountType || undefined,
        deadline: deadline || undefined,
        goal_type: goalType,
        cover_image_url: coverImageUrl || undefined,
      })
      reset()
      onClose()
    } finally {
      setSaving(false)
    }
  }

  const isDesktop = useMediaQuery("(min-width: 1024px)")
  const selectedCover = coverImageUrl || COVER_BY_TYPE[goalType] || COVER_BY_TYPE.other

  const typeSelect = (
    <div className="space-y-1">
      <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide">Tipo</p>
      <select
        value={goalType}
        onChange={(e) => {
          setGoalType(e.target.value as NonNullable<CreateGoalPayload["goal_type"]>)
          setCoverImageUrl("")
        }}
        className="w-full h-10 rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2"
      >
        <option value="travel">✈️ Viagem</option>
        <option value="car">🚗 Carro</option>
        <option value="house">🏠 Casa</option>
        <option value="emergency">🛡️ Emergência</option>
        <option value="other">📦 Outro</option>
      </select>
    </div>
  )

  const imageSection = (
    <div className="space-y-2">
      <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide">Imagem de capa</p>
      <div className="flex gap-1 bg-muted p-1 rounded-lg">
        <button
          aria-pressed={imageTab === "library"}
          onClick={() => setImageTab("library")}
          className={cn("flex-1 text-xs py-1.5 rounded-md transition-colors", imageTab === "library" ? "bg-card shadow-sm font-medium" : "text-muted-foreground")}
        >Biblioteca</button>
        <button
          aria-pressed={imageTab === "url"}
          onClick={() => setImageTab("url")}
          className={cn("flex-1 text-xs py-1.5 rounded-md transition-colors", imageTab === "url" ? "bg-card shadow-sm font-medium" : "text-muted-foreground")}
        >URL / Upload</button>
      </div>
      {imageTab === "library" && (
        <div className="grid grid-cols-3 gap-2">
          {Object.entries(COVER_BY_TYPE).filter(([k]) => k !== "emergency_global").map(([type, url]) => (
            <button
              key={type}
              onClick={() => setCoverImageUrl(url)}
              className={cn("relative h-16 rounded-lg overflow-hidden border-2 transition-colors",
                coverImageUrl === url || (!coverImageUrl && type === goalType) ? "border-primary" : "border-transparent"
              )}
            >
              <img src={url} alt={type} className="w-full h-full object-cover" />
            </button>
          ))}
        </div>
      )}
      {imageTab === "url" && (
        <Input placeholder="https://... URL da imagem" value={coverImageUrl} onChange={(e) => setCoverImageUrl(e.target.value)} />
      )}
      <div className="relative h-28 rounded-lg overflow-hidden">
        <img src={selectedCover} alt="Preview" className="w-full h-full object-cover" />
        <div className="absolute inset-0 bg-gradient-to-b from-transparent to-black/50 flex items-end p-3">
          <span className="text-white text-sm font-semibold truncate">{name || "Nome da reserva"}</span>
        </div>
      </div>
    </div>
  )

  const fieldsSection = (
    <div className="space-y-3">
      <div className="space-y-2">
        <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide">Obrigatório</p>
        <Input placeholder="Nome (ex: Emergência viagem Brasil)" value={name} onChange={(e) => setName(e.target.value)} />
        <Input type="number" placeholder="Meta (ex: 10000)" value={target} onChange={(e) => setTarget(e.target.value)} inputMode="decimal" />
      </div>
      <div className="space-y-2">
        <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide">Opcional</p>
        <Input type="number" placeholder="Saldo inicial (ex: 5000)" value={initial} onChange={(e) => setInitial(e.target.value)} inputMode="decimal" />
        <Input placeholder="Onde está o dinheiro (ex: Nubank, Clear)" value={institution} onChange={(e) => setInstitution(e.target.value)} />
        <Input placeholder="Tipo de conta (ex: CDB, ações, conta corrente)" value={accountType} onChange={(e) => setAccountType(e.target.value)} />
        <Input type="number" placeholder="% do futuro destinado (ex: 15)" value={allocPct} onChange={(e) => setAllocPct(e.target.value)} inputMode="decimal" />
        <Input type="date" placeholder="Prazo para atingir a meta" value={deadline} onChange={(e) => setDeadline(e.target.value)} />
      </div>
      <Button onClick={handleSubmit} disabled={!name || !target || saving} className="w-full">
        {saving ? "Criando..." : "Criar reserva"}
      </Button>
    </div>
  )

  if (isDesktop) {
    return (
      <Dialog open={open} onOpenChange={(v) => { if (!v) { reset(); onClose() } }}>
        <DialogContent className="max-w-2xl p-0 overflow-hidden">
          <DialogHeader className="px-6 pt-6 pb-0">
            <DialogTitle>Nova reserva</DialogTitle>
          </DialogHeader>
          <div className="grid grid-cols-2 gap-0 max-h-[80vh]">
            {/* Left col — image + type */}
            <div className="p-6 space-y-4 border-r border-border overflow-y-auto">
              {typeSelect}
              {imageSection}
            </div>
            {/* Right col — fields */}
            <div className="p-6 overflow-y-auto">
              {fieldsSection}
            </div>
          </div>
        </DialogContent>
      </Dialog>
    )
  }

  return (
    <Sheet open={open} onOpenChange={(v) => { if (!v) { reset(); onClose() } }}>
      <SheetContent side="bottom" className="pb-safe rounded-t-2xl overflow-y-auto max-h-[90vh]">
        <SheetHeader className="mb-4">
          <SheetTitle className="text-base">Nova reserva</SheetTitle>
        </SheetHeader>
        <div className="space-y-4">
          {typeSelect}
          {imageSection}
          {fieldsSection}
        </div>
      </SheetContent>
    </Sheet>
  )
}

// ---------- Main Page ----------

export default function Reservas() {
  const fmt = useFmt()
  const { goals, suggestions, loading, error, refetch, listTransactions, createGoal, addTransaction, updateGoal } = useReservas()
  const [selectedGoal, setSelectedGoal] = useState<Goal | null>(null)
  const [txSheetOpen, setTxSheetOpen] = useState(false)
  const [txInitialSource, setTxInitialSource] = useState<AddTransactionPayload["source"]>("manual")
  const [createSheetOpen, setCreateSheetOpen] = useState(false)
  const [detailGoal, setDetailGoal] = useState<Goal | null>(null)
  const [detailOpen, setDetailOpen] = useState(false)
  const [editGoal, setEditGoal] = useState<Goal | null>(null)
  const [editOpen, setEditOpen] = useState(false)

  function handleOpenAddTransaction(goal: Goal, initialSource?: AddTransactionPayload["source"]) {
    setSelectedGoal(goal)
    setTxInitialSource(initialSource ?? "manual")
    setTxSheetOpen(true)
  }

  function handleOpenDetail(goal: Goal) {
    setDetailGoal(goal)
    setDetailOpen(true)
  }

  function handleOpenEdit(goal: Goal) {
    setEditGoal(goal)
    setEditOpen(true)
  }

  async function handleAddTransaction(goalId: string, payload: AddTransactionPayload) {
    await addTransaction(goalId, payload)
    await refetch()
  }

  const sortedGoals = [...goals].sort((a, b) =>
    a.goal_type === "emergency_global" ? -1 : b.goal_type === "emergency_global" ? 1 : 0
  )

  const totalSaved = goals.reduce((s, g) => s + g.current_amount, 0)
  const totalTarget = goals.reduce((s, g) => s + g.target_amount, 0)

  return (
    <div className="h-[var(--app-height)] lg:h-full flex flex-col bg-background">
      <PageHeader title="Reservas" showBack />

      <div className="flex-1 overflow-y-auto px-4 pt-4 pb-4 space-y-4">
        {/* Summary */}
        <div className="bg-card border border-border rounded-xl p-4 flex justify-between items-center">
          <div>
            <p className="text-xs text-muted-foreground">Total reservado</p>
            <p className="text-2xl font-bold text-primary">{fmt(totalSaved)}</p>
          </div>
          <div className="text-right">
            <p className="text-xs text-muted-foreground">Meta total</p>
            <p className="text-lg font-semibold">{fmt(totalTarget)}</p>
          </div>
        </div>

        {/* Suggested contributions */}
        {suggestions.length > 0 && (
          <div className="bg-card border border-border rounded-xl p-4 space-y-2">
            <p className="text-sm font-semibold">Aportes sugeridos este mês</p>
            <p className="text-xs text-muted-foreground">Baseados nas suas receitas × percentual configurado</p>
            <div className="space-y-1.5 pt-1">
              {suggestions.map((s) => (
                <div key={s.goal_id} className="flex justify-between text-xs">
                  <span>{s.goal_name}</span>
                  <span className="font-medium text-green-600">{fmt(s.suggested_amount)}</span>
                </div>
              ))}
            </div>
          </div>
        )}

        {/* Goals list */}
        {loading && <p className="text-sm text-muted-foreground text-center py-8">Carregando reservas...</p>}
        {error && <p className="text-sm text-destructive text-center py-8">{error}</p>}
        {!loading && !error && goals.length === 0 && (
          <div className="text-center py-12 space-y-3">
            <PiggyBank className="w-12 h-12 text-muted-foreground mx-auto" />
            <p className="text-sm text-muted-foreground">Nenhuma reserva criada ainda.</p>
            <Button size="sm" onClick={() => setCreateSheetOpen(true)}>
              <Plus className="w-4 h-4 mr-1" /> Criar reserva
            </Button>
          </div>
        )}
        {!loading && (
          <div className="grid grid-cols-1 gap-4 lg:grid-cols-3 2xl:grid-cols-4 items-start">
            {sortedGoals.map((g) => (
              <GoalCard
                key={g.id}
                goal={g}
                onAddTransaction={handleOpenAddTransaction}
                onCardClick={handleOpenDetail}
              />
            ))}
          </div>
        )}
      </div>

      {/* FAB */}
      {goals.length > 0 && (
        <button
          onClick={() => setCreateSheetOpen(true)}
          className="fixed bottom-20 lg:bottom-8 right-4 lg:right-8 w-12 h-12 rounded-full bg-primary text-primary-foreground shadow-lg flex items-center justify-center z-10"
          aria-label="Nova reserva"
        >
          <Plus className="w-5 h-5" />
        </button>
      )}

      <GoalDetailSheet
        goal={detailGoal}
        open={detailOpen}
        onClose={() => setDetailOpen(false)}
        onAddTransaction={handleOpenAddTransaction}
        onEditGoal={handleOpenEdit}
        listTransactions={listTransactions}
      />

      <EditGoalSheet
        goal={editGoal}
        open={editOpen}
        onClose={() => setEditOpen(false)}
        onSubmit={updateGoal}
      />

      <AddTransactionSheet
        goal={selectedGoal}
        open={txSheetOpen}
        onClose={() => setTxSheetOpen(false)}
        onSubmit={handleAddTransaction}
        initialSource={txInitialSource}
      />

      <CreateGoalSheet
        open={createSheetOpen}
        onClose={() => setCreateSheetOpen(false)}
        onSubmit={createGoal}
      />

      <BottomNav />
    </div>
  )
}
