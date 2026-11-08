package models

import (
	"time"

	"github.com/google/uuid"
)

// Pantry language, country and currency values accepted by the dispensa module.
// The catalog and the recipes are shown in the user's language; the country tells
// the recipe generator which ingredients are easy to find locally.
const (
	PantryLanguagePtBR = "pt-BR"
	PantryLanguageDe   = "de"

	PantryCountryBR = "BR"
	PantryCountryDE = "DE"

	PantryCurrencyBRL = "BRL"
	PantryCurrencyEUR = "EUR"

	PantryDefaultShoppingIntervalDays = 7
	PantryMaxShoppingIntervalDays     = 60
)

// Pantry product categories and subcategories. Subcategories are free text on the
// database side but the AI prompts and the UI use this fixed list.
const (
	PantryCategoryFood     = "food"
	PantryCategoryCleaning = "cleaning"
	PantryCategoryHygiene  = "hygiene"
	PantryCategoryGeneral  = "general"
)

// Pantry movement types. Quantities carry the sign (positive for entries, negative
// for exits), so the type alone tells the origin of the movement.
const (
	PantryMovementReceiptIn = "receipt_in"
	PantryMovementManualIn  = "manual_in"
	PantryMovementManualOut = "manual_out"
	PantryMovementRecipeOut = "recipe_out"
	PantryMovementAdjust    = "adjust"
)

// PantryUnits lists the units a pantry quantity can be expressed in.
var PantryUnits = []string{"un", "kg", "g", "l", "ml", "pct", "maco", "dz"}

// Pantry product alias sources: how the raw-name-to-product mapping was created.
const (
	PantryAliasSourceUser = "user"
	PantryAliasSourceAI   = "ai"
	PantryAliasSourceSeed = "seed"
)

// Pantry receipt statuses.
const (
	PantryReceiptPendingReview = "pending_review"
	PantryReceiptConfirmed     = "confirmed"
	PantryReceiptDiscarded     = "discarded"
)

// Pantry receipt item match sources: how the raw receipt line was matched to a
// catalog product.
const (
	PantryMatchAlias   = "alias"
	PantryMatchProduct = "product"
	PantryMatchSeed    = "seed"
	PantryMatchAI      = "ai"
	PantryMatchNone    = "none"
)

// Pantry receipt item ignore reasons: why a line is excluded from stock entries.
const (
	PantryIgnoreDeposit    = "deposit"
	PantryIgnoreDiscount   = "discount"
	PantryIgnoreTax        = "tax"
	PantryIgnoreTotal      = "total"
	PantryIgnorePayment    = "payment"
	PantryIgnoreNotProduct = "not_product"
	PantryIgnoreUser       = "user"
)

// PantrySettings stores per-user preferences of the dispensa module. One row per user;
// the pantry itself is per user (not per shared account).
type PantrySettings struct {
	UserID               uuid.UUID `gorm:"primarykey;type:uuid"          json:"user_id"`
	Language             string    `gorm:"not null;default:'pt-BR'"      json:"language"`
	Country              string    `gorm:"not null;default:'BR'"         json:"country"`
	Currency             string    `gorm:"not null;default:'BRL'"        json:"currency"`
	ShoppingIntervalDays int       `gorm:"not null;default:7"            json:"shopping_interval_days"`
	AskProduceUnits      bool      `gorm:"not null;default:true"         json:"ask_produce_units"`
	UpdatedAt            time.Time `gorm:"not null;autoUpdateTime"       json:"updated_at"`
}

func (PantrySettings) TableName() string { return "pantry_settings" }

// DefaultPantrySettings returns the settings a user starts with.
func DefaultPantrySettings(userID uuid.UUID) PantrySettings {
	return PantrySettings{
		UserID:               userID,
		Language:             PantryLanguagePtBR,
		Country:              PantryCountryBR,
		Currency:             PantryCurrencyBRL,
		ShoppingIntervalDays: PantryDefaultShoppingIntervalDays,
		AskProduceUnits:      true,
	}
}

