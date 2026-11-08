# Manual Interativo do Usuário — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Criar um manual interativo com humor leve, exibido como modal no primeiro login e como seção expansível no final da página de Perfil, com débito padrão criado automaticamente no registro.

**Architecture:** Campo `onboarding_seen` na tabela `user_settings` (backend) controla exibição do modal — persiste entre dispositivos sem localStorage. Componente `UserManual` é reutilizado nas duas superfícies via prop `mode`. Débito padrão criado no handler de registro de forma idempotente.

**Tech Stack:** React, TypeScript, Tailwind CSS, shadcn/ui (`Dialog`, `Collapsible`), Go (GORM), PostgreSQL.

---

## Mapa de arquivos

| Arquivo | Ação | Responsabilidade |
|---|---|---|
| `backend/internal/models/user_settings.go` | Modificar | Adicionar campo `OnboardingSeen` |
| `backend/migrations/migrate.go` | Modificar | Migration `ALTER TABLE` para `onboarding_seen` |
| `backend/internal/repository/user.go` | Modificar | Incluir `OnboardingSeen` em `UserWithSettings`, `UpdateSettings` |
| `backend/internal/handlers/user.go` | Modificar | Expor `onboarding_seen` em `Me` e `UpdateSettings` |
| `backend/internal/handlers/auth.go` | Modificar | Injetar `pmRepo`, criar Débito padrão no `Register` |
| `frontend/src/services/api.ts` | Modificar | Adicionar `onboarding_seen` em `getMe` e `updateSettings` |
| `frontend/src/hooks/useSettings.ts` | Modificar | Adicionar `onboardingSeen` e expor no retorno |
| `frontend/src/components/UserManual/UserManual.tsx` | Criar | Conteúdo completo do manual (mode="modal"\|"section") |
| `frontend/src/components/UserManual/UserManualModal.tsx` | Criar | Wrapper Dialog que exibe `UserManual` no primeiro login |
| `frontend/src/pages/Chat.tsx` | Modificar | Montar `UserManualModal` quando `onboardingSeen === false` |
| `frontend/src/pages/Profile.tsx` | Modificar | Adicionar seção `#manual` no final da página |

---

## Task 1: Campo `onboarding_seen` no modelo e migration

**Files:**
- Modify: `backend/internal/models/user_settings.go`
- Modify: `backend/migrations/migrate.go`

- [ ] **Step 1: Adicionar campo no model**

Em `backend/internal/models/user_settings.go`, adicionar após `ShowTools`:

```go
OnboardingSeen bool `gorm:"not null;default:false" json:"onboarding_seen"`
```

O arquivo final do struct fica:
```go
type UserSettings struct {
	UserID               uuid.UUID `gorm:"primarykey;type:uuid"           json:"user_id"`
	BaseCurrency         string    `gorm:"not null;default:'BRL'"         json:"base_currency"`
	DistributionMethod   string    `gorm:"not null;default:'70-30'"       json:"distribution_method"`
	FuturePct            float64   `gorm:"type:numeric(6,4);default:0.30" json:"future_pct"`
	CategoryContext      string    `gorm:"not null;default:'standard'"    json:"category_context"`
	ShowGoals            bool      `gorm:"not null;default:true"          json:"show_goals"`
	UserAPIKey           string    `gorm:"not null;default:''"            json:"user_api_key"`
	ShowLimitsCategories bool      `gorm:"not null;default:false"         json:"show_limits_categories"`
	ShowTools            bool      `gorm:"not null;default:false"         json:"show_tools"`
	OnboardingSeen       bool      `gorm:"not null;default:false"         json:"onboarding_seen"`
	UpdatedAt            time.Time `gorm:"not null;autoUpdateTime"        json:"updated_at"`
}
```

- [ ] **Step 2: Adicionar migration em `addNullableColumns`**

Em `backend/migrations/migrate.go`, dentro da função `addNullableColumns`, adicionar ao final do slice `stmts`:

```go
`ALTER TABLE user_settings ADD COLUMN IF NOT EXISTS onboarding_seen bool NOT NULL DEFAULT false`,
```

- [ ] **Step 3: Compilar o backend para verificar**

```bash
cd backend && go build ./...
```

Expected: sem erros de compilação.

- [ ] **Step 4: Commit**

```bash
git add backend/internal/models/user_settings.go backend/migrations/migrate.go
git commit -m "feat(backend): adicionar campo onboarding_seen em user_settings"
```

