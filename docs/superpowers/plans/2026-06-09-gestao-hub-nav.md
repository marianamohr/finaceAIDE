# Gestão Hub — Reorganização do Bottom Nav Mobile

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Consolidar Painel, Projeção, Limites e Reservas num hub chamado "Gestão", reduzindo o bottom nav mobile de 6+ itens para no máximo 5.

**Architecture:** Novo item "Gestão" no BottomNav aponta para uma página hub (`/gestao`) com 4 cards. O BottomNav mantém "Gestão" ativo quando o usuário navega para qualquer sub-rota. As sub-páginas ganham botão `←` no PageHeader.

**Tech Stack:** React, React Router v6, Zustand, lucide-react, Tailwind CSS, Vitest + React Testing Library

---

## Mapa de arquivos

| Arquivo | Ação | Responsabilidade |
|---------|------|------------------|
| `frontend/src/components/BottomNav/BottomNav.tsx` | Modificar | Remove Painel/Limites/Reservas, adiciona Gestão, atualiza lógica de active |
| `frontend/src/components/BottomNav/BottomNav.test.tsx` | Criar | Testes do BottomNav atualizado |
| `frontend/src/pages/Gestao.tsx` | Criar | Hub page com 4 cards |
| `frontend/src/pages/Gestao.test.tsx` | Criar | Testes da página hub |
| `frontend/src/App.tsx` | Modificar | Adiciona rota `/gestao` |
| `frontend/src/pages/Painel.tsx` | Modificar | Adiciona `showBack` ao PageHeader |
| `frontend/src/pages/ProjecaoDetalhe.tsx` | Modificar | Remove `onBack` hardcoded (mantém `showBack`) |
| `frontend/src/pages/Limites.tsx` | Modificar | Adiciona `showBack` ao PageHeader |
| `frontend/src/pages/Reservas.tsx` | Modificar | Adiciona `showBack` ao PageHeader |

---

## Task 1: Atualizar BottomNav

**Files:**
- Modify: `frontend/src/components/BottomNav/BottomNav.tsx`
- Create: `frontend/src/components/BottomNav/BottomNav.test.tsx`

- [ ] **Step 1: Escrever o teste falhando**

Crie `frontend/src/components/BottomNav/BottomNav.test.tsx`:

```tsx
import { describe, it, expect, vi, beforeEach } from "vitest"
import { render, screen } from "@testing-library/react"
import { BottomNav } from "./BottomNav"

const mockNavigate = vi.fn()
let mockPathname = "/chat"

vi.mock("react-router-dom", () => ({
  useLocation: () => ({ pathname: mockPathname }),
  useNavigate: () => mockNavigate,
}))

vi.mock("@/store/settings", () => ({
  useSettingsStore: (selector: (s: { showGoals: boolean; showTools: boolean; showLimitsCategories: boolean }) => unknown) =>
    selector({ showGoals: true, showTools: false, showLimitsCategories: true }),
}))

vi.mock("@/hooks/useRecurringExpenses", () => ({
  useRecurringReminders: () => ({ pendingCount: 0 }),
}))

beforeEach(() => {
  mockPathname = "/chat"
  mockNavigate.mockClear()
})

describe("BottomNav", () => {
  it("exibe os 4 itens fixos: Chat, Consumo, Caixa, Gestão", () => {
    render(<BottomNav />)
    expect(screen.getByText("Chat")).toBeInTheDocument()
    expect(screen.getByText("Consumo")).toBeInTheDocument()
    expect(screen.getByText("Caixa")).toBeInTheDocument()
    expect(screen.getByText("Gestão")).toBeInTheDocument()
  })

  it("não exibe Painel, Limites nem Reservas como itens do nav", () => {
    render(<BottomNav />)
    expect(screen.queryByText("Painel")).not.toBeInTheDocument()
    expect(screen.queryByText("Limites")).not.toBeInTheDocument()
    expect(screen.queryByText("Reservas")).not.toBeInTheDocument()
  })

  it("item Gestão fica ativo quando pathname é /gestao", () => {
    mockPathname = "/gestao"
    render(<BottomNav />)
    const btn = screen.getByText("Gestão").closest("button")!
    expect(btn.className).toMatch(/text-primary/)
  })

  it("item Gestão fica ativo quando pathname é /painel (sub-rota)", () => {
    mockPathname = "/painel"
    render(<BottomNav />)
    const btn = screen.getByText("Gestão").closest("button")!
    expect(btn.className).toMatch(/text-primary/)
  })

  it("item Gestão fica ativo quando pathname é /projecao (sub-rota)", () => {
    mockPathname = "/projecao"
    render(<BottomNav />)
    const btn = screen.getByText("Gestão").closest("button")!
    expect(btn.className).toMatch(/text-primary/)
  })

  it("item Chat fica ativo quando pathname é /chat", () => {
    mockPathname = "/chat"
    render(<BottomNav />)
    const btn = screen.getByText("Chat").closest("button")!
    expect(btn.className).toMatch(/text-primary/)
  })
})
```

