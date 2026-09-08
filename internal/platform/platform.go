// Package platform builds the dependency graph — the composition root.
package platform

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/valkey-io/valkey-go"

	"github.com/Orctatech-Engineering-Team/orcta-pay/internal/apps"
	"github.com/Orctatech-Engineering-Team/orcta-pay/internal/charges"
	"github.com/Orctatech-Engineering-Team/orcta-pay/internal/config"
	"github.com/Orctatech-Engineering-Team/orcta-pay/internal/gateway"
	"github.com/Orctatech-Engineering-Team/orcta-pay/internal/ledger"
	"github.com/Orctatech-Engineering-Team/orcta-pay/internal/observability"
	"github.com/Orctatech-Engineering-Team/orcta-pay/internal/payouts"
	postgresstore "github.com/Orctatech-Engineering-Team/orcta-pay/internal/storage/postgres"
	valkeystore "github.com/Orctatech-Engineering-Team/orcta-pay/internal/storage/valkey"
	"github.com/Orctatech-Engineering-Team/orcta-pay/internal/webhooks"
)

// App holds the wired dependencies.
type App struct {
	Config   config.Config
	Charges  *charges.Service
	Payouts  *payouts.Service
	Ledger   *ledger.Service
	Apps     *apps.Service
	Webhooks *webhooks.Service
	Router   *gateway.ChargerRouter
	Pool     *pgxpool.Pool
	Valkey   valkey.Client
	Observ   *observability.Provider
	Locker   *valkeystore.Locker
	MemStore *postgresstore.MemoryStore
	PgStore  *postgresstore.PostgresStore
}

// Build wires the graph. Callers must close App.Close.
func Build(ctx context.Context, cfg config.Config) (*App, error) {
	observ, err := observability.Setup(ctx, cfg.Observ)
	if err != nil {
		return nil, fmt.Errorf("platform: observability: %w", err)
	}
	pool, err := newPool(ctx, cfg.Database)
	if err != nil {
		if cfg.Environment == config.EnvProduction {
			return nil, fmt.Errorf("platform: postgres: %w", err)
		}
		observ.Logger.WarnContext(ctx, "postgres pool creation failed, using in-memory fallback", "error", err)
		pool = nil
	} else {
		pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		pingErr := pool.Ping(pingCtx)
		cancel()
		if pingErr != nil {
			pool.Close()
			if cfg.Environment == config.EnvProduction {
				return nil, fmt.Errorf("platform: postgres ping: %w", pingErr)
			}
			observ.Logger.WarnContext(ctx, "postgres ping failed, using in-memory store — data will NOT persist", "error", pingErr)
			pool = nil
		}
	}
	if pool == nil && cfg.Environment == config.EnvProduction {
		return nil, fmt.Errorf("platform: postgres: unavailable in production")
	}
	valkeyClient, err := newValkey(cfg.Valkey)
	if err != nil {
		if cfg.Environment == config.EnvProduction {
			if pool != nil {
				pool.Close()
			}
			return nil, fmt.Errorf("platform: valkey: %w", err)
		}
		observ.Logger.WarnContext(ctx, "valkey not connected, using in-memory fallback", "error", err)
		valkeyClient = nil
	} else {
		pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		pingErr := valkeyClient.Do(pingCtx, valkeyClient.B().Ping().Build()).Error()
		cancel()
		if pingErr != nil {
			valkeyClient.Close()
			if cfg.Environment == config.EnvProduction {
				if pool != nil {
					pool.Close()
				}
				return nil, fmt.Errorf("platform: valkey ping: %w", pingErr)
			}
			observ.Logger.WarnContext(ctx, "valkey ping failed, using in-memory fallback", "error", pingErr)
			valkeyClient = nil
		}
	}
	if valkeyClient == nil && cfg.Environment == config.EnvProduction {
		if pool != nil {
			pool.Close()
		}
		return nil, fmt.Errorf("platform: valkey: unavailable in production")
	}
	health := valkeystore.NewHealthStore(valkeyClient)
	locker := valkeystore.NewLocker(valkeyClient)
	var chargeStore charges.IntentStore
	var payoutStore payouts.ReservationStore
	var ledgerStore ledger.Store
	var appStore apps.Store
	var webhookStore webhooks.Store
	var pgStore *postgresstore.PostgresStore
	var memStore *postgresstore.MemoryStore
	if pool != nil {
		pgStore = postgresstore.NewPostgresStore(pool)
		chargeStore = pgStore
		payoutStore = pgStore
		ledgerStore = pgStore
		webhookStore = pgStore
		appStore = postgresstore.NewPostgresAppsStore(pool)
	} else {
		observ.Logger.WarnContext(ctx, "postgres unavailable, using in-memory store — data will NOT persist")
		memStore = postgresstore.NewMemoryStore()
		mem := memStore
		chargeStore = mem
		payoutStore = mem
		ledgerStore = mem
		webhookStore = mem
		appStore = mem
	}

	callbackBase := cfg.Payments.CallbackBaseURL
	hubtelCB := ""
	paystackCB := ""
	if callbackBase != "" {
		hubtelCB = callbackBase + "/webhooks/hubtel"
		paystackCB = callbackBase + "/webhooks/paystack"
	}
	adapters := map[gateway.Gateway]gateway.AggregatorClient{
		gateway.GatewayHubtel: gateway.NewHubtelAdapter(cfg.Payments.Hubtel,
			gateway.WithHubtelLogger(observ.Logger),
			gateway.WithHubtelCallbackURL(hubtelCB)),
		gateway.GatewayPaystack: gateway.NewPaystackAdapter(cfg.Payments.Paystack,
			gateway.WithPaystackLogger(observ.Logger),
			gateway.WithPaystackCallbackURL(paystackCB)),
		gateway.GatewayMoolre: gateway.NewMoolreAdapter(cfg.Payments.Moolre, callbackBase,
			gateway.WithMoolreLogger(observ.Logger)),
	}
	router := gateway.NewChargerRouter(cfg.Payments, health, adapters)

	app := &App{
		Config:   cfg,
		Charges:  charges.NewService(chargeStore, router),
		Payouts:  payouts.NewService(payoutStore, router, payouts.WithLedger(ledgerStore), payouts.WithLocker(locker)),
		Ledger:   ledger.NewService(ledgerStore),
		Apps:     apps.NewService(appStore, cfg.Environment),
		Webhooks: webhooks.NewService(webhookStore, router),
		Router:   router,
		Pool:     pool,
		Valkey:   valkeyClient,
		Observ:   observ,
		Locker:   locker,
		MemStore: memStore,
		PgStore:  pgStore,
	}
	return app, nil
}

// Close releases resources.
func (a *App) Close() {
	if a.Valkey != nil {
		a.Valkey.Close()
	}
	if a.Pool != nil {
		a.Pool.Close()
	}
	if a.Observ != nil {
		_ = a.Observ.Shutdown(context.Background())
	}
}

func newPool(ctx context.Context, cfg config.DatabaseConfig) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		return nil, err
	}
	poolCfg.MaxConns = cfg.MaxConns
	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, err
	}
	return pool, nil
}

func newValkey(cfg config.ValkeyConfig) (valkey.Client, error) {
	if cfg.Addr == "" {
		return nil, fmt.Errorf("valkey addr empty")
	}
	client, err := valkey.NewClient(valkey.ClientOption{
		InitAddress: []string{cfg.Addr},
		Password:    cfg.Password,
		SelectDB:    cfg.DB,
	})
	if err != nil {
		return nil, err
	}
	return client, nil
}
