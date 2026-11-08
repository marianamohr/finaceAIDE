import { useState } from "react"
import { BottomNav } from "@/components/BottomNav/BottomNav"
import { PageHeader } from "@/components/PageHeader/PageHeader"
import { useLimits, LimitWithProgress, GroupWithProgress, GroupMemberInput } from "@/hooks/useLimits"
import { useCurrencyStore, formatCurrency, currencySymbol, maskCurrency } from "@/store/currency"
import { usePrivacyStore } from "@/store/privacy"
import { useSettings } from "@/hooks/useSettings"
import { useMediaQuery } from "@/hooks/useMediaQuery"
import { api } from "@/services/api"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Sheet, SheetContent, SheetHeader, SheetTitle } from "@/components/ui/sheet"
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { cn } from "@/lib/utils"
import {
  categoryLabel,
  categoryEmoji,
  EXPENSE_SLUGS_STANDARD,
  EXPENSE_SLUGS_FOOD_DETAIL,
} from "@/lib/categories"

interface Category {
  id: string
  slug: string
  label: string
  type: string
  mode: string
}

function LimitCircle({ pct, size }: { pct: number; size: number }) {
  const clamped = Math.min(pct, 100)
  const strokeWidth = size * 0.085
  const r = size / 2 - strokeWidth / 2
  const circumference = 2 * Math.PI * r
  const offset = circumference * (1 - clamped / 100)

  const stroke =
    pct > 100
      ? "hsl(var(--destructive))"
      : pct >= 70
      ? "hsl(var(--warning))"
      : "hsl(var(--primary))"

  return (
    <div
      className="relative inline-flex items-center justify-center shrink-0"
      style={{ width: size, height: size }}
    >
      <svg width={size} height={size} style={{ transform: "rotate(-90deg)" }}>
        <circle
          cx={size / 2}
          cy={size / 2}
          r={r}
          fill="none"
          stroke="hsl(var(--border))"
          strokeWidth={strokeWidth}
        />
        <circle
          cx={size / 2}
          cy={size / 2}
          r={r}
          fill="none"
          stroke={stroke}
          strokeWidth={strokeWidth}
          strokeLinecap="round"
          strokeDasharray={circumference}
          strokeDashoffset={offset}
          style={{ transition: "stroke-dashoffset 0.6s ease" }}
        />
      </svg>
      <span className="absolute text-[10px] font-bold" style={{ color: stroke }}>
        {Math.round(pct)}%
      </span>
    </div>
  )
}

