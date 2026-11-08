# Admin Page Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Redesenhar a página `/admin` com painel de métricas do sistema, tabela de usuários com busca/filtros/paginação, layout responsivo mobile/desktop e confirmação antes de suspender usuário.

**Architecture:** O `Admin.tsx` atual (295 linhas, monolítico) é decomposto em 5 componentes focados em `src/components/admin/`. O backend recebe um novo endpoint `GET /admin/metrics` no `AdminHandler` existente, consultando contagens da tabela `transactions` (recibos = `image_url != ''`). O frontend chama os dois endpoints em paralelo no `useEffect` inicial.

**Tech Stack:** React 18 + TypeScript, Tailwind CSS v3, shadcn/ui (Radix AlertDialog já disponível em `src/components/ui/`), Go + Chi, GORM, PostgreSQL.

---

## Mapa de arquivos

| Arquivo | Ação | Responsabilidade |
|---|---|---|
| `backend/internal/handlers/admin.go` | Modificar | Adicionar handler `GetMetrics` |
| `backend/cmd/server/main.go` | Modificar | Registrar rota `GET /admin/metrics` |
| `frontend/src/services/api.ts` | Modificar | Adicionar `AdminMetrics` type + `getAdminMetrics()` |
| `frontend/src/components/admin/MetricCard.tsx` | Criar | Card de métrica individual |
| `frontend/src/components/admin/SuspendDialog.tsx` | Criar | AlertDialog de confirmação de suspensão |
| `frontend/src/components/admin/InvitePanel.tsx` | Criar | Painel de convite inline |
| `frontend/src/components/admin/UserTable.tsx` | Criar | Tabela desktop com paginação |
| `frontend/src/components/admin/UserCard.tsx` | Criar | Card mobile por usuário |
| `frontend/src/pages/Admin.tsx` | Modificar | Orquestração: estado, fetch, composição |

---

## Task 1: Endpoint GET /admin/metrics no backend

**Files:**
- Modify: `backend/internal/handlers/admin.go`
- Modify: `backend/cmd/server/main.go`

- [ ] **Step 1: Adicionar o método `GetMetrics` ao `AdminHandler`**

Abrir `backend/internal/handlers/admin.go` e adicionar após a função `generateToken()`:

```go
// GetMetrics godoc
// GET /admin/metrics
func (h *AdminHandler) GetMetrics(w http.ResponseWriter, r *http.Request) {
	metrics, err := h.users.GetMetrics()
	if err != nil {
		log.Error().Err(err).Msg("admin: get metrics")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	respondJSON(w, http.StatusOK, metrics)
}
```

- [ ] **Step 2: Adicionar o método `GetMetrics` ao `UserRepository`**

Abrir `backend/internal/repository/user.go` e adicionar ao final do arquivo:

```go
type AdminMetrics struct {
	Receipts24h           int64   `json:"receipts_24h"`
	ExtractionSuccessRate float64 `json:"extraction_success_rate"`
	ExtractionFailures24h int64   `json:"extraction_failures_24h"`
}

// GetMetrics returns system-wide metrics for the admin dashboard.
// Receipts = transactions with image_url != '' created in last 24h.
// Failures = pending_actions with tool_name = 'add_transaction' created in last 24h
// that were NOT executed (expired without confirmation).
// Success rate = (receipts - failures) / receipts, or 1.0 if receipts == 0.
func (r *UserRepository) GetMetrics() (*AdminMetrics, error) {
	since := time.Now().UTC().Add(-24 * time.Hour)

	var receipts int64
	if err := r.db.Raw(
		`SELECT COUNT(*) FROM transactions WHERE image_url != '' AND created_at >= ?`,
		since,
	).Scan(&receipts).Error; err != nil {
		return nil, fmt.Errorf("user repository: get metrics receipts: %w", err)
	}

	var failures int64
	if err := r.db.Raw(
		`SELECT COUNT(*) FROM pending_actions
		 WHERE tool_name = 'add_transaction'
		   AND created_at >= ?
		   AND executed_at IS NULL
		   AND expires_at < NOW()`,
		since,
	).Scan(&failures).Error; err != nil {
		return nil, fmt.Errorf("user repository: get metrics failures: %w", err)
	}

	rate := 1.0
	if receipts > 0 {
		rate = float64(receipts-failures) / float64(receipts)
		if rate < 0 {
			rate = 0
		}
	}

	return &AdminMetrics{
		Receipts24h:           receipts,
		ExtractionSuccessRate: rate,
		ExtractionFailures24h: failures,
	}, nil
}
```