// PantrySeedProduct is a global, read-only catalog entry used to bootstrap product
// matching (Brazilian and German receipt names) before asking the AI.
// Aliases are stored as JSON arrays of normalized names.
type PantrySeedProduct struct {
	ID            uuid.UUID `gorm:"primarykey;type:uuid;default:gen_random_uuid()" json:"id"`
	NamePt        string    `gorm:"not null;uniqueIndex"                           json:"name_pt"`
	NameDe        string    `gorm:"not null;default:''"                            json:"name_de"`
	AliasesPt     []string  `gorm:"type:jsonb;serializer:json"                     json:"aliases_pt"`
	AliasesDe     []string  `gorm:"type:jsonb;serializer:json"                     json:"aliases_de"`
	Category      string    `gorm:"not null"                                       json:"category"`
	Subcategory   string    `gorm:"not null;default:''"                            json:"subcategory"`
	DefaultUnit   string    `gorm:"not null;default:'un'"                          json:"default_unit"`
	IsProduce     bool      `gorm:"not null;default:false"                         json:"is_produce"`
	ShelfLifeDays *int      `                                                      json:"shelf_life_days,omitempty"`
	CreatedAt     time.Time `                                                      json:"created_at"`
}

func (PantrySeedProduct) TableName() string { return "pantry_seed_products" }

// PantryProduct is the canonical catalog entry of a user's pantry, created from a
// seed match, an AI suggestion or manual input.
type PantryProduct struct {
	ID            uuid.UUID `gorm:"primarykey;type:uuid;default:gen_random_uuid()" json:"id"`
	UserID        uuid.UUID `gorm:"not null;index;type:uuid"                       json:"user_id"`
	NamePt        string    `gorm:"not null"                                       json:"name_pt"`
	NameDe        string    `gorm:"not null;default:''"                            json:"name_de"`
	Category      string    `gorm:"not null"                                       json:"category"`
	Subcategory   string    `gorm:"not null;default:''"                            json:"subcategory"`
	DefaultUnit   string    `gorm:"not null"                                       json:"default_unit"`
	IsProduce     bool      `gorm:"not null;default:false"                         json:"is_produce"`
	TrackByUnit   bool      `gorm:"not null;default:false"                         json:"track_by_unit"`
	MinLevel      *float64  `gorm:"type:numeric(12,3)"                             json:"min_level,omitempty"`
	ShelfLifeDays *int      `                                                      json:"shelf_life_days,omitempty"`
	CreatedAt     time.Time `gorm:"not null;autoCreateTime"                        json:"created_at"`
	UpdatedAt     time.Time `gorm:"not null;autoUpdateTime"                        json:"updated_at"`
}

func (PantryProduct) TableName() string { return "pantry_products" }

// PantryStock is the current quantity on hand for a product. One row per product;
// the row is created on the first movement, so a missing row means empty stock.
type PantryStock struct {
	ProductID       uuid.UUID  `gorm:"primarykey;type:uuid"      json:"product_id"`
	UserID          uuid.UUID  `gorm:"not null;index;type:uuid"  json:"user_id"`
	Quantity        float64    `gorm:"not null;default:0;type:numeric(12,3)" json:"quantity"`
	Unit            string     `gorm:"not null"                  json:"unit"`
	AvgPrice        *float64   `gorm:"type:numeric(12,2)"        json:"avg_price,omitempty"`
	Currency        string     `gorm:"not null;default:'BRL'"    json:"currency"`
	LastPurchasedAt *time.Time `                                 json:"last_purchased_at,omitempty"`
	// ExpiresAt is the estimated expiration of the most recent entry
	// (receipt or manual), computed from the product's ShelfLifeDays. The
	// stock model is an aggregate (no per-lot/FIFO tracking), so this is a
	// simplification: a newer entry always overwrites it, even if older
	// stock with an earlier expiry is still on hand.
	ExpiresAt *time.Time `                                 json:"expires_at,omitempty"`
	UpdatedAt time.Time  `gorm:"not null;autoUpdateTime"   json:"updated_at"`
}

func (PantryStock) TableName() string { return "pantry_stock" }

