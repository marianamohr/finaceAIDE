import { useAuthStore } from "@/store/auth";

// Mesmo backend Go do FinanceAI. Chaves de storage com prefixo "dai-" para não
// colidir com o app financeiro quando os dois rodam na mesma origem (dev).
const API_URL = import.meta.env.VITE_API_URL || "/api";
const REFRESH_TOKEN_KEY = "dai-r";

export function saveRefreshToken(token: string) {
  try { localStorage.setItem(REFRESH_TOKEN_KEY, token); } catch { /* storage indisponível */ }
}

export function loadRefreshToken(): string | null {
  try { return localStorage.getItem(REFRESH_TOKEN_KEY); } catch { return null; }
}

export function clearRefreshToken() {
  try { localStorage.removeItem(REFRESH_TOKEN_KEY); } catch { /* storage indisponível */ }
}

export class ApiError extends Error {
  status: number;
  body: unknown;
  constructor(status: number, message: string, body?: unknown) {
    super(message);
    this.status = status;
    this.body = body;
  }
}

async function parseError(res: Response): Promise<ApiError> {
  const body = await res.json().catch(() => null);
  const message = (body && typeof body.error === "string" && body.error) || `Erro ${res.status}`;
  return new ApiError(res.status, message, body);
}

async function tryRefresh(): Promise<string | null> {
  const storedRefresh = loadRefreshToken();
  const headers: Record<string, string> = {};
  if (storedRefresh) headers["X-Refresh-Token"] = storedRefresh;

  const res = await fetch(`${API_URL}/auth/refresh`, {
    method: "POST",
    credentials: "include",
    headers,
  });
  if (!res.ok) return null;

  const data = await res.json();
  if (data.refresh_token) saveRefreshToken(data.refresh_token);
  if (data.user) {
    useAuthStore.getState().setAuth(data.user, data.access_token);
  } else {
    useAuthStore.getState().setToken(data.access_token);
  }
  return data.access_token as string;
}

export function forceLogout() {
  useAuthStore.getState().logout();
  clearRefreshToken();
  try { useAuthStore.persist.clearStorage(); } catch { /* noop */ }
}

async function request<T>(endpoint: string, options: RequestInit = {}): Promise<T> {
  const token = useAuthStore.getState().token;
  const isFormData = options.body instanceof FormData;

  const headers: Record<string, string> = {
    ...(isFormData ? {} : { "Content-Type": "application/json" }),
    ...(options.headers as Record<string, string>),
  };
  if (token) headers["Authorization"] = `Bearer ${token}`;

  let res = await fetch(`${API_URL}${endpoint}`, { ...options, headers, credentials: "include" });

  if (res.status === 401 && useAuthStore.getState().isAuthenticated) {
    const newToken = await tryRefresh();
    if (!newToken) {
      forceLogout();
      throw new ApiError(401, "Sessão expirada");
    }
    headers["Authorization"] = `Bearer ${newToken}`;
    res = await fetch(`${API_URL}${endpoint}`, { ...options, headers, credentials: "include" });
  }

  if (!res.ok) throw await parseError(res);
  if (res.status === 204) return undefined as unknown as T;
  return res.json();
}

// ── Tipos compartilhados ──────────────────────────────────────────────────────

export type PantryLanguage = "pt-BR" | "de";
export type PantryCountry = "BR" | "DE";
export type PantryCurrency = "BRL" | "EUR";

export interface PantrySettings {
  user_id: string;
  language: PantryLanguage;
  country: PantryCountry;
  currency: PantryCurrency;
  shopping_interval_days: number;
  ask_produce_units: boolean;
  updated_at: string;
}

export type PantrySettingsPatch = Partial<
  Pick<PantrySettings, "language" | "country" | "currency" | "shopping_interval_days" | "ask_produce_units">
>;

// ── Pantry products (dispensa) ────────────────────────────────────────────────
// Fonte da verdade: internal/handlers/pantry_product.go e
// internal/services/pantry/service.go (tipo ProductWithStock) no backend.

