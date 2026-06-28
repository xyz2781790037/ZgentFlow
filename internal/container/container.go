// Package container implements dependency injection container setup
// Provides centralized configuration for services, repositories, and handlers
// This package is responsible for wiring up all dependencies and ensuring proper lifecycle management
package container

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/panjf2000/ants/v2"
	"github.com/redis/go-redis/v9"
	"go.uber.org/dig"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/xyz2781790037/ZealRAG/internal/application/repository"
	postgresRepo "github.com/xyz2781790037/ZealRAG/internal/application/repository/retriever/postgres"
	"github.com/xyz2781790037/ZealRAG/internal/application/service"
	chatpipeline "github.com/xyz2781790037/ZealRAG/internal/application/service/chat_pipeline"
	"github.com/xyz2781790037/ZealRAG/internal/application/service/file"
	"github.com/xyz2781790037/ZealRAG/internal/application/service/retriever"
	"github.com/xyz2781790037/ZealRAG/internal/config"
	"github.com/xyz2781790037/ZealRAG/internal/database"
	"github.com/xyz2781790037/ZealRAG/internal/handler"
	"github.com/xyz2781790037/ZealRAG/internal/handler/session"
	"github.com/xyz2781790037/ZealRAG/internal/infrastructure/docparser"
	infraemail "github.com/xyz2781790037/ZealRAG/internal/infrastructure/email"
	infra_web_search "github.com/xyz2781790037/ZealRAG/internal/infrastructure/web_search"
	"github.com/xyz2781790037/ZealRAG/internal/logger"
	"github.com/xyz2781790037/ZealRAG/internal/models/chat"
	"github.com/xyz2781790037/ZealRAG/internal/models/embedding"
	"github.com/xyz2781790037/ZealRAG/internal/models/utils/ollama"
	"github.com/xyz2781790037/ZealRAG/internal/router"
	"github.com/xyz2781790037/ZealRAG/internal/stream"
	"github.com/xyz2781790037/ZealRAG/internal/tracing/langfuse"
	"github.com/xyz2781790037/ZealRAG/internal/types"
	"github.com/xyz2781790037/ZealRAG/internal/types/interfaces"
)