- [ ] **Step 2: Rodar o teste e confirmar que falha**

```bash
cd frontend && npm test -- BottomNav.test.tsx
```

Esperado: FAIL — "Gestão" não existe no componente ainda.

- [ ] **Step 3: Implementar a mudança no BottomNav**

Substitua o conteúdo de `frontend/src/components/BottomNav/BottomNav.tsx`:

```tsx
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
```

- [ ] **Step 4: Rodar os testes e confirmar que passam**

```bash
cd frontend && npm test -- BottomNav.test.tsx
```

Esperado: todos os testes PASS.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/BottomNav/BottomNav.tsx frontend/src/components/BottomNav/BottomNav.test.tsx
git commit -m "feat(nav): substitui Painel/Limites/Reservas pelo item Gestão no BottomNav"
```

---

## Task 2: Criar a página hub Gestão

**Files:**
- Create: `frontend/src/pages/Gestao.tsx`
- Create: `frontend/src/pages/Gestao.test.tsx`

- [ ] **Step 1: Escrever o teste falhando**

Crie `frontend/src/pages/Gestao.test.tsx`:

```tsx
import { describe, it, expect, vi, beforeEach } from "vitest"
import { render, screen, fireEvent } from "@testing-library/react"
import Gestao from "./Gestao"

const mockNavigate = vi.fn()

// Controla os valores das flags entre testes
const mockSettings = { showGoals: true, showTools: false, showLimitsCategories: true }

vi.mock("react-router-dom", () => ({
  useNavigate: () => mockNavigate,
  useLocation: () => ({ pathname: "/gestao" }),
}))

vi.mock("@/store/settings", () => ({
  useSettingsStore: (selector: (s: typeof mockSettings) => unknown) =>
    selector(mockSettings),
}))

vi.mock("@/components/BottomNav/BottomNav", () => ({
  BottomNav: () => <div data-testid="bottom-nav" />,
}))

vi.mock("@/components/PageHeader/PageHeader", () => ({
  PageHeader: ({ title }: { title: string }) => <div data-testid="page-header">{title}</div>,
}))

beforeEach(() => {
  mockNavigate.mockClear()
  mockSettings.showGoals = true
  mockSettings.showLimitsCategories = true
  mockSettings.showTools = false
})