export type PantryProductCategory = "food" | "cleaning" | "hygiene" | "general";
export type PantryProductUnit = "un" | "kg" | "g" | "l" | "ml" | "pct" | "maco" | "dz";
export type PantryProductStatus = "ok" | "low" | "out" | "running_out";
export type PantryMinLevelSource = "manual" | "auto";

export interface PantryProduct {
  id: string;
  user_id: string;
  name_pt: string;
  name_de: string;
  category: PantryProductCategory;
  subcategory: string;
  default_unit: PantryProductUnit;
  is_produce: boolean;
  track_by_unit: boolean;
  // min_level é o mínimo EFETIVO (configurado pela usuária ou sugerido pelo
  // ritmo de consumo — ver min_level_source), não o campo bruto salvo no
  // catálogo (services/pantry.ProductWithStock no backend).
  min_level?: number;
  min_level_source: PantryMinLevelSource;
  shelf_life_days?: number;
  created_at: string;
  updated_at: string;
  // Campos de estoque, achatados pelo backend (services/pantry.ProductWithStock).
  quantity: number;
  unit: string;
  avg_price?: number;
  currency: string;
  last_purchased_at?: string;
  status: PantryProductStatus;
  // Etapa 5: previsão de consumo e validade.
  daily_rate?: number;
  run_out_at?: string;
  expires_at?: string;
  expiring: boolean;
}

export interface PantryMovement {
  id: string;
  user_id: string;
  product_id: string;
  type: "receipt_in" | "manual_in" | "manual_out" | "recipe_out" | "adjust";
  quantity: number;
  unit: string;
  unit_price?: number;
  currency?: string;
  reference_type?: string;
  reference_id?: string;
  note: string;
  occurred_at: string;
  created_at: string;
}

export interface PantryProductListFilters {
  q?: string;
  category?: PantryProductCategory;
  // "expiring" é um filtro à parte do enum de status (ver Service.ListWithStock
  // no backend): filtra pelo flag `expiring`, não pelo campo `status`.
  status?: PantryProductStatus | "expiring";
}

export interface CreatePantryProductInput {
  name_pt: string;
  name_de?: string;
  category: PantryProductCategory;
  subcategory?: string;
  unit: PantryProductUnit;
  is_produce?: boolean;
  track_by_unit?: boolean;
  min_level?: number;
  initial_quantity?: number;
  shelf_life_days?: number;
}

export type UpdatePantryProductInput = Omit<CreatePantryProductInput, "initial_quantity">;

// Catálogo global read-only usado pela tela de inventário inicial
// ("Fazer inventário") — GET /pantry/seed-products. Espelha
// models.PantrySeedProduct no backend (id não é exposto: a tela cria
// produtos por name_pt, não referencia o seed depois de criado).
export interface PantrySeedProduct {
  name_pt: string;
  name_de: string;
  category: PantryProductCategory;
  subcategory: string;
  default_unit: PantryProductUnit;
  is_produce: boolean;
  shelf_life_days?: number;
}

// Consumo por texto livre ("usei 2 ovos") — POST /pantry/products/consume-text.
// Espelha pantry.ConsumeTextResult/ConsumedItem no backend.
export interface ConsumedPantryItem {
  raw_name: string;
  product: PantryProduct;
}

export interface ConsumePantryTextResult {
  matched: ConsumedPantryItem[];
  unmatched: string[];
}

// GET /pantry/ai-usage — espelha handlers.PantryAIUsageHandler no backend.
export interface PantryAIUsageBreakdown {
  count: number;
  input_tokens: number;
  output_tokens: number;
}

export interface PantryAIUsage {
  period: string;
  receipts: PantryAIUsageBreakdown;
  recipes: PantryAIUsageBreakdown;
  total_input_tokens: number;
  total_output_tokens: number;
  estimated_cost_usd: number;
}

