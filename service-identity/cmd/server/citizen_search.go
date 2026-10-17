package main

// The citizen-directory SEARCH wiring (ADR 0101 #7–#11): Elasticsearch index, Kafka outbox relay and
// consumer, and the operator's reindex command. Kept out of run() so the decisions sit beside their code.
//
// WHAT HAPPENS WHEN A DEPENDENCY IS ABSENT — dev only; staging/prod refuse to START without any of them,
// because identity declares config.Elasticsearch, config.KafkaEvent and config.SearchHash (ADR 0057):
//
//	ELASTICSEARCH_ADDRS or SEARCH_HASH_KEY empty   index OFF: keyword search on PostgreSQL, attribute and
//	                                              created-date filters 503; nothing is indexed. Warned.
//	KAFKA_EVENT_ADDRESS empty                     outbox rows accumulate unpublished; nothing is indexed
//	                                              until a broker is configured (then the relay drains them).
//	SEARCH_HASH_KEY ≠ the index's fingerprint     index OFF and an ERROR at every startup naming the
//	                                              variable; searches 503; the consumer retries without
//	                                              committing. Fix: restore the key, or run the reindex
//	                                              command with `all` to adopt the new one.
//
// NEVER A CRASH for a missing search dependency in dev: the directory itself works without search.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/vihat/vigov/core/config"
	"github.com/vihat/vigov/core/events/kafka"
	coresearch "github.com/vihat/vigov/core/search"
	"github.com/vihat/vigov/core/searchhash"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/app"
	"github.com/vihat/vigov/service-identity/internal/search"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
	"github.com/vihat/vigov/service-identity/internal/store/crosstenant"
)

// reindexFlag is the operator's command: `identity -reindex-citizen-contacts=<tenant_id>` rebuilds one
// commune; `=all` rebuilds every commune AND adopts the configured SEARCH_HASH_KEY (the only way to accept
// a new key). Run in a one-off pod with the deployment's environment, like `-migrate-only`.
const reindexFlag = "-reindex-citizen-contacts="

// indexOpenRetry is how often a startup that could not reach the cluster tries again.
const indexOpenRetry = 30 * time.Second

// buildCitizenIndex returns the index, or nil when it is not configured (dev). A key mismatch returns the
// index OFF (searches 503) with an error log; a cluster that does not answer yet is retried in background.
func buildCitizenIndex(ctx context.Context, cfg config.Config, log *slog.Logger) (*search.CitizenIndex, error) {
	addrs := cfg.ElasticsearchAddrs()
	if len(addrs) == 0 || cfg.SearchHashKey().Rong() {
		log.Warn("CẢNH BÁO CẤU HÌNH", "chi_tiet",
			"ELASTICSEARCH_ADDRS hoặc SEARCH_HASH_KEY trống — tìm kiếm danh bạ chạy trên PostgreSQL, lọc theo thuộc tính/ngày tạo trả 503, không ghi chỉ mục (ADR 0101)")
		return nil, nil
	}
	client, err := coresearch.New(addrs, cfg.ElasticsearchUsername(), cfg.ElasticsearchPassword(), cfg.Env == config.EnvDev)
	if err != nil {
		return nil, fmt.Errorf("identity: Elasticsearch: %w", err)
	}
	key, err := searchhash.NewKey(cfg.SearchHashKey())
	if err != nil {
		return nil, fmt.Errorf("identity: SEARCH_HASH_KEY: %w", err)
	}
	idx := search.New(client, key, cfg.ElasticsearchIndexPrefix())
	go func() {
		for {
			err := idx.Open(ctx)
			switch {
			case err == nil:
				log.Info("chỉ mục tìm kiếm danh bạ sẵn sàng", "index", idx.Name())
				return
			case errors.Is(err, search.ErrKeyMismatch):
				// Named, never the value. The index stays OFF until restart with the right key, or the
				// reindex-all command adopts the new one (owner, 17/10/2026).
				log.Error("SEARCH_HASH_KEY KHÔNG KHỚP chỉ mục — tìm kiếm và ghi chỉ mục danh bạ TẮT; "+
					"khôi phục khoá cũ, hoặc chạy `identity "+reindexFlag+"all` để dựng lại mọi xã bằng khoá mới",
					"index", idx.Name())
				return
			}
			log.Warn("chưa mở được chỉ mục tìm kiếm danh bạ — thử lại", "index", idx.Name(), "err", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(indexOpenRetry):
			}
		}
	}()
	return idx, nil
}

