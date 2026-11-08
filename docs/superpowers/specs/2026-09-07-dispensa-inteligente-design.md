# Dispensa Inteligente (dispensaAI)

Data: 2026-09-07
Status: Proposto (aguardando aprovação)

## Resumo

Módulo de controle de dispensa (alimentos, limpeza, higiene e itens gerais de casa)
composto por:

- **Backend**: novo grupo de rotas `/pantry/*` no servidor Go existente (`backend/`),
  mesma auth JWT, mesmas tabelas de usuário/conta.
- **Frontend**: app **separado** em `dispensaAI/` (Vite + React + TS + shadcn +
  Tailwind + PWA), deploy próprio no Railway, consumindo o mesmo backend.
- **IA**: Claude com visão lê a foto da nota (BR e DE) e devolve itens em JSON
  validado por schema; Claude gera receitas priorizando o que há na dispensa.

### Decisões fechadas com a usuária (Fase 2)

| # | Decisão |
|---|---------|
| 1 | Só Claude com visão, sem OCR separado. |
| 3 | O catálogo de produtos do usuário vai no prompt para normalização. |
| 4 | Consumo rápido: botões na lista ("−1", "acabou", ajuste); campo de texto livre fica para a Etapa 5. |
| 5 | Compras semanais → intervalo padrão de reposição = 7 dias. |
| 6 | Validade opcional (Etapa 5). |
| 7 | Melhor custo-benefício: `claude-sonnet-5` nas duas tarefas. **Não alterar nada do fluxo financeiro atual (FinanceAI).** |
| 8 | Estender o cliente HTTP atual (sem SDK). |
| 9 | Stack do `dispensaAI/` igual à do `frontend/`. |
| 10 | **Uma dispensa por usuário** (`user_id`), não por conta. |
| 11 | A pessoa escolhe o idioma (pt-BR ou de) da interface e do catálogo. |
| 12 | Seed de aliases alemão → português + normalização pelo Claude. |
| 13 | Migrations seguem o padrão atual (`migrate.go`); dívida técnica anotada. |
| 14 | Endpoint novo `POST /pantry/receipts`, que também gera o lançamento financeiro. |
| 15 | Imagens de `backend/uploads` podem ser usadas como fixtures. Não há amostras de nota alemã. |
| 16 | Restrições alimentares escolhidas a cada pedido de receita. |
| — | Fotos das notas vão para o mesmo bucket S3 (`storage.Storage`). |

---

## 1. Modelo de dados

Todas as tabelas novas têm prefixo `pantry_`, PK `uuid` com `gen_random_uuid()`,
`user_id uuid NOT NULL` indexado (dispensa por usuário) e timestamps.
Quantidades em `numeric(12,3)`, dinheiro em `numeric(12,2)`.

### 1.1 `pantry_settings` (1 por usuário)

| coluna | tipo | obs |
|---|---|---|
| user_id | uuid PK | |
| language | text NOT NULL default 'pt-BR' | `pt-BR` \| `de` |
| country | text NOT NULL default 'BR' | `BR` \| `DE` — país da dispensa atual (disponibilidade de ingredientes nas receitas) |
| currency | text NOT NULL default 'BRL' | moeda dos preços médios |
| shopping_interval_days | int NOT NULL default 7 | usado no mínimo sugerido e na previsão |
| ask_produce_units | bool NOT NULL default true | perguntar unidades do hortifruti |
| updated_at | timestamptz | |

### 1.2 `pantry_products` (catálogo canônico do usuário)

| coluna | tipo | obs |
|---|---|---|
| id | uuid PK | |
| user_id | uuid NOT NULL idx | |
| name_pt | text NOT NULL | nome canônico em português |
| name_de | text NOT NULL default '' | nome canônico em alemão (pode ser vazio) |
| category | text NOT NULL | `food` \| `cleaning` \| `hygiene` \| `general` |
| subcategory | text NOT NULL default '' | ex.: `produce`, `dairy`, `meat`, `grains`, `beverages`, `snacks`, `frozen`, `laundry`, `kitchen`, `bathroom`, `paper`, `oral`, `hair`, `body`, `pet`, `other` |
| default_unit | text NOT NULL | `un` \| `kg` \| `g` \| `l` \| `ml` \| `pct` \| `maco` \| `dz` |
| is_produce | bool NOT NULL default false | fruta, legume, verdura |
| track_by_unit | bool NOT NULL default false | hortifruti controlado por unidade (pergunta de contagem) |
| min_level | numeric(12,3) | nível mínimo; NULL = usar sugestão automática |
| shelf_life_days | int | validade padrão (Etapa 5) |
| created_at / updated_at | timestamptz | |

Índices: `UNIQUE (user_id, lower(name_pt))`; `(user_id, category)`.

### 1.3 `pantry_product_aliases` (nome bruto da nota → produto)

| coluna | tipo | obs |
|---|---|---|
| id | uuid PK | |
| user_id | uuid NOT NULL idx | |
| product_id | uuid NOT NULL FK → pantry_products ON DELETE CASCADE | |
| raw_name | text NOT NULL | normalizado: maiúsculas, sem acento, espaços colapsados (mesma função de `merchant_aliases`) |
| lang | text NOT NULL default '' | `pt` \| `de` \| '' |
| source | text NOT NULL | `user` \| `ai` \| `seed` |
| created_at | timestamptz | |

