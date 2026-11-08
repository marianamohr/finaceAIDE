import { useEffect, useState, useMemo } from "react";
import { api, AdminUser, AdminMetrics } from "@/services/api";
import { useAuth } from "@/hooks/useAuth";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { LogOut, RefreshCw, Search, UserPlus } from "lucide-react";
import { MetricCard } from "@/components/admin/MetricCard";
import { UserTable } from "@/components/admin/UserTable";
import { UserCard } from "@/components/admin/UserCard";
import { InvitePanel } from "@/components/admin/InvitePanel";
import { SuspendDialog } from "@/components/admin/SuspendDialog";
import { cn } from "@/lib/utils";

type StatusFilter = "all" | "active" | "pending" | "suspended";

const STATUS_FILTERS: { key: StatusFilter; label: string }[] = [
  { key: "all",       label: "Todos"     },
  { key: "active",    label: "Ativos"    },
  { key: "pending",   label: "Pendentes" },
  { key: "suspended", label: "Suspensos" },
];

export default function Admin() {
  const { logout } = useAuth();

  // ── Data ──────────────────────────────────────────────────────
  const [users, setUsers]     = useState<AdminUser[]>([]);
  const [metrics, setMetrics] = useState<AdminMetrics | null>(null);
  const [loading, setLoading] = useState(true);

  // ── Filters ───────────────────────────────────────────────────
  const [search, setSearch]             = useState("");
  const [statusFilter, setStatusFilter] = useState<StatusFilter>("all");

  // ── UI state ──────────────────────────────────────────────────
  const [showInvite, setShowInvite]           = useState(false);
  const [linkMode, setLinkMode]               = useState<string | null>(null);
  const [actionLoading, setActionLoading]     = useState<string | null>(null);
  const [copiedAccountId, setCopiedAccountId] = useState<string | null>(null);
  const [suspendTarget, setSuspendTarget]     = useState<AdminUser | null>(null);
  const [suspendLoading, setSuspendLoading]   = useState(false);

  // ── Fetch ─────────────────────────────────────────────────────
  const fetchAll = async () => {
    setLoading(true);
    const [usersData, metricsData] = await Promise.allSettled([
      api.getAdminUsers(),
      api.getAdminMetrics(),
    ]);
    if (usersData.status === "fulfilled")   setUsers(usersData.value ?? []);
    if (metricsData.status === "fulfilled") setMetrics(metricsData.value);
    setLoading(false);
  };

  useEffect(() => { fetchAll(); }, []);

  // ── Derived ───────────────────────────────────────────────────
  const filtered = useMemo(() => {
    const q = search.toLowerCase();
    return users.filter((u) => {
      const matchSearch =
        !q || u.email.toLowerCase().includes(q) || u.account_id.toLowerCase().includes(q);
      const matchStatus =
        statusFilter === "all" || u.status === statusFilter;
      return matchSearch && matchStatus;
    });
  }, [users, search, statusFilter]);

  const userStats = useMemo(() => ({
    total:     users.length,
    pending:   users.filter((u) => u.status === "pending").length,
    suspended: users.filter((u) => u.status === "suspended").length,
  }), [users]);

  const linkedGroups = useMemo(() => {
    const map = new Map<string, string[]>();
    for (const u of users) {
      const existing = map.get(u.account_id);
      if (existing) {
        existing.push(u.email);
      } else {
        map.set(u.account_id, [u.email]);
      }
    }
    for (const [key, emails] of map) {
      if (emails.length < 2) map.delete(key);
    }
    return map;
  }, [users]);

  // ── Handlers ──────────────────────────────────────────────────
  const handleStatus = async (id: string, status: string) => {
    setActionLoading(id + status);
    try {
      await api.updateUserStatus(id, status);
      setUsers((prev) => prev.map((u) => u.id === id ? { ...u, status } : u));
    } catch {
      // silent
    } finally {
      setActionLoading(null);
    }
  };

  const handleToggleTools = async (id: string, current: boolean) => {
    const next = !current;
    setUsers((prev) => prev.map((u) => u.id === id ? { ...u, show_tools: next } : u));
    try {
      await api.updateUserShowTools(id, next);
    } catch {
      setUsers((prev) => prev.map((u) => u.id === id ? { ...u, show_tools: current } : u));
    }
  };

  const handleLink = async (targetId: string) => {
    if (!linkMode || linkMode === targetId) return;
    const source = users.find((u) => u.id === linkMode);
    if (!source) return;
    setActionLoading(targetId + "link");
    try {
      await api.updateUserAccount(targetId, source.account_id);
      setUsers((prev) => prev.map((u) => u.id === targetId ? { ...u, account_id: source.account_id } : u));
      setLinkMode(null);
    } catch {
      // silent
    } finally {
      setActionLoading(null);
    }
  };

  const handleSuspendConfirm = async () => {
    if (!suspendTarget) return;
    setSuspendLoading(true);
    try {
      await api.updateUserStatus(suspendTarget.id, "suspended");
      setUsers((prev) => prev.map((u) => u.id === suspendTarget.id ? { ...u, status: "suspended" } : u));
      setSuspendTarget(null);
    } catch {
      // silent
    } finally {
      setSuspendLoading(false);
    }
  };

  const copyAccountId = (accountId: string) => {
    navigator.clipboard.writeText(accountId);
    setCopiedAccountId(accountId);
    setTimeout(() => setCopiedAccountId(null), 2000);
  };

  // ── Shared action props ───────────────────────────────────────
  const actionProps = {
    actionLoading,
    linkMode,
    copiedAccountId,
    onSuspendClick:  setSuspendTarget,
    onApprove:       (id: string) => handleStatus(id, "active"),
    onReactivate:    (id: string) => handleStatus(id, "active"),
    onToggleTools:   handleToggleTools,
    onLinkStart:     setLinkMode,
    onLinkConfirm:   handleLink,
    onCopyAccountId: copyAccountId,
  };

  const formatRate = (rate: number | null | undefined) =>
    rate != null ? `${(rate * 100).toFixed(1)}%` : "—";

  return (
    <div className="min-h-[var(--app-height)] flex flex-col bg-background">

      {/* Header */}
      <header className="px-4 sm:px-6 pt-6 pb-4 flex items-center justify-between border-b border-border">
        <div>
          <h1 className="font-heading text-lg font-bold">Admin</h1>
          <p className="text-xs text-muted-foreground mt-0.5">Gerenciar usuários · FinanceAI</p>
        </div>
        <div className="flex items-center gap-2">
          <Button variant="ghost" size="icon" onClick={fetchAll} disabled={loading}>
            <RefreshCw className={cn("w-4 h-4", loading && "animate-spin")} />
          </Button>
          <Button
            variant="ghost"
            onClick={logout}
            className="text-destructive hover:text-destructive hover:bg-destructive/10"
          >
            <LogOut className="w-4 h-4 mr-1" />
            Sair
          </Button>
        </div>
      </header>

      <div className="flex-1 px-4 sm:px-6 py-5 space-y-5 max-w-5xl mx-auto w-full">

        {/* Métricas */}
        <section>
          <p className="text-[10px] font-semibold uppercase tracking-widest text-muted-foreground mb-3">
            Status do sistema
          </p>
          <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
            <MetricCard
              label="Total de usuários"
              value={userStats.total}
              sub={`${userStats.pending} pendentes · ${userStats.suspended} suspensos`}
              variant="primary"
            />
            <MetricCard
              label="Sucesso IA"
              value={metrics ? formatRate(metrics.extraction_success_rate) : "—"}
              sub="Extração RAG · 24h"
              variant="success"
            />
            <MetricCard
              label="Recibos · 24h"
              value={metrics?.receipts_24h ?? "—"}
              sub="Processados via upload"
              variant="warning"
            />
            <MetricCard
              label="Falhas · 24h"
              value={metrics?.extraction_failures_24h ?? "—"}
              sub="Ações não confirmadas"
              variant="destructive"
            />
          </div>
        </section>

        {/* Seção de usuários */}
        <section>
          <p className="text-[10px] font-semibold uppercase tracking-widest text-muted-foreground mb-3">
            Usuários
          </p>

          {/* Toolbar */}
          <div className="flex flex-wrap items-center gap-2 mb-3">
            <div className="relative flex-1 min-w-[180px] max-w-xs">
              <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-muted-foreground pointer-events-none" />
              <Input
                placeholder="Buscar por email ou ID…"
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                className="pl-8 h-8 text-xs bg-card border-border"
              />
            </div>
            <div className="flex items-center gap-1">
              {STATUS_FILTERS.map(({ key, label }) => (
                <button
                  key={key}
                  onClick={() => setStatusFilter(key)}
                  className={cn(
                    "px-3 py-1.5 rounded-lg text-xs font-medium transition-colors border",
                    statusFilter === key
                      ? "border-primary text-primary bg-primary/10"
                      : "border-border text-muted-foreground bg-card hover:bg-secondary"
                  )}
                >
                  {label}
                </button>
              ))}
            </div>
            <Button
              size="sm"
              className="ml-auto h-8 text-xs"
              onClick={() => setShowInvite((v) => !v)}
            >
              <UserPlus className="w-3.5 h-3.5 mr-1.5" />
              Convidar
            </Button>
          </div>

          {/* InvitePanel */}
          {showInvite && (
            <div className="mb-3">
              <InvitePanel onClose={() => setShowInvite(false)} />
            </div>
          )}

          {/* Link mode banner */}
          {linkMode && (
            <div className="mb-3 rounded-xl bg-primary/10 border border-primary/20 px-4 py-3 text-sm text-primary">
              Selecione outro usuário para vincular à mesma conta de{" "}
              <strong>{users.find((u) => u.id === linkMode)?.email}</strong>.{" "}
              <button onClick={() => setLinkMode(null)} className="underline">
                Cancelar
              </button>
            </div>
          )}

          {/* Lista */}
          {loading ? (
            <div className="text-center text-sm text-muted-foreground py-16">Carregando…</div>
          ) : filtered.length === 0 ? (
            <div className="text-center text-sm text-muted-foreground py-16">Nenhum usuário encontrado.</div>
          ) : (
            <>
              {/* Desktop: tabela */}
              <div className="hidden sm:block">
                <UserTable users={filtered} linkedGroups={linkedGroups} {...actionProps} />
              </div>
              {/* Mobile: cards */}
              <div className="sm:hidden space-y-3">
                {filtered.map((user) => (
                  <UserCard key={user.id} user={user} linkedGroups={linkedGroups} {...actionProps} />
                ))}
              </div>
            </>
          )}
        </section>
      </div>

      {/* Dialog de confirmação de suspensão */}
      {suspendTarget && (
        <SuspendDialog
          email={suspendTarget.email}
          open={!!suspendTarget}
          loading={suspendLoading}
          onConfirm={handleSuspendConfirm}
          onCancel={() => setSuspendTarget(null)}
        />
      )}
    </div>
  );
}
