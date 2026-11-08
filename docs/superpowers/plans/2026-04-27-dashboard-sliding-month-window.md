# Dashboard — Janela Deslizante de Meses — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Substituir a barra de pills estática por uma janela deslizante de 5 meses centrada no mês selecionado, com expansão dinâmica para o passado e para o futuro (quando há transações).

**Architecture:** O estado central passa a ser um offset inteiro relativo ao mês atual (0 = hoje). `selectedYear` e `selectedMonth` são derivados desse offset. A janela sempre exibe 5 pills (`offset-2` a `offset+2`). Um hook de prefetch verifica silenciosamente se o mês seguinte ao limite futuro tem dados, controlando se o pill direito extremo é clicável.

**Tech Stack:** React, TypeScript, Tailwind CSS, shadcn/ui — sem dependências novas.

---

## File Map

| Arquivo | Ação | Responsabilidade |
|---|---|---|
| `frontend/src/hooks/useDashboard.ts` | Modificar | Adicionar export `usePrefetchNextMonth` |
| `frontend/src/pages/Dashboard.tsx` | Modificar | Trocar estado de mês por offset, nova barra de pills |

---

### Task 1: Adicionar `usePrefetchNextMonth` ao hook

**Files:**
- Modify: `frontend/src/hooks/useDashboard.ts`

- [ ] **Step 1: Adicionar o hook ao final do arquivo**

Abrir [frontend/src/hooks/useDashboard.ts](frontend/src/hooks/useDashboard.ts) e adicionar após a função `useDashboard`:

```typescript
export function usePrefetchNextMonth(year: number, month: number): boolean {
  const [hasData, setHasData] = useState(false);

  useEffect(() => {
    setHasData(false);
    let cancelled = false;
    const next = new Date(year, month, 1); // month é 1-based, Date usa 0-based internamente
    const nextYear = next.getFullYear();
    const nextMonth = next.getMonth() + 1;

    api
      .get<DashboardSummary>(`/transactions/summary?year=${nextYear}&month=${nextMonth}`)
      .then((summary) => {
        if (!cancelled) {
          setHasData(summary.income !== 0 || summary.expenses !== 0);
        }
      })
      .catch(() => {
        if (!cancelled) setHasData(false);
      });

    return () => {
      cancelled = true;
    };
  }, [year, month]);

  return hasData;
}
```

> Nota: `new Date(year, month, 1)` com `month` em 1-based: passar `month` (ex: 12) faz o Date avançar automaticamente para janeiro do ano seguinte, lidando com a virada de ano corretamente.

- [ ] **Step 2: Verificar que o arquivo compila sem erros**

```bash
cd frontend
npx tsc --noEmit
```

Expected: sem erros de tipo.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/hooks/useDashboard.ts
git commit -m "feat(dashboard): add usePrefetchNextMonth hook"
```

---

### Task 2: Utilitário `offsetToYearMonth`

**Files:**
- Modify: `frontend/src/pages/Dashboard.tsx`

- [ ] **Step 1: Substituir `generateMonths` / `MONTHS` pelo utilitário de offset**

No topo de [frontend/src/pages/Dashboard.tsx](frontend/src/pages/Dashboard.tsx), remover:

```typescript
function generateMonths(count = 12) { ... }
const MONTHS = generateMonths(12);
```

E adicionar no lugar:

```typescript
function offsetToYearMonth(offset: number): { year: number; month: number; label: string } {
  const now = new Date();
  const d = new Date(now.getFullYear(), now.getMonth() + offset, 1);
  return {
    year: d.getFullYear(),
    month: d.getMonth() + 1,
    label: d.toLocaleDateString("pt-BR", { month: "short", year: "2-digit" }),
  };
}
```

- [ ] **Step 2: Verificar que não há referências a `MONTHS` ou `generateMonths` restantes**

```bash
grep -n "MONTHS\|generateMonths" frontend/src/pages/Dashboard.tsx
```

Expected: nenhuma linha retornada.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/pages/Dashboard.tsx
git commit -m "feat(dashboard): replace static MONTHS with offsetToYearMonth utility"
```

---

### Task 3: Trocar estado de mês por offset no componente

**Files:**
- Modify: `frontend/src/pages/Dashboard.tsx`

- [ ] **Step 1: Substituir estado `selectedYear/selectedMonth` por offset**

