import { MessageSquare, BarChart3, Wallet, Wrench, LayoutDashboard, CalendarCheck } from "lucide-react";
import { useLocation, useNavigate } from "react-router-dom";
import { cn } from "@/lib/utils";
import { useSettingsStore } from "@/store/settings";
import { useRecurringReminders } from "@/hooks/useRecurringExpenses";

const GESTAO_SUBROUTES = ["/painel", "/projecao", "/limites", "/reservas"]

export function BottomNav() {
  const location = useLocation();
  const navigate = useNavigate();
  const showTools = useSettingsStore((s) => s.showTools);

  const now = new Date();
  const { pendingCount } = useRecurringReminders(now.getFullYear(), now.getMonth() + 1);

  const navItems = [
    { path: "/chat", icon: MessageSquare, label: "Chat" },
    { path: "/consumo", icon: BarChart3, label: "Consumo" },
    { path: "/caixa", icon: Wallet, label: "Caixa" },
    { path: "/gestao", icon: LayoutDashboard, label: "Gestão" },
    ...(showTools ? [{ path: "/tools", icon: Wrench, label: "Tools" } as const] : []),
  ];

  return (
    <nav className="border-t border-border bg-background lg:hidden shrink-0">
      <div className="flex items-center justify-around h-14 max-w-lg mx-auto">
        {navItems.map(({ path, icon: Icon, label }) => {
          const active =
            location.pathname === path ||
            (path === "/gestao" && GESTAO_SUBROUTES.includes(location.pathname));
          return (
            <button
              key={path}
              onClick={() => navigate(path)}
              className={cn(
                "relative flex flex-col items-center gap-0.5 px-4 py-1 transition-colors",
                active ? "text-primary" : "text-muted-foreground"
              )}
            >
              <Icon className="w-5 h-5" />
              <span className="text-[10px] font-medium">{label}</span>
            </button>
          );
        })}

        {pendingCount > 0 && (
          <button
            onClick={() => navigate("/gastos-fixos/lembretes")}
            className={cn(
              "relative flex flex-col items-center gap-0.5 px-4 py-1 transition-colors",
              location.pathname === "/gastos-fixos/lembretes" ? "text-primary" : "text-warning"
            )}
          >
            <span className="relative">
              <CalendarCheck className="w-5 h-5" />
              <span className="absolute -top-1.5 -right-1.5 min-w-[16px] h-4 rounded-full bg-destructive text-white text-[9px] font-bold flex items-center justify-center px-0.5">
                {pendingCount}
              </span>
            </span>
            <span className="text-[10px] font-medium">Pendentes</span>
          </button>
        )}
      </div>
    </nav>
  );
}