function GroupCard({
  group,
  fmt,
  onEdit,
  onDelete,
}: {
  group: GroupWithProgress
  fmt: (v: number) => string
  onEdit: (group: GroupWithProgress) => void
  onDelete: (group: GroupWithProgress) => void
}) {
  const [expanded, setExpanded] = useState(false)
  const isOver = group.pct > 100
  const isWarn = group.pct >= 70 && group.pct <= 100

  return (
    <div
      className={cn(
        "rounded-xl border",
        isOver ? "border-destructive/40 bg-destructive/5" : "border-border bg-card"
      )}
    >
      <div
        className="flex items-center gap-3 p-3.5 cursor-pointer"
        onClick={() => setExpanded((v) => !v)}
      >
        <LimitCircle pct={group.pct} size={52} />
        <div className="flex-1 min-w-0">
          <p className="font-semibold text-sm truncate">
            📦 {group.label}
            <span className="text-xs font-normal text-muted-foreground ml-1.5">
              ({group.members.length} cat.)
            </span>
          </p>
          <p
            className={cn(
              "text-xs mt-0.5",
              isOver ? "text-destructive" : isWarn ? "text-warning" : "text-muted-foreground"
            )}
          >
            {fmt(group.spent)} de {fmt(group.monthly_limit)}
            {isOver && " ⚠️"}
          </p>
        </div>
        <div className="flex items-center gap-1 shrink-0">
          <button
            className="text-muted-foreground hover:text-primary text-xs px-2 py-1"
            onClick={(e) => { e.stopPropagation(); onEdit(group) }}
          >
            ✏️
          </button>
          <button
            className="text-muted-foreground hover:text-destructive text-xs px-2 py-1"
            onClick={(e) => { e.stopPropagation(); onDelete(group) }}
          >
            ✕
          </button>
          <span className="text-muted-foreground text-xs px-1 select-none">
            {expanded ? "▲" : "▼"}
          </span>
        </div>
      </div>

      {expanded && (
        <div className="border-t border-border divide-y divide-border">
          {group.members.map((m) => (
            <div key={m.id} className="flex items-center gap-3 px-3.5 py-2.5">
              <LimitCircle pct={m.pct} size={36} />
              <div className="flex-1 min-w-0">
                <p className="text-sm truncate">
                  {categoryEmoji(m.category_slug)} {categoryLabel(m.category_slug)}
                </p>
                <p className="text-xs text-muted-foreground">
                  {fmt(m.spent)} de {fmt(m.monthly_limit)}
                </p>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

function LimitCard({
  item,
  fmt,
  onEdit,
  onDelete,
}: {
  item: LimitWithProgress
  fmt: (v: number) => string
  onEdit: (item: LimitWithProgress) => void
  onDelete: (item: LimitWithProgress) => void
}) {
  const isOver = item.pct > 100
  const isWarn = item.pct >= 70 && item.pct <= 100

  return (
    <div
      className={cn(
        "flex items-center gap-3 p-3.5 rounded-xl border cursor-pointer",
        isOver ? "border-destructive/40 bg-destructive/5" : "border-border bg-card"
      )}
      onClick={() => onEdit(item)}
    >
      <LimitCircle pct={item.pct} size={52} />

      <div className="flex-1 min-w-0">
        <p className="font-semibold text-sm truncate">
          {categoryEmoji(item.category_slug)} {categoryLabel(item.category_slug)}
        </p>
        <p
          className={cn(
            "text-xs mt-0.5",
            isOver
              ? "text-destructive"
              : isWarn
              ? "text-warning"
              : "text-muted-foreground"
          )}
        >
          {fmt(item.spent)} de {fmt(item.monthly_limit)}
          {isOver && " ⚠️"}
        </p>
      </div>

      <button
        className="text-muted-foreground hover:text-destructive text-xs px-2 py-1 shrink-0"
        onClick={(e) => {
          e.stopPropagation()
          onDelete(item)
        }}
      >
        ✕
      </button>
    </div>
  )
}

function LimitForm({
  categories,
  existingSlugs,
  editing,
  editingGroup,
  currencyLabel,
  onSave,
  onSaveGroup,
  onCancel,
}: {
  categories: Category[]
  existingSlugs: string[]
  editing: LimitWithProgress | null
  editingGroup: GroupWithProgress | null
  currencyLabel: string
  onSave: (categoryId: string, monthlyLimit: number) => void
  onSaveGroup: (label: string, members: GroupMemberInput[]) => void
  onCancel: () => void
}) {
  const isEditingGroup = !!editingGroup
  const [mode, setMode] = useState<"individual" | "group">(isEditingGroup ? "group" : "individual")

  // Individual mode state
  const [selectedId, setSelectedId] = useState(editing?.category_id ?? "")
  const [amount, setAmount] = useState(editing ? String(editing.monthly_limit) : "")

  // Group mode state
  const [groupLabel, setGroupLabel] = useState(editingGroup?.label ?? "")
  const [selectedIds, setSelectedIds] = useState<string[]>(
    editingGroup ? editingGroup.members.map((m) => m.category_id) : []
  )
  const [memberLimits, setMemberLimits] = useState<Record<string, string>>(
    editingGroup
      ? Object.fromEntries(editingGroup.members.map((m) => [m.category_id, String(m.monthly_limit)]))
      : {}
  )

  const toggleId = (id: string) =>
    setSelectedIds((prev) =>
      prev.includes(id) ? prev.filter((x) => x !== id) : [...prev, id]
    )

  const available = categories.filter(
    (c) => !existingSlugs.includes(c.slug) || c.id === editing?.category_id
  )

  const groupValid =
    mode === "group" &&
    groupLabel.trim() !== "" &&
    selectedIds.length >= 2 &&
    selectedIds.every((id) => Number(memberLimits[id]) > 0)

  const individualValid =
    mode === "individual" &&
    (editing ? true : !!selectedId) &&
    Number(amount) > 0

  const handleSave = () => {
    if (mode === "group") {
      onSaveGroup(
        groupLabel.trim(),
        selectedIds.map((id) => ({ category_id: id, monthly_limit: Number(memberLimits[id]) }))
      )
    } else {
      onSave(editing ? editing.category_id : selectedId, Number(amount))
    }
  }

  return (
    <div className="flex flex-col gap-4 pt-2">
      {/* Mode selector — only when creating new */}
      {!editing && !editingGroup && (
        <div className="flex gap-1">
          <button
            onClick={() => setMode("individual")}
            className={cn(
              "flex-1 py-1.5 rounded-lg text-xs font-semibold border transition-colors",
              mode === "individual"
                ? "border-primary bg-primary/10 text-primary"
                : "border-transparent bg-muted/40 hover:bg-muted/60"
            )}
          >
            Individual
          </button>
          <button
            onClick={() => setMode("group")}
            className={cn(
              "flex-1 py-1.5 rounded-lg text-xs font-semibold border transition-colors",
              mode === "group"
                ? "border-primary bg-primary/10 text-primary"
                : "border-transparent bg-muted/40 hover:bg-muted/60"
            )}
          >
            Grupo
          </button>
        </div>
      )}

      {mode === "individual" && (
        <>
          {!editing && (
            <div>
              <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground mb-2">
                Categoria
              </p>
              <div className="flex flex-col gap-1.5 max-h-48 overflow-y-auto">
                {available.map((c) => (
                  <button
                    key={c.id}
                    onClick={() => setSelectedId(c.id)}
                    className={cn(
                      "flex items-center gap-2.5 px-3 py-2 rounded-lg text-sm font-medium text-left border transition-colors",
                      selectedId === c.id
                        ? "border-primary bg-primary/10 text-primary"
                        : "border-transparent bg-muted/40 hover:bg-muted/60"
                    )}
                  >
                    <span>{categoryEmoji(c.slug)}</span>
                    <span>{categoryLabel(c.slug)}</span>
                  </button>
                ))}
              </div>
            </div>
          )}

          <div>
            <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground mb-2">
              Limite mensal ({currencyLabel})
            </p>
            <Input
              type="number"
              inputMode="decimal"
              min="0"
              step="0.01"
              placeholder="0,00"
              value={amount}
              onChange={(e) => setAmount(e.target.value)}
            />
          </div>
        </>
      )}

      {mode === "group" && (
        <>
          <div>
            <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground mb-2">
              Apelido do grupo
            </p>
            <Input
              placeholder="ex: Alimentação"
              value={groupLabel}
              onChange={(e) => setGroupLabel(e.target.value)}
            />
          </div>

          <div>
            <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground mb-2">
              Categorias e limites ({currencyLabel})
            </p>
            <div className="flex flex-col gap-2 max-h-56 overflow-y-auto">
              {available.map((c) => {
                const isSelected = selectedIds.includes(c.id)
                return (
                  <div key={c.id} className="flex flex-col gap-1">
                    <button
                      onClick={() => toggleId(c.id)}
                      className={cn(
                        "flex items-center gap-2.5 px-3 py-2 rounded-lg text-sm font-medium text-left border transition-colors",
                        isSelected
                          ? "border-primary bg-primary/10 text-primary"
                          : "border-transparent bg-muted/40 hover:bg-muted/60"
                      )}
                    >
                      <span>{categoryEmoji(c.slug)}</span>
                      <span className="flex-1">{categoryLabel(c.slug)}</span>
                      {isSelected && <span className="text-xs opacity-70">✓</span>}
                    </button>
                    {isSelected && (
                      <Input
                        type="number"
                        inputMode="decimal"
                        min="0"
                        step="0.01"
                        placeholder={`Limite (${currencyLabel})`}
                        value={memberLimits[c.id] ?? ""}
                        onChange={(e) =>
                          setMemberLimits((prev) => ({ ...prev, [c.id]: e.target.value }))
                        }
                        className="ml-6 h-8 text-xs"
                      />
                    )}
                  </div>
                )
              })}
            </div>
          </div>
        </>
      )}

      <div className="flex gap-2 pt-1">
        <Button variant="outline" className="flex-1" onClick={onCancel}>
          Cancelar
        </Button>
        <Button
          className="flex-1"
          disabled={!(groupValid || individualValid)}
          onClick={handleSave}
        >
          Salvar limite
        </Button>
      </div>
    </div>
  )
}

export default function Limites() {
  const { individual, groups, isLoading, upsert, remove, createGroup, updateGroup, removeGroup } = useLimits()
  const { currency } = useCurrencyStore()
  const hidden = usePrivacyStore((s) => s.valuesHidden)
  const fmt = (v: number) =>
    hidden ? maskCurrency(v, currency) : formatCurrency(v, currency)
  const { categoryContext } = useSettings()
  const isDesktop = useMediaQuery("(min-width: 1024px)")

  const [allCategories, setAllCategories] = useState<Category[]>([])
  const [sheetOpen, setSheetOpen] = useState(false)
  const [editing, setEditing] = useState<LimitWithProgress | null>(null)
  const [editingGroup, setEditingGroup] = useState<GroupWithProgress | null>(null)

  const loadCategories = async () => {
    if (allCategories.length === 0) {
      const cats = await api.get<Category[]>("/categories")
      setAllCategories(cats ?? [])
    }
  }

  const openAdd = async () => {
    await loadCategories()
    setEditing(null)
    setEditingGroup(null)
    setSheetOpen(true)
  }

  const openEdit = async (item: LimitWithProgress) => {
    await loadCategories()
    setEditing(item)
    setEditingGroup(null)
    setSheetOpen(true)
  }

  const openEditGroup = async (group: GroupWithProgress) => {
    await loadCategories()
    setEditingGroup(group)
    setEditing(null)
    setSheetOpen(true)
  }

  const handleSave = async (categoryId: string, monthlyLimit: number) => {
    await upsert(categoryId, monthlyLimit)
    setSheetOpen(false)
    setEditing(null)
  }

  const handleSaveGroup = async (label: string, members: GroupMemberInput[]) => {
    if (editingGroup) {
      await updateGroup(editingGroup.id, label, members)
    } else {
      await createGroup(label, members)
    }
    setSheetOpen(false)
    setEditingGroup(null)
  }

  const handleDelete = async (item: LimitWithProgress) => {
    await remove(item.category_id)
  }

  const handleDeleteGroup = async (group: GroupWithProgress) => {
    await removeGroup(group.id)
  }

  const handleClose = () => {
    setSheetOpen(false)
    setEditing(null)
    setEditingGroup(null)
  }

  const expenseSlugs =
    categoryContext === "food_detail" ? EXPENSE_SLUGS_FOOD_DETAIL : EXPENSE_SLUGS_STANDARD
  const expenseCategories = allCategories.filter((c) => expenseSlugs.includes(c.slug))
  const existingSlugs = individual.map((l) => l.category_slug)

  const sorted = [...individual].sort((a, b) => {
    if (a.pct > 100 && b.pct <= 100) return -1
    if (b.pct > 100 && a.pct <= 100) return 1
    return b.pct - a.pct
  })

  const isEmpty = individual.length === 0 && groups.length === 0

  const now = new Date()
  const monthLabel = now.toLocaleDateString("pt-BR", { month: "long", year: "numeric" })

  const sheetTitle = editingGroup ? "Editar grupo" : editing ? "Editar limite" : "Novo limite"

  const formContent = (
    <LimitForm
      categories={expenseCategories}
      existingSlugs={existingSlugs}
      editing={editing}
      editingGroup={editingGroup}
      currencyLabel={currencySymbol(currency)}
      onSave={handleSave}
      onSaveGroup={handleSaveGroup}
      onCancel={handleClose}
    />
  )

  return (
    <div className="flex flex-col h-full">
      <PageHeader title="Limites" showBack />
      <div className="flex-1 overflow-y-auto px-4 pt-4 pb-4 flex flex-col gap-3 min-h-0">
        <div className="flex items-center justify-between">
          <p className="text-xs text-muted-foreground capitalize">{monthLabel}</p>
          <Button size="sm" onClick={openAdd}>
            + Adicionar
          </Button>
        </div>

        {isLoading ? (
          <p className="text-sm text-muted-foreground text-center pt-8">Carregando...</p>
        ) : isEmpty ? (
          <p className="text-sm text-muted-foreground text-center pt-8">
            Nenhum limite configurado. Adicione um limite para acompanhar seus gastos.
          </p>
        ) : (
          <>
            {groups.map((group) => (
              <GroupCard
                key={group.id}
                group={group}
                fmt={fmt}
                onEdit={openEditGroup}
                onDelete={handleDeleteGroup}
              />
            ))}
            {sorted.map((item) => (
              <LimitCard
                key={item.id}
                item={item}
                fmt={fmt}
                onEdit={openEdit}
                onDelete={handleDelete}
              />
            ))}
          </>
        )}
      </div>

      <BottomNav />

      {isDesktop ? (
        <Dialog open={sheetOpen} onOpenChange={(open) => { if (!open) handleClose() }}>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>{sheetTitle}</DialogTitle>
            </DialogHeader>
            {formContent}
          </DialogContent>
        </Dialog>
      ) : (
        <Sheet open={sheetOpen} onOpenChange={(open) => { if (!open) handleClose() }}>
          <SheetContent side="bottom" className="rounded-t-2xl pb-8">
            <SheetHeader>
              <SheetTitle>{sheetTitle}</SheetTitle>
            </SheetHeader>
            {formContent}
          </SheetContent>
        </Sheet>
      )}
    </div>
  )
}