// PantryMovement is an append-only entry in the stock history of a product.
// Quantity carries the sign: positive for entries, negative for exits.
type PantryMovement struct {
	ID            uuid.UUID  `gorm:"primarykey;type:uuid;default:gen_random_uuid()" json:"id"`
	UserID        uuid.UUID  `gorm:"not null;type:uuid"                             json:"user_id"`
	ProductID     uuid.UUID  `gorm:"not null;type:uuid"                             json:"product_id"`
	Type          string     `gorm:"not null"                                       json:"type"`
	Quantity      float64    `gorm:"not null;type:numeric(12,3)"                    json:"quantity"`
	Unit          string     `gorm:"not null"                                       json:"unit"`
	UnitPrice     *float64   `gorm:"type:numeric(12,2)"                             json:"unit_price,omitempty"`
	Currency      *string    `                                                      json:"currency,omitempty"`
	ReferenceType *string    `                                                      json:"reference_type,omitempty"`
	ReferenceID   *uuid.UUID `gorm:"type:uuid"                                      json:"reference_id,omitempty"`
	Note          string     `gorm:"not null;default:''"                            json:"note"`
	OccurredAt    time.Time  `gorm:"not null"                                       json:"occurred_at"`
	CreatedAt     time.Time  `gorm:"not null;autoCreateTime"                        json:"created_at"`
}

func (PantryMovement) TableName() string { return "pantry_movements" }

// PantryProductAlias maps a raw name found on a receipt (normalized) to a
// canonical product, so the same OCR/AI text resolves without asking again.
type PantryProductAlias struct {
	ID        uuid.UUID `gorm:"primarykey;type:uuid;default:gen_random_uuid()" json:"id"`
	UserID    uuid.UUID `gorm:"not null;index;type:uuid"                       json:"user_id"`
	ProductID uuid.UUID `gorm:"not null;type:uuid"                             json:"product_id"`
	RawName   string    `gorm:"not null"                                       json:"raw_name"` // normalized: see normalizeRawName in the repository
	Lang      string    `gorm:"not null;default:''"                            json:"lang"`     // "pt" | "de" | ""
	Source    string    `gorm:"not null"                                       json:"source"`   // "user" | "ai" | "seed"
	CreatedAt time.Time `gorm:"not null;autoCreateTime"                        json:"created_at"`
}

func (PantryProductAlias) TableName() string { return "pantry_product_aliases" }

// PantryReceipt is a grocery receipt read by the AI, pending user review before
// it is confirmed into pantry stock and a financial transaction.
type PantryReceipt struct {
	ID             uuid.UUID  `gorm:"primarykey;type:uuid;default:gen_random_uuid()" json:"id"`
	UserID         uuid.UUID  `gorm:"not null;index;type:uuid"                       json:"user_id"`
	AccountID      uuid.UUID  `gorm:"not null;index;type:uuid"                       json:"account_id"` // financial account used for the transaction on confirmation
	Status         string     `gorm:"not null;default:'pending_review'"              json:"status"`     // "pending_review" | "confirmed" | "discarded"
	ImageKey       string     `gorm:"not null"                                       json:"image_key"`
	Country        string     `gorm:"not null;default:'BR'"                          json:"country"`
	Currency       string     `gorm:"not null;default:'BRL'"                         json:"currency"`
	Locale         string     `gorm:"not null;default:'BR'"                          json:"locale"`
	Merchant       string     `gorm:"not null;default:''"                            json:"merchant"`
	PurchasedAt    time.Time  `gorm:"not null"                                       json:"purchased_at"`
	Total          *float64   `gorm:"type:numeric(12,2)"                             json:"total,omitempty"`
	AccessKey      string     `gorm:"not null;default:''"                            json:"access_key"`
	AIModel        string     `gorm:"not null;default:''"                            json:"ai_model"`
	AIInputTokens  int        `gorm:"not null;default:0"                             json:"ai_input_tokens"`
	AIOutputTokens int        `gorm:"not null;default:0"                             json:"ai_output_tokens"`
	RawAIJSON      string     `gorm:"type:text;not null;default:''"                  json:"-"`                         // raw AI response, audit/debug only — never exposed publicly
	TransactionIDs []string   `gorm:"type:jsonb;serializer:json"                     json:"transaction_ids,omitempty"` // ids of the financial transactions created on confirmation
	CreatedAt      time.Time  `gorm:"not null;autoCreateTime"                        json:"created_at"`
	ConfirmedAt    *time.Time `                                                      json:"confirmed_at,omitempty"`
}

func (PantryReceipt) TableName() string { return "pantry_receipts" }