Verificar que `"fmt"` e `"time"` já estão nos imports de `user.go`; adicionar se necessário.

- [ ] **Step 3: Registrar a rota no main.go**

Abrir `backend/cmd/server/main.go`, localizar o bloco `/admin` (linha ~334) e adicionar a nova rota:

```go
r.Route("/admin", func(r chi.Router) {
    r.Use(adminMiddleware)
    r.Get("/users", adminHandler.ListUsers)
    r.Get("/metrics", adminHandler.GetMetrics)   // ← adicionar esta linha
    r.Patch("/users/{id}/status", adminHandler.UpdateStatus)
    r.Patch("/users/{id}/account", adminHandler.UpdateAccount)
    r.Patch("/users/{id}/settings", adminHandler.UpdateUserSettings)
    r.Post("/invites", adminHandler.CreateInvite)
})
```

- [ ] **Step 4: Compilar o backend e verificar sem erros**

```bash
cd backend
go build ./...
```

Esperado: sem erros de compilação.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/handlers/admin.go \
        backend/internal/repository/user.go \
        backend/cmd/server/main.go
git commit -m "feat(admin): add GET /admin/metrics endpoint"
```

---

## Task 2: Tipo AdminMetrics e método getAdminMetrics no frontend

**Files:**
- Modify: `frontend/src/services/api.ts`

- [ ] **Step 1: Adicionar interface `AdminMetrics` em `api.ts`**

Abrir `frontend/src/services/api.ts`. Após a interface `AdminUser` (linha 82), adicionar:

```typescript
export interface AdminMetrics {
  receipts_24h: number;
  extraction_success_rate: number;
  extraction_failures_24h: number;
}
```

- [ ] **Step 2: Adicionar método `getAdminMetrics` no objeto `api`**

No objeto `api`, após `getAdminUsers` (linha ~216), adicionar:

```typescript
getAdminMetrics: () =>
  request<AdminMetrics>("/admin/metrics"),
```

- [ ] **Step 3: Verificar TypeScript sem erros**

```bash
cd frontend
npx tsc --noEmit
```

Esperado: sem erros.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/services/api.ts
git commit -m "feat(admin): add AdminMetrics type and getAdminMetrics()"
```

---

## Task 3: Componente MetricCard

**Files:**
- Create: `frontend/src/components/admin/MetricCard.tsx`

- [ ] **Step 1: Criar o diretório e o componente**

```bash
mkdir -p frontend/src/components/admin
```

Criar `frontend/src/components/admin/MetricCard.tsx`:

```typescript
import { cn } from "@/lib/utils";

type MetricVariant = "primary" | "success" | "warning" | "destructive";

interface MetricCardProps {
  label: string;
  value: string | number;
  sub?: string;
  variant: MetricVariant;
}

const accentClass: Record<MetricVariant, string> = {
  primary:     "border-t-primary",
  success:     "border-t-success",
  warning:     "border-t-warning",
  destructive: "border-t-destructive",
};

const valueClass: Record<MetricVariant, string> = {
  primary:     "text-foreground",
  success:     "text-success",
  warning:     "text-warning",
  destructive: "text-destructive",
};

export function MetricCard({ label, value, sub, variant }: MetricCardProps) {
  return (
    <div className={cn(
      "rounded-xl bg-card border border-border border-t-2 p-4",
      accentClass[variant]
    )}>
      <p className="text-[10px] font-semibold uppercase tracking-widest text-muted-foreground">
        {label}
      </p>
      <p className={cn("font-heading text-2xl font-bold mt-1 tracking-tight", valueClass[variant])}>
        {value}
      </p>
      {sub && (
        <p className="text-[10px] text-muted-foreground mt-1">{sub}</p>
      )}
    </div>
  );
}
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/components/admin/MetricCard.tsx
git commit -m "feat(admin): add MetricCard component"
```

---

## Task 4: Componente SuspendDialog

**Files:**
- Create: `frontend/src/components/admin/SuspendDialog.tsx`

O shadcn `AlertDialog` já existe em `frontend/src/components/ui/alert-dialog.tsx`.

- [ ] **Step 1: Criar `SuspendDialog.tsx`**

Criar `frontend/src/components/admin/SuspendDialog.tsx`:

