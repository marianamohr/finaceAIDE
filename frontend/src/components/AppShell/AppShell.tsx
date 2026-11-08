import { cn } from "@/lib/utils"
import { useLocation, useNavigate } from "react-router-dom"
import { useState } from "react"
import { MessageSquare, BarChart3, Wallet, BarChart2, Wrench, User, LogOut, PanelLeft, DollarSign, PiggyBank, Target, CalendarCheck } from "lucide-react"
import { useAuthStore } from "@/store/auth"
import { useAuth } from "@/hooks/useAuth"
import { useSettingsStore } from "@/store/settings"
import { useRecurringReminders } from "@/hooks/useRecurringExpenses"

const ALL_NAV_ITEMS = [
  { path: "/chat",     icon: MessageSquare, label: "Chat",    key: "chat" },
  { path: "/consumo",  icon: BarChart3,     label: "Consumo", key: "consumo" },
  { path: "/caixa",    icon: Wallet,        label: "Caixa",   key: "caixa" },
  { path: "/limites",  icon: Target,        label: "Limites", key: "limites" },
  { path: "/reservas", icon: PiggyBank,     label: "Reservas", key: "reservas" },
  { path: "/painel",   icon: BarChart2,     label: "Painel",  key: "painel" },
  { path: "/tools",    icon: Wrench,        label: "Tools",   key: "tools" },
  { path: "/profile",  icon: User,          label: "Perfil",  key: "profile" },
]

export function AppShell({ children }: { children: React.ReactNode }) {
  const [collapsed, setCollapsed] = useState(false)
  const location = useLocation()
  const navigate = useNavigate()
  const user = useAuthStore((s) => s.user)
  const { logout } = useAuth()
  const showGoals = useSettingsStore((s) => s.showGoals)
  const showTools = useSettingsStore((s) => s.showTools)
  const showLimitsCategories = useSettingsStore((s) => s.showLimitsCategories)

  const now = new Date()
  const { pendingCount } = useRecurringReminders(now.getFullYear(), now.getMonth() + 1)

  const NAV_ITEMS = ALL_NAV_ITEMS.filter(item => {
    if (item.key === "limites" && !showLimitsCategories) return false
    if (item.key === "reservas" && !showGoals) return false
    if (item.key === "tools" && !showTools) return false
    return true
  })

  return (
    <div className="flex h-[var(--app-height)] w-full bg-background">
      {/* Sidebar — desktop only */}
      <aside className={cn(
        "hidden lg:flex flex-col border-r border-border bg-sidebar transition-[width] duration-200 ease-linear shrink-0",
        collapsed ? "w-14" : "w-56"
      )}>
        {/* Header */}
        <div className={cn(
          "flex items-center border-b border-border h-14 shrink-0 px-3",
          collapsed ? "justify-center" : "gap-3"
        )}>
          <div className="w-8 h-8 rounded-lg bg-primary/10 flex items-center justify-center shrink-0">
            <DollarSign className="w-4 h-4 text-primary" />
          </div>
          {!collapsed && <span className="text-sm font-semibold truncate">FinanceAI</span>}
          <button
            onClick={() => setCollapsed(v => !v)}
            className={cn(
              "w-7 h-7 flex items-center justify-center rounded-md hover:bg-sidebar-accent transition-colors",
              collapsed ? "ml-0" : "ml-auto"
            )}
            aria-label="Toggle sidebar"
          >
            <PanelLeft className="w-4 h-4 text-sidebar-foreground" />
          </button>
        </div>

        {/* Nav */}
        <nav className="flex-1 px-2 py-3 space-y-1 overflow-y-auto">
          {NAV_ITEMS.map(({ path, icon: Icon, label }) => {
            const active =
              location.pathname === path ||
              (path !== "/chat" && location.pathname.startsWith(path))
            return (
              <button
                key={path}
                onClick={() => navigate(path)}
                title={collapsed ? label : undefined}
                className={cn(
                  "w-full flex items-center gap-3 px-2 py-2 rounded-lg text-sm font-medium transition-colors",
                  "hover:bg-sidebar-accent hover:text-sidebar-accent-foreground",
                  active
                    ? "bg-sidebar-accent text-sidebar-accent-foreground font-semibold"
                    : "text-sidebar-foreground",
                  collapsed && "justify-center px-0"
                )}
              >
                <Icon className="w-4 h-4 shrink-0" />
                {!collapsed && <span className="truncate">{label}</span>}
              </button>
            )
          })}

          {pendingCount > 0 && (
            <button
              onClick={() => navigate("/gastos-fixos/lembretes")}
              title={collapsed ? `${pendingCount} pendente(s)` : undefined}
              className={cn(
                "w-full flex items-center gap-3 px-2 py-2 rounded-lg text-sm font-medium transition-colors",
                "hover:bg-sidebar-accent hover:text-sidebar-accent-foreground",
                location.pathname === "/gastos-fixos/lembretes"
                  ? "bg-sidebar-accent text-sidebar-accent-foreground font-semibold"
                  : "text-warning",
                collapsed && "justify-center px-0"
              )}
            >
              <span className="relative shrink-0">
                <CalendarCheck className="w-4 h-4" />
                <span className="absolute -top-1.5 -right-1.5 min-w-[14px] h-3.5 rounded-full bg-destructive text-white text-[9px] font-bold flex items-center justify-center px-0.5">
                  {pendingCount}
                </span>
              </span>
              {!collapsed && <span className="truncate">Pendentes</span>}
            </button>
          )}
        </nav>

        {/* Footer */}
        <div className="border-t border-border p-3 space-y-2 shrink-0">
          <div className={cn("flex items-center gap-2", collapsed && "justify-center")}>
            <div className="w-8 h-8 rounded-full bg-primary/10 flex items-center justify-center shrink-0">
              <span className="text-xs font-bold text-primary">
                {user?.email?.charAt(0).toUpperCase() ?? "U"}
              </span>
            </div>
            {!collapsed && (
              <p className="text-xs text-sidebar-foreground truncate flex-1">{user?.email}</p>
            )}
          </div>
          <button
            onClick={logout}
            title={collapsed ? "Sair" : undefined}
            className={cn(
              "w-full flex items-center gap-2 px-2 py-1.5 rounded-lg text-xs text-destructive",
              "hover:bg-destructive/10 transition-colors",
              collapsed && "justify-center px-0"
            )}
          >
            <LogOut className="w-3.5 h-3.5 shrink-0" />
            {!collapsed && <span>Sair da conta</span>}
          </button>
        </div>
      </aside>

      {/* Conteúdo principal */}
      <div className="flex-1 min-w-0 flex flex-col h-full">
        {children}
      </div>
    </div>
  )
}