// BuildContainer constructs the dependency injection container
// Registers all components, services, repositories and handlers needed by the application
// Creates a fully configured application container with proper dependency resolution
// Parameters:
//   - container: Base dig container to add dependencies to
//
// Returns:
//   - Configured container with all application dependencies registered
func BuildContainer(container *dig.Container) *dig.Container {
	ctx := context.Background()
	logger.Debugf(ctx, "[Container] Starting container initialization...")

	// Register resource cleaner for proper cleanup of resources
	must(container.Provide(NewResourceCleaner, dig.As(new(interfaces.ResourceCleaner))))

	// Core infrastructure configuration
	logger.Debugf(ctx, "[Container] Registering core infrastructure...")
	must(container.Provide(config.LoadConfig))
	must(container.Provide(initLangfuse))
	must(container.Provide(initDatabase))
	must(container.Provide(initFileService))
	must(container.Provide(initRedisClient))
	must(container.Provide(infraemail.NewSMTPSender))
	must(container.Provide(initAntsPool))

	must(container.Invoke(registerLangfuseCleanup))

	// Register goroutine pool cleanup handler
	must(container.Invoke(registerPoolCleanup))

	// Initialize retrieval engine registry for search capabilities
	logger.Debugf(ctx, "[Container] Registering retrieval engine registry...")
	must(container.Provide(initRetrieveEngineRegistry))

	// External service clients
	logger.Debugf(ctx, "[Container] Registering external service clients...")
	must(container.Provide(initDocReaderClient))
	must(container.Provide(initOllamaService))
	must(container.Provide(stream.NewStreamManager))
	// Data repositories layer
	logger.Debugf(ctx, "[Container] Registering repositories...")
	must(container.Provide(repository.NewTenantRepository))
	must(container.Provide(repository.NewKnowledgeBaseRepository))
	must(container.Provide(repository.NewKnowledgeRepository))
	must(container.Provide(repository.NewKnowledgeSpanRepository))
	must(container.Provide(repository.NewChunkRepository))
	must(container.Provide(repository.NewSessionRepository))
	must(container.Provide(repository.NewMessageRepository))
	must(container.Provide(repository.NewModelRepository))
	must(container.Provide(repository.NewModelAPIConfigRepository))
	must(container.Provide(repository.NewUserRepository))
	must(container.Provide(repository.NewSystemSettingRepository))
	must(container.Provide(service.NewWebSearchStateService))
	must(container.Provide(repository.NewTaskPendingOpsRepository))
	must(container.Provide(repository.NewTaskDeadLetterRepository))

	// Business service layer
	logger.Debugf(ctx, "[Container] Registering business services...")
	must(container.Provide(service.NewTenantService))
	must(container.Provide(service.NewKnowledgeBaseService))
	must(container.Provide(service.NewKnowledgeBaseShareService))
	must(container.Provide(service.NewKnowledgeService))
	must(container.Provide(service.NewSpanTracker))
	must(container.Provide(service.NewChunkService))
	must(container.Provide(embedding.NewBatchEmbedder))
	must(container.Provide(service.NewModelService))
	must(container.Provide(service.NewModelAPIConfigService))
	must(container.Provide(service.NewUserService))
	must(container.Provide(service.NewAuthService))
	must(container.Provide(service.NewSystemSettingService))
	must(container.Provide(service.NewPromptService))

	must(container.Provide(service.NewKnowledgePostProcessService, dig.Name("knowledgePostProcess")))

	must(container.Provide(service.NewMessageService))
	must(container.Provide(service.NewAnswerModeService))
	must(container.Provide(service.NewKBFullRebuildService))

	// Web search service
	logger.Debugf(ctx, "[Container] Registering web search registry and providers...")
	must(container.Provide(infra_web_search.NewRegistry))
	must(container.Invoke(registerWebSearchProviders))
	must(container.Provide(repository.NewWebSearchProviderRepository))
	must(container.Provide(retriever.NewFixedStoreOwnership))
	must(container.Provide(service.NewWebSearchService))
	must(container.Provide(service.NewWebSearchProviderService))

	logger.Debugf(ctx, "[Container] Registering session service...")
	must(container.Provide(service.NewSessionService))

	logger.Debugf(ctx, "[Container] Registering task enqueuer...")
	must(container.Provide(router.NewAsyncqClient, dig.As(new(interfaces.TaskEnqueuer))))
	must(container.Provide(router.NewAsynqServer))
	must(container.Provide(router.NewAsynqInspector))
	must(container.Provide(router.NewAsynqTaskInspector))

	// Chat pipeline components for processing chat requests
	logger.Debugf(ctx, "[Container] Registering chat pipeline plugins...")

	must(container.Provide(service.NewHousekeepingService))
	must(container.Invoke(startHousekeepingService))
	logger.Debugf(ctx, "[Container] Knowledge housekeeping runner registered")
	must(container.Provide(chatpipeline.NewEventManager))
	must(container.Invoke(chatpipeline.NewPluginSearch))
	must(container.Invoke(chatpipeline.NewPluginRerank))
	must(container.Invoke(chatpipeline.NewPluginWebFetch))
	must(container.Invoke(chatpipeline.NewPluginMerge))
	must(container.Invoke(chatpipeline.NewPluginIntoChatMessage))
	must(container.Invoke(chatpipeline.NewPluginChatCompletion))
	must(container.Invoke(chatpipeline.NewPluginChatCompletionStream))
	must(container.Invoke(chatpipeline.NewPluginFilterTopK))
	must(container.Invoke(chatpipeline.NewPluginQueryUnderstand))
	must(container.Invoke(chatpipeline.NewPluginLoadHistory))
	logger.Debugf(ctx, "[Container] Chat pipeline plugins registered")

	// HTTP handlers layer
	logger.Debugf(ctx, "[Container] Registering HTTP handlers...")
	must(container.Provide(handler.NewWorkspaceHandler))
	must(container.Provide(handler.NewKnowledgeBaseHandler))
	must(container.Provide(handler.NewKnowledgeBaseShareHandler))
	must(container.Provide(handler.NewKnowledgeHandler))
	must(container.Provide(handler.NewChunkHandler))
	must(container.Provide(handler.NewFAQHandler))
	must(container.Provide(session.NewHandler))
	must(container.Provide(handler.NewMessageHandler))
	must(container.Provide(handler.NewModelHandler))
	must(container.Provide(handler.NewModelAPIConfigHandler))
	must(container.Provide(handler.NewInitializationHandler))
	must(container.Provide(handler.NewSystemHandler))
	must(container.Provide(handler.NewModelCredentialsHandler))
	must(container.Provide(handler.NewWebSearchProviderCredentialsHandler))
	must(container.Provide(handler.NewWebSearchProviderHandler))
	must(container.Provide(handler.NewAnswerModeHandler))
	must(container.Provide(handler.NewPromptHandler))
	must(container.Provide(handler.NewAuthHandler))

	logger.Debugf(ctx, "[Container] HTTP handlers registered")

	// Wire the chat package's local image resolver so multimodal chat can read
	// local:// images that live under a tenant's configured storage PathPrefix
	// (which is not encoded in the local:// URL).
	must(container.Invoke(registerChatLocalImageResolver))

	// Router configuration
	logger.Debugf(ctx, "[Container] Registering router and starting task server...")
	must(container.Provide(router.NewRouter))
	must(container.Invoke(router.RunAsynqServer))

	logger.Infof(ctx, "[Container] Container initialization completed successfully")
	return container
}

