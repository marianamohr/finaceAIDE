# Manual Interativo do Usuário

**Data:** 2026-05-25
**Status:** Aprovado
**Funcionalidades cobertas:** Chat, Dashboard, Consumo, Caixa, Formas de Pagamento, Limites, Reservas, Gastos Fixos (7 seções)

---

## Visão geral

Manual didático com humor leve embutido no produto, com duas superfícies de entrega:

1. **Modal de boas-vindas** — exibido uma única vez no primeiro acesso após login/registro
2. **Seção expansível no Perfil** — sempre disponível no final da página de perfil

O conteúdo é o mesmo componente React reutilizado nas duas superfícies via prop `mode`.

---

## Comportamento: Primeiro Login

**Gatilho:** ao montar `/chat`, verificar `settings.onboardingSeen` (vem do backend via `useSettings`).

**Se `false`:** exibir o `<UserManualModal />`.

**Ao fechar o modal (qualquer botão):** chamar `updateSetting({ onboardingSeen: true })`.

**Botão "Ver manual completo":** fecha o modal, navega para `/profile` e faz scroll até a seção `#manual`.

**Não usa `localStorage`** — persiste no backend para funcionar em qualquer dispositivo.

---

## Componente: `UserManual`

```
src/components/UserManual/
  UserManual.tsx        — conteúdo completo do manual
  UserManualModal.tsx   — wrapper modal (mode="modal")
```

**Props de `UserManual`:**
- `mode: "modal" | "section"` — controla padding, botões e comportamento de fechamento

**Reutilização:**
- `UserManualModal` renderiza `<UserManual mode="modal" />` dentro de um `<Dialog>`
- A seção do perfil renderiza `<UserManual mode="section" />` diretamente

---

## Backend: campo `onboarding_seen`

### Model (`user_settings.go`)

Adicionar campo:
```go
OnboardingSeen bool `gorm:"not null;default:false" json:"onboarding_seen"`
```

### Migration (`migrate.go` → `addNullableColumns`)

Adicionar na lista de statements:
```sql
ALTER TABLE user_settings ADD COLUMN IF NOT EXISTS onboarding_seen bool NOT NULL DEFAULT false
```

Segue o padrão já usado por todos os outros campos adicionados à `user_settings` após a criação inicial da tabela (`show_goals`, `show_limits_categories`, etc.).

### Handler/Frontend

O campo `onboarding_seen` já é exposto pelo endpoint existente `GET /account/settings` e atualizado via `PATCH /account/settings`. Nenhum endpoint novo necessário.

No `useSettings` hook, adicionar:
- `onboardingSeen: boolean` no estado
- incluir no `updateSetting` — já suportado genericamente

---

## Backend: Débito padrão no registro

**Onde:** handler de `/auth/register` (e fluxo de convite, se aplicável), após criar o usuário e a conta.

**O que criar:**
```go
PaymentMethod{
  AccountID: account.ID,
  Name:      "Débito",
  Kind:      KindDebit,
  Color:     "#6366f1",
  Active:    true,
}
```

**Idempotência:** verificar se já existe ao menos um `PaymentMethod` para a conta antes de criar. Não duplicar em re-execuções.

**Exibição no modal:** o texto menciona explicitamente que o Débito já foi criado — o usuário não precisa fazer nada para começar a registrar gastos.

---

## Conteúdo do Manual

### Bloco "Por onde começar" (destacado em amarelo)

Ordem de setup recomendada:

1. **Chave API** (Perfil → Chave API) — necessária para ativar o Chat com IA. Sem ela, o chat não funciona. Veja o passo a passo abaixo.
2. **Formas de Pagamento** (Perfil → Formas de Pagamento) — adicione cartões, vales e outros. Um *Débito* já foi criado automaticamente.
3. **Gastos Fixos** (Perfil → Gastos Fixos) — aluguel, streaming, mensalidades recorrentes.
4. **Limites** (Menu → Limites) — tetos de gasto por categoria.
5. **Reservas** (Menu → Reservas) — metas de poupança.

### Funcionalidades (8 seções)

| Ícone | Nome | Descrição com humor |
|---|---|---|
| 💬 | Chat financeiro | Registre gastos em linguagem natural. Mencionar a forma de pagamento direciona o gasto ao lugar certo — cartão vai pra fatura, vale sai do saldo do vale. |
| 📊 | Dashboard | Gráficos por categoria, comparação entre meses, evolução temporal. Prepare o coração antes de abrir. |
| 🧾 | Consumo | Lista de todas as transações. Edite, delete ou corrija qualquer lançamento — inclusive a forma de pagamento esquecida. |
| 💰 | Caixa | Saldo em conta, faturas de cartão e dois recursos especiais: transferência de saldo do mês anterior e confirmação de depósito de vale mensal. |
| 💳 | Formas de Pagamento | Cartão de crédito (fatura), Vale VR/VA (saldo separado), Débito/PIX (saldo em conta), Dinheiro. |
| 🎯 | Limites | Tetos por categoria. Amarelo em 70%, vermelho ao estourar. |
| 🐷 | Reservas | Metas de poupança. Aportes, rendimentos e saques também funcionam pelo chat. |
| 📅 | Gastos Fixos | Contas recorrentes — em dois sabores. Veja a seção abaixo. |

### Formas de Pagamento — comportamentos especiais