```typescript
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";

interface SuspendDialogProps {
  email: string;
  open: boolean;
  loading: boolean;
  onConfirm: () => void;
  onCancel: () => void;
}

export function SuspendDialog({ email, open, loading, onConfirm, onCancel }: SuspendDialogProps) {
  return (
    <AlertDialog open={open} onOpenChange={(v) => { if (!v) onCancel(); }}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Suspender usuário?</AlertDialogTitle>
          <AlertDialogDescription>
            Esta ação vai impedir que <strong className="text-foreground">{email}</strong> acesse
            a plataforma. Você pode reativar a qualquer momento.
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel onClick={onCancel} disabled={loading}>
            Cancelar
          </AlertDialogCancel>
          <AlertDialogAction
            onClick={onConfirm}
            disabled={loading}
            className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
          >
            {loading ? "Suspendendo…" : "Suspender"}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
```

- [ ] **Step 2: Verificar que `alert-dialog` existe**

```bash
ls frontend/src/components/ui/alert-dialog.tsx
```

Esperado: arquivo existe. Se não existir, rodar `npx shadcn@latest add alert-dialog`.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/admin/SuspendDialog.tsx
git commit -m "feat(admin): add SuspendDialog component with AlertDialog confirmation"
```

---

## Task 5: Componente InvitePanel

**Files:**
- Create: `frontend/src/components/admin/InvitePanel.tsx`

- [ ] **Step 1: Criar `InvitePanel.tsx`**

Criar `frontend/src/components/admin/InvitePanel.tsx`:

```typescript
import { useState } from "react";
import { api } from "@/services/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Copy, X } from "lucide-react";

interface InvitePanelProps {
  onClose: () => void;
}