Índice: `UNIQUE (user_id, raw_name)`.

### 1.4 `pantry_seed_products` (global, somente leitura)

Seed idempotente com ~150 produtos comuns: `name_pt`, `name_de`, `aliases_pt text[]`,
`aliases_de text[]`, `category`, `subcategory`, `default_unit`, `is_produce`,
`shelf_life_days`. Aproveita a lista da branch `feat/cookbook` (50 termos de → pt)
e é usada para: (a) sugerir o canônico antes de chamar a IA, (b) criar o
`pantry_product` do usuário na primeira confirmação.

### 1.5 `pantry_stock` (1 por produto)

| coluna | tipo | obs |
|---|---|---|
| product_id | uuid PK FK → pantry_products ON DELETE CASCADE | |
| user_id | uuid NOT NULL idx | |
| quantity | numeric(12,3) NOT NULL default 0 | nunca negativa (clamp em 0 com movimento de ajuste) |
| unit | text NOT NULL | unidade de controle |
| avg_price | numeric(12,2) | média ponderada das últimas 5 entradas |
| currency | text NOT NULL default 'BRL' | |
| last_purchased_at | timestamptz | |
| updated_at | timestamptz | |

### 1.6 `pantry_movements` (histórico)

| coluna | tipo | obs |
|---|---|---|
| id | uuid PK | |
| user_id | uuid NOT NULL | |
| product_id | uuid NOT NULL FK | |
| type | text NOT NULL | `receipt_in` \| `manual_in` \| `manual_out` \| `recipe_out` \| `adjust` |
| quantity | numeric(12,3) NOT NULL | sinal: entradas +, saídas − |
| unit | text NOT NULL | |
| unit_price | numeric(12,2) | só em entradas |
| currency | text | |
| reference_type / reference_id | text / uuid | `receipt` \| `recipe_log` \| NULL |
| note | text default '' | |
| occurred_at | timestamptz NOT NULL | |
| created_at | timestamptz | |

Índice: `(user_id, product_id, occurred_at DESC)`.

### 1.7 `pantry_receipts` e `pantry_receipt_items`

`pantry_receipts`:

| coluna | tipo | obs |
|---|---|---|
| id | uuid PK | |
| user_id / account_id | uuid NOT NULL | account_id para o lançamento financeiro |
| status | text NOT NULL | `pending_review` \| `confirmed` \| `discarded` |
| image_key | text NOT NULL | chave no bucket; URL via `/image/{key}` |
| country / currency / locale | text | detectados pela IA, editáveis |
| merchant | text default '' | |
| purchased_at | timestamptz | data da nota ou do envio |
| total | numeric(12,2) | |
| access_key | text default '' | chave NFC-e (44 dígitos) quando legível |
| ai_model | text | |
| ai_input_tokens / ai_output_tokens | int | custo observado |
| raw_ai_json | jsonb | resposta bruta validada (auditoria) |
| transaction_ids | jsonb | ids das transações financeiras criadas na confirmação |
| created_at / confirmed_at | timestamptz | |

`pantry_receipt_items`:

| coluna | tipo | obs |
|---|---|---|
| id | uuid PK | |
| receipt_id | uuid NOT NULL FK ON DELETE CASCADE | |
| user_id | uuid NOT NULL | |
| line_no | int NOT NULL | |
| raw_name | text NOT NULL | como está na nota |
| name_pt / name_de | text | canônico sugerido |
| product_id | uuid FK nullable | match encontrado |
| match_source | text | `alias` \| `product` \| `seed` \| `ai` \| `none` |
| category / subcategory | text | |
| quantity | numeric(12,3) NOT NULL | como na nota |
| unit | text NOT NULL | |
| unit_price / total_price | numeric(12,2) | |
| is_produce | bool NOT NULL | |
| needs_unit_count | bool NOT NULL | hortifruti por peso → precisa de contagem |
| unit_count | numeric(12,3) | resposta da usuária |
| keep_weight | bool NOT NULL default false | usuária optou por manter em kg |
| ignored | bool NOT NULL default false | |
| ignore_reason | text default '' | `deposit` \| `discount` \| `tax` \| `total` \| `payment` \| `not_product` \| `user` |
| confidence | numeric(4,3) | |

### 1.8 `pantry_shopping_items`

| coluna | tipo | obs |
|---|---|---|
| id | uuid PK | |
| user_id | uuid NOT NULL idx | |
| product_id | uuid FK nullable | NULL para item manual sem produto |
| name | text NOT NULL | exibição |
| quantity | numeric(12,3) | sugerida |
| unit | text | |
| origin | text NOT NULL | `auto` \| `manual` \| `recipe` |
| reason | text default '' | `out` \| `below_min` \| `running_out` |
| status | text NOT NULL | `open` \| `bought` \| `dismissed` |
| bought_receipt_item_id | uuid nullable | marcado automaticamente por nota |
| created_at / bought_at | timestamptz | |

