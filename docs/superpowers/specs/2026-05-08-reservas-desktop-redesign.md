# Reservas — Redesign Desktop & Mobile

**Data:** 2026-05-08

## Contexto

A página `/reservas` foi construída com layout mobile-first e nunca recebeu tratamento de desktop. No desktop, os cards ficam empilhados em coluna única — idêntico ao mobile — desperdiçando o espaço horizontal disponível. Além disso, a barra de progresso reta não transmite bem a sensação de avanço de uma meta financeira. O objetivo é tornar a página visualmente atraente e funcionalmente mais rica no desktop, mantendo usabilidade no mobile.

---

## Design

### Desktop — Grid "Travel Poster" (Opção A)

**Layout geral:**
- Grid adaptativo: `lg:grid-cols-3` (padrão), `xl:grid-cols-3`, `2xl:grid-cols-4`
- Gap de 20px entre cards
- Header da página mantém estilo atual (PageHeader)

**Anatomia do card (desktop):**
```
┌─────────────────────────────┐
│  [imagem de capa 130px]     │  ← gradiente escuro na base
│  Nome do goal sobreposto    │
├─────────────────────────────┤
│     ◯  progresso 80px       │  ← círculo SVG centralizado
│                             │
│  Reservado  │  Meta         │
│  Contrib.   │  Prazo        │
├─────────────────────────────┤
│  Inst. · Conta  [+ Mover]   │
└─────────────────────────────┘
```

**Imagem de capa:**
- Biblioteca curada por categoria (`goal_type`): viagem → paisagem, carro → carro, casa → casa, emergência → cofre/escudo
- Upload próprio: campo opcional na tela de criar/editar goal
- Fallback: gradiente baseado no nome (hash → cor) quando sem imagem
- Campo novo no modelo: `cover_image_url?: string`

**Progresso circular:**
- SVG puro com `stroke-dasharray` / `stroke-dashoffset`
- Cor: `var(--primary)` no arco, `var(--border)` no fundo
- Centro: percentual em destaque + label "meta" abaixo
- Animação de entrada: `transition stroke-dashoffset 0.6s ease`

**Modal "Mover" (desktop):**
- Substitui o bottom sheet por `Dialog` (modal centralizado)
- Mesmo conteúdo do `AddTransactionSheet` atual
- No mobile continua como bottom sheet

---

### Mobile — Cards empilhados redesenhados

**Layout:**
- Continua uma coluna (empilhado)
- Card recebe imagem de capa (mesma lógica do desktop, altura ~100px)
- Progresso circular menor (60px) posicionado à direita do bloco de texto, dentro do card
- Clique no card → abre modal de detalhe (em vez de expandir inline)

**Modal de detalhe (mobile):**
- Bottom sheet (comportamento atual) com:
  - Cabeçalho: nome + imagem + progresso circular (maior, ~100px)
  - Valores: reservado, meta, contribuição sugerida, prazo
  - Histórico de movimentações (lazy load, como hoje)
  - Botões: "+ Mover" e "- Retirar"
- Substitui o comportamento de expand/collapse inline atual

---

## Dados

**Modelo `Goal` — campo novo:**
```typescript
cover_image_url?: string  // URL da imagem custom ou da biblioteca
```

**Biblioteca de imagens por `goal_type`:**
```typescript
const COVER_IMAGES: Record<string, string> = {
  travel:    "https://images.unsplash.com/...",
  car:       "https://images.unsplash.com/...",
  house:     "https://images.unsplash.com/...",
  emergency: "https://images.unsplash.com/...",
  default:   "https://images.unsplash.com/...",
}
```

**Backend:** campo `cover_image_url` adicionado ao modelo `Goal` e ao endpoint `PUT /goals/:id`.

---

## Componentes

| Componente | Arquivo | Status |
|---|---|---|
| `GoalCard` | `Reservas.tsx` | Reescrever |
| `CircularProgress` | `Reservas.tsx` (novo sub-componente) | Novo |
| `GoalDetailSheet` | `Reservas.tsx` (novo sub-componente) | Novo (substitui expand inline) |
| `AddTransactionModal` | `Reservas.tsx` | Adaptar (Dialog no desktop, Sheet no mobile) |
| Backend `Goal` model | `models/agent.go` + migration | Campo novo |

---

## Verificação

1. Desktop (≥1024px): grid de 3 colunas com imagens, círculo de progresso, modal "Mover"
2. Mobile (<1024px): cards empilhados com imagem + círculo, clique abre bottom sheet de detalhe
3. Goal sem imagem custom: mostra imagem da biblioteca pelo `goal_type`, ou gradiente fallback
4. Upload de imagem: campo visível no formulário de criar/editar goal
5. Animação do círculo ao carregar a página
6. Transações ainda carregam lazy no modal de detalhe