Localizar e remover:
```typescript
const [selectedYear, setSelectedYear] = useState(now.getFullYear());
const [selectedMonth, setSelectedMonth] = useState(now.getMonth() + 1);
```

Adicionar no lugar:
```typescript
const [selectedOffset, setSelectedOffset] = useState(0);
const [minOffset, setMinOffset] = useState(-12);
const [maxOffset, setMaxOffset] = useState(2);

const { year: selectedYear, month: selectedMonth } = offsetToYearMonth(selectedOffset);
```

- [ ] **Step 2: Atualizar a chamada de `useDashboard`**

A linha:
```typescript
const { summary, transactions, loading, deleteTransaction, updateTransaction } = useDashboard(selectedYear, selectedMonth);
```

Não muda — `selectedYear` e `selectedMonth` agora são derivados do offset, então a chamada continua igual.

- [ ] **Step 3: Atualizar o `useEffect` de reset de filtros**

Localizar:
```typescript
useEffect(() => {
  setSelectedCategories([]);
  setShowAllCategories(false);
}, [selectedYear, selectedMonth]);
```

Substituir por:
```typescript
useEffect(() => {
  setSelectedCategories([]);
  setShowAllCategories(false);
}, [selectedOffset]);
```

- [ ] **Step 4: Atualizar o `useEffect` de reset de paginação**

Localizar:
```typescript
useEffect(() => {
  setCurrentPage(1);
}, [selectedCategories, selectedYear, selectedMonth]);
```

Substituir por:
```typescript
useEffect(() => {
  setCurrentPage(1);
}, [selectedCategories, selectedOffset]);
```

- [ ] **Step 5: Remover o `useRef` e `useEffect` de scroll automático**

Remover:
```typescript
const scrollRef = useRef<HTMLDivElement>(null);
```

E remover o useEffect:
```typescript
useEffect(() => {
  if (scrollRef.current) {
    scrollRef.current.scrollLeft = scrollRef.current.scrollWidth;
  }
}, []);
```

Remover também o import de `useRef` se não usado em mais nenhum lugar.

- [ ] **Step 6: Verificar que compila sem erros**

```bash
cd frontend
npx tsc --noEmit
```

Expected: sem erros de tipo.

- [ ] **Step 7: Commit**

```bash
git add frontend/src/pages/Dashboard.tsx
git commit -m "feat(dashboard): replace selectedYear/selectedMonth state with offset"
```

---

### Task 4: Integrar prefetch e construir nova barra de pills

**Files:**
- Modify: `frontend/src/pages/Dashboard.tsx`

- [ ] **Step 1: Importar `usePrefetchNextMonth`**

Localizar a linha de import de `useDashboard`:
```typescript
import { useDashboard, Transaction } from "@/hooks/useDashboard";
```

Substituir por:
```typescript
import { useDashboard, usePrefetchNextMonth, Transaction } from "@/hooks/useDashboard";
```

- [ ] **Step 2: Chamar o hook de prefetch**

Logo após a chamada de `useDashboard`, adicionar:

```typescript
const { year: maxYear, month: maxMonth } = offsetToYearMonth(maxOffset);
const hasNextMonth = usePrefetchNextMonth(maxYear, maxMonth);
```

- [ ] **Step 3: Calcular os 5 offsets da janela visível**

Adicionar antes do return:

```typescript
const windowOffsets = [
  selectedOffset - 2,
  selectedOffset - 1,
  selectedOffset,
  selectedOffset + 1,
  selectedOffset + 2,
];
```

- [ ] **Step 4: Substituir a barra de pills no JSX**

Localizar o bloco:
```tsx
<div
  ref={scrollRef}
  className="flex gap-2 px-4 pb-2 overflow-x-auto no-scrollbar"
>
  {MONTHS.map(({ year, month, label }) => {
    const active = year === selectedYear && month === selectedMonth;
    return (
      <button
        key={`${year}-${month}`}
        onClick={() => { setSelectedYear(year); setSelectedMonth(month); }}
        className={cn(
          "shrink-0 px-3 py-1 rounded-full text-xs font-medium transition-colors",
          active
            ? "bg-primary text-primary-foreground"
            : "bg-secondary text-secondary-foreground hover:bg-secondary/80"
        )}
      >
        {label}
      </button>
    );
  })}
</div>
```

