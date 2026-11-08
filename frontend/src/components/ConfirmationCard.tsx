import { useMutation, useQueryClient } from "@tanstack/react-query"
import { useState } from "react"
import { api, PendingAction, ConfirmedTransaction, ClarifyDraft } from "@/services/api"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardFooter } from "@/components/ui/card"
import { useChatStore } from "@/store/chat"

function buildSavedMessages(txs: ConfirmedTransaction[]): { content: string; displayContent: string } {
  const lines = txs.map((tx) => {
    const label = tx.description || tx.category
    const amount = Math.abs(tx.amount).toFixed(2).replace(".", ",")
    return { display: `• ${label} — R$ ${amount}`, hidden: ` (id:${tx.id})` }
  })
  const prefix = txs.length > 1
    ? `Nota fiscal salva com ${txs.length} transações (todas fazem parte desta nota):`
    : `Transação salva:`
  return {
    content: `${prefix}\n${lines.map((l) => l.display + l.hidden).join("\n")}`,
    displayContent: `Salvo:\n${lines.map((l) => l.display).join("\n")}`,
  }
}

interface ConfirmationCardProps {
  pendingAction: PendingAction
}

function formatDraftSummary(draft: ClarifyDraft): string {
  const label = draft.tx.description ?? draft.tx.category
  const amount = Math.abs(draft.tx.amount).toFixed(2).replace(".", ",")
  return `${label} — R$ ${amount} (${draft.tx.date})`
}

interface ClarifyPaymentMethodCardProps {
  pendingAction: PendingAction
}

// optionsMatch returns true when every draft has the same set of PM options.
// When true, we assume the drafts came from a single source (e.g. one receipt)
// and offer a single shared PM picker instead of one row per draft.
function optionsMatch(drafts: ClarifyDraft[]): boolean {
  if (drafts.length < 2) return false
  const first = drafts[0].needs.options.map((o) => o.payment_method_id).sort().join(",")
  return drafts.every(
    (d) => d.needs.options.map((o) => o.payment_method_id).sort().join(",") === first,
  )
}

function ClarifyPaymentMethodCard({ pendingAction }: ClarifyPaymentMethodCardProps) {
  const queryClient = useQueryClient()
  const { resolvedActions, resolveAction, addMessage } = useChatStore()
  const resolution = resolvedActions[pendingAction.id]

  const drafts = pendingAction.payload?.drafts ?? []
  const shared = optionsMatch(drafts)
  const [individualMode, setIndividualMode] = useState(false)
  const [picked, setPicked] = useState<Record<number, string>>({})
  const [sharedPick, setSharedPick] = useState<string | undefined>(undefined)

  const useShared = shared && !individualMode
  const allPicked = useShared
    ? sharedPick !== undefined
    : drafts.length > 0 && drafts.every((_, i) => picked[i] !== undefined)

  const confirm = useMutation({
    mutationFn: () => {
      const resolutions = useShared && sharedPick
        ? drafts.map((_, i) => ({ index: i, payment_method_id: sharedPick }))
        : Object.entries(picked).map(([idx, pmId]) => ({
            index: Number(idx),
            payment_method_id: pmId,
          }))
      return api.confirmClarifyPaymentMethod(pendingAction.id, resolutions)
    },
    onSuccess: (data) => {
      resolveAction(pendingAction.id, "confirmed")
      queryClient.invalidateQueries({ queryKey: ["transactions"] })
      queryClient.invalidateQueries({ queryKey: ["goals"] })
      queryClient.invalidateQueries({ queryKey: ["dashboard"] })
      if (data.transactions && data.transactions.length > 0) {
        const { content, displayContent } = buildSavedMessages(data.transactions)
        addMessage({
          id: crypto.randomUUID(),
          role: "assistant",
          content,
          displayContent,
          createdAt: new Date(),
        })
      }
    },
  })

  if (resolution === "cancelled") {
    return (
      <Card className="mt-2 border-muted">
        <CardContent className="pt-4 text-sm text-muted-foreground">
          Ação cancelada.
        </CardContent>
      </Card>
    )
  }

  if (resolution === "confirmed") {
    return (
      <Card className="mt-2 border-green-500">
        <CardContent className="pt-4 text-sm text-green-700 dark:text-green-400">
          ✓ Transação salva.
        </CardContent>
      </Card>
    )
  }

  if (useShared) {
    const options = drafts[0].needs.options
    return (
      <Card className="mt-2">
        <CardContent className="pt-4 space-y-4">
          <div>
            <p className="text-sm font-medium">
              {drafts.length} {drafts.length === 1 ? "transação" : "transações"} da mesma nota
            </p>
            <p className="text-xs text-muted-foreground">Qual forma de pagamento você usou?</p>
          </div>
          {drafts[0].needs.detected_payment_label && (
            <p className="text-xs text-muted-foreground">
              A nota indica: {drafts[0].needs.detected_payment_label}
            </p>
          )}
          <ul className="space-y-1 text-xs text-muted-foreground">
            {drafts.map((draft, i) => (
              <li key={i}>• {formatDraftSummary(draft)}</li>
            ))}
          </ul>
          <div className="flex flex-wrap gap-2">
            {options.map((opt) => (
              <Button
                key={opt.payment_method_id}
                size="sm"
                variant={sharedPick === opt.payment_method_id ? "default" : "outline"}
                onClick={() => setSharedPick(opt.payment_method_id)}
                disabled={confirm.isPending}
              >
                {opt.label}
              </Button>
            ))}
          </div>
          <button
            type="button"
            className="text-xs text-muted-foreground underline"
            onClick={() => setIndividualMode(true)}
            disabled={confirm.isPending}
          >
            Definir uma forma de pagamento por item
          </button>
        </CardContent>
        <CardFooter className="gap-2">
          <Button
            size="sm"
            onClick={() => confirm.mutate()}
            disabled={!allPicked || confirm.isPending}
          >
            {confirm.isPending ? "Confirmando..." : "Confirmar"}
          </Button>
          <Button
            size="sm"
            variant="outline"
            onClick={() => resolveAction(pendingAction.id, "cancelled")}
            disabled={confirm.isPending}
          >
            Cancelar
          </Button>
        </CardFooter>
      </Card>
    )
  }

  return (
    <Card className="mt-2">
      <CardContent className="pt-4 space-y-4">
        {shared && (
          <button
            type="button"
            className="text-xs text-muted-foreground underline"
            onClick={() => setIndividualMode(false)}
            disabled={confirm.isPending}
          >
            ← Voltar para forma de pagamento única
          </button>
        )}
        {drafts.map((draft, i) => (
          <div key={i} className="space-y-2">
            <p className="text-sm font-medium">{formatDraftSummary(draft)}</p>
            <p className="text-xs text-muted-foreground">{draft.needs.prompt}</p>
            {draft.needs.detected_payment_label && (
              <p className="text-xs text-muted-foreground">
                A nota indica: {draft.needs.detected_payment_label}
              </p>
            )}
            <div className="flex flex-wrap gap-2">
              {draft.needs.options.map((opt) => (
                <Button
                  key={opt.payment_method_id}
                  size="sm"
                  variant={picked[i] === opt.payment_method_id ? "default" : "outline"}
                  onClick={() => setPicked((prev) => ({ ...prev, [i]: opt.payment_method_id }))}
                  disabled={confirm.isPending}
                >
                  {opt.label}
                </Button>
              ))}
            </div>
          </div>
        ))}
      </CardContent>
      <CardFooter className="gap-2">
        <Button
          size="sm"
          onClick={() => confirm.mutate()}
          disabled={!allPicked || confirm.isPending}
        >
          {confirm.isPending ? "Confirmando..." : "Confirmar"}
        </Button>
        <Button
          size="sm"
          variant="outline"
          onClick={() => resolveAction(pendingAction.id, "cancelled")}
          disabled={confirm.isPending}
        >
          Cancelar
        </Button>
      </CardFooter>
    </Card>
  )
}

