# Spec: Chave API Anthropic por Usuário

## Context

O sistema atualmente usa uma única `ANTHROPIC_API_KEY` global (env var) para todas as chamadas de IA. A coluna `user_api_key` já existe na tabela `user_settings` e já é persistida via `PATCH /user/settings`, mas nunca é utilizada nas chamadas à Anthropic. O objetivo é permitir que cada usuário configure sua própria chave, que será usada no chat em seu lugar. Sem a chave configurada, o chat fica bloqueado com uma mensagem de orientação.

## Escopo

- **Backend**: mascarar a chave no `GET /user/me`; aplicar a chave do usuário nas chamadas do `ChatHandler`
- **Frontend (Perfil)**: campo para inserir/alterar a chave, mascarado após salvar
- **Frontend (Chat)**: estado de bloqueio quando a chave não está configurada

- **`InsightWorker`**: desabilitado (não será mais iniciado)
- **`VisionExtractor`**: também usa a chave do usuário (mesmo comportamento do ChatHandler)

---

## Backend

### 1. Mascaramento em `GET /user/me`

`UserHandler.Me` retorna atualmente `user_api_key` em plain text. Deve retornar:
- `""` se não configurada
- `"sk-ant-****...****"` (mantendo os primeiros 10 e últimos 4 caracteres, restante substituído por `****`) se configurada

Isso é suficiente para o frontend distinguir "configurada / não configurada" sem expor o segredo.

**Arquivo:** `backend/internal/handlers/user.go`

### 2. Chave por chamada no `AnthropicProvider`

Adicionar método `WithUserKey(key string) *AnthropicProvider` que retorna uma cópia shallow do provider com a `apiKey` substituída. Não altera o provider original (imutabilidade).

```go
func (p *AnthropicProvider) WithUserKey(key string) *AnthropicProvider {
    clone := *p
    clone.apiKey = key
    return &clone
}
```

**Arquivo:** `backend/internal/services/ai/anthropic.go`

### 3. Aplicar chave do usuário no `ChatHandler`

No método `Handle` do `ChatHandler`, após buscar o usuário (`userRepo.FindByID`), verificar se `user.UserAPIKey != ""`. Se sim, usar `h.aiProvider.(*ai.AnthropicProvider).WithUserKey(user.UserAPIKey)` para as chamadas da requisição. Caso contrário, usar `h.aiProvider` normalmente.

Para evitar type assertion quebrável, o `Provider` interface deve expor `WithUserKey` — ou alternativamente, o `ChatHandler` recebe o `*AnthropicProvider` concreto. A abordagem mais simples é adicionar `WithUserKey` à interface `Provider` como método opcional via type assertion no handler.

**Arquivo:** `backend/internal/handlers/chat.go`

### 4. Aplicar chave do usuário no `UploadHandler` / `VisionExtractor`

O `UploadHandler` recebe o `VisionExtractor` no construtor. O `VisionExtractor` usa o `aiProvider` internamente. Mesma lógica do ChatHandler: após buscar o usuário, se `user.UserAPIKey != ""`, criar um extractor com a chave substituída para a requisição.

Adicionar método `WithUserKey(key string) *Extractor` ao `VisionExtractor`, similar ao do `AnthropicProvider`.

**Arquivos:** `backend/internal/services/vision/ocr.go`, `backend/internal/handlers/upload.go`

### 5. Desabilitar `InsightWorker`

Remover a criação e o `.Start()` do `InsightWorker` em `main.go`. O worker não tem contexto de usuário individual, então não tem como usar a chave por usuário de forma segura.

**Arquivo:** `backend/cmd/server/main.go`

---

## Frontend — Perfil

### Campo "Chave API" em `Profile.tsx`

Novo card inserido no bloco de settings, com:

- **Estado "não configurada"**: campo de input do tipo `password` vazio + botão "Salvar"
- **Estado "configurada"**: exibe o valor mascarado retornado pelo backend (ex: `sk-ant-****...****`) em read-only, sem opção de revelar (o frontend nunca recebe a chave completa) + botão "Alterar" que limpa o campo para edição
- Ícone de olho para revelar/esconder o texto **somente enquanto o usuário está digitando** (antes de salvar)
- Ao salvar: chama `updateSetting({ userApiKey: value })`, mostra feedback inline ("Salvo" por 2s)
- Ao alterar: limpa o campo local, foca o input

**Arquivo:** `frontend/src/pages/Profile.tsx`

O `useSettings` já expõe `userApiKey` e `updateSetting` — nenhuma alteração necessária no hook.

---

## Frontend — Chat

### Bloqueio quando sem chave

Em `Chat.tsx`, importar `useSettings`. Se `userApiKey === ""` e `loading === false`:

- Substituir `<MessageList>` e `<ChatInput>` por um estado de bloqueio centralizado:
  - Ícone (ex: `KeyRound` do lucide)
  - Título: "Configure sua chave API"
  - Descrição: "Para usar o assistente, você precisa adicionar sua chave da Anthropic no perfil."
  - Botão: "Ir para o Perfil" → navega para `/profile`

Enquanto `loading === true`, não bloquear (evitar flash).

**Arquivo:** `frontend/src/pages/Chat.tsx`

---

## Fluxo de dados

```
Usuário salva chave no Perfil
  → PATCH /user/settings { user_api_key: "sk-ant-..." }
  → Backend persiste em user_settings.user_api_key
  → GET /user/me retorna user_api_key mascarada ("sk-ant-****...****")
  → useSettings atualiza userApiKey no estado local

Usuário envia mensagem no Chat
  → POST /chat
  → ChatHandler busca user.UserAPIKey do banco
  → Se não vazio: usa AnthropicProvider.WithUserKey(userAPIKey)
  → Se vazio: usa provider padrão (chave do sistema)
```

---

## Verificação

1. **Sem chave**: acessar `/chat` sem chave configurada → ver tela de bloqueio com botão para o perfil
2. **Configurar chave**: ir ao perfil, inserir chave válida, salvar → campo mostra valor mascarado
3. **Chat funcional**: com chave configurada, enviar mensagem → resposta normal da IA
4. **Alterar chave**: clicar "Alterar" no perfil → campo limpa para nova entrada
5. **Mascaramento no GET**: verificar que `GET /user/me` não retorna a chave completa após salvar
6. **Upload de recibo**: enviar foto de recibo no chat → VisionExtractor usa a chave do usuário
7. **InsightWorker desabilitado**: verificar que o worker não é iniciado no boot do servidor