---

## Task 2: Expor `onboarding_seen` no repositório e handler

**Files:**
- Modify: `backend/internal/repository/user.go`
- Modify: `backend/internal/handlers/user.go`

- [ ] **Step 1: Adicionar `OnboardingSeen` em `UserWithSettings`**

Em `backend/internal/repository/user.go`, adicionar no struct `UserWithSettings` após `FuturePct`:

```go
OnboardingSeen bool `gorm:"column:onboarding_seen"`
```

- [ ] **Step 2: Incluir `onboarding_seen` na query de `FindByID`**

A função `FindByID` usa um JOIN com `user_settings`. Verificar se a query usa `SELECT *` ou lista colunas explicitamente:

```bash
grep -A 20 "func (r \*UserRepository) FindByID" backend/internal/repository/user.go
```

Se usar `SELECT *` ou `Scan`, o campo será incluído automaticamente pelo GORM. Se listar colunas explicitamente, adicionar `us.onboarding_seen`.

- [ ] **Step 3: Incluir `OnboardingSeen` em `UpdateSettings`**

Em `backend/internal/handlers/user.go`, na struct do request de `UpdateSettings`, adicionar:

```go
OnboardingSeen *bool `json:"onboarding_seen"`
```

No bloco de merge, adicionar após o bloco de `ShowTools`:

```go
if req.OnboardingSeen != nil {
    updated.OnboardingSeen = *req.OnboardingSeen
}
```

- [ ] **Step 4: Expor em `Me` e resposta de `UpdateSettings`**

Em `backend/internal/handlers/user.go`, na struct `meResponse` de `Me`, adicionar:

```go
OnboardingSeen bool `json:"onboarding_seen"`
```

No `respondJSON` de `Me`, adicionar:

```go
OnboardingSeen: user.OnboardingSeen,
```

Na struct `settingsResponse` de `UpdateSettings`, adicionar:

```go
OnboardingSeen bool `json:"onboarding_seen"`
```

No `respondJSON` de `UpdateSettings`, adicionar:

```go
OnboardingSeen: updated.OnboardingSeen,
```

- [ ] **Step 5: Compilar**

```bash
cd backend && go build ./...
```

Expected: sem erros.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/repository/user.go backend/internal/handlers/user.go
git commit -m "feat(backend): expor onboarding_seen em /user/me e PATCH /user/settings"
```

---

## Task 3: Débito padrão no registro

**Files:**
- Modify: `backend/internal/handlers/auth.go`

- [ ] **Step 1: Injetar `pmRepo` no `AuthHandler`**

Em `backend/internal/handlers/auth.go`, adicionar o campo no struct:

```go
type AuthHandler struct {
	users    *repository.UserRepository
	accounts *repository.AccountRepository
	invites  *repository.InviteRepository
	jwt      *auth.Manager
	pmRepo   *repository.PaymentMethodRepository
	secure   bool
}
```

Atualizar `NewAuthHandler`:

```go
func NewAuthHandler(
	users *repository.UserRepository,
	accounts *repository.AccountRepository,
	invites *repository.InviteRepository,
	jwt *auth.Manager,
	pmRepo *repository.PaymentMethodRepository,
	secure bool,
) *AuthHandler {
	return &AuthHandler{users: users, accounts: accounts, invites: invites, jwt: jwt, pmRepo: pmRepo, secure: secure}
}
```

- [ ] **Step 2: Atualizar chamada de `NewAuthHandler` no `main.go`**

Em `backend/cmd/server/main.go`, a linha atual é:

```go
authHandler := handlers.NewAuthHandler(userRepo, accountRepo, inviteRepo, jwtManager, secure)
```

Alterar para:

```go
authHandler := handlers.NewAuthHandler(userRepo, accountRepo, inviteRepo, jwtManager, pmRepo, secure)
```

- [ ] **Step 3: Criar Débito padrão no `Register`**

Em `backend/internal/handlers/auth.go`, no método `Register`, após a criação do usuário (após `h.users.Create(user)`) e antes de emitir o token, adicionar:

```go
// Criar forma de pagamento padrão (Débito) — idempotente
var count int64
if err := h.pmRepo.DB().Model(&models.PaymentMethod{}).
	Where("account_id = ?", account.ID).Count(&count).Error; err == nil && count == 0 {
	defaultPM := &models.PaymentMethod{
		AccountID: account.ID,
		Name:      "Débito",
		Kind:      models.KindDebit,
		Color:     "#6366f1",
		Active:    true,
	}
	if err := h.pmRepo.Create(defaultPM); err != nil {
		log.Warn().Err(err).Msg("register: create default payment method")
		// Non-fatal: continue registration
	}
}
```

- [ ] **Step 4: Verificar se `PaymentMethodRepository` expõe `DB()`**

```bash
grep -n "func.*DB\(\)" backend/internal/repository/payment_method.go
```

Se não existir, adicionar ao repositório em `backend/internal/repository/payment_method.go`:

```go
func (r *PaymentMethodRepository) DB() *gorm.DB {
	return r.db
}
```

- [ ] **Step 5: Compilar**

```bash
cd backend && go build ./...
```

Expected: sem erros.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/handlers/auth.go backend/internal/repository/payment_method.go backend/cmd/server/main.go
git commit -m "feat(backend): criar débito padrão ao registrar novo usuário"
```

