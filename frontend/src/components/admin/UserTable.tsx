import { useState } from "react";
import { AdminUser } from "@/services/api";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";
import { CheckCircle, XCircle, Link, Copy, Users } from "lucide-react";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";

const PAGE_SIZE = 20;

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

interface UserTableProps {
  users: AdminUser[];
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

export function UserTable({
  users,
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
}: UserTableProps) {
  const [page, setPage] = useState(1);
  const totalPages = Math.max(1, Math.ceil(users.length / PAGE_SIZE));
  const paged = users.slice((page - 1) * PAGE_SIZE, page * PAGE_SIZE);

  return (
    <div className="rounded-xl border border-border overflow-hidden">
      <table className="w-full border-collapse text-sm">
        <thead>
          <tr className="bg-secondary">
            {["Usuário", "Cargo", "Status", "Tools", "Ações"].map((col) => (
              <th
                key={col}
                className="text-left text-[10px] font-semibold uppercase tracking-widest text-muted-foreground px-4 py-2.5 border-b border-border"
              >
                {col}
              </th>
            ))}
          </tr>
        </thead>
        <tbody className="bg-card divide-y divide-border">
          {paged.map((user) => {
            const isAdmin = user.role === "admin";
            const isLinkTarget = !!linkMode && linkMode !== user.id;
            return (
              <tr
                key={user.id}
                className={cn(
                  "transition-colors",
                  isLinkTarget
                    ? "cursor-pointer ring-1 ring-inset ring-primary/30 hover:ring-primary/60 hover:bg-primary/5"
                    : "hover:bg-secondary/50"
                )}
                onClick={() => isLinkTarget && onLinkConfirm(user.id)}
              >
                {/* Usuário */}
                <td className="px-4 py-3">
                  <p className="font-semibold text-foreground">{user.email}</p>
                  <div className="mt-0.5">
                    {(() => {
                      const peers = linkedGroups.get(user.account_id);
                      if (peers && peers.length >= 2) {
                        return (
                          <Tooltip>
                            <TooltipTrigger asChild>
                              <span className="inline-flex items-center gap-1 text-[10px] font-semibold px-1.5 py-0.5 rounded bg-primary/10 text-primary border border-primary/20 cursor-default select-none">
                                <Users className="w-2.5 h-2.5" />
                                Vinculado · {peers.length} usuários
                              </span>
                            </TooltipTrigger>
                            <TooltipContent side="right" className="text-xs max-w-[220px]">
                              <p className="font-semibold mb-1 text-muted-foreground uppercase tracking-wide text-[10px]">Mesma conta</p>
                              <ul className="space-y-0.5">
                                {peers.map((email) => (
                                  <li key={email} className={email === user.email ? "text-primary font-medium" : "text-foreground"}>
                                    {email}
                                  </li>
                                ))}
                              </ul>
                            </TooltipContent>
                          </Tooltip>
                        );
                      }
                      return (
                        <div className="flex items-center gap-1">
                          <span className="font-mono text-[10px] text-muted-foreground">
                            {user.account_id.slice(0, 8)}…
                          </span>
                          <button
                            onClick={(e) => { e.stopPropagation(); onCopyAccountId(user.account_id); }}
                            className="text-muted-foreground hover:text-foreground transition-colors"
                            title="Copiar account ID"
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
                </td>
                {/* Cargo */}
                <td className="px-4 py-3">
                  <span className={cn(
                    "text-[10px] font-semibold px-1.5 py-0.5 rounded",
                    isAdmin
                      ? "text-primary bg-primary/10 border border-primary/20"
                      : "text-muted-foreground bg-secondary border border-border"
                  )}>
                    {isAdmin ? "Admin" : "Usuário"}
                  </span>
                </td>
                {/* Status */}
                <td className="px-4 py-3">
                  <span className={cn(
                    "inline-flex items-center gap-1.5 text-[10px] font-semibold px-2 py-1 rounded-full uppercase tracking-wide",
                    statusBadge[user.status] ?? "bg-secondary text-foreground border border-border"
                  )}>
                    <span className={cn("w-1.5 h-1.5 rounded-full", statusDot[user.status])} />
                    {statusLabel[user.status] ?? user.status}
                  </span>
                </td>
                {/* Tools */}
                <td className="px-4 py-3">
                  {isAdmin ? (
                    <span className="text-muted-foreground/30">—</span>
                  ) : (
                    <Switch
                      checked={user.show_tools}
                      onCheckedChange={() => onToggleTools(user.id, user.show_tools)}
                      onClick={(e) => e.stopPropagation()}
                      className="scale-75 origin-left"
                    />
                  )}
                </td>
                {/* Ações */}
                <td className="px-4 py-3">
                  {isAdmin ? (
                    <span className="text-muted-foreground/30">—</span>
                  ) : (
                    <div className="flex items-center gap-1.5">
                      {user.status === "pending" && (
                        <Button
                          size="sm"
                          variant="outline"
                          className="h-7 text-xs text-success border-success/30 hover:bg-success/10"
                          disabled={!!actionLoading}
                          onClick={(e) => { e.stopPropagation(); onApprove(user.id); }}
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
                          onClick={(e) => { e.stopPropagation(); onSuspendClick(user); }}
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
                          onClick={(e) => { e.stopPropagation(); onReactivate(user.id); }}
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
                          onClick={(e) => { e.stopPropagation(); onLinkStart(user.id); }}
                        >
                          <Link className="w-3 h-3 mr-1" />
                          Vincular
                        </Button>
                      )}
                    </div>
                  )}
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>

      {/* Paginação */}
      <div className="flex items-center justify-between px-4 py-3 bg-secondary border-t border-border text-xs text-muted-foreground">
        <span>
          Mostrando {Math.min((page - 1) * PAGE_SIZE + 1, users.length)}–
          {Math.min(page * PAGE_SIZE, users.length)} de {users.length} usuários
        </span>
        <div className="flex items-center gap-1">
          <button
            disabled={page === 1}
            onClick={() => setPage((p) => p - 1)}
            className="px-2 py-1 rounded border border-border bg-card disabled:opacity-40 hover:bg-secondary transition-colors"
          >
            ← Anterior
          </button>
          {Array.from({ length: totalPages }, (_, i) => i + 1)
            .filter((p) => p === 1 || p === totalPages || Math.abs(p - page) <= 1)
            .map((p, idx, arr) => (
              <span key={p}>
                {idx > 0 && arr[idx - 1] !== p - 1 && (
                  <span className="px-1 text-muted-foreground/50">…</span>
                )}
                <button
                  onClick={() => setPage(p)}
                  className={cn(
                    "w-7 h-7 rounded border text-xs transition-colors",
                    p === page
                      ? "border-primary text-primary bg-primary/10"
                      : "border-border bg-card hover:bg-secondary"
                  )}
                >
                  {p}
                </button>
              </span>
            ))}
          <button
            disabled={page === totalPages}
            onClick={() => setPage((p) => p + 1)}
            className="px-2 py-1 rounded border border-border bg-card disabled:opacity-40 hover:bg-secondary transition-colors"
          >
            Próxima →
          </button>
        </div>
      </div>
    </div>
  );
}