// ── Pantry receipts (leitura de nota fiscal) ─────────────────────────────────
// Fonte da verdade: internal/handlers/pantry_receipt.go, internal/models/pantry.go
// e internal/services/transactions/validate.go (ClarificationNeed) no backend.

export type PantryReceiptStatus = "pending_review" | "confirmed" | "discarded";
export type PantryReceiptItemMatchSource = "" | "alias" | "product" | "seed" | "ai" | "none";
export type PantryReceiptItemIgnoreReason =
  | ""
  | "deposit"
  | "discount"
  | "tax"
  | "total"
  | "payment"
  | "not_product"
  | "user";

export interface PantryReceipt {
  id: string;
  user_id: string;
  account_id: string;
  status: PantryReceiptStatus;
  image_key: string;
  country: PantryCountry;
  currency: PantryCurrency;
  locale: string;
  merchant: string;
  purchased_at: string;
  total?: number;
  access_key: string;
  ai_model: string;
  ai_input_tokens: number;
  ai_output_tokens: number;
  transaction_ids?: string[];
  created_at: string;
  confirmed_at?: string;
}

export interface PantryReceiptItem {
  id: string;
  receipt_id: string;
  user_id: string;
  line_no: number;
  raw_name: string;
  name_pt: string;
  name_de: string;
  product_id?: string;
  match_source: PantryReceiptItemMatchSource;
  category: PantryProductCategory | "";
  subcategory: string;
  quantity: number;
  unit: PantryProductUnit;
  unit_price?: number;
  total_price?: number;
  is_produce: boolean;
  needs_unit_count: boolean;
  unit_count?: number;
  keep_weight: boolean;
  ignored: boolean;
  ignore_reason: PantryReceiptItemIgnoreReason;
  confidence: number;
}

export interface CreatePantryReceiptInput {
  image: File;
  instruction?: string;
  country?: PantryCountry;
}

// Formato de um item no body de UpdateItems e (opcionalmente) Confirm — espelha
// pantryReceiptItemUpdate no handler Go. A tela de revisão sempre envia o
// objeto inteiro de cada item que quer alterar.
export interface UpdatePantryReceiptItemInput {
  id: string;
  name_pt: string;
  name_de: string;
  category: string;
  subcategory: string;
  quantity: number;
  unit: string;
  unit_count?: number;
  keep_weight: boolean;
  ignored: boolean;
}

export interface ConfirmPantryReceiptInput {
  create_transaction: boolean;
  payment_method_id?: string;
  items?: UpdatePantryReceiptItemInput[];
}

export interface ConfirmPantryReceiptResult {
  receipt: PantryReceipt;
  movements_created: number;
  products_created: number;
  // Transações financeiras criadas (models.Transaction no backend); a
  // dispensaAI não modela esse tipo, então fica como unknown por ora.
  transactions: unknown[];
}

// Corpo do erro 422 quando itens de hortifrúti vendidos por peso ainda
// precisam da quantidade de unidades antes de confirmar (pendingUnitCountItemIDs
// no handler Go). ATENÇÃO: este 422 usa respondJSON (não respondError), mas
// ainda carrega "error" como mensagem — só o campo extra muda.
export interface PantryReceiptPendingUnitCountError {
  error: string;
  pending_unit_count_item_ids: string[];
}

// Corpo do erro 422 de forma de pagamento pendente na confirmação
// (transactions.ClarificationNeed no backend). ATENÇÃO: ao contrário dos
// outros erros da API, este corpo NÃO tem campo "error" — é o ClarificationNeed
// serializado diretamente.
export type PantryPaymentClarificationKind = "ambiguous" | "unknown_term" | "not_mentioned";

export interface PaymentMethodOption {
  payment_method_id: string;
  label: string;
  kind: string;
}

export interface PantryPaymentClarification {
  field: string;
  kind: PantryPaymentClarificationKind;
  prompt: string;
  options: PaymentMethodOption[];
  detected_payment_label?: string;
}