Índice parcial: `UNIQUE (user_id, product_id) WHERE status = 'open' AND product_id IS NOT NULL`.

### 1.9 `pantry_recipes` e `pantry_recipe_logs`

`pantry_recipes`: `id, user_id, title, language, servings int, prep_minutes int,
filters jsonb, ingredients jsonb, steps jsonb, tips text, is_favorite bool,
source ('ai'|'user'), ai_model, created_at`.

`ingredients` (jsonb): `[{name, product_id|null, quantity, unit, have: bool, have_quantity}]`.

`pantry_recipe_logs`: `id, user_id, recipe_id FK, cooked_at, servings, deductions jsonb
[{product_id, quantity, unit}]`. Cada log gera N `pantry_movements` do tipo `recipe_out`.

### 1.10 Migration

Seguindo o padrão do projeto: função `migratePantry(db)` em `migrations/migrate.go`
chamada por `Run`, com `AutoMigrate` das structs + SQL idempotente para índices
parciais e seed de `pantry_seed_products`.

**Dívida técnica (aceita):** o projeto não tem migrations reversíveis. Será
entregue `backend/migrations/rollback_pantry.sql` (DROP das tabelas na ordem
correta) como rollback manual documentado.

---

## 2. Rotas da API (`/pantry`, JWT obrigatório)

Todas devolvem JSON; erros no formato `{"error": "mensagem"}` (helpers existentes
`respondJSON`/`respondError`). `POST /pantry/receipts` entra na regra de timeout
longo (120 s) do `main.go`, generalizada para uma lista de paths.

### 2.1 Configurações

- `GET /pantry/settings` → `PantrySettings`
- `PUT /pantry/settings` ← `{language, country, currency, shopping_interval_days, ask_produce_units}`

### 2.2 Produtos e estoque

- `GET /pantry/products?q=&category=&status=` → lista com estoque e previsão:

```json
[{
  "id": "…", "name": "Banana prata", "name_pt": "Banana prata", "name_de": "Banane",
  "category": "food", "subcategory": "produce", "unit": "un", "is_produce": true,
  "quantity": 4, "min_level": 6, "min_level_source": "auto",
  "avg_price": 0.79, "currency": "BRL", "last_purchased_at": "2026-09-01T…",
  "status": "low",            // ok | low | out | running_out
  "daily_rate": 0.9, "run_out_at": "2026-09-11T…"
}]
```

- `POST /pantry/products` ← `{name_pt, name_de?, category, subcategory?, unit, is_produce?, track_by_unit?, min_level?, initial_quantity?}`
- `PUT /pantry/products/{id}` (mesmos campos)
- `DELETE /pantry/products/{id}`
- `GET /pantry/products/{id}/movements?limit=50` → `PantryMovement[]`
- `POST /pantry/products/{id}/consume` ← `{quantity: 1, note?}` → estoque atualizado (movimento `manual_out`)
- `POST /pantry/products/{id}/out` → zera o estoque (movimento `manual_out` com o saldo) e gera item na lista
- `POST /pantry/products/{id}/adjust` ← `{quantity: 3.5}` → define quantidade absoluta (movimento `adjust`)

### 2.3 Nota fiscal

- `POST /pantry/receipts` (multipart: `image`, `instruction?`, `country?` = BR|DE, `client_datetime`)
  → `202`-like `200` com a nota em `pending_review`:

```json
{
  "receipt": {
    "id": "…", "status": "pending_review", "image_url": "https://…/image/abc.jpg",
    "country": "DE", "currency": "EUR", "locale": "DE", "merchant": "REWE",
    "purchased_at": "2026-09-06T…", "total": 34.17, "access_key": ""
  },
  "items": [{
    "id": "…", "line_no": 3, "raw_name": "BIO BANANEN 1,24 kg",
    "name": "Banana", "name_pt": "Banana", "name_de": "Banane",
    "product_id": "…", "match_source": "alias",
    "category": "food", "subcategory": "produce",
    "quantity": 1.24, "unit": "kg", "unit_price": 1.99, "total_price": 2.47,
    "is_produce": true, "needs_unit_count": true, "unit_count": null,
    "ignored": false, "ignore_reason": "", "confidence": 0.9
  }, {
    "id": "…", "line_no": 9, "raw_name": "PFAND 0,25", "ignored": true,
    "ignore_reason": "deposit", "quantity": 1, "unit": "un", "total_price": 0.25
  }],
  "review": { "needs_unit_count": 2, "unmatched": 1, "ignored": 3 },
  "payment_methods": [{"id": "…", "name": "Nubank", "kind": "credit_card"}],
  "financial_category_context": "food_detail"
}
```

- `GET /pantry/receipts?status=` e `GET /pantry/receipts/{id}` (mesmo formato)
- `PUT /pantry/receipts/{id}/items` ← lista completa de itens editados
  (`{id, name_pt, name_de, product_id?, category, subcategory, quantity, unit, unit_count?, keep_weight?, ignored}`)
- `POST /pantry/receipts/{id}/confirm` ←

