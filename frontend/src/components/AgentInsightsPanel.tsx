import { useAgentInsights, useMarkInsightRead } from "@/hooks/useAgentInsights"
import { useNavigate } from "react-router-dom"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"

export function AgentInsightsPanel() {
  const { data: insights = [] } = useAgentInsights()
  const markRead = useMarkInsightRead()
  const navigate = useNavigate()

  const latest = insights.slice(0, 3)

  if (latest.length === 0) return null

  function handleClick(id: string) {
    markRead.mutate(id)
    navigate("/chat")
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">Sugestões do agente</CardTitle>
      </CardHeader>
      <CardContent className="space-y-3 max-h-48 overflow-y-auto">
        {latest.map((insight) => (
          <div key={insight.id} className="flex items-start justify-between gap-2">
            <p className="text-sm line-clamp-3">{insight.body}</p>
            <Button
              size="sm"
              variant="ghost"
              className="shrink-0"
              onClick={() => handleClick(insight.id)}
            >
              Ver no chat
            </Button>
          </div>
        ))}
      </CardContent>
    </Card>
  )
}