// registerChatLocalImageResolver lets chat models read stored local images.
func registerChatLocalImageResolver(fileSvc interfaces.FileService) {
	chat.LocalImageResolver = func(storageURL string) ([]byte, bool) {
		ctx := context.Background()
		rc, err := fileSvc.GetFile(ctx, storageURL)
		if err != nil {
			return nil, false
		}
		defer rc.Close()
		data, err := io.ReadAll(rc)
		if err != nil {
			return nil, false
		}
		return data, true
	}
}

// must is a helper function for error handling
// Panics if the error is not nil, useful for configuration steps that must succeed
// Parameters:
//   - err: Error to check
func must(err error) {
	if err != nil {
		panic(err)
	}
}

// initLangfuse initializes the Langfuse ingestion client.
// Configuration is read from LANGFUSE_* environment variables (see
// docs/langfuse.md). Returns a disabled manager if credentials are absent —
// never an error — so deployments that don't use Langfuse are unaffected.
func initLangfuse() (*langfuse.Manager, error) {
	cfg := langfuse.LoadConfigFromEnv()
	return langfuse.Init(cfg)
}

func initRedisClient() (*redis.Client, error) {
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		return nil, fmt.Errorf("REDIS_ADDR is required")
	}
	db, err := strconv.Atoi(os.Getenv("REDIS_DB"))
	if err != nil {
		db = 0
	}

	client := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Username: os.Getenv("REDIS_USERNAME"),
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       db,
	})

	_, err = client.Ping(context.Background()).Result()
	if err != nil {
		return nil, fmt.Errorf("连接Redis失败: %w", err)
	}

	return client, nil
}

