import { useAuth } from "@/hooks/useAuth";
import { Button } from "@/components/ui/button";
import { Clock, LogOut } from "lucide-react";

export default function PendingApproval() {
  const { logout } = useAuth();

  return (
    <div className="h-[var(--app-height)] flex flex-col items-center justify-center bg-background px-6 text-center space-y-6">
      <div className="w-16 h-16 rounded-full bg-primary/10 flex items-center justify-center">
        <Clock className="w-8 h-8 text-primary" />
      </div>

      <div className="space-y-2">
        <h1 className="font-heading text-xl font-bold">Aguardando aprovação</h1>
        <p className="text-sm text-muted-foreground max-w-xs">
          Sua conta foi criada com sucesso. Um administrador irá aprová-la em breve.
          Você receberá acesso assim que for aprovado.
        </p>
      </div>

      <Button
        variant="ghost"
        onClick={logout}
        className="text-destructive hover:text-destructive hover:bg-destructive/10"
      >
        <LogOut className="w-4 h-4 mr-2" />
        Sair da conta
      </Button>
    </div>
  );
}
