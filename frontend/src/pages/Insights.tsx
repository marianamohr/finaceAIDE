import { BottomNav } from "@/components/BottomNav/BottomNav"
import { PageHeader } from "@/components/PageHeader/PageHeader"
import { useAgentInsights, useMarkInsightRead } from "@/hooks/useAgentInsights"
import { Check, Lightbulb } from "lucide-react"

export default function Insights() {
  const { data: insights = [], isLoading } = useAgentInsights()
  const markRead = useMarkInsightRead()

  return (
    <div className="h-[var(--app-height)] flex flex-col bg-background">
      <PageHeader title="Insights do agente" />
      <div className="flex-1 overflow-y-auto px-4 py-4 space-y-3">
        {isLoading && (
          <p className="text-sm text-muted-foreground text-center pt-8">Carregando...</p>
        )}
        {!isLoading && insights.length === 0 && (
          <div className="flex flex-col items-center gap-3 pt-16 text-muted-foreground">
            <Lightbulb className="w-10 h-10" />
            <p className="text-sm">Nenhum insight disponível ainda.</p>
          </div>
        )}
        {insights.map((insight) => (
          <div
            key={insight.id}
            className="rounded-xl border border-border bg-card p-4 flex items-start gap-3"
          >
            <p className="text-sm leading-relaxed flex-1">{insight.body}</p>
            {!insight.read_at && (
              <button
                className="shrink-0 mt-0.5 text-muted-foreground hover:text-primary transition-colors"
                title="Marcar como lido"
                onClick={() => markRead.mutate(insight.id)}
              >
                <Check className="w-4 h-4" />
              </button>
            )}
          </div>
        ))}
      </div>
      <BottomNav />
    </div>
  )
}