// ── Pantry shopping list (lista de compras da dispensa) ──────────────────────
// Fonte da verdade: internal/handlers/pantry_shopping_list.go e
// internal/models/pantry.go (PantryShoppingItem) no backend.

export type PantryShoppingOrigin = "auto" | "manual" | "recipe";
export type PantryShoppingReason = "" | "out" | "below_min" | "running_out";
export type PantryShoppingStatus = "open" | "bought" | "dismissed";

export interface PantryShoppingItem {
  id: string;
  user_id: string;
  product_id?: string;
  name: string;
  quantity?: number;
  unit: string;
  origin: PantryShoppingOrigin;
  reason: PantryShoppingReason;
  status: PantryShoppingStatus;
  bought_receipt_item_id?: string;
  created_at: string;
  bought_at?: string;
}

export interface AddPantryShoppingItemInput {
  name: string;
  product_id?: string;
  quantity?: number;
  unit?: string;
}

// ── Pantry recipes (receitas da dispensa) ────────────────────────────────────
// Fonte da verdade: internal/handlers/pantry_recipe.go,
// internal/services/pantry/recipe.go (GenerateRecipeParams, CookDeduction) e
// internal/models/pantry.go (PantryRecipe, PantryRecipeIngredient) no backend.

export type PantryRecipeSource = "ai" | "user";

export interface PantryRecipeIngredient {
  name: string;
  product_id?: string;
  quantity: number;
  unit: string;
  have: boolean;
  have_quantity: number;
  optional: boolean;
}

export interface PantryRecipe {
  id: string;
  user_id: string;
  title: string;
  language: PantryLanguage;
  servings: number;
  prep_minutes: number;
  filters: string[] | null;
  ingredients: PantryRecipeIngredient[];
  steps: string[];
  tips: string;
  is_favorite: boolean;
  source: PantryRecipeSource;
  ai_model: string;
  created_at: string;
}

export interface GenerateRecipeInput {
  filters?: string[];
  restrictions?: string[];
  servings?: number;
  free_text?: string;
  language?: PantryLanguage;
}

export interface GenerateRecipeResult {
  recipe: PantryRecipe;
  have_count: number;
  missing_count: number;
}

// Espelha services/pantry.CookDeduction no backend.
export interface CookDeduction {
  product_id: string;
  name: string;
  quantity: number;
  unit: string;
  available: number;
  will_deduct: number;
}

export interface CookPreviewResult {
  deductions: CookDeduction[];
  missing: PantryRecipeIngredient[];
}

export interface ConfirmCookDeduction {
  product_id: string;
  quantity: number;
  unit: string;
}

// A resposta de Cook devolve o PantryRecipeLog criado (internal/models/pantry.go),
// mas a dispensaAI não precisa modelar esse tipo nesta etapa.
export interface CookPantryRecipeResult {
  log: unknown;
  movements_created: number;
}

// ── Payment methods (mesma conta do app financeiro) ──────────────────────────
// Fonte da verdade: internal/handlers/payment_method.go e
// internal/models/payment_method.go no backend. GET /payment-methods devolve
// um array direto (não { payment_methods: [...] }).

export type PaymentMethodKind = "credit_card" | "debit" | "pix" | "voucher" | "cash";

export interface PaymentMethod {
  id: string;
  account_id: string;
  name: string;
  kind: PaymentMethodKind;
  closing_day?: number;
  due_day?: number;
  credit_limit?: number;
  last_four_digits?: string;
  color: string;
  active: boolean;
  created_at: string;
  // Presente apenas para kind "voucher"/"cash" (saldo calculado).
  balance?: number;
}

export interface AuthUser {
  id?: string;
  email?: string;
  role: string;
  status: string;
  account_id?: string;
}

export interface AuthResponse {
  user: AuthUser;
  access_token: string;
  refresh_token?: string;
}

