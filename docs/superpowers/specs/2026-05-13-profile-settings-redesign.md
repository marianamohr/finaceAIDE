# Profile Settings Redesign

**Date:** 2026-05-13  
**Status:** Approved for implementation

## Context

A página de Perfil atual embute o `MerchantAliasManager` inline, mistura itens de configuração rápida com listas longas e não tem agrupamento visual. Em desktop, inputs se esticam horizontalmente. O objetivo é transformar a página num "Menu de Configurações" estilo iOS/Android Settings: hierarquia clara em 3 blocos, funcionalidades complexas movidas para sub-fluxos (modais ou rotas dedicadas).

---

## Estrutura Final da Página

### Layout — Mobile-First

O app é mobile-first. Todas as features funcionam identicamente em mobile e desktop. Não há grid de colunas nem conteúdo exclusivo por breakpoint.

**Mobile (padrão):** lista em coluna única, full-width, padding lateral `px-4`.

**Desktop (≥768px):** o container recebe `max-w-2xl mx-auto w-full` para centralizar o conteúdo e evitar que inputs se estiquem horizontalmente. Nenhuma feature é removida ou alterada.

**Modais em mobile:** `DialogContent` do shadcn sem restrição de largura abaixo de `sm` — ocupa quase a tela toda. Acima de `sm`: `sm:max-w-sm` ou `sm:max-w-md` conforme o modal. Isso é o comportamento padrão do componente Dialog do shadcn.

### Bloco 1 — Conta

- Item clicável: Avatar inicial + email + "Conta pessoal"
- Ao clicar: abre **Modal Editar Perfil**

### Bloco 2 — Preferências

- `CurrencyToggle` (mantém comportamento atual)
- Reservas toggle inline (mantém comportamento atual)
- Categorias ativas → label de estado ("Modo padrão" / "Alimentação detalhado") + ao clicar abre **Modal Categorias**

### Bloco 3 — Ferramentas e Segurança

- Dicionário de Mercados → ChevronRight → navega `/profile/merchant-alias`
- Chave API → label de estado ("Configurada" / "Não configurada") + ao clicar abre **Modal Chave API**
- Face ID (mantém comportamento atual)
- Notificações (estático, mantém)
- Segurança → substituído por botão "Alterar senha" que abre **Modal Alterar Senha** diretamente

### Rodapé

- Botão "Sair da conta" (mantém)

---

## Modais

### Modal: Editar Perfil

**Trigger:** clique no bloco Conta  
**Componente:** `Dialog` (shadcn/Radix), `sm:max-w-sm`

Conteúdo:
- Email (read-only, `<p>` com label)
- Tipo de conta: "Conta pessoal" (read-only)
- Botão secundário no footer: "Alterar senha →"
  - Fecha este modal e abre Modal Alterar Senha

Footer:
- Botão "Fechar"
- Botão "Alterar senha →" (variant outline)

### Modal: Alterar Senha

**Trigger:** botão "Alterar senha →" no Modal Editar Perfil  
**Componente:** `Dialog` (shadcn), `sm:max-w-sm`  
**Fluxo:** Modal Editar Perfil fecha → Modal Alterar Senha abre (sequencial, nunca empilhados)

Campos:
- Senha atual (type="password")
- Nova senha (type="password")
- Confirmar nova senha (type="password")

Validações inline (antes de chamar API):
- Nova senha ≠ senha atual
- Confirmar nova senha === nova senha
- Mínimo 8 caracteres na nova senha

Chamada: `POST /user/password` com `{ current_password, new_password }`

Feedback:
- Sucesso: toast + fecha modal
- Erro 401/400 (senha atual incorreta): mensagem inline no campo
- Erro genérico: mensagem inline no footer

### Modal: Categorias Ativas

**Trigger:** item "Categorias ativas" no bloco Preferências  
**Componente:** `Dialog`, `sm:max-w-md`

Conteúdo extraído de `ProfileCategories.tsx`:
- Toggle "Modo alimentação detalhado" (usa `updateContext` de `useSettings`)
- Grid de categorias ativas (fetch `/categories`, filter por tipo e modo)
- Texto "X categorias ativas"

Footer:
- Botão "Fechar"

A rota `/profile/categories` pode ser mantida temporariamente e removida depois.

### Modal: Chave API

**Trigger:** item "Chave API" no bloco Ferramentas  
**Componente:** `Dialog`, `sm:max-w-sm`

Conteúdo:
- Se chave configurada: mostra hash mascarado (`maskKey()`) + botão "Alterar"
- Campo input `type="password"` com toggle de visibilidade (Eye/EyeOff)
- Placeholder: `sk-ant-...`