describe("Gestao hub page", () => {
  it("renderiza o título 'Gestão'", () => {
    render(<Gestao />)
    expect(screen.getByTestId("page-header")).toHaveTextContent("Gestão")
  })

  it("exibe cards Painel e Projeção sempre", () => {
    render(<Gestao />)
    expect(screen.getByText("Painel")).toBeInTheDocument()
    expect(screen.getByText("Projeção")).toBeInTheDocument()
  })

  it("exibe cards Limites e Reservas quando flags ativas", () => {
    render(<Gestao />)
    expect(screen.getByText("Limites")).toBeInTheDocument()
    expect(screen.getByText("Reservas")).toBeInTheDocument()
  })

  it("navega para /painel ao clicar no card Painel", () => {
    render(<Gestao />)
    fireEvent.click(screen.getByText("Painel").closest("button")!)
    expect(mockNavigate).toHaveBeenCalledWith("/painel")
  })

  it("navega para /projecao ao clicar no card Projeção", () => {
    render(<Gestao />)
    fireEvent.click(screen.getByText("Projeção").closest("button")!)
    expect(mockNavigate).toHaveBeenCalledWith("/projecao")
  })

  it("oculta card Limites quando showLimitsCategories é false", () => {
    mockSettings.showLimitsCategories = false
    render(<Gestao />)
    expect(screen.queryByText("Limites")).not.toBeInTheDocument()
  })

  it("oculta card Reservas quando showGoals é false", () => {
    mockSettings.showGoals = false
    render(<Gestao />)
    expect(screen.queryByText("Reservas")).not.toBeInTheDocument()
  })

  it("renderiza o BottomNav", () => {
    render(<Gestao />)
    expect(screen.getByTestId("bottom-nav")).toBeInTheDocument()
  })
})
```

- [ ] **Step 2: Rodar o teste e confirmar que falha**

```bash
cd frontend && npm test -- Gestao.test.tsx
```

Esperado: FAIL — módulo `./Gestao` não encontrado.

- [ ] **Step 3: Criar a página Gestao.tsx**

Crie `frontend/src/pages/Gestao.tsx`:

```tsx
import { BarChart2, TrendingUp, Target, PiggyBank, ChevronRight } from "lucide-react"
import { useNavigate } from "react-router-dom"
import { BottomNav } from "@/components/BottomNav/BottomNav"
import { PageHeader } from "@/components/PageHeader/PageHeader"
import { useSettingsStore } from "@/store/settings"

interface HubCard {
  label: string
  subtitle: string
  path: string
  icon: React.ElementType
  iconBg: string
  iconColor: string
}

const CARDS: HubCard[] = [
  {
    label: "Painel",
    subtitle: "Histórico e tendências",
    path: "/painel",
    icon: BarChart2,
    iconBg: "rgba(59,130,246,0.15)",
    iconColor: "#3b82f6",
  },
  {
    label: "Projeção",
    subtitle: "Próximos 6 meses",
    path: "/projecao",
    icon: TrendingUp,
    iconBg: "rgba(168,85,247,0.15)",
    iconColor: "#a855f7",
  },
  {
    label: "Limites",
    subtitle: "Orçamento por categoria",
    path: "/limites",
    icon: Target,
    iconBg: "rgba(245,158,11,0.15)",
    iconColor: "hsl(38,92%,50%)",
  },
  {
    label: "Reservas",
    subtitle: "Metas de poupança",
    path: "/reservas",
    icon: PiggyBank,
    iconBg: "rgba(20,179,122,0.15)",
    iconColor: "hsl(160,84%,45%)",
  },
]

export default function Gestao() {
  const navigate = useNavigate()
  const showGoals = useSettingsStore((s) => s.showGoals)
  const showLimitsCategories = useSettingsStore((s) => s.showLimitsCategories)

  const visibleCards = CARDS.filter((card) => {
    if (card.path === "/limites") return showLimitsCategories
    if (card.path === "/reservas") return showGoals
    return true
  })

  return (
    <div className="h-[var(--app-height)] lg:h-full flex flex-col bg-background">
      <div className="lg:hidden">
        <PageHeader title="Gestão" />
      </div>
      <div className="flex-1 overflow-y-auto p-4 flex flex-col gap-3">
        {visibleCards.map(({ label, subtitle, path, icon: Icon, iconBg, iconColor }) => (
          <button
            key={path}
            onClick={() => navigate(path)}
            className="flex items-center gap-4 p-4 rounded-xl bg-card border border-border text-left hover:bg-secondary/50 transition-colors w-full"
          >
            <div
              className="w-10 h-10 rounded-xl flex items-center justify-center shrink-0"
              style={{ background: iconBg }}
            >
              <Icon className="w-5 h-5" style={{ color: iconColor }} />
            </div>
            <div className="flex-1 min-w-0">
              <p className="font-semibold text-sm text-foreground">{label}</p>
              <p className="text-xs text-muted-foreground mt-0.5">{subtitle}</p>
            </div>
            <ChevronRight className="w-4 h-4 text-muted-foreground shrink-0" />
          </button>
        ))}
      </div>
      <BottomNav />
    </div>
  )
}
```

- [ ] **Step 4: Rodar os testes e confirmar que passam**

```bash
cd frontend && npm test -- Gestao.test.tsx
```

Esperado: todos os testes PASS.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/pages/Gestao.tsx frontend/src/pages/Gestao.test.tsx
git commit -m "feat(gestao): página hub com cards para Painel, Projeção, Limites e Reservas"
```

