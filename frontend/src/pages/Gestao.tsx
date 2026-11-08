import { BarChart2, TrendingUp, Target, PiggyBank, ChevronRight } from "lucide-react"
import { useNavigate } from "react-router-dom"
import { BottomNav } from "@/components/BottomNav/BottomNav"
import { PageHeader } from "@/components/PageHeader/PageHeader"
import { useSettingsStore } from "@/store/settings"

interface HubCard {
  label: string
  subtitle: string
  path: string
  icon: React.ElementType
  iconBg: string
  iconColor: string
}

const CARDS: HubCard[] = [
  {
    label: "Painel",
    subtitle: "Histórico e tendências",
    path: "/painel",
    icon: BarChart2,
    iconBg: "rgba(59,130,246,0.15)",
    iconColor: "#3b82f6",
  },
  {
    label: "Projeção",
    subtitle: "Próximos 6 meses",
    path: "/projecao",
    icon: TrendingUp,
    iconBg: "rgba(168,85,247,0.15)",
    iconColor: "#a855f7",
  },
  {
    label: "Limites",
    subtitle: "Orçamento por categoria",
    path: "/limites",
    icon: Target,
    iconBg: "rgba(245,158,11,0.15)",
    iconColor: "hsl(38,92%,50%)",
  },
  {
    label: "Reservas",
    subtitle: "Metas de poupança",
    path: "/reservas",
    icon: PiggyBank,
    iconBg: "rgba(20,179,122,0.15)",
    iconColor: "hsl(160,84%,45%)",
  },
]

export default function Gestao() {
  const navigate = useNavigate()
  const showGoals = useSettingsStore((s) => s.showGoals)
  const showLimitsCategories = useSettingsStore((s) => s.showLimitsCategories)

  const visibleCards = CARDS.filter((card) => {
    if (card.path === "/limites") return showLimitsCategories
    if (card.path === "/reservas") return showGoals
    return true
  })

  return (
    <div className="h-[var(--app-height)] lg:h-full flex flex-col bg-background">
      <div className="lg:hidden">
        <PageHeader title="Gestão" />
      </div>
      <div className="flex-1 overflow-y-auto p-4 flex flex-col gap-3">
        {visibleCards.map(({ label, subtitle, path, icon: Icon, iconBg, iconColor }) => (
          <button
            key={path}
            onClick={() => navigate(path)}
            className="flex items-center gap-4 p-4 rounded-xl bg-card border border-border text-left hover:bg-secondary/50 transition-colors w-full"
          >
            <div
              className="w-10 h-10 rounded-xl flex items-center justify-center shrink-0"
              style={{ background: iconBg }}
            >
              <Icon className="w-5 h-5" style={{ color: iconColor }} />
            </div>
            <div className="flex-1 min-w-0">
              <p className="font-semibold text-sm text-foreground">{label}</p>
              <p className="text-xs text-muted-foreground mt-0.5">{subtitle}</p>
            </div>
            <ChevronRight className="w-4 h-4 text-muted-foreground shrink-0" />
          </button>
        ))}
      </div>
      <BottomNav />
    </div>
  )
}