export function ConfirmationCard({ pendingAction }: ConfirmationCardProps) {
  if (
    pendingAction.tool_name === 'clarify_payment_method' ||
    pendingAction.tool_name === 'clarify_installment_card'
  ) {
    return <ClarifyPaymentMethodCard pendingAction={pendingAction} />
  }

  const queryClient = useQueryClient()
  const { resolvedActions, resolveAction, addMessage } = useChatStore()
  const resolution = resolvedActions[pendingAction.id]

  const confirm = useMutation({
    mutationFn: () => api.confirmAction(pendingAction.id),
    onSuccess: (data) => {
      resolveAction(pendingAction.id, "confirmed")
      queryClient.invalidateQueries({ queryKey: ["transactions"] })
      queryClient.invalidateQueries({ queryKey: ["goals"] })
      queryClient.invalidateQueries({ queryKey: ["dashboard"] })
      if (data.transactions && data.transactions.length > 0) {
        const { content, displayContent } = buildSavedMessages(data.transactions)
        addMessage({
          id: crypto.randomUUID(),
          role: "assistant",
          content,
          displayContent,
          createdAt: new Date(),
        })
      }
    },
  })

  if (resolution === "cancelled") {
    return (
      <Card className="mt-2 border-muted">
        <CardContent className="pt-4 text-sm text-muted-foreground">
          Ação cancelada.
        </CardContent>
      </Card>
    )
  }

  if (resolution === "confirmed") {
    return (
      <Card className="mt-2 border-green-500">
        <CardContent className="pt-4 text-sm text-green-700 dark:text-green-400">
          ✓ Transação salva.
        </CardContent>
      </Card>
    )
  }

  return (
    <Card className="mt-2">
      <CardContent className="pt-4">
        <p className="text-sm font-medium whitespace-pre-wrap">{pendingAction.preview}</p>
        <p className="text-xs text-muted-foreground mt-1">Confirma esta ação?</p>
      </CardContent>
      <CardFooter className="gap-2">
        <Button
          size="sm"
          onClick={() => confirm.mutate()}
          disabled={confirm.isPending}
        >
          {confirm.isPending ? "Confirmando..." : "Confirmar"}
        </Button>
        <Button
          size="sm"
          variant="outline"
          onClick={() => resolveAction(pendingAction.id, "cancelled")}
          disabled={confirm.isPending}
        >
          Cancelar
        </Button>
      </CardFooter>
    </Card>
  )
}