```json
{
  "items": [ /* opcional: mesma forma do PUT, aplica antes de confirmar */ ],
  "create_transaction": true,
  "payment_method_id": "…",
  "purchased_at": "2026-09-06T…"
}
```

  → `{"receipt": {...status:"confirmed"}, "movements_created": 18, "products_created": 3,
     "shopping_items_bought": 4, "transactions": [ /* models.Transaction */ ]}`

  Regras da confirmação (transação de banco única):
  1. Rejeita (422) se algum item não ignorado tem `needs_unit_count && unit_count == null && !keep_weight`,
     ou não tem `product_id` nem `name_pt`.
  2. Para item sem `product_id`: cria `pantry_product` (do seed se houver, senão do JSON da IA)
     e grava alias `raw_name → product` com `source = 'user'`.
  3. Quantidade de controle: `unit_count` quando informado (unidade `un`); senão a quantidade
     da nota convertida para a unidade do produto (só g↔kg e ml↔l; outras combinações
     mantêm a unidade da nota e atualizam `pantry_stock.unit` se o estoque estiver zerado).
  4. Cria `pantry_movements` (`receipt_in`), atualiza `pantry_stock` (quantidade, preço médio,
     `last_purchased_at`).
  5. Marca como `bought` os `pantry_shopping_items` abertos com o mesmo `product_id`.
  6. Recalcula a lista automática (§ 2.4).
  7. Se `create_transaction`: agrupa os itens não ignorados por categoria financeira
     (§ 2.3.1), cria `transactions` com `source = 'photo'`, `image_url`, moeda/locale da nota
     e conversão para a moeda base (mesma lógica de `applyCurrency`, replicada no serviço da
     dispensa para não tocar o handler financeiro). Forma de pagamento via
     `transactions.Validator`; se o validador pedir esclarecimento, responde 422 com as opções
     (a tela de revisão já pede a forma de pagamento, então isso só ocorre se a usuária deixar
     em branco).

- `DELETE /pantry/receipts/{id}` → `discarded` (imagem permanece no bucket).

#### 2.3.1 Mapa categoria da dispensa → categoria financeira

| dispensa | `category_context = food_detail` | `standard` |
|---|---|---|
| food / produce | `frutaria` | `alimentacao` |
| food / demais | `comida` | `alimentacao` |
| food / beverages (não alcoólicas) | `bebidas_nao_alcoolicas` | `alimentacao` |
| food / alcohol | `bebidas_alcoolicas` | `alimentacao` |
| hygiene | `higiene_pessoal` | `outros` |
| cleaning | `limpeza_da_casa` | `casa` |
| general | `outros` | `outros` |

Linhas ignoradas do tipo `deposit` (Pfand) entram no valor da categoria `outros`
quando positivas; `discount` reduz proporcionalmente o total do grupo. `tax`, `total`,
`payment` são descartadas.

### 2.4 Lista de compras

- `GET /pantry/shopping-list?status=open` → agrupada por categoria:

```json
{"items": [{"id": "…", "product_id": "…", "name": "Leite", "quantity": 2, "unit": "l",
            "origin": "auto", "reason": "out", "status": "open", "category": "food"}],
 "counts": {"open": 7, "bought_today": 3}}
```

- `POST /pantry/shopping-list` ← `{name, product_id?, quantity?, unit?}` (manual)
- `PATCH /pantry/shopping-list/{id}` ← `{status?: bought|open|dismissed, quantity?}`
- `POST /pantry/shopping-list/regenerate` → recalcula os itens `auto` (também chamado
  internamente após qualquer movimento de estoque):
  - `out`: `quantity <= 0`
  - `below_min`: `quantity < min_level` (informado ou sugerido)
  - `running_out`: `run_out_at <= now + shopping_interval_days`
  - quantidade sugerida = `max(min_level * 2 − quantity, 1)` arredondada à unidade
  - itens `auto` que deixaram de atender a regra são `dismissed`; manuais nunca são tocados.
- `POST /pantry/shopping-list/from-recipe` ← `{recipe_id | ingredients: [...]}` → cria itens
  `origin = recipe` para os ingredientes com `have = false`.

### 2.5 Receitas

- `POST /pantry/recipes/generate` ←

```json
{"filters": ["chicken", "quick", "cheap", "use_expiring"],
 "restrictions": ["gluten_free", "lactose_free"],
 "servings": 2, "language": "pt-BR", "free_text": "algo com arroz"}
```

  → `Recipe` (não salva) no formato do § 3.3, mais `"have_count": 6, "missing_count": 2`.
  Filtros aceitos: `beef, chicken, fish, pork, vegetarian, vegan, pasta, rice, soup, salad,
  breakfast, dessert, quick, cheap, use_expiring, use_running_out`.
  Restrições: `gluten_free, lactose_free, nut_free, egg_free, low_carb, no_sugar`.

- `POST /pantry/recipes` ← `Recipe` → salva (`is_favorite = true`)
- `GET /pantry/recipes`, `GET /pantry/recipes/{id}`, `DELETE /pantry/recipes/{id}`
- `POST /pantry/recipes/{id}/cook/preview` ← `{servings?}` →
  `{"deductions": [{"product_id", "name", "quantity", "unit", "available": 2, "will_deduct": 1}],
    "missing": [...]}`