export function InvitePanel({ onClose }: InvitePanelProps) {
  const [email, setEmail] = useState("");
  const [link, setLink] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [copied, setCopied] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!email) return;
    setLoading(true);
    setLink(null);
    try {
      const data = await api.post<{ link: string }>("/admin/invites", { email });
      setLink(data.link);
      setEmail("");
    } catch {
      // silent
    } finally {
      setLoading(false);
    }
  };

  const copyLink = () => {
    if (!link) return;
    navigator.clipboard.writeText(link);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div className="rounded-xl bg-card border border-border p-4 space-y-3">
      <div className="flex items-center justify-between">
        <p className="text-sm font-semibold">Convidar usuário</p>
        <button onClick={onClose} className="text-muted-foreground hover:text-foreground transition-colors">
          <X className="w-4 h-4" />
        </button>
      </div>
      <form onSubmit={handleSubmit} className="flex gap-2">
        <Input
          type="email"
          placeholder="email@exemplo.com"
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          required
          className="h-9 text-sm bg-secondary border-0"
        />
        <Button type="submit" size="sm" disabled={loading} className="shrink-0">
          {loading ? "…" : "Gerar link"}
        </Button>
      </form>
      {link && (
        <div className="flex items-center gap-2 rounded-lg bg-secondary px-3 py-2">
          <p className="text-xs text-muted-foreground truncate flex-1">{link}</p>
          <button onClick={copyLink} className="shrink-0 text-primary hover:text-primary/80 transition-colors">
            <Copy className="w-4 h-4" />
          </button>
          {copied && <span className="text-xs text-primary">Copiado!</span>}
        </div>
      )}
    </div>
  );
}
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/components/admin/InvitePanel.tsx
git commit -m "feat(admin): add InvitePanel component"
```

---

## Task 6: Componente UserTable (desktop)

**Files:**
- Create: `frontend/src/components/admin/UserTable.tsx`

- [ ] **Step 1: Criar `UserTable.tsx`**

Criar `frontend/src/components/admin/UserTable.tsx`:

```typescript
import { useState } from "react";
import { AdminUser } from "@/services/api";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";
import { CheckCircle, XCircle, Link, Copy } from "lucide-react";
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
                  <div className="flex items-center gap-1 mt-0.5">
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
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/components/admin/UserTable.tsx
git commit -m "feat(admin): add UserTable component with pagination"
```

---

## Task 7: Componente UserCard (mobile)

**Files:**
- Create: `frontend/src/components/admin/UserCard.tsx`

- [ ] **Step 1: Criar `UserCard.tsx`**

Criar `frontend/src/components/admin/UserCard.tsx`:

```typescript
import { AdminUser } from "@/services/api";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";
import { CheckCircle, XCircle, Link, Copy } from "lucide-react";
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
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/components/admin/UserCard.tsx
git commit -m "feat(admin): add UserCard component for mobile layout"
```

---

## Task 8: Reescrever Admin.tsx

**Files:**
- Modify: `frontend/src/pages/Admin.tsx`

Este é o passo de integração. O `Admin.tsx` torna-se orquestrador puro: estado, fetch, handlers, composição.

- [ ] **Step 1: Substituir o conteúdo de `Admin.tsx`**

Substituir **todo** o conteúdo de `frontend/src/pages/Admin.tsx` por:

```typescript
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
  const [search, setSearch]           = useState("");
  const [statusFilter, setStatusFilter] = useState<StatusFilter>("all");

  // ── UI state ──────────────────────────────────────────────────
  const [showInvite, setShowInvite]         = useState(false);
  const [linkMode, setLinkMode]             = useState<string | null>(null);
  const [actionLoading, setActionLoading]   = useState<string | null>(null);
  const [copiedAccountId, setCopiedAccountId] = useState<string | null>(null);
  const [suspendTarget, setSuspendTarget]   = useState<AdminUser | null>(null);
  const [suspendLoading, setSuspendLoading] = useState(false);

  // ── Fetch ─────────────────────────────────────────────────────
  const fetchAll = async () => {
    setLoading(true);
    const [usersData, metricsData] = await Promise.allSettled([
      api.getAdminUsers(),
      api.getAdminMetrics(),
    ]);
    if (usersData.status === "fulfilled")  setUsers(usersData.value ?? []);
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
    active:    users.filter((u) => u.status === "active").length,
    pending:   users.filter((u) => u.status === "pending").length,
    suspended: users.filter((u) => u.status === "suspended").length,
  }), [users]);

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
    onSuspendClick: setSuspendTarget,
    onApprove:      (id: string) => handleStatus(id, "active"),
    onReactivate:   (id: string) => handleStatus(id, "active"),
    onToggleTools:  handleToggleTools,
    onLinkStart:    setLinkMode,
    onLinkConfirm:  handleLink,
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

        {/* Toolbar */}
        <section>
          <p className="text-[10px] font-semibold uppercase tracking-widest text-muted-foreground mb-3">
            Usuários
          </p>
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
                <UserTable users={filtered} {...actionProps} />
              </div>
              {/* Mobile: cards */}
              <div className="sm:hidden space-y-3">
                {filtered.map((user) => (
                  <UserCard key={user.id} user={user} {...actionProps} />
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
```

- [ ] **Step 2: Verificar TypeScript**

```bash
cd frontend
npx tsc --noEmit
```

Esperado: sem erros.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/pages/Admin.tsx
git commit -m "feat(admin): redesign admin page with metrics, table, search, filters and responsive layout"
```

---

## Task 9: Verificação end-to-end

- [ ] **Step 1: Subir o backend**

```bash
cd backend
go run ./cmd/server/main.go
```

Esperado: servidor sobe sem erros.

- [ ] **Step 2: Subir o frontend**

```bash
cd frontend
npm run dev
```

Abrir `http://localhost:5173/admin` logado como admin.

- [ ] **Step 3: Verificar métricas**

Os 4 MetricCards aparecem. O de "Sucesso IA", "Recibos · 24h" e "Falhas · 24h" exibem valores reais (ou `—` se o banco estiver vazio). O de "Total de usuários" mostra o total correto.

- [ ] **Step 4: Verificar busca e filtros**

Digitar um email parcial no campo de busca → lista filtra em tempo real. Clicar nos filtros de status → lista filtra corretamente. Limpar busca → lista volta completa.

- [ ] **Step 5: Verificar fluxo de Suspender**

Clicar "Suspender" em um usuário ativo → AlertDialog abre com email do usuário → clicar "Cancelar" → dialog fecha, usuário continua ativo → clicar "Suspender" novamente → confirmar → usuário aparece como "Suspenso" na lista.

- [ ] **Step 6: Verificar responsividade**

No DevTools (F12), definir viewport para 375px → tabela some, UserCards aparecem. Voltar para 1024px → tabela aparece com paginação.

- [ ] **Step 7: Verificar paginação**

Com menos de 20 usuários: botões de paginação mostram "1–N de N usuários" e os botões Anterior/Próxima ficam desabilitados. Com 20+ usuários: paginação navega corretamente.

- [ ] **Step 8: Verificar InvitePanel**

Clicar "+ Convidar" → painel aparece. Inserir email → "Gerar link" → link aparece com botão de copiar → clicar "✕" → painel fecha.

- [ ] **Step 9: Commit final se tudo ok**

```bash
git add -A
git status  # confirmar que não há arquivos não rastreados inesperados
git commit -m "chore(admin): verify end-to-end admin redesign"
```