---

## Task 4: Frontend — `onboarding_seen` em `api.ts` e `useSettings`

**Files:**
- Modify: `frontend/src/services/api.ts`
- Modify: `frontend/src/hooks/useSettings.ts`

- [ ] **Step 1: Adicionar `onboarding_seen` na tipagem de `getMe`**

Em `frontend/src/services/api.ts`, na função `getMe`, adicionar no tipo de retorno:

```typescript
onboarding_seen: boolean;
```

- [ ] **Step 2: Adicionar `onboarding_seen` em `updateSettings`**

Na tipagem do parâmetro `patch` de `updateSettings`, adicionar:

```typescript
onboarding_seen?: boolean;
```

Na tipagem do retorno de `updateSettings`, adicionar:

```typescript
onboarding_seen: boolean;
```

- [ ] **Step 3: Adicionar `onboardingSeen` em `useSettings`**

Em `frontend/src/hooks/useSettings.ts`, na interface `Settings`, adicionar:

```typescript
onboardingSeen: boolean;
```

No objeto `defaults`, adicionar:

```typescript
onboardingSeen: false,
```

No `useEffect` que chama `api.getMe()`, adicionar ao objeto `next`:

```typescript
onboardingSeen: me.onboarding_seen,
```

Na função `updateSetting`, no tipo do `patch`, adicionar:

```typescript
onboardingSeen?: boolean;
```

No bloco que monta `body`, adicionar:

```typescript
if (patch.onboardingSeen !== undefined) body.onboarding_seen = patch.onboardingSeen;
```

No re-fetch após `updateSettings`, adicionar ao objeto `next`:

```typescript
onboardingSeen: me.onboarding_seen,
```

No `return` de `useSettings`, adicionar:

```typescript
onboardingSeen: settings.onboardingSeen,
```

- [ ] **Step 4: Verificar tipos com TypeScript**

```bash
cd frontend && npx tsc --noEmit
```

Expected: sem erros de tipo.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/services/api.ts frontend/src/hooks/useSettings.ts
git commit -m "feat(frontend): adicionar onboarding_seen em useSettings e api"
```

---

## Task 5: Componente `UserManual`

**Files:**
- Create: `frontend/src/components/UserManual/UserManual.tsx`

- [ ] **Step 1: Criar o componente**

Criar `frontend/src/components/UserManual/UserManual.tsx`:

```tsx
import { useState } from "react";
import { ChevronDown, ChevronUp } from "lucide-react";
import { cn } from "@/lib/utils";

interface UserManualProps {
  mode: "modal" | "section";
  onClose?: () => void;
  onViewFull?: () => void;
}

interface SectionProps {
  icon: string;
  title: string;
  children: React.ReactNode;
}

function ManualSection({ icon, title, children }: SectionProps) {
  return (
    <div className="flex items-start gap-3">
      <span className="text-base shrink-0 mt-0.5">{icon}</span>
      <div className="min-w-0">
        <p className="text-sm font-semibold">{title}</p>
        <div className="text-xs text-muted-foreground leading-relaxed mt-0.5">{children}</div>
      </div>
    </div>
  );
}