export const api = {
  get: <T>(url: string) => request<T>(url),
  post: <T>(url: string, body?: unknown) =>
    request<T>(url, { method: "POST", body: body ? JSON.stringify(body) : undefined }),
  put: <T>(url: string, body?: unknown) =>
    request<T>(url, { method: "PUT", body: body ? JSON.stringify(body) : undefined }),
  patch: <T>(url: string, body?: unknown) =>
    request<T>(url, { method: "PATCH", body: body ? JSON.stringify(body) : undefined }),
  delete: <T>(url: string) => request<T>(url, { method: "DELETE" }),
  upload: <T>(url: string, form: FormData) => request<T>(url, { method: "POST", body: form }),

  login: (email: string, password: string) =>
    request<AuthResponse>("/auth/login", { method: "POST", body: JSON.stringify({ email, password }) }),
  logout: () => request<void>("/auth/logout", { method: "POST" }).catch(() => undefined),

  getPantrySettings: () => request<PantrySettings>("/pantry/settings"),
  updatePantrySettings: (patch: PantrySettingsPatch) =>
    request<PantrySettings>("/pantry/settings", { method: "PUT", body: JSON.stringify(patch) }),

  listPantryProducts: (filters?: PantryProductListFilters) => {
    const params = new URLSearchParams();
    if (filters?.q) params.set("q", filters.q);
    if (filters?.category) params.set("category", filters.category);
    if (filters?.status) params.set("status", filters.status);
    const qs = params.toString();
    return request<{ products: PantryProduct[] }>(`/pantry/products${qs ? `?${qs}` : ""}`);
  },
  createPantryProduct: (input: CreatePantryProductInput) =>
    request<PantryProduct>("/pantry/products", { method: "POST", body: JSON.stringify(input) }),
  updatePantryProduct: (id: string, input: UpdatePantryProductInput) =>
    request<{ id: string; updated: boolean }>(`/pantry/products/${id}`, {
      method: "PUT",
      body: JSON.stringify(input),
    }),
  deletePantryProduct: (id: string) => request<void>(`/pantry/products/${id}`, { method: "DELETE" }),
  getPantryProductMovements: (id: string, limit?: number) =>
    request<{ movements: PantryMovement[] }>(
      `/pantry/products/${id}/movements${limit ? `?limit=${limit}` : ""}`
    ),
  consumePantryProduct: (id: string, quantity: number, note?: string) =>
    request<PantryProduct>(`/pantry/products/${id}/consume`, {
      method: "POST",
      body: JSON.stringify({ quantity, note: note ?? "" }),
    }),
  markPantryProductOut: (id: string) =>
    request<PantryProduct>(`/pantry/products/${id}/out`, { method: "POST" }),
  adjustPantryProduct: (id: string, quantity: number, note?: string) =>
    request<PantryProduct>(`/pantry/products/${id}/adjust`, {
      method: "POST",
      body: JSON.stringify({ quantity, note: note ?? "" }),
    }),
  // Converte a quantidade em estoque (unidades da mesma dimensão física) ou
  // zera o estoque com um movimento de ajuste (quando não há conversão
  // possível, ex: kg -> un) — ver pantry.Service.ChangeUnit no backend.
  changePantryProductUnit: (id: string, unit: PantryProductUnit) =>
    request<PantryProduct>(`/pantry/products/${id}/change-unit`, {
      method: "POST",
      body: JSON.stringify({ unit }),
    }),
  consumePantryProductFromText: (text: string) =>
    request<ConsumePantryTextResult>("/pantry/products/consume-text", {
      method: "POST",
      body: JSON.stringify({ text }),
    }),

  getPantryAIUsage: () => request<PantryAIUsage>("/pantry/ai-usage"),

  listPantrySeedProducts: () =>
    request<{ seed_products: PantrySeedProduct[] }>("/pantry/seed-products"),

  createPantryReceipt: (input: CreatePantryReceiptInput) => {
    const form = new FormData();
    form.set("image", input.image);
    if (input.instruction) form.set("instruction", input.instruction);
    if (input.country) form.set("country", input.country);
    return request<{ receipt: PantryReceipt; items: PantryReceiptItem[] }>("/pantry/receipts", {
      method: "POST",
      body: form,
    });
  },
  listPantryReceipts: (status?: PantryReceiptStatus) =>
    request<{ receipts: PantryReceipt[] }>(`/pantry/receipts${status ? `?status=${status}` : ""}`),
  getPantryReceipt: (id: string) =>
    request<{ receipt: PantryReceipt; items: PantryReceiptItem[] }>(`/pantry/receipts/${id}`),
  updatePantryReceiptItems: (id: string, items: UpdatePantryReceiptItemInput[]) =>
    request<{ items: PantryReceiptItem[] }>(`/pantry/receipts/${id}/items`, {
      method: "PUT",
      body: JSON.stringify({ items }),
    }),
  confirmPantryReceipt: (id: string, input: ConfirmPantryReceiptInput) =>
    request<ConfirmPantryReceiptResult>(`/pantry/receipts/${id}/confirm`, {
      method: "POST",
      body: JSON.stringify(input),
    }),
  discardPantryReceipt: (id: string) => request<void>(`/pantry/receipts/${id}`, { method: "DELETE" }),

  // Sem status: o backend usa "open" como padrão (tela principal da lista).
  // "all" pede todos os status explicitamente.
  listPantryShoppingItems: (status?: PantryShoppingStatus | "all") =>
    request<{ items: PantryShoppingItem[] }>(
      `/pantry/shopping-list${status ? `?status=${status}` : ""}`
    ),
  addPantryShoppingItem: (input: AddPantryShoppingItemInput) =>
    request<PantryShoppingItem>("/pantry/shopping-list", { method: "POST", body: JSON.stringify(input) }),
  updatePantryShoppingItemStatus: (id: string, status: PantryShoppingStatus, quantity?: number) =>
    request<{ id: string; status: PantryShoppingStatus }>(`/pantry/shopping-list/${id}`, {
      method: "PATCH",
      body: JSON.stringify({ status, quantity }),
    }),
  regeneratePantryShoppingList: () =>
    request<{ items: PantryShoppingItem[] }>("/pantry/shopping-list/regenerate", { method: "POST" }),

  listPaymentMethods: () => request<PaymentMethod[]>("/payment-methods"),

  generatePantryRecipe: (input: GenerateRecipeInput) =>
    request<GenerateRecipeResult>("/pantry/recipes/generate", { method: "POST", body: JSON.stringify(input) }),
  savePantryRecipe: (recipe: PantryRecipe) =>
    request<PantryRecipe>("/pantry/recipes", { method: "POST", body: JSON.stringify(recipe) }),
  listPantryRecipes: (favoriteOnly?: boolean) =>
    request<{ recipes: PantryRecipe[] }>(`/pantry/recipes${favoriteOnly ? "?favorite=true" : ""}`),
  getPantryRecipe: (id: string) => request<PantryRecipe>(`/pantry/recipes/${id}`),
  deletePantryRecipe: (id: string) => request<void>(`/pantry/recipes/${id}`, { method: "DELETE" }),
  previewCookPantryRecipe: (id: string, servings?: number) =>
    request<CookPreviewResult>(`/pantry/recipes/${id}/cook/preview`, {
      method: "POST",
      body: JSON.stringify({ servings: servings ?? 0 }),
    }),
  cookPantryRecipe: (id: string, servings: number, deductions: ConfirmCookDeduction[]) =>
    request<CookPantryRecipeResult>(`/pantry/recipes/${id}/cook`, {
      method: "POST",
      body: JSON.stringify({ servings, deductions }),
    }),
  addMissingRecipeIngredientsToList: (recipeId: string) =>
    request<{ added: number }>("/pantry/shopping-list/from-recipe", {
      method: "POST",
      body: JSON.stringify({ recipe_id: recipeId }),
    }),
};
