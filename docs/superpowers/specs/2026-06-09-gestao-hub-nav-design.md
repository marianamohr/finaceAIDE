# Gestão Hub — Reorganização do Bottom Nav Mobile

**Data:** 2026-06-09  
**Status:** Aprovado

---

## Problema

O bottom nav mobile acumula até 8 itens quando todas as flags de settings estão ativas (Chat, Consumo, Caixa, Limites, Reservas, Painel, Tools, Pendentes), tornando a navegação visualmente poluída e os alvos de toque pequenos demais.

Além disso, a página Projeção (`/projecao`) não tem entrada no nav — é acessível apenas via um link dentro do gráfico do Painel, o que prejudica o descobrimento de uma feature importante.

---

## Solução

Consolidar os 4 itens de uso esporádico (Painel, Projeção, Limites, Reservas) em um único item de navegação chamado **"Gestão"**, que abre uma página hub com cards para cada seção.

---

## Design

### Bottom Nav

| Posição | Item | Rota | Visibilidade |
|---------|------|------|--------------|
| 1 | Chat | `/chat` | sempre |
| 2 | Consumo | `/consumo` | sempre |
| 3 | Caixa | `/caixa` | sempre |
| 4 | Gestão | `/gestao` | sempre |
| 5 | Pendentes | `/gastos-fixos/lembretes` | dinâmico (quando `pendingCount > 0`) |

**Tools** (`showTools`) está desabilitado para usuários — não entra no nav.

O nav nunca ultrapassa 5 itens.

### Página Hub `/gestao`

Título da página: **"Gestão"**

4 cards empilhados verticalmente, na ordem:

| Card | Ícone | Cor de destaque | Subtítulo | Rota de destino | Visibilidade |
|------|-------|-----------------|-----------|-----------------|--------------|
| Painel | `BarChart2` | azul (`#3b82f6`) | "Histórico e tendências" | `/painel` | sempre |
| Projeção | `TrendingUp` | roxo (`#a855f7`) | "Próximos 6 meses" | `/projecao` | sempre |
| Limites | `Target` | âmbar (`hsl(38,92%,50%)`) | "Orçamento por categoria" | `/limites` | `showLimitsCategories === true` |
| Reservas | `PiggyBank` | verde primário (`hsl(160,84%,45%)`) | "Metas de poupança" | `/reservas` | `showGoals === true` |

- Fundo e bordas seguem tokens do tema: `bg-card`, `border-border`.
- Cada card é um botão que navega para a rota correspondente com `useNavigate`.
- O ícone de cada card tem fundo com opacidade reduzida da cor de destaque (ex: `rgba(59,130,246,0.15)` para o Painel).
- Chevron `›` à direita em `text-muted-foreground`.

### Flags de settings preservadas

`showGoals` e `showLimitsCategories` continuam controlando a visibilidade dos cards dentro do hub. Quando ambas as flags estão desativadas, o hub exibe apenas Painel e Projeção.

### Rotas existentes

Todas as rotas existentes (`/painel`, `/reservas`, `/limites`, `/projecao`) permanecem inalteradas. O hub apenas navega para elas.

---

## Comportamentos de navegação obrigatórios

### ① Perfil sempre acessível via Header

O `PageHeader` já renderiza um botão `User` que navega para `/profile` em todas as páginas exceto a própria `/profile`. Esse comportamento é mantido — a nova página `/gestao` usa `PageHeader` normalmente.

### ② BottomNav nunca some

O `BottomNav` deve estar presente em **todas** as páginas do app, incluindo a nova `/gestao`. As páginas `/painel`, `/projecao`, `/limites` e `/reservas` já têm `<BottomNav />` e continuam com ele.

### ③ Item "Gestão" ativo nas sub-rotas

Quando o usuário navega do hub para uma sub-página (`/painel`, `/projecao`, `/limites`, `/reservas`), o item **Gestão** no BottomNav deve permanecer destacado (cor primária), sinalizando que o usuário está dentro desse contexto.

A lógica de estado ativo no `BottomNav` passa a ser:

```ts
const GESTAO_SUBROUTES = ["/painel", "/projecao", "/limites", "/reservas"]

const active =
  location.pathname === path ||
  (path === "/gestao" && GESTAO_SUBROUTES.includes(location.pathname))
```

### ④ Voltar ao hub a partir das sub-páginas

As páginas `/painel`, `/projecao`, `/limites` e `/reservas` devem exibir o botão de voltar (`showBack`) no `PageHeader` quando renderizadas em mobile. O comportamento padrão de `navigate(-1)` leva de volta ao hub `/gestao` (ou ao histórico do navegador, se o acesso foi direto).

**Mudança**: adicionar `showBack` ao `<PageHeader>` dessas quatro páginas.

---

## O que muda no BottomNav

O componente `BottomNav.tsx` é simplificado:

**Remove:**
- `{ path: "/limites", icon: Target, label: "Limites" }` (com flag `showLimitsCategories`)
- `{ path: "/reservas", icon: PiggyBank, label: "Reservas" }` (com flag `showGoals`)
- `{ path: "/painel", icon: BarChart2, label: "Painel" }`

**Adiciona:**
- `{ path: "/gestao", icon: LayoutDashboard, label: "Gestão" }`

**Atualiza lógica de active**: conforme item ③ acima.

O item Pendentes dinâmico permanece exatamente como hoje.

---

## Arquivos afetados

| Arquivo | Tipo de mudança |
|---------|-----------------|
| `frontend/src/components/BottomNav/BottomNav.tsx` | Remoção de 3 itens, adição de 1, lógica de active atualizada |
| `frontend/src/pages/Gestao.tsx` | Criação — nova página hub |
| `frontend/src/App.tsx` | Nova rota `/gestao` com `ProtectedRoute` + `AppShell` |
| `frontend/src/pages/Painel.tsx` | Adicionar `showBack` ao `PageHeader` |
| `frontend/src/pages/ProjecaoDetalhe.tsx` | Adicionar `showBack` ao `PageHeader` |
| `frontend/src/pages/Limites.tsx` | Adicionar `showBack` ao `PageHeader` |
| `frontend/src/pages/Reservas.tsx` | Adicionar `showBack` ao `PageHeader` |

---

## Fora de escopo

- Badge de Pendentes no ícone Gestão (pode ser feito numa iteração futura)
- Versão desktop (o `BottomNav` já é `lg:hidden`)