export function UserManual({ mode, onClose, onViewFull }: UserManualProps) {
  const isSection = mode === "section";
  const [expanded, setExpanded] = useState(!isSection);

  const content = (
    <div className={cn("flex flex-col gap-4", isSection && "px-1 pb-2 pt-1")}>
      {/* Por onde começar */}
      <div className="rounded-lg bg-amber-50 border border-amber-200 dark:bg-amber-950/30 dark:border-amber-800 p-3">
        <p className="text-xs font-bold mb-2">🚀 Por onde começar</p>
        <ol className="text-xs text-muted-foreground space-y-1.5 list-none">
          <li><span className="font-semibold text-foreground">1. Chave API</span> — Perfil → Chave API. Necessária para ativar o Chat. Veja o passo a passo na seção abaixo.</li>
          <li><span className="font-semibold text-foreground">2. Formas de Pagamento</span> — Perfil → Formas de Pagamento. Já criamos um <em>Débito</em> pra você. Adicione seus cartões.</li>
          <li><span className="font-semibold text-foreground">3. Gastos Fixos</span> — Perfil → Gastos Fixos. Aluguel, streaming, mensalidades.</li>
          <li><span className="font-semibold text-foreground">4. Limites</span> — Menu → Limites. Tetos de gasto por categoria.</li>
          <li><span className="font-semibold text-foreground">5. Reservas</span> — Menu → Reservas. Metas de poupança.</li>
        </ol>
      </div>

      {/* Funcionalidades */}
      <ManualSection icon="🔑" title="Chave API">
        <p>Para usar o Chat, você precisa de uma chave Anthropic:</p>
        <ol className="mt-1 space-y-0.5 list-decimal list-inside">
          <li>Acesse <strong>console.anthropic.com</strong> e crie uma conta</li>
          <li>No menu lateral, clique em <strong>"API Keys"</strong></li>
          <li>Clique em <strong>"Create Key"</strong>, dê um nome e confirme</li>
          <li>Copie a chave — começa com <code className="font-mono">sk-ant-...</code></li>
          <li>Cole em <strong>Perfil → Chave API</strong> e salve</li>
        </ol>
        <p className="mt-1 italic">A chave é exibida uma única vez. Se perder, gere outra.</p>
      </ManualSection>

      <ManualSection icon="💬" title="Chat financeiro">
        Registre gastos em linguagem natural. <em>"Gastei 45 no iFood no Nubank"</em> já salva, categoriza e vincula ao cartão.
        Sempre mencione a forma de pagamento: cartão de crédito vai pra fatura, vale sai do saldo do vale.
        Esqueceu? Corrija depois na tela Consumo.
      </ManualSection>

      <ManualSection icon="📊" title="Dashboard">
        Gráficos de gastos por categoria, comparação entre meses e evolução temporal. Prepare o coração antes de abrir.
      </ManualSection>

      <ManualSection icon="🧾" title="Consumo">
        Lista de todas as transações. Edite, delete ou corrija qualquer lançamento — inclusive a forma de pagamento esquecida.
      </ManualSection>

      <ManualSection icon="💰" title="Caixa">
        <p>Saldo em conta, faturas de cartão e dois recursos especiais:</p>
        <p className="mt-1"><span className="font-semibold text-foreground">Saldo do mês anterior:</span> se sobrou dinheiro, o sistema pergunta se quer trazer pro mês atual.</p>
        <p className="mt-1"><span className="font-semibold text-foreground">Depósito de vale (VR/VA):</span> no início do mês, confirme quanto entrou no seu cartão de alimentação.</p>
      </ManualSection>

      <ManualSection icon="💳" title="Formas de Pagamento">
        <p><span className="font-semibold text-foreground">Cartão de crédito:</span> informe dia de fechamento e vencimento — o sistema monta a fatura.</p>
        <p className="mt-1"><span className="font-semibold text-foreground">Vale (VR/VA):</span> gastos saem do saldo do vale, não do seu bolso.</p>
        <p className="mt-1"><span className="font-semibold text-foreground">Débito/PIX:</span> débita direto da conta. Já existe um <em>Débito</em> criado pra você.</p>
      </ManualSection>

      <ManualSection icon="🎯" title="Limites">
        Tetos de gasto por categoria. Amarelo em 70%, vermelho ao estourar. O sistema avisa — como um amigo que sabe que você não vai ouvir, mas avisa assim mesmo.
      </ManualSection>

      <ManualSection icon="🐷" title="Reservas">
        <p>Metas de poupança com nome, valor-alvo e prazo. Aportes, rendimentos e saques também funcionam pelo chat.</p>
        <p className="mt-1 font-semibold text-foreground">⚠️ Atenção:</p>
        <p>Todo aporte sai do seu saldo em conta. Antes de um aporte grande, confira o saldo no Caixa — se não cobrir, o caixa fica negativo.</p>
      </ManualSection>

      <ManualSection icon="📅" title="Gastos Fixos">
        <p>Dois tipos na mesma tela:</p>
        <p className="mt-1"><span className="font-semibold text-foreground">Gasto fixo</span> (aluguel, internet, academia): aparece como lembrete mensal no Caixa — você registra o pagamento manualmente.</p>
        <p className="mt-1"><span className="font-semibold text-foreground">Assinatura de cartão</span> (Netflix, Spotify): vinculada a um cartão de crédito, entra na fatura automaticamente.</p>
        <p className="mt-1 italic">Regra: se você paga, é gasto fixo. Se o cartão cobra, é assinatura.</p>
      </ManualSection>
    </div>
  );

  if (isSection) {
    return (
      <div id="manual" className="rounded-xl bg-card border border-border overflow-hidden">
        <button
          onClick={() => setExpanded((e) => !e)}
          className="w-full flex items-center gap-3 px-4 py-4 hover:bg-secondary/50 transition-colors text-left"
        >
          <div className="w-9 h-9 rounded-lg bg-secondary flex items-center justify-center shrink-0">
            <span className="text-base">📖</span>
          </div>
          <div className="flex-1">
            <p className="text-sm font-semibold">Manual do sistema</p>
            <p className="text-xs text-muted-foreground">7 funcionalidades · toque para {expanded ? "fechar" : "expandir"}</p>
          </div>
          {expanded
            ? <ChevronUp className="w-4 h-4 text-muted-foreground shrink-0" />
            : <ChevronDown className="w-4 h-4 text-muted-foreground shrink-0" />
          }
        </button>
        {expanded && (
          <div className="border-t border-border px-4 py-4">
            {content}
          </div>
        )}
      </div>
    );
  }

  // mode === "modal": renderiza só o conteúdo, sem wrapper (Dialog fica no UserManualModal)
  return <div className="flex flex-col gap-4 py-2">{content}</div>;
}
```

- [ ] **Step 2: Verificar tipos**

```bash
cd frontend && npx tsc --noEmit
```

Expected: sem erros.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/UserManual/UserManual.tsx
git commit -m "feat(frontend): criar componente UserManual com conteúdo completo"
```

