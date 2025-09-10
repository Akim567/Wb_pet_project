package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"

	"wb-orders/internal/cache"
	ikafka "wb-orders/internal/kafka"
	"wb-orders/internal/storage"
)

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func mustOpenDB() *sql.DB {
	const dsn = "host=127.0.0.1 port=5433 dbname=wb_orders user=wb_user password=wb_pass sslmode=disable"

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		log.Fatal("open db:", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var lastErr error
	for attempt := 1; ; attempt++ {
		if err = db.PingContext(ctx); err == nil {
			return db
		}
		lastErr = err
		wait := time.Duration(attempt) * time.Second
		log.Printf("db not ready (attempt %d): %v — retry in %v", attempt, err, wait)

		select {
		case <-time.After(wait):
		case <-ctx.Done():
			log.Fatalf("ping db: %v", lastErr)
		}
	}
}

func main() {
	if err := godotenv.Load(".env"); err != nil {
		log.Printf(".env not loaded: %v (ok if vars set by shell/docker)", err)
	}

	// 1) Подключение к БД и репозиторий
	db := mustOpenDB()
	defer db.Close()
	fmt.Println("Connected to Postgres")
	repo := storage.New(db)

	// 2) Кэш (LRU на 1000 заказов)
	orderCache := cache.NewLRU(1000)

	// 2.1) Прогрев кэша последними 100 заказами
	if err := warmUpCache(context.Background(), repo, orderCache, 100); err != nil {
		log.Printf("cache warm-up error: %v", err)
	}

	// 3) Контекст для graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 4) Kafka consumer
	cons := ikafka.NewConsumer(repo, orderCache)
	defer cons.Close()

	go func() {
		if err := cons.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			log.Printf("kafka consumer stopped: %v", err)
		}
	}()

	// 5) Роутер
	mux := http.NewServeMux()

	// раздача статики (index.html и прочее из ./static)
	fs := http.FileServer(http.Dir("./web"))
	mux.Handle("/", fs)

	// healthcheck
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	// debug: статистика кэша
	mux.HandleFunc("/debug/cache", func(w http.ResponseWriter, r *http.Request) {
		hits, misses := orderCache.Stats()
		writeJSON(w, http.StatusOK, map[string]any{
			"len":    orderCache.Len(),
			"hits":   hits,
			"misses": misses,
		})
	})

	// GET /order/{id}
	mux.HandleFunc("/order/", func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/order/")
		if id == "" {
			http.Error(w, "missing order id", http.StatusBadRequest)
			return
		}

		// 1) Кэш
		if o, ok := orderCache.Get(id); ok {
			log.Printf("cache HIT id=%s len=%d", id, orderCache.Len())
			writeJSON(w, http.StatusOK, o)
			return
		}
		log.Printf("cache MISS id=%s", id)

		// 2) БД с таймаутом
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()

		o, err := repo.GetOrderByID(ctx, id)
		if err != nil {
			http.Error(w, "not found or db error: "+err.Error(), http.StatusNotFound)
			return
		}

		// 3) Кладём в кэш и отдаём
		orderCache.Set(id, o)
		writeJSON(w, http.StatusOK, o)
	})

	// HTTP сервер с graceful shutdown
	addr := ":8081"
	srv := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	go func() {
		log.Printf("HTTP server listening on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http server error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutdown signal received")

	shCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shCtx); err != nil {
		log.Printf("http shutdown error: %v", err)
	}
	log.Println("bye")
}

// Прогрев кэша: загружаем последние N заказов
func warmUpCache(ctx context.Context, repo *storage.Repo, c *cache.LRU, n int) error {
	const q = `SELECT order_uid FROM orders ORDER BY date_created DESC LIMIT $1`
	rows, err := repo.DB.QueryContext(ctx, q, n)
	if err != nil {
		return err
	}
	defer rows.Close()

	ids := make([]string, 0, n)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, id := range ids {
		o, err := repo.GetOrderByID(ctx, id)
		if err != nil {
			log.Printf("warm-up skip id=%s: %v", id, err)
			continue
		}
		c.Set(id, o)
	}
	log.Printf("cache warm-up done: loaded=%d current_len=%d", len(ids), c.Len())
	return nil
}