Estado local migrado de `Profile.tsx`:
- `keyInput`, `keyVisible`, `keyEditing` → ficam no modal
- Ao fechar sem salvar: descarta `keyInput`

Footer:
- Botão "Cancelar"
- Botão "Salvar" (disabled se `!keyInput.trim()`)

---

## Nova Rota: /profile/merchant-alias

**Arquivo:** `frontend/src/pages/ProfileMerchantAlias.tsx`

```tsx
export default function ProfileMerchantAlias() {
  return (
    <div className="h-[var(--app-height)] lg:h-full flex flex-col bg-background">
      <PageHeader title="Dicionário de Mercados" showBack />
      <div className="flex-1 overflow-y-auto px-4 pb-4 pt-4 max-w-2xl mx-auto w-full">
        <MerchantAliasManager />
      </div>
    </div>
  );
}
```

Registrar em `App.tsx` dentro do grupo protegido/AppShell.

---

## Backend: Endpoint de Troca de Senha

### 1. Repository — novo método

**Arquivo:** `backend/internal/repository/user.go`

```go
func (r *UserRepository) UpdatePassword(id uuid.UUID, newHash string) error
// UPDATE users SET password_hash = $1, updated_at = NOW() WHERE id = $2
```

### 2. Handler

**Arquivo:** `backend/internal/handlers/user.go`  
**Método:** `(h *UserHandler) ChangePassword(w http.ResponseWriter, r *http.Request)`

```
POST /user/password  (authenticated)
Body: { "current_password": string, "new_password": string }
```

Fluxo:
1. `userID := auth.UserIDFromContext(r.Context())`
2. Busca user por ID (precisa do `PasswordHash`)
3. `auth.CheckPassword(user.PasswordHash, req.CurrentPassword)` → 400 se inválido
4. Valida `new_password` ≥ 8 chars, ≠ `current_password`
5. `auth.HashPassword(req.NewPassword)`
6. `repo.UpdatePassword(userID, newHash)`
7. Retorna `204 No Content`

Erros:
- `400` senha atual incorreta: `{ "error": "senha atual incorreta" }`
- `400` nova senha inválida: `{ "error": "nova senha deve ter no mínimo 8 caracteres" }`
- `500` falha interna

### 3. Rota

**Arquivo:** `backend/cmd/server/main.go`  
No grupo autenticado (onde ficam `/user/me` e `/user/settings`):

```go
r.Post("/user/password", userHandler.ChangePassword)
```

### 4. Repository: FindByID precisa retornar PasswordHash

Verificar se `FindByID` retorna o campo `password_hash`. Se a query atual usa JOIN com settings e não seleciona `password_hash`, adicionar ao SELECT. Alternativa: criar `FindByIDForAuth(id) (*models.User, error)` que retorna apenas `id + password_hash`.

---

## Arquivos Críticos

| Arquivo | Ação |
|---|---|
| `frontend/src/pages/Profile.tsx` | Reescrever estrutura completa |
| `frontend/src/pages/ProfileMerchantAlias.tsx` | Criar (novo) |
| `frontend/src/App.tsx` | Adicionar rota `/profile/merchant-alias` |
| `backend/internal/repository/user.go` | Adicionar `UpdatePassword()` |
| `backend/internal/handlers/user.go` | Adicionar `ChangePassword()` |
| `backend/cmd/server/main.go` | Registrar `POST /user/password` |

---

## Verificação

1. **Backend:**
   - `POST /user/password` com senha correta → 204
   - `POST /user/password` com senha errada → 400 com mensagem em português
   - Rota sem token → 401

2. **Frontend — Profile:**
   - Página carrega sem o `MerchantAliasManager` inline
   - 3 seções visíveis com títulos de grupo
   - Em desktop (≥768px): conteúdo centralizado, não esticado

3. **Modal Categorias:**
   - Abre ao clicar no item
   - Toggle de modo funciona e persiste
   - Fecha sem perder estado

4. **Modal Chave API:**
   - Mostra estado mascarado se chave já existe
   - Salva e fecha; item de menu atualiza label
   - Fechar sem salvar não altera nada

5. **Modal Editar Perfil + Alterar Senha:**
   - Fluxo: Perfil abre → clica "Alterar senha" → Perfil fecha → Senha abre
   - Validações inline funcionam antes do submit
   - Senha atual incorreta mostra erro no campo, não fecha o modal

6. **Rota Dicionário:**
   - Clicar no item navega para `/profile/merchant-alias`
   - Back button volta para `/profile`
   - `MerchantAliasManager` funciona normalmente