// startCitizenSearchPipeline starts the outbox relay (when a broker is configured) and the consumer (when a
// broker AND the index are configured). Both stop with ctx. The returned func closes the Kafka clients.
func startCitizenSearchPipeline(ctx context.Context, cfg config.Config, db *sql.DB, kho *store.DB,
	idx *search.CitizenIndex, log *slog.Logger) (func(), error) {
	broker := cfg.KafkaEventBroker()
	if !broker.Configured() {
		log.Warn("CẢNH BÁO CẤU HÌNH", "chi_tiet",
			"KAFKA_EVENT_ADDRESS trống — sự kiện danh bạ nằm chờ trong event_outbox, chỉ mục tìm kiếm không được cập nhật (ADR 0101)")
		return func() {}, nil
	}
	// cfg.Env prefixes every topic and group: dev and prod share one cluster (core/events/names.go).
	pub, err := kafka.New("vigov-identity", cfg.Env, broker)
	if err != nil {
		return nil, fmt.Errorf("identity: Kafka publisher: %w", err)
	}
	relay := app.NewEventRelay(crosstenant.NewEventOutboxCommunes(db), idstore.NewEventOutboxStore(kho), pub, log)
	go relay.Run(ctx)
	closers := []func(){pub.Close}

	if idx != nil {
		consumer, err := kafka.NewConsumer("vigov-identity", cfg.Env, "identity", "citizen-search",
			[]string{app.EventCitizenContactChanged}, broker, log)
		if err != nil {
			pub.Close()
			return nil, fmt.Errorf("identity: Kafka consumer: %w", err)
		}
		indexer := app.NewCitizenSearchIndexer(idstore.NewCitizenDirectoryStore(kho), idstore.NewCitizenFieldStore(kho), idx, log)
		go func() {
			log.Info("khởi động consumer chỉ mục danh bạ", "group", consumer.Group())
			if err := consumer.Run(ctx, indexer.Handle); err != nil {
				log.Error("consumer chỉ mục danh bạ dừng", "err", err)
			}
		}()
		closers = append(closers, consumer.Close)
	}
	return func() {
		for _, c := range closers {
			c()
		}
	}, nil
}

// runReindex is the operator command (reindexFlag). It never migrates and never serves.
func runReindex(log *slog.Logger, target string) error {
	cfg, err := config.Load("identity", configUses)
	if err != nil {
		return err
	}
	db, err := sql.Open("pgx", cfg.DatabaseDSN.Lo())
	if err != nil {
		return err
	}
	defer db.Close()
	kho := store.New(db)
	ctx := context.Background()
	if len(cfg.ElasticsearchAddrs()) == 0 || cfg.SearchHashKey().Rong() {
		return errors.New("dựng lại chỉ mục: ELASTICSEARCH_ADDRS và SEARCH_HASH_KEY phải có giá trị")
	}
	client, err := coresearch.New(cfg.ElasticsearchAddrs(), cfg.ElasticsearchUsername(), cfg.ElasticsearchPassword(),
		cfg.Env == config.EnvDev)
	if err != nil {
		return err
	}
	key, err := searchhash.NewKey(cfg.SearchHashKey())
	if err != nil {
		return err
	}
	idx := search.New(client, key, cfg.ElasticsearchIndexPrefix())
	indexer := app.NewCitizenSearchIndexer(idstore.NewCitizenDirectoryStore(kho), idstore.NewCitizenFieldStore(kho), idx, log)

	if strings.EqualFold(target, "all") {
		n, err := indexer.ReindexAll(ctx, crosstenant.NewEventOutboxCommunes(db))
		log.Info("dựng lại chỉ mục danh bạ mọi xã", "so_nguoi", n, "index", idx.Name())
		return err
	}
	t := tenant.ID(strings.TrimSpace(target))
	if !t.Valid() {
		return fmt.Errorf("dựng lại chỉ mục: %q không phải mã xã (tenant_id) — hoặc dùng all", target)
	}
	if err := idx.Open(ctx); err != nil {
		if errors.Is(err, search.ErrKeyMismatch) {
			return fmt.Errorf("%w — một xã không thể dựng lại bằng khoá khác; dùng %sall", err, reindexFlag)
		}
		return err
	}
	n, err := indexer.ReindexCommune(tenant.Into(ctx, t))
	log.Info("dựng lại chỉ mục danh bạ một xã", "xa", string(t), "so_nguoi", n, "index", idx.Name())
	return err
}