| Tipo | Campos obrigatórios | Comportamento |
|---|---|---|
| Cartão de crédito | Dia de fechamento, dia de vencimento | Agrupa compras em faturas mensais. Aparece no Caixa para pagar. |
| Vale (VR/VA) | Nenhum no cadastro | No início de cada mês, Caixa pergunta o valor do depósito recebido. Gastos saem do saldo do vale, não do saldo em conta. |
| Débito | Nenhum | Débita direto do saldo em conta. |
| PIX | Nenhum | Igual ao débito. |
| Dinheiro | Nenhum | Saldo de espécie isolado. |

### Frases que o Chat entende

**Gastos simples:**
- "Gastei 45 reais no iFood"
- "Paguei 80 de Uber hoje"

**Com forma de pagamento (recomendado):**
- "Comprei 120 no Pão de Açúcar no Nubank"
- "Almocei 35 no VR"
- "Paguei academia 90 no débito"

**Receitas:**
- "Recebi meu salário de 4000"
- "Entrou 500 de freelance"

**Consultas:**
- "Quanto gastei em alimentação esse mês?"
- "Qual meu saldo atual?"
- "Estou dentro do meu limite de lazer?"

**Reservas:**
- "Depositar 200 na reserva de viagem"
- "Sacar 100 da reserva emergência"

> Se a forma de pagamento não for mencionada, o gasto fica com `payment_method_id = null` e é tratado como débito genérico. Pode ser corrigido depois na tela Consumo.

### Chave API — passo a passo para gerar

O manual deve incluir instruções dentro da seção "Chave API":

1. Acesse **console.anthropic.com** e crie uma conta (ou faça login)
2. No menu lateral, clique em **"API Keys"**
3. Clique em **"Create Key"**, dê um nome (ex: "Finanças") e confirme
4. Copie a chave gerada — ela começa com `sk-ant-...`
5. Cole em **Perfil → Chave API** no app e salve

> A chave é exibida apenas uma vez na Anthropic. Se perder, gere uma nova.

---

### Gastos Fixos vs. Assinaturas de Cartão

São dois tipos dentro da mesma tela — a diferença é quem cobra:

| | Gasto Fixo | Assinatura de Cartão |
|---|---|---|
| **Exemplos** | Aluguel, internet, academia, água | Netflix, Spotify, iCloud, antivírus |
| **Forma de pagamento** | Débito, PIX, boleto | Cartão de crédito (obrigatório) |
| **Campo extra** | Dia de vencimento (opcional) | Qual cartão de crédito |
| **Como funciona** | Aparece como lembrete mensal no Caixa — você registra o pagamento manualmente | Entra automaticamente na fatura do cartão todo mês — sem precisar fazer nada |

**Regra prática:** se você paga, é gasto fixo. Se o cartão cobra, é assinatura.

Se cadastrar o Netflix como gasto fixo em vez de assinatura, ele vai gerar um lembrete mas não vai entrar na fatura automaticamente — você acabaria registrando duas vezes.

---

### Reservas e o fluxo de caixa

Todo aporte em uma reserva é uma **saída do seu saldo em conta**. O sistema registra internamente como uma transferência — o dinheiro sai do caixa e vai para a meta.

Isso significa:
- Se você aportar R$ 500 na reserva de viagem, seu saldo em conta cai R$ 500
- Se o saldo em conta não cobrir o aporte, o caixa vai ficar negativo

**Antes de fazer um aporte grande, confira o saldo no Caixa.** Saques funcionam no sentido inverso — tiram da reserva e devolvem ao caixa.

---

### Transferência de saldo entre meses

Quando o saldo do mês anterior for positivo, o Caixa exibe um banner no início do mês seguinte: *"Sobrou R$ X no mês anterior. Trazer para este mês?"*. Confirmar cria uma transação de receita no mês atual. Ignorar dispensa o banner por aquele mês.

---

## Seção no Perfil

- Adicionada após o botão "Sair da conta" (final da página)
- Header: `COMO USAR O SISTEMA`
- Card expansível com ícone 📖, título "Manual do sistema", subtítulo com contagem de funcionalidades
- Conteúdo: bloco amarelo "Por onde começar" + 8 seções de funcionalidades
- Âncora `id="manual"` para suportar scroll direto via navegação do modal

---

## Responsividade

O app é mobile-first. O manual deve seguir os mesmos padrões já usados no projeto:

- **Modal:** usa `<Dialog>` do shadcn/ui com `sm:max-w-sm` — já responsivo por padrão. Em mobile ocupa a tela toda (bottom sheet behavior); em desktop centraliza.
- **Seção do Perfil:** segue o padrão da página `Profile.tsx` — `max-w-2xl mx-auto w-full`, padding `px-4`. Funciona em ambos os tamanhos sem ajuste extra.
- **Conteúdo interno:** texto com `text-sm`/`text-xs`, sem colunas side-by-side que quebrem em telas pequenas. O bloco "Por onde começar" e as 8 seções são empilhados verticalmente.
- **Nenhum breakpoint custom** necessário — o layout do projeto já lida com isso via `lg:h-full` e `flex-col`.

---

## Fora do escopo

- Tour passo-a-passo com "Próximo →" (pode ser adicionado futuramente)
- Progresso estilo conquistas (requer estado persistente adicional)
- Vídeos ou GIFs explicativos