// initDatabase initializes database connection
// Creates and configures database connection based on environment configuration
// Supports multiple database backends (PostgreSQL)
// Parameters:
//   - cfg: Application configuration
//
// Returns:
//   - Configured database connection
//   - Error if connection fails
func initDatabase(cfg *config.Config) (*gorm.DB, error) {
	driver := strings.TrimSpace(os.Getenv("DB_DRIVER"))
	if driver != "" && driver != "postgres" {
		return nil, fmt.Errorf("unsupported database driver %q: ZealRAG requires postgres", driver)
	}

	gormDSN := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable TimeZone=UTC",
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_NAME"),
	)
	dialector := postgres.Open(gormDSN)
	encodedPassword := url.QueryEscape(os.Getenv("DB_PASSWORD"))
	migrateDSN := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable&options=-c%%20app.skip_embedding=false",
		os.Getenv("DB_USER"),
		encodedPassword,
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_NAME"),
	)
	logger.Infof(context.Background(), "DB Config: user=%s host=%s port=%s dbname=%s",
		os.Getenv("DB_USER"), os.Getenv("DB_HOST"), os.Getenv("DB_PORT"), os.Getenv("DB_NAME"))
	db, err := gorm.Open(dialector, &gorm.Config{
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != nil {
		return nil, err
	}

	if name := db.Dialector.Name(); name != "postgres" {
		return nil, fmt.Errorf("unsupported gorm dialector %q: expected postgres", name)
	}

	// Run database migrations automatically (optional, can be disabled via env var)
	// To disable auto-migration, set AUTO_MIGRATE=false
	// To enable auto-recovery from dirty state, set AUTO_RECOVER_DIRTY=true
	if os.Getenv("AUTO_MIGRATE") != "false" {
		logger.Infof(context.Background(), "Running database migrations...")

		autoRecover := os.Getenv("AUTO_RECOVER_DIRTY") != "false"
		migrationOpts := database.MigrationOptions{
			AutoRecoverDirty: autoRecover,
		}

		// Run base migrations (all versioned migrations including embeddings)
		// The embeddings migration will be conditionally executed based on skip_embedding parameter in DSN
		if err := database.RunMigrationsWithOptions(migrateDSN, migrationOpts); err != nil {
			// Log warning but don't fail startup - migrations might be handled externally
			logger.Warnf(context.Background(), "Database migration failed: %v", err)
			logger.Warnf(
				context.Background(),
				"Continuing with application startup. Please run migrations manually if needed.",
			)
		}

		// Post-migration: declarative built-in models from config/builtin_models.yaml (optional).
		if err := types.LoadBuiltinModelsConfig(context.Background(), db, config.ConfigDir()); err != nil {
			logger.Warnf(context.Background(), "Load builtin models config failed: %v", err)
		}
	} else {
		logger.Infof(context.Background(), "Auto-migration is disabled (AUTO_MIGRATE=false)")
	}

	// Get underlying SQL DB object
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	// Configure connection pool parameters
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(time.Duration(10) * time.Minute)

	return db, nil
}

// syncSequences ensures PostgreSQL sequences for auto-increment columns (seq_id)
// are at least as high as the current MAX value in each table. This is needed
// because older code assigned seq_id via application-level MAX()+1, which could
// advance values past the DB sequence counter and cause duplicate key errors.
func syncSequences(db *gorm.DB) {
	if db.Dialector.Name() != "postgres" {
		return
	}
	pairs := [][2]string{
		{"chunks", "chunks_seq_id_seq"},
	}
	for _, p := range pairs {
		table, seq := p[0], p[1]
		sql := fmt.Sprintf(
			`SELECT setval('%s', GREATEST(nextval('%s'), (SELECT COALESCE(MAX(seq_id), 0) FROM %s)))`,
			seq, seq, table,
		)
		if err := db.Exec(sql).Error; err != nil {
			logger.Warnf(context.Background(), "Failed to sync sequence %s: %v", seq, err)
		} else {
			logger.Infof(context.Background(), "Synced sequence %s with table %s", seq, table)
		}
	}
}

// initFileService initializes ZealRAG's local filesystem storage.
func initFileService() (interfaces.FileService, error) {
	baseDir := strings.TrimSpace(os.Getenv("LOCAL_STORAGE_BASE_DIR"))
	if baseDir == "" {
		baseDir = "/data/files"
	}
	externalURL := strings.TrimSpace(os.Getenv("APP_EXTERNAL_URL"))
	return file.NewLocalFileService(baseDir, externalURL), nil
}

// initRetrieveEngineRegistry registers ZealRAG's fixed PostgreSQL/pgvector backend.
func initRetrieveEngineRegistry(db *gorm.DB) (interfaces.RetrieveEngineRegistry, error) {
	registry := retriever.NewRetrieveEngineRegistry()
	repo := postgresRepo.NewPostgresRetrieveEngineRepository(db)
	if err := registry.Register(retriever.NewKVHybridRetrieveEngine(repo, types.PostgresRetrieverEngineType)); err != nil {
		return nil, fmt.Errorf("register postgres retrieve engine: %w", err)
	}
	logger.Infof(context.Background(), "PostgreSQL/pgvector retrieval engine registered")
	return registry, nil
}

// initAntsPool initializes the goroutine pool
// Creates a managed goroutine pool for concurrent task execution
// Parameters:
//   - cfg: Application configuration
//
// Returns:
//   - Configured goroutine pool
//   - Error if initialization fails
func initAntsPool(cfg *config.Config) (*ants.Pool, error) {
	// Default to 5 if not specified in config
	poolSize := os.Getenv("CONCURRENCY_POOL_SIZE")
	if poolSize == "" {
		poolSize = "5"
	}
	poolSizeInt, err := strconv.Atoi(poolSize)
	if err != nil {
		return nil, err
	}
	// Set up the pool with pre-allocation for better performance
	return ants.NewPool(poolSizeInt, ants.WithPreAlloc(true))
}

// registerPoolCleanup registers the goroutine pool for cleanup
// Ensures proper cleanup of the goroutine pool when application shuts down
// Parameters:
//   - pool: Goroutine pool
//   - cleaner: Resource cleaner
func registerPoolCleanup(pool *ants.Pool, cleaner interfaces.ResourceCleaner) {
	cleaner.RegisterWithName("AntsPool", func() error {
		pool.Release()
		return nil
	})
}

// registerLangfuseCleanup ensures buffered Langfuse events are flushed on
// shutdown. A 5-second timeout matches other external-service cleanups and
// balances data durability against a slow remote endpoint holding up exit.
func registerLangfuseCleanup(mgr *langfuse.Manager, cleaner interfaces.ResourceCleaner) {
	if mgr == nil {
		return
	}
	cleaner.RegisterWithName("Langfuse", func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return mgr.Shutdown(ctx)
	})
}

