# DispensaAI

App separado (PWA, mobile-first) do módulo **Dispensa Inteligente**. Usa o **mesmo
backend Go** do FinanceAI (`../backend`), com as rotas `/pantry/*` e a mesma
autenticação JWT.

## Rodar localmente

```bash
# backend (na raiz do repo): docker compose up postgres backend
cd dispensaAI
npm install
npm run dev          # http://localhost:5174
```

`VITE_API_URL` aponta para o backend (`.env.local` → `http://localhost:8080`).
No backend, a origem `http://localhost:5174` precisa estar em `FRONTEND_URL`
(lista separada por vírgula) para o CORS liberar.

Login: mesma conta do FinanceAI (em dev, `user@financeai.com` / senha em `SEED_USER_PASSWORD` do `.env`).

## Testes e build

```bash
npm test             # vitest
npm run lint
npm run build        # dist/
```

## Deploy (Railway)

Novo serviço apontando para a pasta `dispensaAI/` (Dockerfile igual ao do
`frontend/`), com a variável `VITE_API_URL` = URL pública do backend. No serviço
do backend, adicionar a URL pública deste app em `FRONTEND_URL`.

## Estrutura

- `src/services/api.ts` — wrapper de `fetch` com refresh automático (chaves `dai-*` no storage)
- `src/i18n/` — dicionários pt-BR e de; `useT()` lê o idioma das configurações
- `src/store/` — zustand (auth, settings)
- `src/hooks/usePantrySettings.ts` — `GET/PUT /pantry/settings`
- `src/pages/` — Dispensa, Nota, Lista, Receitas, Ajustes, Login