- `POST /pantry/recipes/{id}/cook` ← `{servings, deductions: [{product_id, quantity, unit}]}`
  → cria `pantry_recipe_log`, movimentos `recipe_out`, atualiza estoque, recalcula a lista.

---

## 3. Prompts de IA

Cliente: `internal/services/ai/anthropic.go` ganha um método **novo**
`ChatStructured(ctx, StructuredRequest)` que aceita `system`, blocos de imagem
opcionais, `output_config.format = {type: "json_schema", schema}` e
`output_config.effort`, com retry (2 tentativas, backoff) para 429/529/5xx e
timeouts. Os métodos existentes (`Chat`, `ChatWithImage`, `ChatWithTools`) **não
mudam**. Uma interface nova `ai.StructuredProvider` é usada pelos serviços da
dispensa, de modo que mocks existentes de `ai.Provider` não precisam ser alterados.

Modelo: `claude-sonnet-5` (constantes novas `TaskPantryReceipt`, `TaskPantryRecipe`
em `router.go`, sem alterar o `default` usado pelo financeiro). Thinking adaptativo
(padrão do modelo) com `effort: "low"` na extração e `"medium"` na receita.
Sem `temperature`. Sem prefill.

Validação após o parse: enums, quantidades ≥ 0, unidade permitida, `country ∈ {BR, DE}`,
moeda coerente com o país, no máximo 200 itens. Se o JSON não bater com o schema
(ou o `stop_reason` for `max_tokens`/`refusal`), uma segunda chamada é feita com a
mensagem de erro; se falhar de novo, a API responde 502 com "Não consegui ler a
nota. Tente uma foto mais nítida, com a nota inteira e sem dobras."

Toda saída da IA é tratada como dado: nomes de itens nunca são interpretados como
instrução, e a `instruction` da usuária vai em bloco delimitado, como hoje.

### 3.1 Extração da nota (visão + structured output)

**System prompt (resumido; texto final em pt-BR e de):**

```
Você é um extrator de itens de notas fiscais de supermercado, drogaria e lojas de
casa, para um app de controle de dispensa. Lê notas do Brasil (NFC-e, cupom fiscal)
e da Alemanha (Kassenbon: REWE, Edeka, Aldi, Lidl, Kaufland, dm, Rossmann, Netto,
Penny, Müller e outros).

Regras:
1. Detecte country (BR|DE), currency (BRL|EUR) e locale pela estrutura, moeda e
   idioma. Se o usuário informar o país, respeite.
2. Formato numérico alemão: vírgula decimal (1,99). Linhas como "2 x 0,89" ou
   "0,532 kg x 2,49 EUR/kg" → quantity=2 unit=un, ou quantity=0.532 unit=kg e
   unit_price=2.49.
3. Uma entrada por linha de produto, na ordem da nota (line_no). Não agrupe.
4. raw_name = texto exato da linha. name_pt e name_de = nome canônico curto,
   sem marca e sem embalagem ("BANANA PRATA KG" → "Banana prata" / "Banane";
   "SPÜLM. LEMON" → "Detergente" / "Spülmittel"). Sempre preencha os dois idiomas.
5. Se o item corresponder a um produto do CATÁLOGO abaixo, devolva product_id;
   senão null.
6. category ∈ food|cleaning|hygiene|general; subcategory conforme lista.
7. is_produce=true para frutas, legumes e verduras. sold_by_weight=true quando a
   unidade é kg ou g. Folhas e maços: unit=maco.
8. Marque ignored=true com ignore_reason para linhas que não são produto:
   deposit (Pfand, Leergut, depósito de garrafa), discount (Rabatt, Preisvorteil,
   Coupon, Payback, desconto), tax (A 7%, B 19%, ICMS, tributos), total (Summe,
   Total, Subtotal, Rückgeld, troco), payment (EC-Karte, Kreditkarte, PIX, cartão,
   dinheiro), not_product (cabeçalhos, CNPJ, endereço, agradecimentos).
9. Se um valor não estiver legível, devolva null e confidence baixa; nunca invente.
10. Devolva também merchant, purchased_at (ISO 8601 ou null), total e access_key
    (44 dígitos da NFC-e, ou "" se não houver).

CATÁLOGO DO USUÁRIO (id → nome): …
SUBCATEGORIAS: produce, dairy, meat, fish, grains, bakery, frozen, snacks,
beverages, alcohol, spices, canned, laundry, kitchen, bathroom, paper, oral, hair,
body, baby, pet, other.
```

**Mensagem do usuário:** bloco de imagem (JPEG ≤ 2576 px no lado maior) +
texto `"País informado: DE. Instrução do usuário: <delimitada>"` quando existir.

**Schema JSON (output_config.format):**

