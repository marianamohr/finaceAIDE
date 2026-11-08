import { Bell } from "lucide-react"
import { useAgentInsights, useMarkInsightRead } from "@/hooks/useAgentInsights"
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { useNavigate } from "react-router-dom"
import { useChatStore } from "@/store/chat"

export function NotificationCenter() {
  const { data: insights = [] } = useAgentInsights(true)
  const markRead = useMarkInsightRead()
  const navigate = useNavigate()

  const unreadCount = insights.length

  function handleInsightClick(id: string, body: string) {
    markRead.mutate(id)
    useChatStore.getState().injectInsight(body)
    navigate("/chat")
  }

  return (
    <Popover>
      <PopoverTrigger asChild>
        <Button variant="ghost" size="icon" className="relative">
          <Bell className="h-5 w-5" />
          {unreadCount > 0 && (
            <Badge className="absolute -top-1 -right-1 h-5 w-5 p-0 flex items-center justify-center text-xs">
              {unreadCount}
            </Badge>
          )}
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-80" align="end">
        <div className="space-y-2">
          <h4 className="font-medium text-sm">Insights do agente</h4>
          {insights.length === 0 ? (
            <p className="text-sm text-muted-foreground">Nenhum insight novo.</p>
          ) : (
            insights.map((insight) => (
              <button
                key={insight.id}
                className="w-full text-left p-2 rounded hover:bg-muted text-sm"
                onClick={() => handleInsightClick(insight.id, insight.body)}
              >
                {insight.body}
              </button>
            ))
          )}
        </div>
      </PopoverContent>
    </Popover>
  )
}
