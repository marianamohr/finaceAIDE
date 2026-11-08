# Admin Page Redesign — FinanceAI

## Contexto

A página de admin atual (`/admin`) é funcional mas excessivamente minimalista: lista de cards sem busca ou filtragem, sem métricas de sistema, e ações destrutivas (Suspender) sem confirmação. Com crescimento previsto para 50–500 usuários, a interface precisa de uma tabela densa com busca/filtros, um painel de status do sistema com métricas da IA, e proteções contra ações acidentais.

---

## Paleta e tipografia

Usar exclusivamente as CSS variables do sistema já definidas em `frontend/src/index.css` (dark mode, classe `.dark`):

| Token | Valor HSL | Uso |
|---|---|---|
| `--background` | `220 20% 7%` | Fundo da página |
| `--card` | `220 18% 10%` | Superfícies de cards e tabela |
| `--secondary` | `220 14% 16%` | Thead, toolbar, paginação |
| `--border` | `220 14% 18%` | Bordas e separadores |
| `--foreground` | `210 20% 95%` | Texto principal |
| `--muted-foreground` | `215 12% 55%` | Labels, IDs, texto secundário |
| `--primary` | `160 84% 45%` | Acento principal, filtros ativos, toggle on |
| `--destructive` | `0 62% 45%` | Botão Suspender, badge Suspenso |
| `--success` | `142 71% 45%` | Badge Ativo, card taxa de sucesso IA |
| `--warning` | `38 92% 50%` | Badge Pendente, card recibos processados |

**Tipografia:** `font-heading` (Sora) para `h1` e valores de métricas; `font-body` (Manrope) para todo o restante. Border radius padrão: `var(--radius)` = `0.75rem`.

---

## Arquitetura de informação

```
/admin
├── Header
│   ├── Título "Admin" + subtítulo
│   └── Ações: Atualizar (RefreshCw) | Sair (LogOut)
├── Seção: Status do sistema
│   └── 4 MetricCards (Total usuários · Sucesso IA · Recibos 24h · Falhas 24h)
├── Seção: Usuários
│   ├── Toolbar (busca + filtros de status + botão Convidar)
│   ├── Desktop: Tabela com paginação
│   ├── Mobile: Lista de UserCards
│   └── InvitePanel (aparece ao clicar "+ Convidar")
└── Dialog de confirmação (ao clicar "Suspender")
```

---

## Componentes

### 1. MetricCard

Quatro cards em grid horizontal (desktop: `grid-cols-4`; mobile: `grid-cols-2`).

Cada card tem uma faixa colorida de 2px no topo (`border-top`) como acento visual:

| Card | Acento | Cor do valor |
|---|---|---|
| Total de usuários | `--primary` | `--foreground` |
| Taxa de sucesso IA | `--success` | `--success` |
| Recibos processados · 24h | `--warning` | `--warning` |
| Falhas de extração · 24h | `--destructive` | `hsl(0 70% 65%)` |

Os dados de métricas (recibos, falhas, taxa de sucesso) vêm de um novo endpoint `GET /admin/metrics`. Os dados de contagem de usuários são derivados client-side da lista já carregada.

### 2. Toolbar

- **Busca:** filtragem client-side por email e account_id (debounce 200ms)
- **Filtros de status:** pills/botões `Todos | Ativos | Pendentes | Suspensos` — somente um ativo por vez
- **Botão "+ Convidar usuário":** abre o `InvitePanel` inline abaixo da toolbar (substitui o card fixo atual)

### 3. Tabela (desktop, `sm:` breakpoint e acima)

Colunas: **Usuário** | **Cargo** | **Status** | **Tools** | **Ações**

