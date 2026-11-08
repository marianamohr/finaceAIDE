import { BottomNav } from "@/components/BottomNav";
import { usePantrySettings } from "@/hooks/usePantrySettings";

/**
 * Layout mobile-first: conteúdo rolável + barra inferior fixa.
 * Também dispara o carregamento das configurações (idioma) logo após o login.
 */
export function AppShell({ children }: { children: React.ReactNode }) {
  usePantrySettings();

  return (
    <div className="h-[var(--app-height)] w-full flex flex-col bg-background">
      <div className="flex-1 min-h-0 flex flex-col">{children}</div>
      <BottomNav />
    </div>
  );
}