// PantryReceiptItem is one line of a PantryReceipt, matched (or not) to a
// catalog product and editable by the user before confirmation.
type PantryReceiptItem struct {
	ID             uuid.UUID  `gorm:"primarykey;type:uuid;default:gen_random_uuid()" json:"id"`
	ReceiptID      uuid.UUID  `gorm:"not null;index;type:uuid"                       json:"receipt_id"`
	UserID         uuid.UUID  `gorm:"not null;index;type:uuid"                       json:"user_id"`
	LineNo         int        `gorm:"not null"                                       json:"line_no"`
	RawName        string     `gorm:"not null"                                       json:"raw_name"`
	NamePt         string     `gorm:"not null;default:''"                            json:"name_pt"`
	NameDe         string     `gorm:"not null;default:''"                            json:"name_de"`
	ProductID      *uuid.UUID `gorm:"type:uuid"                                      json:"product_id,omitempty"`
	MatchSource    string     `gorm:"not null;default:''"                            json:"match_source"` // "alias" | "product" | "seed" | "ai" | "none"
	Category       string     `gorm:"not null;default:''"                            json:"category"`
	Subcategory    string     `gorm:"not null;default:''"                            json:"subcategory"`
	Quantity       float64    `gorm:"not null;type:numeric(12,3)"                    json:"quantity"`
	Unit           string     `gorm:"not null"                                       json:"unit"`
	UnitPrice      *float64   `gorm:"type:numeric(12,2)"                             json:"unit_price,omitempty"`
	TotalPrice     *float64   `gorm:"type:numeric(12,2)"                             json:"total_price,omitempty"`
	IsProduce      bool       `gorm:"not null;default:false"                         json:"is_produce"`
	NeedsUnitCount bool       `gorm:"not null;default:false"                         json:"needs_unit_count"`
	UnitCount      *float64   `gorm:"type:numeric(12,3)"                             json:"unit_count,omitempty"`
	KeepWeight     bool       `gorm:"not null;default:false"                         json:"keep_weight"`
	Ignored        bool       `gorm:"not null;default:false"                         json:"ignored"`
	IgnoreReason   string     `gorm:"not null;default:''"                            json:"ignore_reason"` // "" | "deposit" | "discount" | "tax" | "total" | "payment" | "not_product" | "user"
	Confidence     float64    `gorm:"not null;default:0"                             json:"confidence"`
}

func (PantryReceiptItem) TableName() string { return "pantry_receipt_items" }

// Pantry shopping list item origins: how the item was added to the list.
const (
	PantryShoppingOriginAuto   = "auto"
	PantryShoppingOriginManual = "manual"
	PantryShoppingOriginRecipe = "recipe"
)

// Pantry shopping list item reasons: why the automatic regeneration added the
// item. Only relevant for origin = "auto".
const (
	PantryShoppingReasonOut        = "out"
	PantryShoppingReasonBelowMin   = "below_min"
	PantryShoppingReasonRunningOut = "running_out"
)

// Pantry shopping list item statuses.
const (
	PantryShoppingStatusOpen      = "open"
	PantryShoppingStatusBought    = "bought"
	PantryShoppingStatusDismissed = "dismissed"
)

// PantryShoppingItem is one entry of a user's shopping list: automatically
// suggested from low/empty stock, added manually, or generated from a recipe
// (recipe generation lands in a later stage). An open item for the same
// product is unique (see the partial index in the migration), so at most one
// open entry per product exists at a time regardless of origin.
type PantryShoppingItem struct {
	ID                  uuid.UUID  `gorm:"primarykey;type:uuid;default:gen_random_uuid()" json:"id"`
	UserID              uuid.UUID  `gorm:"not null;index;type:uuid"                       json:"user_id"`
	ProductID           *uuid.UUID `gorm:"type:uuid"                                      json:"product_id,omitempty"` // nil para item manual sem produto do catálogo
	Name                string     `gorm:"not null"                                       json:"name"`
	Quantity            *float64   `gorm:"type:numeric(12,3)"                             json:"quantity,omitempty"`
	Unit                string     `gorm:"not null;default:''"                            json:"unit"`
	Origin              string     `gorm:"not null"                                       json:"origin"` // "auto" | "manual" | "recipe"
	Reason              string     `gorm:"not null;default:''"                            json:"reason"` // "" | "out" | "below_min"
	Status              string     `gorm:"not null;default:'open'"                        json:"status"` // "open" | "bought" | "dismissed"
	BoughtReceiptItemID *uuid.UUID `gorm:"type:uuid"                                      json:"bought_receipt_item_id,omitempty"`
	CreatedAt           time.Time  `gorm:"not null;autoCreateTime"                        json:"created_at"`
	BoughtAt            *time.Time `                                                      json:"bought_at,omitempty"`
}