```json
{
  "type": "object", "additionalProperties": false,
  "required": ["country", "currency", "locale", "merchant", "purchased_at", "total", "access_key", "items"],
  "properties": {
    "country": {"enum": ["BR", "DE"]},
    "currency": {"enum": ["BRL", "EUR"]},
    "locale": {"enum": ["BR", "DE"]},
    "merchant": {"type": "string"},
    "purchased_at": {"type": ["string", "null"]},
    "total": {"type": ["number", "null"]},
    "access_key": {"type": "string"},
    "items": {"type": "array", "items": {
      "type": "object", "additionalProperties": false,
      "required": ["line_no", "raw_name", "name_pt", "name_de", "product_id", "category",
                   "subcategory", "quantity", "unit", "unit_price", "total_price",
                   "is_produce", "sold_by_weight", "ignored", "ignore_reason", "confidence"],
      "properties": {
        "line_no": {"type": "integer"},
        "raw_name": {"type": "string"},
        "name_pt": {"type": "string"}, "name_de": {"type": "string"},
        "product_id": {"type": ["string", "null"]},
        "category": {"enum": ["food", "cleaning", "hygiene", "general"]},
        "subcategory": {"type": "string"},
        "quantity": {"type": "number"},
        "unit": {"enum": ["un", "kg", "g", "l", "ml", "pct", "maco", "dz"]},
        "unit_price": {"type": ["number", "null"]},
        "total_price": {"type": ["number", "null"]},
        "is_produce": {"type": "boolean"},
        "sold_by_weight": {"type": "boolean"},
        "ignored": {"type": "boolean"},
        "ignore_reason": {"enum": ["", "deposit", "discount", "tax", "total", "payment", "not_product"]},
        "confidence": {"type": "number"}
      }
    }}
  }
}
```

**Pós-processamento no Go (matching determinístico tem prioridade sobre a IA):**

1. `raw_name` normalizado → `pantry_product_aliases` do usuário (`match_source = alias`).
2. Senão, `name_pt` normalizado igual a `pantry_products.name_pt` (`product`).
3. Senão, `name_pt`/`name_de`/`raw_name` contra `pantry_seed_products` e seus aliases (`seed`).
4. Senão, `product_id` devolvido pela IA se existir no catálogo do usuário (`ai`).
5. Senão `none` (a tela de revisão mostra como "novo produto").
6. `needs_unit_count = is_produce && unit ∈ {kg, g} && settings.ask_produce_units && !(produto.track_by_unit == false && produto já existe com unit kg)`.

### 3.2 Normalização avulsa (texto, sem imagem)

Usada ao cadastrar produto manualmente ou adicionar item na lista por nome livre.
Entrada: lista de até 20 nomes brutos + catálogo. Saída (schema): `[{raw, name_pt,
name_de, product_id|null, category, subcategory, default_unit, is_produce}]`.
Mesma cadeia de matching determinístico antes de chamar a IA; só chama se sobrar
algo sem match.

### 3.3 Geração de receita (texto, structured output)

**System prompt (resumido):**

```
Você é um chef caseiro prático. Crie UMA receita usando prioritariamente os
ingredientes disponíveis na dispensa do usuário, respeitando os filtros e
restrições. Escreva no idioma pedido (pt-BR ou de). Use nomes de ingredientes e
produtos comuns no país da dispensa (BR ou DE); não sugira ingredientes difíceis
de encontrar lá. Prefira itens marcados como "acabando" ou "vencendo" quando o
filtro pedir. Mantenha quantidades realistas para o número de porções. Para cada
ingrediente informe se o usuário tem (have=true, com product_id) ou não. Não
invente que o usuário tem algo fora da lista. Sal, água, óleo e pimenta podem ser
assumidos como disponíveis (marque have=true e product_id=null).
```

**Mensagem do usuário:** JSON com `language`, `country`, `servings`, `filters`,
`restrictions`, `free_text` e `pantry: [{product_id, name, quantity, unit, status}]`
(no máximo 150 itens, ordenados por `running_out`/`low` primeiro).

**Schema:**

```json
{"type": "object", "additionalProperties": false,
 "required": ["title", "language", "servings", "prep_minutes", "ingredients", "steps", "tips"],
 "properties": {
   "title": {"type": "string"}, "language": {"enum": ["pt-BR", "de"]},
   "servings": {"type": "integer"}, "prep_minutes": {"type": "integer"},
   "ingredients": {"type": "array", "items": {"type": "object", "additionalProperties": false,
     "required": ["name", "product_id", "quantity", "unit", "have", "optional"],
     "properties": {"name": {"type": "string"}, "product_id": {"type": ["string", "null"]},
       "quantity": {"type": "number"}, "unit": {"enum": ["un", "kg", "g", "l", "ml", "pct", "maco", "dz", "colher_sopa", "colher_cha", "xicara", "a_gosto"]},
       "have": {"type": "boolean"}, "optional": {"type": "boolean"}}}},
   "steps": {"type": "array", "items": {"type": "string"}},
   "tips": {"type": "string"}}}
```

Pós-processamento: `have` é recalculado no Go a partir do `product_id` e do estoque
(a IA só sugere); ingredientes com `product_id` inválido viram `have=false`.

---

## 4. Fluxo de revisão do hortifruti

1. A extração marca `needs_unit_count = true` em itens `is_produce` vendidos por peso.
2. A resposta de `POST /pantry/receipts` traz `review.needs_unit_count = N`; a tela
   de revisão abre com esses itens no topo, destacados.