---

## Task 6: Componente `UserManualModal`

**Files:**
- Create: `frontend/src/components/UserManual/UserManualModal.tsx`

- [ ] **Step 1: Criar o modal**

Criar `frontend/src/components/UserManual/UserManualModal.tsx`:

```tsx
import { useNavigate } from "react-router-dom";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { UserManual } from "./UserManual";
import { useSettings } from "@/hooks/useSettings";

interface UserManualModalProps {
  open: boolean;
  onClose: () => void;
}

export function UserManualModal({ open, onClose }: UserManualModalProps) {
  const navigate = useNavigate();
  const { updateSetting } = useSettings();

  const handleClose = async () => {
    try {
      await updateSetting({ onboardingSeen: true });
    } catch {
      // Non-fatal: close anyway
    }
    onClose();
  };

  const handleViewFull = async () => {
    await handleClose();
    navigate("/profile");
    // Scroll após navegação
    setTimeout(() => {
      document.getElementById("manual")?.scrollIntoView({ behavior: "smooth" });
    }, 300);
  };

  return (
    <Dialog open={open} onOpenChange={(v) => !v && handleClose()}>
      <DialogContent className="sm:max-w-md max-h-[85vh] flex flex-col">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <span>👋</span> Bem-vindo(a)!
          </DialogTitle>
        </DialogHeader>
        <div className="flex-1 overflow-y-auto -mx-6 px-6">
          <UserManual mode="modal" onClose={handleClose} onViewFull={handleViewFull} />
        </div>
        <div className="flex gap-2 pt-4 border-t border-border shrink-0">
          <Button variant="outline" className="flex-1" onClick={handleViewFull}>
            Ver manual completo
          </Button>
          <Button className="flex-1" onClick={handleClose}>
            Começar →
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  );
}
```

- [ ] **Step 2: Verificar tipos**

```bash
cd frontend && npx tsc --noEmit
```

