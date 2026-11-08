package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	walib "github.com/go-webauthn/webauthn/webauthn"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/marianamohr/financas/config"
	"github.com/marianamohr/financas/internal/agent"
	"github.com/marianamohr/financas/internal/auth"
	"github.com/marianamohr/financas/internal/currency"
	"github.com/marianamohr/financas/internal/handlers"
	"github.com/marianamohr/financas/internal/middleware"
	"github.com/marianamohr/financas/internal/repository"
	aiservice "github.com/marianamohr/financas/internal/services/ai"
	"github.com/marianamohr/financas/internal/services/pantry"
	"github.com/marianamohr/financas/internal/services/rag"
	"github.com/marianamohr/financas/internal/services/storage"
	"github.com/marianamohr/financas/internal/services/transactions"
	"github.com/marianamohr/financas/internal/services/vision"
	"github.com/marianamohr/financas/migrations"
)

func main() {
	// Structured JSON logs in production; pretty in development.
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	if os.Getenv("APP_ENV") != "production" {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	}

	// ── Config ──────────────────────────────────────────────────────────────
	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("configuration error")
	}

	// ── Database ─────────────────────────────────────────────────────────────
	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to database")
	}

	sqlDB, err := db.DB()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to get sql.DB")
	}
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)

	// ── Migrations ───────────────────────────────────────────────────────────
	if os.Getenv("SKIP_MIGRATIONS") != "true" {
		if err := migrations.Run(db); err != nil {
			log.Fatal().Err(err).Msg("failed to run migrations")
		}
		log.Info().Msg("migrations applied")
	} else {
		log.Info().Msg("migrations skipped (SKIP_MIGRATIONS=true)")
	}

	// ── Auth ─────────────────────────────────────────────────────────────────
	jwtManager, err := auth.NewManager(cfg.JWTPrivateKey, cfg.JWTPublicKey)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to initialise JWT manager")
	}

	secure := os.Getenv("APP_ENV") == "production"

	// ── Repositories ─────────────────────────────────────────────────────────
	userRepo := repository.NewUserRepository(db)
	accountRepo := repository.NewAccountRepository(db)
	inviteRepo := repository.NewInviteRepository(db)
	txRepo := repository.NewTransactionRepository(db)
	chatRepo := repository.NewChatRepository(db)
	passkeyRepo := repository.NewPasskeyRepository(db)
	aliasRepo := repository.NewMerchantAliasRepository(db)
	goalRepo := repository.NewGoalRepository(db)
	limitRepo := repository.NewCategoryLimitRepository(db)
	pendingRepo := repository.NewPendingActionRepository(db)
	insightRepo := repository.NewInsightRepository(db)
	catRepo := repository.NewCategoryRepository(db)
	pmRepo := repository.NewPaymentMethodRepository(db)
	pantrySettingsRepo := repository.NewPantrySettingsRepository(db)
	pantryProductRepo := repository.NewPantryProductRepository(db)
	pantryStockRepo := repository.NewPantryStockRepository(db)
	pantryMovementRepo := repository.NewPantryMovementRepository(db)
	pantryAliasRepo := repository.NewPantryProductAliasRepository(db)
	pantrySeedProductRepo := repository.NewPantrySeedProductRepository(db)
	pantryReceiptRepo := repository.NewPantryReceiptRepository(db)
	pantryReceiptItemRepo := repository.NewPantryReceiptItemRepository(db)
	pantryShoppingItemRepo := repository.NewPantryShoppingItemRepository(db)
	pantryRecipeRepo := repository.NewPantryRecipeRepository(db)
	pantryRecipeLogRepo := repository.NewPantryRecipeLogRepository(db)

	// ── Services ─────────────────────────────────────────────────────────────
	aiProvider := aiservice.NewAnthropicProvider(cfg.AnthropicAPIKey)
	ragBuilder := rag.NewBuilder(txRepo, goalRepo, catRepo, pmRepo)
	visionExtractor := vision.NewExtractor(aiProvider)
	currencyService := currency.NewService(db, cfg.ExchangeRateAPIURL)
	currencyHandler := currency.NewHandler(currencyService)

	var storageBackend storage.Storage

	if cfg.AWSEndpointURL != "" && cfg.AWSAccessKeyID != "" {
		log.Info().
			Str("endpoint", cfg.AWSEndpointURL).
			Str("region", cfg.AWSRegion).
			Str("bucket", cfg.AWSS3BucketName).
			Str("access_key_id", cfg.AWSAccessKeyID).
			Str("secret_key_set", boolStr(cfg.AWSSecretKey != "")).
			Msg("storage: connecting to S3 bucket...")

		s3Client, err := storage.NewClient(
			cfg.AWSEndpointURL,
			cfg.AWSRegion,
			cfg.AWSAccessKeyID,
			cfg.AWSSecretKey,
			cfg.AWSS3BucketName,
		)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[FATAL] storage: failed to connect to S3 bucket\n")
			fmt.Fprintf(os.Stderr, "  error:          %v\n", err)
			fmt.Fprintf(os.Stderr, "  endpoint:       %s\n", cfg.AWSEndpointURL)
			fmt.Fprintf(os.Stderr, "  region:         %s\n", cfg.AWSRegion)
			fmt.Fprintf(os.Stderr, "  bucket:         %s\n", cfg.AWSS3BucketName)
			fmt.Fprintf(os.Stderr, "  access_key_id:  %s\n", cfg.AWSAccessKeyID)
			fmt.Fprintf(os.Stderr, "  secret_key_set: %s\n", boolStr(cfg.AWSSecretKey != ""))
			os.Exit(1)
		}
		storageBackend = s3Client
		log.Info().
			Str("bucket", cfg.AWSS3BucketName).
			Str("endpoint", cfg.AWSEndpointURL).
			Msg("storage: S3 bucket connected successfully")
	} else {
		localClient, err := storage.NewLocalClient(cfg.LocalUploadDir, cfg.LocalUploadURL)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to initialise local storage")
		}
		storageBackend = localClient
		log.Info().Str("dir", cfg.LocalUploadDir).Msg("storage: using local disk")
	}

	// ── WebAuthn ─────────────────────────────────────────────────────────────
	var webAuthnHandler *handlers.WebAuthnHandler
	if cfg.WebAuthnRPID != "" && cfg.WebAuthnRPOrigin != "" {
		wa, err := walib.New(&walib.Config{
			RPID:          cfg.WebAuthnRPID,
			RPDisplayName: cfg.WebAuthnRPName,
			RPOrigins:     []string{cfg.WebAuthnRPOrigin},
		})
		if err != nil {
			log.Fatal().Err(err).Msg("failed to initialise WebAuthn")
		}
		webAuthnHandler = handlers.NewWebAuthnHandler(wa, passkeyRepo, userRepo, jwtManager)
		log.Info().Str("rpid", cfg.WebAuthnRPID).Msg("WebAuthn passkeys enabled")
	} else {
		log.Warn().Msg("WebAuthn disabled — set WEBAUTHN_RPID and WEBAUTHN_RPORIGIN to enable passkeys")
	}

	// ── Agent registry ───────────────────────────────────────────────────────
	readTools := []agent.ReadTool{
		agent.NewGetTransactionsTool(txRepo),
		agent.NewGetSpendingSummaryTool(txRepo),
		agent.NewGetCategoriesTool(txRepo),
		agent.NewGetMerchantAliasesTool(aliasRepo),
		agent.NewGetGoalsTool(goalRepo),
		agent.NewGetGoalSuggestionsTool(goalRepo),
		agent.NewComparePeriodsTool(txRepo),
		agent.NewProjectBalanceTool(txRepo),
		agent.NewDetectAnomaliesTool(txRepo),
	}
	txValidator := transactions.NewValidator(catRepo, pmRepo)
	installmentRepo := repository.NewInstallmentPurchaseRepository(db)
	writeTools := []agent.WriteTool{
		agent.NewCreateTransactionTool(txRepo, catRepo, currencyService, pmRepo, pendingRepo, txValidator),
		agent.NewUpdateTransactionCategoryTool(txRepo, catRepo),
		agent.NewCreateMerchantAliasTool(aliasRepo),
		agent.NewCreateGoalTool(goalRepo),
		agent.NewUpdateGoalProgressTool(goalRepo),
		agent.NewAddGoalTransactionTool(goalRepo, txRepo, currencyService),
		agent.NewAddTransactionNoteTool(txRepo),
		agent.NewReplaceTransactionTool(txRepo, catRepo, currencyService),
		agent.NewCreateInstallmentPurchaseTool(installmentRepo, pmRepo, catRepo),
	}
	agentRegistry := agent.NewRegistry(readTools, writeTools)

	// ── Handlers ─────────────────────────────────────────────────────────────
	authHandler := handlers.NewAuthHandler(userRepo, accountRepo, inviteRepo, jwtManager, pmRepo, secure)
	txHandler := handlers.NewTransactionHandler(txRepo, catRepo, pmRepo, txValidator)
	paymentMethodHandler := handlers.NewPaymentMethodHandler(pmRepo, txRepo, catRepo)
	chatHandler := handlers.NewChatHandler(aiProvider, ragBuilder, chatRepo, txRepo, userRepo, agentRegistry, pendingRepo, currencyService, pmRepo, txValidator)
	uploadHandler := handlers.NewUploadHandler(visionExtractor, storageBackend, txRepo, userRepo, currencyService, aliasRepo, pendingRepo, catRepo, pmRepo, installmentRepo, txValidator)
	agentHandler := handlers.NewAgentHandler(agentRegistry, pendingRepo, insightRepo, txRepo, pmRepo, catRepo, installmentRepo, chatRepo, txValidator)
	imageHandler := handlers.NewImageHandler(storageBackend)
	userHandler := handlers.NewUserHandler(userRepo)
	adminHandler := handlers.NewAdminHandler(userRepo, inviteRepo, cfg.FrontendURL)
	aliasHandler := handlers.NewMerchantAliasHandler(aliasRepo)
	goalHandler := handlers.NewGoalHandler(goalRepo, txRepo, currencyService)
	groupRepo := repository.NewLimitGroupRepository(db)
	groupHandler := handlers.NewLimitGroupHandler(groupRepo)
	limitHandler := handlers.NewCategoryLimitHandler(limitRepo, groupRepo, txRepo, catRepo)
	recurringRepo := repository.NewRecurringExpenseRepository(db)
	recurringHandler := handlers.NewRecurringExpenseHandler(recurringRepo, txRepo, catRepo, pmRepo, txValidator)
	installmentHandler := handlers.NewInstallmentPurchaseHandler(installmentRepo, pmRepo, catRepo)
	dashboardHandler := handlers.NewDashboardHandler(txRepo, recurringRepo, catRepo, pmRepo)
	cashflowHandler := handlers.NewCashflowHandler(txRepo)
	pantrySettingsHandler := handlers.NewPantrySettingsHandler(pantrySettingsRepo)
	pantrySeedProductHandler := handlers.NewPantrySeedProductHandler(pantrySeedProductRepo)
	pantryAIUsageHandler := handlers.NewPantryAIUsageHandler(pantryReceiptRepo, pantryRecipeRepo)
	pantryService := pantry.NewService(pantryProductRepo, pantryStockRepo, pantryMovementRepo, db)
	pantryShoppingListService := pantry.NewShoppingListService(
		pantryProductRepo,
		pantryStockRepo,
		pantryShoppingItemRepo,
		pantryMovementRepo,
		pantrySettingsRepo,
	)
	pantryConsumeTextService := pantry.NewConsumeTextService(
		aiProvider,
		pantryProductRepo,
		pantryAliasRepo,
		pantryService,
	)
	pantryProductHandler := handlers.NewPantryProductHandler(
		pantryService,
		pantryMovementRepo,
		pantryShoppingListService,
		pantrySettingsRepo,
		pantryConsumeTextService,
	)
	pantryRecipeService := pantry.NewRecipeService(
		aiProvider,
		pantryProductRepo,
		pantryStockRepo,
		pantrySettingsRepo,
		pantryRecipeRepo,
		pantryMovementRepo,
	)
	pantryShoppingListHandler := handlers.NewPantryShoppingListHandler(
		pantryShoppingListService,
		pantryShoppingItemRepo,
		pantryRecipeService,
		pantryRecipeRepo,
	)
	pantryRecipeHandler := handlers.NewPantryRecipeHandler(
		pantryRecipeService,
		pantryRecipeRepo,
		pantryRecipeLogRepo,
		pantryProductRepo,
		pantryService,
		pantryShoppingListService,
	)
	pantryReceiptService := pantry.NewReceiptService(
		aiProvider,
		pantryProductRepo,
		pantryAliasRepo,
		pantrySeedProductRepo,
		pantrySettingsRepo,
		pantryReceiptRepo,
		pantryReceiptItemRepo,
		db,
	)
	pantryReceiptHandler := handlers.NewPantryReceiptHandler(
		pantryReceiptService,
		pantryReceiptRepo,
		pantryReceiptItemRepo,
		pantryAliasRepo,
		pantryProductRepo,
		pantryService,
		pantryShoppingListService,
		storageBackend,
		userRepo,
		catRepo,
		pmRepo,
		txRepo,
		currencyService,
		txValidator,
	)

	// ── Router ───────────────────────────────────────────────────────────────
	r := chi.NewRouter()

	// Global middlewares
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Recoverer)
	// Per-request timeout. Routes that run a synchronous Claude vision call
	// (/upload and the pantry receipt reader) can take ~30-60s plus the storage
	// upload, so they get a longer budget than the rest of the API. We must apply
	// exactly ONE timeout per request: nesting chimiddleware.Timeout doesn't work
	// because a child context.WithTimeout can only shorten a parent deadline,
	// never extend it — which is why a plain r.With(Timeout(120s)) override on
	// /upload was capped at the global 30s.
	longTimeoutPaths := map[string]bool{
		"/upload":                       true,
		"/pantry/receipts":              true,
		"/pantry/recipes/generate":      true,
		"/pantry/products/consume-text": true,
	}
	shortTimeout := chimiddleware.Timeout(30 * time.Second)
	longTimeout := chimiddleware.Timeout(120 * time.Second)
	r.Use(func(next http.Handler) http.Handler {
		short := shortTimeout(next)
		long := longTimeout(next)
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if req.Method == http.MethodPost && longTimeoutPaths[req.URL.Path] {
				long.ServeHTTP(w, req)
				return
			}
			short.ServeHTTP(w, req)
		})
	})
	r.Use(middleware.SecurityHeaders)
	r.Use(middleware.CORS(cfg.FrontendURLs...))

	// Health check — no auth required
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`)) //nolint:errcheck
	})

	// Serve local uploads in development (no-op in production when S3 is used)
	if cfg.AWSEndpointURL == "" {
		fs := http.StripPrefix("/uploads/", http.FileServer(http.Dir(cfg.LocalUploadDir)))
		r.Get("/uploads/*", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fs.ServeHTTP(w, r)
		}))
	}

	// Image proxy — public so <img> tags can load without a Bearer token.
	// Generates a short-lived presigned URL and redirects.
	r.Get("/image/{key}", imageHandler.Get)

	// Public auth routes
	r.Route("/auth", func(r chi.Router) {
		// Rate-limited register: 5 attempts / 1 hour per IP
		r.With(middleware.RegisterRateLimit(5, 3600)).Post("/register", authHandler.Register)
		// Rate-limited login: 5 attempts / 15 min per IP
		r.With(middleware.LoginRateLimit(cfg.RateLimitRequests, cfg.RateLimitSeconds)).
			Post("/login", authHandler.Login)
		r.Post("/refresh", authHandler.Refresh)
		r.Post("/logout", authHandler.Logout)
		r.Post("/invite/accept", authHandler.AcceptInvite)

		// Passkey / WebAuthn routes (only registered when WebAuthn is configured)
		if webAuthnHandler != nil {
			r.Post("/passkey/login/begin", webAuthnHandler.LoginBegin)
			r.Post("/passkey/login/finish", webAuthnHandler.LoginFinish)
			r.Group(func(r chi.Router) {
				r.Use(jwtManager.Middleware)
				r.Post("/passkey/register/begin", webAuthnHandler.RegisterBegin)
				r.Post("/passkey/register/finish", webAuthnHandler.RegisterFinish)
			})
		}
	})

	// Protected routes — all handlers below require a valid JWT
	r.Group(func(r chi.Router) {
		r.Use(jwtManager.Middleware)

		r.Get("/categories", txHandler.ListCategories)
		r.Get("/dashboard", dashboardHandler.Dashboard)
		r.Get("/cashflow", cashflowHandler.Cashflow)

		r.Route("/payment-methods", func(r chi.Router) {
			r.Get("/", paymentMethodHandler.List)
			r.Post("/", paymentMethodHandler.Create)
			r.Put("/{id}", paymentMethodHandler.Update)
			r.Delete("/{id}", paymentMethodHandler.Delete)
			r.Post("/{id}/invoices/{year}/{month}/pay", paymentMethodHandler.PayInvoice)
			r.Delete("/{id}/invoices/{year}/{month}/pay", paymentMethodHandler.UnpayInvoice)
		})

		// Transaction routes
		r.Route("/transactions", func(r chi.Router) {
			r.Get("/", txHandler.List)
			r.Post("/", txHandler.Create)
			r.Get("/summary", txHandler.Summary)
			r.Get("/payment-methods-analytics", txHandler.PaymentMethodAnalytics)
			r.Get("/expense-projection", txHandler.ExpenseProjection)
			r.Get("/expense-projection/detail", txHandler.ExpenseProjectionDetail)
			r.Get("/balance-transfer/pending", txHandler.BalanceTransferPending)
			r.Post("/balance-transfer", txHandler.ConfirmBalanceTransfer)
			r.Get("/voucher-deposit/pending", txHandler.VoucherDepositPending)
			r.Post("/voucher-deposit", txHandler.ConfirmVoucherDeposit)
			r.Get("/{id}", txHandler.Get)
			r.Put("/{id}", txHandler.Update)
			r.Delete("/{id}", txHandler.Delete)
		})

		r.Route("/merchant-aliases", func(r chi.Router) {
			r.Get("/", aliasHandler.List)
			r.Get("/pending-count", aliasHandler.PendingCount)
			r.Post("/", aliasHandler.Create)
			r.Post("/apply", aliasHandler.ApplyToTransactions)
			r.Put("/{id}", aliasHandler.Update)
			r.Delete("/{id}", aliasHandler.Delete)
		})

		r.Post("/chat", chatHandler.Chat)
		// Upload's longer (120s) timeout is applied by the per-request timeout
		// middleware above — see the comment on the global middleware block.
		r.Post("/upload", uploadHandler.Upload)

		r.Post("/agent/confirm", agentHandler.Confirm)
		r.Route("/agent/insights", func(r chi.Router) {
			r.Get("/", agentHandler.ListInsights)
			r.Patch("/{id}/read", agentHandler.MarkInsightRead)
		})

		// User settings
		r.Get("/user/me", userHandler.Me)
		r.Patch("/user/settings", userHandler.UpdateSettings)
		r.Post("/user/password", userHandler.ChangePassword)

		// Currency preferences and exchange rate
		r.Get("/settings/currency", currencyHandler.GetCurrency)
		r.Put("/settings/currency", currencyHandler.SetCurrency)
		r.Get("/exchange-rate", currencyHandler.GetExchangeRate)

		// Goals / reserves routes
		r.Route("/goals", func(r chi.Router) {
			r.Get("/", goalHandler.List)
			r.Post("/", goalHandler.Create)
			r.Get("/suggestions", goalHandler.Suggestions)
			r.Get("/monthly-history", goalHandler.MonthlyHistory)
			r.Put("/{id}", goalHandler.Update)
			r.Get("/{id}/transactions", goalHandler.ListTransactions)
			r.Post("/{id}/transactions", goalHandler.AddTransaction)
		})

		r.Route("/account/limits", func(r chi.Router) {
			r.Get("/", limitHandler.List)
			r.Get("/check", limitHandler.Check)
			r.Post("/groups", groupHandler.Create)
			r.Put("/groups/{group_id}", groupHandler.Update)
			r.Delete("/groups/{group_id}", groupHandler.Delete)
			r.Put("/{category_id}", limitHandler.Upsert)
			r.Delete("/{category_id}", limitHandler.Delete)
		})

		r.Route("/recurring-expenses", func(r chi.Router) {
			r.Get("/", recurringHandler.List)
			r.Post("/generate-card-transactions", recurringHandler.GenerateCardTransactions)
			r.Get("/reminders", recurringHandler.Reminders)
			r.Post("/", recurringHandler.Create)
			r.Put("/{id}", recurringHandler.Update)
			r.Delete("/{id}", recurringHandler.Delete)
			r.Post("/{id}/pay", recurringHandler.MarkPaid)
			r.Delete("/{id}/pay", recurringHandler.UnmarkPaid)
		})

		r.Route("/installment-purchases", func(r chi.Router) {
			r.Get("/", installmentHandler.List)
			r.Post("/", installmentHandler.Create)
			r.Get("/{id}", installmentHandler.Get)
			r.Put("/{id}", installmentHandler.Update)
			r.Delete("/{id}", installmentHandler.Delete)
		})

		// Dispensa (dispensaAI) — módulo de estoque de casa, um por usuário
		r.Route("/pantry", func(r chi.Router) {
			r.Get("/settings", pantrySettingsHandler.Get)
			r.Put("/settings", pantrySettingsHandler.Update)
			r.Get("/ai-usage", pantryAIUsageHandler.Get)
			r.Get("/seed-products", pantrySeedProductHandler.List)

			r.Route("/products", func(r chi.Router) {
				r.Get("/", pantryProductHandler.List)
				r.Post("/", pantryProductHandler.Create)
				r.Post("/consume-text", pantryProductHandler.ConsumeText)
				r.Put("/{id}", pantryProductHandler.Update)
				r.Delete("/{id}", pantryProductHandler.Delete)
				r.Get("/{id}/movements", pantryProductHandler.Movements)
				r.Post("/{id}/consume", pantryProductHandler.Consume)
				r.Post("/{id}/out", pantryProductHandler.Out)
				r.Post("/{id}/adjust", pantryProductHandler.Adjust)
				r.Post("/{id}/change-unit", pantryProductHandler.ChangeUnit)
			})

			// Shopping list: automatic (regenerated after every stock change and
			// on receipt confirmation, see the handlers above) plus manual
			// add/status endpoints.
			r.Route("/shopping-list", func(r chi.Router) {
				r.Get("/", pantryShoppingListHandler.List)
				r.Post("/", pantryShoppingListHandler.Create)
				r.Post("/from-recipe", pantryShoppingListHandler.FromRecipe)
				r.Patch("/{id}", pantryShoppingListHandler.UpdateStatus)
				r.Post("/regenerate", pantryShoppingListHandler.Regenerate)
			})

			// Recipes: AI generation, save/favorite, and the "vou fazer" cook
			// flow that discounts used ingredients from stock. Generate's
			// longer (120s) timeout is applied by the per-request timeout
			// middleware above — see the comment on the global middleware
			// block.
			r.Route("/recipes", func(r chi.Router) {
				r.Post("/generate", pantryRecipeHandler.Generate)
				r.Post("/", pantryRecipeHandler.Create)
				r.Get("/", pantryRecipeHandler.List)
				r.Get("/{id}", pantryRecipeHandler.Get)
				r.Delete("/{id}", pantryRecipeHandler.Delete)
				r.Post("/{id}/cook/preview", pantryRecipeHandler.CookPreview)
				r.Post("/{id}/cook", pantryRecipeHandler.Cook)
			})

			// Receipt reading: photo upload+extraction, review, confirmation
			// into stock (and optionally a financial transaction), discard.
			// Create's longer (120s) timeout is applied by the per-request
			// timeout middleware above — see the comment on the global
			// middleware block.
			r.Route("/receipts", func(r chi.Router) {
				r.Get("/", pantryReceiptHandler.List)
				r.Post("/", pantryReceiptHandler.Create)
				r.Get("/{id}", pantryReceiptHandler.Get)
				r.Put("/{id}/items", pantryReceiptHandler.UpdateItems)
				r.Post("/{id}/confirm", pantryReceiptHandler.Confirm)
				r.Delete("/{id}", pantryReceiptHandler.Discard)
			})
		})

		// Admin routes — require role="admin"
		r.Route("/admin", func(r chi.Router) {
			r.Use(auth.RequireAdmin)
			r.Get("/users", adminHandler.ListUsers)
			r.Get("/metrics", adminHandler.GetMetrics)
			r.Patch("/users/{id}/status", adminHandler.UpdateStatus)
			r.Patch("/users/{id}/account", adminHandler.UpdateAccount)
			r.Patch("/users/{id}/settings", adminHandler.UpdateUserSettings)
			r.Post("/invites", adminHandler.CreateInvite)
		})
	})

	// ── Server ───────────────────────────────────────────────────────────────
	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 130 * time.Second, // must exceed the longest chi middleware timeout (upload: 120s)
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine so graceful shutdown works.
	go func() {
		log.Info().Str("port", cfg.Port).Msg("server starting")
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal().Err(err).Msg("server error")
		}
	}()

	// ── Graceful shutdown ────────────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info().Msg("shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Error().Err(err).Msg("server forced to shutdown")
	}

	if err := sqlDB.Close(); err != nil {
		log.Error().Err(err).Msg("error closing database")
	}

	log.Info().Msg("server stopped")
}

func boolStr(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}