---

## Task 3: Registrar rota /gestao no App.tsx

**Files:**
- Modify: `frontend/src/App.tsx`

- [ ] **Step 1: Adicionar import de Gestao**

Em `frontend/src/App.tsx`, após a linha `import ProjecaoDetalhe from "./pages/ProjecaoDetalhe";`, adicione:

```tsx
import Gestao from "./pages/Gestao";
```

- [ ] **Step 2: Adicionar a rota**

Após a linha com `/projecao`, adicione:

```tsx
<Route path="/gestao" element={<ProtectedRoute><AppShell><Gestao /></AppShell></ProtectedRoute>} />
```

- [ ] **Step 3: Verificar que o build não tem erros**

```bash
cd frontend && npm run build 2>&1 | tail -5
```

Esperado: sem erros de TypeScript ou importação.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/App.tsx
git commit -m "feat(router): adiciona rota /gestao"
```

---

## Task 4: Adicionar showBack nas sub-páginas

**Files:**
- Modify: `frontend/src/pages/Painel.tsx:9`
- Modify: `frontend/src/pages/Limites.tsx:538`
- Modify: `frontend/src/pages/Reservas.tsx:756`
- Modify: `frontend/src/pages/ProjecaoDetalhe.tsx` (múltiplas ocorrências)

- [ ] **Step 1: Painel — adicionar showBack**

Em `frontend/src/pages/Painel.tsx`, linha 9, mude:

```tsx
        <PageHeader title="Painel" />
```

para:

```tsx
        <PageHeader title="Painel" showBack />
```

- [ ] **Step 2: Limites — adicionar showBack**

Em `frontend/src/pages/Limites.tsx`, linha 538, mude:

```tsx
      <PageHeader title="Limites" />
```

para:

```tsx
      <PageHeader title="Limites" showBack />
```

- [ ] **Step 3: Reservas — adicionar showBack**

Em `frontend/src/pages/Reservas.tsx`, linha 756, mude:

```tsx
      <PageHeader title="Reservas" />
```

para:

```tsx
      <PageHeader title="Reservas" showBack />
```

- [ ] **Step 4: ProjecaoDetalhe — remover onBack hardcoded**

`ProjecaoDetalhe.tsx` já tem `showBack` mas com `onBack={() => navigate("/painel")}` hardcoded em todas as ocorrências. Como Projeção agora é acessível também pelo hub, o back deve usar `navigate(-1)` (padrão do PageHeader). Substitua **todas** as ocorrências de:

```tsx
<PageHeader title="Projeção de Gastos" showBack onBack={() => navigate("/painel")} />
```

por:

```tsx
<PageHeader title="Projeção de Gastos" showBack />
```

O arquivo tem 4 ocorrências (estados: loading, error, empty e o render principal). Use busca+substituição no editor.

- [ ] **Step 5: Verificar que o build não tem erros**

```bash
cd frontend && npm run build 2>&1 | tail -5
```

Esperado: sem erros.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/pages/Painel.tsx frontend/src/pages/Limites.tsx frontend/src/pages/Reservas.tsx frontend/src/pages/ProjecaoDetalhe.tsx
git commit -m "feat(nav): adiciona botão voltar no header das sub-páginas de Gestão"
```

---

## Verificação final

Após todas as tasks, rode a suíte completa:

```bash
cd frontend && npm test
```

Esperado: todos os testes passando, sem regressões.

Para verificar visualmente, rode o app (`npm run dev`) e valide:
1. BottomNav mobile mostra 4 itens: Chat, Consumo, Caixa, Gestão
2. Tapping "Gestão" abre o hub com cards Painel, Projeção, Limites, Reservas
3. Tapping um card navega para a página correta e mantém "Gestão" ativo no nav
4. Botão `←` no header de cada sub-página volta ao hub
5. Ícone de perfil (👤) visível no header de todas as páginas