3. Cada item mostra: "A nota tem **1,24 kg de Banana**. Quantas bananas são?" com
   um campo numérico grande (teclado numérico), botões rápidos (+1, +5) e o link
   "Manter em kg" (`keep_weight = true`).
4. O botão "Confirmar entrada" fica desabilitado enquanto houver item pendente
   (`needs_unit_count && unit_count == null && !keep_weight`). O backend valida a
   mesma regra (422).
5. Na confirmação, o produto passa a ser controlado em `un` (`track_by_unit = true`);
   nas próximas notas a pergunta continua, mas com a última contagem/kg como
   sugestão ("da última vez foram ~6 bananas por kg").
6. Itens vendidos por unidade (`Gurke 1 St`, `Alface un`) não perguntam nada.
7. Se `ask_produce_units = false` nas configurações, nada é perguntado e o controle
   fica em kg.

---

## 5. Frontend `dispensaAI/`

### 5.1 Estrutura

```
dispensaAI/
  Dockerfile, railway.toml, nginx.conf (cópias do frontend/)
  index.html, vite.config.ts (PWA, nome "DispensaAI", ícone próprio)
  src/
    services/api.ts          (wrapper fetch + refresh, copiado de frontend/)
    store/auth.ts, settings.ts
    hooks/useBootstrapAuth.ts, useCamera.ts, usePantry.ts, useReceipts.ts,
          useShoppingList.ts, useRecipes.ts
    i18n/{pt-BR,de}.ts + useT()  (dicionário próprio, sem biblioteca)
    lib/resizeImage.ts (limite 2576 px), units.ts, categories.ts
    components/ui/* (shadcn), BottomNav, PageHeader, ProductRow, QuantitySheet,
          ReceiptItemCard, ProduceCountSheet, RecipeCard, DeductionSheet
    pages/Login, Dispensa, Nota, NotaRevisao, Lista, Receitas, ReceitaDetalhe,
          Favoritas, Ajustes
```

Rotas: `/login`, `/` (Dispensa), `/nota`, `/nota/:id` (revisão), `/lista`,
`/receitas`, `/receitas/:id`, `/receitas/favoritas`, `/ajustes`.
BottomNav (5 itens): Dispensa · Nota · Lista · Receitas · Ajustes.
Login: e-mail/senha reutilizando `/auth/login` e `/auth/refresh`; passkey fica fora
do escopo inicial.

### 5.2 Telas (mobile-first)

- **Dispensa**: busca, chips de categoria, filtro de status (Acabando / Zerado),
  lista com nome, quantidade + unidade, badge de status, botões `−1` e `Acabou`;
  toque abre `QuantitySheet` (ajustar quantidade, mínimo, histórico). Botão `+`
  para produto manual.
- **Nota**: botão grande "Fotografar nota" (câmera) e "Escolher da galeria";
  seletor de país (auto / BR / DE); campo opcional de instrução; estado de
  envio com progresso ("lendo a nota…", até 60 s).
- **Revisão da nota**: cabeçalho (loja, data, total, moeda); seção "Precisa de
  contagem" (hortifruti); lista de itens editáveis (nome canônico com sugestão,
  categoria, quantidade/unidade, preço); "novos produtos" destacados; linhas
  ignoradas colapsadas ("3 linhas ignoradas: Pfand, Rabatt, Summe") com opção de
  reativar; rodapé com "Lançar no financeiro" (toggle, forma de pagamento) e
  botão "Confirmar entrada"; tela de resumo ao final.
- **Lista de compras**: agrupada por categoria, checkbox marca como comprado,
  motivo em texto pequeno ("acabou", "abaixo do mínimo", "acaba em 3 dias"),
  adicionar manual, botão "Copiar lista".
- **Receitas**: chips de filtros e restrições, porções, texto livre, "Gerar
  receita"; resultado com ingredientes separados em "Tenho" / "Falta", botões
  "Adicionar faltantes à lista", "Salvar" e "Vou fazer" → `DeductionSheet` com
  quantidades editáveis → confirmar → toast "estoque atualizado".
  Aba "Favoritas" com "Fazer de novo".
- **Ajustes**: idioma (pt-BR/de), país da dispensa, moeda, intervalo de
  compras, perguntar unidades do hortifruti, sair.

### 5.3 Deploy

Novo serviço Railway apontando para `dispensaAI/` (Dockerfile igual ao do
`frontend/`, `VITE_API_URL` = URL do backend). No backend, incluir a origem do
novo app em `FRONTEND_URL` (lista separada por vírgula, já suportada pelo CORS).
Nenhuma outra mudança de configuração.

---

## 6. Ordem de entrega

Cada etapa termina com testes passando, commit e um resumo (feito / como testar /
pendente). Sem `git push`.

**Etapa 0 — Fundação (backend + app vazio)**
- `migratePantry` com `pantry_settings` e `pantry_seed_products` + seed.
- `GET/PUT /pantry/settings`; timeout longo generalizado no `main.go`.
- `dispensaAI/` com login, refresh, shell, BottomNav, i18n, tela de Ajustes,
  Dockerfile e railway.toml.
