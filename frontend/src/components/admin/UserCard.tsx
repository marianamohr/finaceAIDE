import { AdminUser } from "@/services/api";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";
import { CheckCircle, XCircle, Link, Copy, Users } from "lucide-react";
import { cn } from "@/lib/utils";

const statusLabel: Record<string, string> = {
  active: "Ativo",
  pending: "Pendente",
  suspended: "Suspenso",
};

const statusDot: Record<string, string> = {
  active:    "bg-success",
  pending:   "bg-warning",
  suspended: "bg-destructive",
};

const statusBadge: Record<string, string> = {
  active:    "bg-success/10 text-success border border-success/25",
  pending:   "bg-warning/10 text-warning border border-warning/25",
  suspended: "bg-destructive/10 text-destructive border border-destructive/25",
};

interface UserCardProps {
  user: AdminUser;
  actionLoading: string | null;
  linkMode: string | null;
  copiedAccountId: string | null;
  linkedGroups: Map<string, string[]>;
  onSuspendClick: (user: AdminUser) => void;
  onApprove: (id: string) => void;
  onReactivate: (id: string) => void;
  onToggleTools: (id: string, current: boolean) => void;
  onLinkStart: (id: string) => void;
  onLinkConfirm: (targetId: string) => void;
  onCopyAccountId: (accountId: string) => void;
}

export function UserCard({
  user,
  actionLoading,
  linkMode,
  copiedAccountId,
  linkedGroups,
  onSuspendClick,
  onApprove,
  onReactivate,
  onToggleTools,
  onLinkStart,
  onLinkConfirm,
  onCopyAccountId,
}: UserCardProps) {
  const isAdmin = user.role === "admin";
  const isLinkTarget = !!linkMode && linkMode !== user.id;

  return (
    <div
      className={cn(
        "rounded-xl bg-card border border-border p-4 space-y-3",
        isLinkTarget && "cursor-pointer ring-2 ring-primary/30 hover:ring-primary/60"
      )}
      onClick={() => isLinkTarget && onLinkConfirm(user.id)}
    >
      {/* Linha 1: email + badge status */}
      <div className="flex items-start justify-between gap-2">
        <div className="min-w-0 flex-1">
          <p className="font-semibold text-sm text-foreground truncate">{user.email}</p>
          {(() => {
            const peers = linkedGroups.get(user.account_id);
            if (peers && peers.length >= 2) {
              const others = peers.filter((e) => e !== user.email);
              return (
                <div className="mt-1 space-y-0.5">
                  <span className="inline-flex items-center gap-1 text-[10px] font-semibold px-1.5 py-0.5 rounded bg-primary/10 text-primary border border-primary/20">
                    <Users className="w-2.5 h-2.5" />
                    Vinculado · {peers.length} usuários
                  </span>
                  {others.length > 0 && (
                    <p className="text-[10px] text-muted-foreground leading-snug">
                      com {others.join(", ")}
                    </p>
                  )}
                </div>
              );
            }
            return (
              <div className="flex items-center gap-1 mt-0.5">
                <span className="font-mono text-[10px] text-muted-foreground">
                  {user.account_id.slice(0, 8)}…
                </span>
                <button
                  onClick={(e) => { e.stopPropagation(); onCopyAccountId(user.account_id); }}
                  className="text-muted-foreground hover:text-foreground transition-colors"
                >
                  <Copy className="w-3 h-3" />
                </button>
                {copiedAccountId === user.account_id && (
                  <span className="text-[10px] text-primary">Copiado!</span>
                )}
              </div>
            );
          })()}
        </div>
        <span className={cn(
          "inline-flex items-center gap-1.5 text-[10px] font-semibold px-2 py-1 rounded-full uppercase tracking-wide shrink-0",
          statusBadge[user.status] ?? "bg-secondary text-foreground border border-border"
        )}>
          <span className={cn("w-1.5 h-1.5 rounded-full", statusDot[user.status])} />
          {statusLabel[user.status] ?? user.status}
        </span>
      </div>

      {/* Linha 2: cargo + tools toggle */}
      <div className="flex items-center gap-2">
        <span className={cn(
          "text-[10px] font-semibold px-1.5 py-0.5 rounded",
          isAdmin
            ? "text-primary bg-primary/10 border border-primary/20"
            : "text-muted-foreground bg-secondary border border-border"
        )}>
          {isAdmin ? "Admin" : "Usuário"}
        </span>
        {!isAdmin && (
          <div className="flex items-center gap-1.5 ml-auto" onClick={(e) => e.stopPropagation()}>
            <span className="text-xs text-muted-foreground">Tools</span>
            <Switch
              checked={user.show_tools}
              onCheckedChange={() => onToggleTools(user.id, user.show_tools)}
              className="scale-75 origin-right"
            />
          </div>
        )}
      </div>

      {/* Linha 3: ações */}
      {!isAdmin && (
        <div className="flex flex-wrap items-center gap-2" onClick={(e) => e.stopPropagation()}>
          {user.status === "pending" && (
            <Button
              size="sm"
              variant="outline"
              className="h-7 text-xs text-success border-success/30 hover:bg-success/10"
              disabled={!!actionLoading}
              onClick={() => onApprove(user.id)}
            >
              <CheckCircle className="w-3 h-3 mr-1" />
              Aprovar
            </Button>
          )}
          {user.status === "active" && (
            <Button
              size="sm"
              variant="outline"
              className="h-7 text-xs text-destructive border-destructive/30 hover:bg-destructive/10"
              disabled={!!actionLoading}
              onClick={() => onSuspendClick(user)}
            >
              <XCircle className="w-3 h-3 mr-1" />
              Suspender
            </Button>
          )}
          {user.status === "suspended" && (
            <Button
              size="sm"
              variant="outline"
              className="h-7 text-xs text-primary border-primary/30 hover:bg-primary/10"
              disabled={!!actionLoading}
              onClick={() => onReactivate(user.id)}
            >
              <CheckCircle className="w-3 h-3 mr-1" />
              Reativar
            </Button>
          )}
          {!linkMode && (
            <Button
              size="sm"
              variant="outline"
              className="h-7 text-xs"
              disabled={!!actionLoading}
              onClick={() => onLinkStart(user.id)}
            >
              <Link className="w-3 h-3 mr-1" />
              Vincular
            </Button>
          )}
        </div>
      )}
    </div>
  );
}