func (PantryShoppingItem) TableName() string { return "pantry_shopping_items" }

// Pantry recipe sources: how the recipe was created.
const (
	PantryRecipeSourceAI   = "ai"
	PantryRecipeSourceUser = "user"
)

// PantryRecipeIngredient is one entry of a recipe's ingredient list, stored as
// JSON inside PantryRecipe.Ingredients. ProductID links it to the user's
// catalog when the ingredient matches a known product; Have/HaveQuantity are
// always recomputed from live stock when the recipe is read, never trusted
// from what was stored — see the recipe service (a later task).
type PantryRecipeIngredient struct {
	Name         string     `json:"name"`
	ProductID    *uuid.UUID `json:"product_id,omitempty"`
	Quantity     float64    `json:"quantity"`
	Unit         string     `json:"unit"`
	Have         bool       `json:"have"`
	HaveQuantity float64    `json:"have_quantity"`
	Optional     bool       `json:"optional"`
}

// PantryRecipe is a cooking recipe suggested by the AI (or created by the
// user) from the pantry's stock, with its ingredient list and steps.
type PantryRecipe struct {
	ID             uuid.UUID                `gorm:"primarykey;type:uuid;default:gen_random_uuid()" json:"id"`
	UserID         uuid.UUID                `gorm:"not null;index;type:uuid"                       json:"user_id"`
	Title          string                   `gorm:"not null"                                       json:"title"`
	Language       string                   `gorm:"not null;default:'pt-BR'"                       json:"language"`
	Servings       int                      `gorm:"not null;default:1"                             json:"servings"`
	PrepMinutes    int                      `gorm:"not null;default:0"                             json:"prep_minutes"`
	Filters        []string                 `gorm:"type:jsonb;serializer:json"                     json:"filters"`
	Ingredients    []PantryRecipeIngredient `gorm:"type:jsonb;serializer:json"                     json:"ingredients"`
	Steps          []string                 `gorm:"type:jsonb;serializer:json"                     json:"steps"`
	Tips           string                   `gorm:"not null;default:''"                            json:"tips"`
	IsFavorite     bool                     `gorm:"not null;default:false"                         json:"is_favorite"`
	Source         string                   `gorm:"not null;default:'ai'"                          json:"source"` // "ai" | "user"
	AIModel        string                   `gorm:"not null;default:''"                            json:"ai_model"`
	AIInputTokens  int                      `gorm:"not null;default:0"                             json:"ai_input_tokens"`
	AIOutputTokens int                      `gorm:"not null;default:0"                             json:"ai_output_tokens"`
	CreatedAt      time.Time                `gorm:"not null;autoCreateTime"                        json:"created_at"`
}

func (PantryRecipe) TableName() string { return "pantry_recipes" }

// PantryRecipeDeduction is one stock deduction caused by cooking a recipe,
// stored as JSON inside PantryRecipeLog.Deductions — the exact quantities
// actually discounted (which may differ from the recipe's listed quantities,
// e.g. scaled to a different serving count or adjusted by the user before
// confirming).
type PantryRecipeDeduction struct {
	ProductID uuid.UUID `json:"product_id"`
	Quantity  float64   `json:"quantity"`
	Unit      string    `json:"unit"`
}

// PantryRecipeLog records that the user cooked a recipe ("fiz essa receita"),
// with the stock deductions it caused. Each log generates N PantryMovement
// rows of type "recipe_out".
type PantryRecipeLog struct {
	ID         uuid.UUID               `gorm:"primarykey;type:uuid;default:gen_random_uuid()" json:"id"`
	UserID     uuid.UUID               `gorm:"not null;index;type:uuid"                       json:"user_id"`
	RecipeID   uuid.UUID               `gorm:"not null;index;type:uuid"                       json:"recipe_id"`
	CookedAt   time.Time               `gorm:"not null"                                       json:"cooked_at"`
	Servings   int                     `gorm:"not null;default:1"                             json:"servings"`
	Deductions []PantryRecipeDeduction `gorm:"type:jsonb;serializer:json"                     json:"deductions"`
	CreatedAt  time.Time               `gorm:"not null;autoCreateTime"                        json:"created_at"`
}

func (PantryRecipeLog) TableName() string { return "pantry_recipe_logs" }