- Teste: login no novo app, salvar idioma/país.

**Etapa 1 — Catálogo, estoque e consumo manual**
- Tabelas `pantry_products`, `pantry_product_aliases`, `pantry_stock`,
  `pantry_movements`; repositórios e serviço `pantry.Service` (entrada, saída,
  ajuste, preço médio).
- Rotas de produtos/estoque/consumo; tela Dispensa completa com `−1`/`Acabou`.
- Testes unitários: conversão de unidades, baixa de estoque com clamp, preço médio.

**Etapa 2 — Nota fiscal → revisão → entrada**
- `ai.ChatStructured` com retry; serviço `pantry/receipts` (prompt, schema,
  validação, matching, hortifruti); tabelas de nota; rotas de nota e confirmação
  (incluindo lançamento financeiro via `Validator`).
- Telas Nota e Revisão.
- Testes: fixtures de resposta da IA (BR e DE) → itens esperados; matching por
  alias/seed; regra do `needs_unit_count`; confirmação em transação atômica
  (integração); mapa de categorias financeiras.

**Etapa 3 — Lista de compras**
- Tabela e rotas; regeneração automática após qualquer movimento; marcação
  automática na confirmação da nota; tela Lista.
- Testes: geração por `out`/`below_min`; item manual preservado; marcado como
  comprado pela nota.

**Etapa 4 — Receitas**
- Prompt e schema de receita; tabelas; rotas de geração, favoritos, preview e
  baixa; telas Receitas/Detalhe/Favoritas; "faltantes → lista".
- Testes: recálculo de `have`; preview de baixa respeita estoque; baixa cria
  movimentos `recipe_out`.

**Etapa 5 — Previsão e refinamentos**
- Ritmo de consumo (§ 6.1), `run_out_at`, mínimo sugerido, `running_out` na lista.
- Validade (`shelf_life_days`, `expires_at` por entrada, filtro "vencendo").
- Consumo por texto livre ("usei 2 ovos") usando a normalização § 3.2.
- Captura da chave NFC-e; avaliação da consulta SEFAZ conforme a UF.
- Cache do system prompt e métricas de tokens no admin.

### 6.1 Ritmo de consumo (Etapa 5)

- `rate_out` = soma das saídas (`manual_out` + `recipe_out`) nos últimos 60 dias ÷ dias
  observados (mínimo 14 dias).
- Se não houver saídas registradas e houver ≥ 2 entradas: `rate_in` = soma das
  quantidades compradas entre a primeira e a penúltima entrada ÷ dias entre elas.
- `daily_rate = rate_out` se existir, senão `rate_in`, senão NULL (sem previsão).
- `run_out_at = now + quantity / daily_rate`.
- Mínimo sugerido = `daily_rate × shopping_interval_days × 1.2`, arredondado para
  cima na unidade do produto (mínimo 1).

---

## 7. Estimativa de custo (claude-sonnet-5: US$ 2 / 1M entrada, US$ 10 / 1M saída)

| Chamada | Entrada | Saída | Custo |
|---|---|---|---|
| Nota (imagem 2576 px ≈ 4,8k tokens + system ≈ 1,5k + catálogo 200 itens ≈ 0,8k) | ≈ 7k | 40 itens ≈ 3k | ≈ US$ 0,045 |
| Nota pequena (1920 px, 15 itens) | ≈ 5k | ≈ 1,2k | ≈ US$ 0,022 |
| Receita (dispensa 100 itens ≈ 2k + system 0,5k) | ≈ 2,5k | ≈ 1,2k | ≈ US$ 0,017 |
| Normalização avulsa (10 nomes) | ≈ 1k | ≈ 0,4k | ≈ US$ 0,006 |

Uso mensal estimado (6 notas + 15 receitas + 10 normalizações): **≈ US$ 0,60**.
Uma retentativa por erro de schema dobra o custo daquela chamada. Tokens reais
serão gravados em `pantry_receipts.ai_*_tokens` para acompanhar.

---

## 8. Testes e qualidade

- Regras de negócio com testes unitários puros (sem banco): matching e
  normalização, conversão de unidades, baixa/ajuste de estoque, geração da lista,
  previsão de consumo, validação do JSON da IA (fixtures em `testdata/`).
- Repositórios e confirmação da nota com testes `//go:build integration`, rodados
  com `-p 1` (limitação conhecida do banco compartilhado).
- Frontend: vitest para `units.ts`, `i18n`, reducer da tela de revisão e
  `ProduceCountSheet`.
- Nenhum teste existente do financeiro é alterado.

## 9. Riscos e dívidas aceitas

- Migrations sem "down" (rollback manual em `rollback_pantry.sql`).
- Lógica de conversão de moeda replicada do handler de upload (≈ 15 linhas) para
  não tocar o financeiro.
- Sem amostras reais de nota alemã: o prompt DE será calibrado com fixtures
  sintéticas; ajustar após as primeiras notas reais.
- Sem CI: rodar `go test ./...`, `go vet ./...` e `npx vitest run` a cada etapa.
- `user_settings.user_api_key` em texto puro (já existente); a dispensa reutiliza
  a mesma chave por usuário.