- **Usuário:** email em `font-semibold` + account_id truncado em `font-mono text-xs text-muted-foreground` com botão de copiar
- **Cargo:** pill `Admin` (acento primary) ou `Usuário` (secondary)
- **Status:** badge com dot colorido + label (`Ativo` / `Pendente` / `Suspenso`)
- **Tools:** Switch toggle (Radix) — visível apenas para não-admins; dash `—` para admins
- **Ações:** botões contextuais por status (ver abaixo) — dash `—` para admins

Thead com `bg-secondary`, linhas com `hover:bg-card/80`. Paginação na base: 20 itens por página, controles Anterior/Próxima + números de página.

### 4. UserCard (mobile, abaixo de `sm:`)

Card por usuário com:
- Linha 1: email + badge de status (direita)
- Linha 2: pill de cargo + toggle de tools (direita, se não-admin)
- Linha 3: botões de ação

### 5. Ações contextuais

| Status do usuário | Ações disponíveis |
|---|---|
| `active` (não-admin) | Suspender · Vincular conta |
| `pending` | Aprovar · Vincular conta |
| `suspended` | Reativar · Vincular conta |
| `active` (admin) | — |

**Suspender:** ao clicar, abre um `AlertDialog` (Radix) com título "Suspender usuário?", descrição com email do usuário em negrito, botões "Cancelar" (ghost) e "Suspender" (destructive). Só executa `api.updateUserStatus` após confirmação.

**Vincular conta:** mantém o comportamento atual (modo link com highlight nas outras linhas/cards).

### 6. InvitePanel

Aparece/desaparece com estado local `showInvite`. Exibe o campo de email e botão "Gerar link". Após gerar, mostra o link com botão de copiar. Fecha ao clicar fora ou em "✕".

---

## Novo endpoint de métricas

```
GET /admin/metrics
Authorization: cookie de sessão (já existente)

Response:
{
  "receipts_24h": 1847,
  "extraction_success_rate": 0.982,
  "extraction_failures_24h": 33
}
```

O frontend chama este endpoint em paralelo com `GET /admin/users` no `useEffect` inicial. Falha silenciosa: se o endpoint retornar erro, os cards de métricas exibem `—` no lugar do valor.

---

## Responsividade

| Breakpoint | Layout da lista | Métricas | Filtros |
|---|---|---|---|
| `< 640px` (mobile) | UserCards empilhados | `grid-cols-2` | Chips horizontais com scroll |
| `≥ 640px` (desktop) | Tabela com paginação | `grid-cols-4` | Pills horizontais inline |

Implementar com as classes Tailwind `hidden sm:block` / `sm:hidden` nos dois layouts de lista.

---

## Organização do código

O `Admin.tsx` atual tem ~295 linhas. Com o redesign, extrair:

- `components/admin/MetricCard.tsx` — card de métrica individual
- `components/admin/UserTable.tsx` — tabela desktop com paginação
- `components/admin/UserCard.tsx` — card mobile por usuário
- `components/admin/InvitePanel.tsx` — painel de convite
- `components/admin/SuspendDialog.tsx` — AlertDialog de confirmação

`Admin.tsx` mantém apenas o estado global, fetch dos dados e composição dos componentes filhos.

---

## Verificação (como testar)

1. `npm run dev` no frontend, acessar `localhost:5173/admin` logado como admin
2. Confirmar que os 4 MetricCards aparecem com valores reais (ou `—` se o endpoint ainda não existir)
3. Digitar na busca e confirmar filtro client-side por email
4. Clicar nos filtros de status e confirmar que a lista filtra corretamente
5. Clicar "Suspender" em um usuário ativo → confirmar que o AlertDialog abre → clicar "Cancelar" → usuário não muda → clicar "Suspender" novamente → confirmar → usuário aparece como Suspenso
6. Em viewport `< 640px` (DevTools), confirmar que a tabela some e os UserCards aparecem
7. Em viewport `≥ 640px`, confirmar tabela com paginação de 20 itens
8. Clicar "+ Convidar", inserir email, gerar link, copiar — confirmar comportamento
9. Confirmar que toggle de Tools funciona e persiste via API
