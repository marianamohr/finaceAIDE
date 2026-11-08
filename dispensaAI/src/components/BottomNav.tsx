import { Refrigerator, Receipt, ListChecks, ChefHat, Settings } from "lucide-react";
import { useLocation, useNavigate } from "react-router-dom";
import { cn } from "@/lib/utils";
import { useT, type Key } from "@/i18n";

const NAV_ITEMS: { path: string; icon: React.ElementType; label: Key }[] = [
  { path: "/", icon: Refrigerator, label: "nav.pantry" },
  { path: "/nota", icon: Receipt, label: "nav.receipt" },
  { path: "/lista", icon: ListChecks, label: "nav.list" },
  { path: "/receitas", icon: ChefHat, label: "nav.recipes" },
  { path: "/ajustes", icon: Settings, label: "nav.settings" },
];

export function BottomNav() {
  const location = useLocation();
  const navigate = useNavigate();
  const t = useT();

  return (
    <nav className="border-t border-border bg-background shrink-0 safe-bottom">
      <div className="flex items-center justify-around h-14 max-w-lg mx-auto">
        {NAV_ITEMS.map(({ path, icon: Icon, label }) => {
          const active = path === "/" ? location.pathname === "/" : location.pathname.startsWith(path);
          return (
            <button
              key={path}
              type="button"
              onClick={() => navigate(path)}
              aria-current={active ? "page" : undefined}
              className={cn(
                "flex flex-col items-center gap-0.5 px-3 py-1 transition-colors min-w-[56px]",
                active ? "text-primary" : "text-muted-foreground",
              )}
            >
              <Icon className="w-5 h-5" />
              <span className="text-[10px] font-medium">{t(label)}</span>
            </button>
          );
        })}
      </div>
    </nav>
  );
}