Substituir por:

```tsx
<div className="flex gap-1.5 px-4 pb-2">
  {windowOffsets.map((offset) => {
    const { year, month, label } = offsetToYearMonth(offset);
    const isSelected = offset === selectedOffset;
    const isToday = offset === 0;
    const isLeftEdge = offset === windowOffsets[0];
    const isRightEdge = offset === windowOffsets[4];
    const isDisabled = isRightEdge && offset > maxOffset && !hasNextMonth;

    const handleClick = () => {
      if (isDisabled) return;
      if (isLeftEdge && offset <= minOffset) {
        setMinOffset((prev) => prev - 1);
      }
      if (isRightEdge && offset > maxOffset) {
        setMaxOffset((prev) => prev + 1);
      }
      setSelectedOffset(offset);
    };

    return (
      <button
        key={`${year}-${month}`}
        onClick={handleClick}
        disabled={isDisabled}
        className={cn(
          "flex-1 flex flex-col items-center py-1 rounded-full text-xs font-medium transition-colors",
          isSelected
            ? "bg-primary text-primary-foreground"
            : "bg-secondary text-secondary-foreground hover:bg-secondary/80",
          isDisabled && "opacity-40 pointer-events-none"
        )}
      >
        <span>{label}</span>
        {isToday && !isSelected && (
          <span className="w-1 h-1 rounded-full bg-primary mt-0.5" />
        )}
      </button>
    );
  })}
</div>
```

- [ ] **Step 5: Verificar que compila sem erros**

```bash
cd frontend
npx tsc --noEmit
```

Expected: sem erros de tipo.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/pages/Dashboard.tsx
git commit -m "feat(dashboard): implement 5-pill sliding window with future month prefetch"
```

---

### Task 5: Teste manual e ajustes visuais

**Files:**
- Modify: `frontend/src/pages/Dashboard.tsx` (se necessário)

- [ ] **Step 1: Subir o servidor de desenvolvimento**

```bash
cd frontend
npm run dev
```

- [ ] **Step 2: Verificar comportamento esperado**

Abrir o dashboard e verificar:

1. **Mês atual está no centro** — o pill do meio deve ser o mês corrente (ex: "abr/26")
2. **Dois meses passados visíveis** à esquerda (ex: "fev/26", "mar/26")
3. **Dois meses futuros visíveis** à direita (ex: "mai/26", "jun/26")
4. **Ponto indicador** aparece abaixo do label quando o mês atual não está selecionado
5. **Clicar em pill esquerdo extremo** desloca a janela para o passado
6. **Pill direito** fica desabilitado (opaco) se não há transações no mês seguinte a `maxOffset`
7. **Ao navegar para um mês com parcelas futuras**, a janela continua deslizando corretamente
8. **Filtros de categoria e página resetam** ao trocar de mês

- [ ] **Step 3: Se houver problema visual, ajustar classes Tailwind**

Exemplos comuns:
- Pills muito estreitos em mobile: aumentar `py-1.5` ou reduzir `text-xs` para `text-[10px]`
- Gap entre pills: ajustar `gap-1.5` para `gap-1` ou `gap-2`
- Ponto indicador fora de posição: ajustar `mt-0.5`

- [ ] **Step 4: Commit final se houve ajustes**

```bash
git add frontend/src/pages/Dashboard.tsx
git commit -m "fix(dashboard): adjust month pill visual spacing"
```

---

## Self-Review

**Spec coverage:**
- ✅ Offset como estado central com `minOffset`/`maxOffset`
- ✅ Janela de 5 pills fixos, mês atual no centro no mount
- ✅ Expansão do passado sempre ao clicar no extremo esquerdo
- ✅ Expansão do futuro só se `hasNextMonth == true`
- ✅ Pill direito desabilitado quando sem dados futuros
- ✅ Ponto indicador no mês atual quando não selecionado
- ✅ Nenhuma mudança no backend
- ✅ Reset de filtros e paginação ao trocar de mês

**Tipos consistentes:**
- `offsetToYearMonth` retorna `{ year, month, label }` — usado uniformemente nas Tasks 2, 3 e 4
- `usePrefetchNextMonth(year, month): boolean` — definido na Task 1, importado e usado na Task 4
- `selectedYear`/`selectedMonth` são derivados em todas as tarefas, nunca setState direto