Expected: sem erros.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/UserManual/UserManualModal.tsx
git commit -m "feat(frontend): criar UserManualModal para primeiro login"
```

---

## Task 7: Integrar modal em `Chat.tsx`

**Files:**
- Modify: `frontend/src/pages/Chat.tsx`

- [ ] **Step 1: Adicionar estado e lógica do modal**

Em `frontend/src/pages/Chat.tsx`, adicionar import:

```tsx
import { useState, useEffect } from "react";
import { UserManualModal } from "@/components/UserManual/UserManualModal";
```

Dentro do componente `Chat`, adicionar após os hooks existentes:

```tsx
const { userApiKey, loading: settingsLoading, onboardingSeen } = useSettings();
const [manualOpen, setManualOpen] = useState(false);

useEffect(() => {
  if (!settingsLoading && onboardingSeen === false) {
    setManualOpen(true);
  }
}, [settingsLoading, onboardingSeen]);
```

- [ ] **Step 2: Remover a desestruturação duplicada de `useSettings`**

A linha atual `const { userApiKey, loading: settingsLoading } = useSettings();` deve ser substituída pela nova desestruturação que inclui `onboardingSeen` (já feito no passo anterior).

- [ ] **Step 3: Adicionar `UserManualModal` no JSX**

No `return` de `Chat`, antes do fechamento da div principal, adicionar:

```tsx
<UserManualModal open={manualOpen} onClose={() => setManualOpen(false)} />
```

- [ ] **Step 4: Verificar tipos**

```bash
cd frontend && npx tsc --noEmit
```

Expected: sem erros.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/pages/Chat.tsx
git commit -m "feat(frontend): exibir UserManualModal no primeiro login em /chat"
```

---

## Task 8: Seção do manual no Perfil

**Files:**
- Modify: `frontend/src/pages/Profile.tsx`

- [ ] **Step 1: Adicionar import**

Em `frontend/src/pages/Profile.tsx`, adicionar import:

```tsx
import { UserManual } from "@/components/UserManual/UserManual";
```

- [ ] **Step 2: Adicionar seção no final da página**

Em `Profile.tsx`, antes do botão "Sair da conta" (que está dentro do `<div className="flex-1 overflow-y-auto...">`), adicionar a nova section:

```tsx
{/* ── COMO USAR O SISTEMA ── */}
<section>
  <h2 className="px-2 mb-2 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
    Como usar o sistema
  </h2>
  <UserManual mode="section" />
</section>
```

- [ ] **Step 3: Verificar tipos**

```bash
cd frontend && npx tsc --noEmit
```

Expected: sem erros.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/pages/Profile.tsx
git commit -m "feat(frontend): adicionar seção de manual no final da página de Perfil"
```

---

## Task 9: Testar o fluxo completo

- [ ] **Step 1: Subir o backend**

```bash
cd backend && go run ./cmd/server
```

Verificar nos logs que a migration `onboarding_seen` rodou sem erro.

- [ ] **Step 2: Subir o frontend**

```bash
cd frontend && bun run dev
```

- [ ] **Step 3: Testar primeiro login**

1. Criar um novo usuário em `/login` (registrar)
2. Verificar que o modal aparece em `/chat`
3. Verificar que o Débito padrão foi criado em Perfil → Formas de Pagamento
4. Clicar "Começar →" — modal fecha
5. Fazer logout e login novamente — modal **não** deve aparecer
6. Fazer login em outro browser (ou aba anônima) com o mesmo usuário — modal **não** deve aparecer (confirmando que persiste no backend)

- [ ] **Step 4: Testar "Ver manual completo"**

1. Criar outro usuário novo
2. No modal, clicar "Ver manual completo"
3. Verificar que navega para `/profile` e a seção `#manual` fica visível com scroll

- [ ] **Step 5: Testar seção no Perfil**

1. Navegar para `/profile`
2. Rolar até o final — verificar seção "Como usar o sistema"
3. Clicar no card — verificar que expande com o conteúdo
4. Verificar em mobile (DevTools → responsive) e desktop

- [ ] **Step 6: Testar responsividade do modal**

1. Em DevTools, simular iPhone 14 (390px)
2. Verificar que o modal ocupa a tela corretamente
3. Verificar que o conteúdo é rolável se ultrapassar a altura disponível

- [ ] **Step 7: Commit final se tudo passar**

```bash
git add .
git commit -m "test: validar fluxo completo do manual interativo"
```
