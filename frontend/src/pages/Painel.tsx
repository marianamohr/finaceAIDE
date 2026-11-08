import { AnalyticsDashboard } from "@/components/analytics/AnalyticsDashboard"
import { BottomNav } from "@/components/BottomNav/BottomNav"
import { PageHeader } from "@/components/PageHeader/PageHeader"

export default function Painel() {
  return (
    <div className="h-[var(--app-height)] lg:h-full flex flex-col bg-background">
      <div className="lg:hidden">
        <PageHeader title="Painel" showBack />
      </div>
      <div className="flex-1 overflow-y-auto">
        <AnalyticsDashboard />
      </div>
      <BottomNav />
    </div>
  )
}