// initDocReaderClient initializes the DocumentReader client (lightweight API).
func initDocReaderClient(cfg *config.Config) (interfaces.DocumentReader, error) {
	addr := strings.TrimSpace(os.Getenv("DOCREADER_ADDR"))
	transport := strings.TrimSpace(os.Getenv("DOCREADER_TRANSPORT"))
	if transport == "" {
		transport = "grpc"
	}
	if addr == "" {
		logger.Infof(context.Background(), "[DocConverter] No DOCREADER_ADDR configured, starting disconnected")
	}
	transport = strings.ToLower(transport)
	switch transport {
	case "http", "https":
		if addr != "" && !strings.HasPrefix(addr, "http://") && !strings.HasPrefix(addr, "https://") {
			addr = "http://" + addr
		}
		return docparser.NewHTTPDocumentReader(addr)
	default:
		return docparser.NewGRPCDocumentReader(addr)
	}
}

// initOllamaService initializes the Ollama service client
// Creates a client for interacting with Ollama API for model inference
// Parameters:
//   - None
//
// Returns:
//   - Configured Ollama service client
//   - Error if initialization fails
func initOllamaService() (*ollama.OllamaService, error) {
	// Get Ollama service from existing factory function
	return ollama.GetOllamaService()
}

// registerWebSearchProviders registers all web search provider types to the registry.
// Each provider type is registered with its factory function that accepts parameters.
// Provider instances are created on-demand when tenants configure them.
func registerWebSearchProviders(registry *infra_web_search.Registry) {
	registry.Register("duckduckgo", infra_web_search.NewDuckDuckGoProvider)
	registry.Register("google", infra_web_search.NewGoogleProvider)
	registry.Register("bing", infra_web_search.NewBingProvider)
	registry.Register("tavily", infra_web_search.NewTavilyProvider)
	registry.Register("ollama", infra_web_search.NewOllamaProvider)
	registry.Register("baidu", infra_web_search.NewBaiduProvider)
	registry.Register("searxng", infra_web_search.NewSearxngProvider)
}

// startHousekeepingService starts the knowledge housekeeping cron and registers
// cleanup. This is the safety net that recovers any knowledge stuck in
// "processing" past a configurable threshold (see HousekeepingService for
// rationale). Best-effort: a startup error is logged but does NOT abort the
// container — the rest of the system stays usable.
func startHousekeepingService(
	svc *service.HousekeepingService,
	kbService interfaces.KnowledgeBaseService,
	knowledgeService interfaces.KnowledgeService,
	cleaner interfaces.ResourceCleaner,
) {
	if svc == nil {
		return
	}
	svc.ConfigureTrashServices(kbService, knowledgeService)
	if err := svc.Start(context.Background()); err != nil {
		logger.Warnf(context.Background(), "[Container] housekeeping start failed: %v", err)
	}
	cleaner.RegisterWithName("KnowledgeHousekeeping", func() error {
		svc.Stop()
		return nil
	})
}
