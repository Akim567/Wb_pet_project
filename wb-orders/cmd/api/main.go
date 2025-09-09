package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	// 1) Подключение к БД
	dsn := "host=127.0.0.1 port=5433 dbname=wb_orders user=wb_user password=wb_pass sslmode=disable"
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		log.Fatal("open db:", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatal("ping db:", err)
	}
	fmt.Println("Connected to Postgres")

	// 2) Роутер
	mux := http.NewServeMux()

	// healthcheck
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	// /order/<id>
	mux.HandleFunc("/order/", func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/order/")
		if id == "" {
			http.Error(w, "missing order_uid", http.StatusBadRequest)
			return
		}

		// небольшой таймаут на запрос к БД
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()

		const q = `
		select jsonb_build_object(
		'order_uid', o.order_uid,
		'track_number', o.track_number,
		'entry', o.entry,
		'delivery', jsonb_build_object(
			'name', d.name, 'phone', d.phone, 'zip', d.zip,
			'city', d.city, 'address', d.address, 'region', d.region, 'email', d.email
		),
		'payment', jsonb_build_object(
			'transaction', p.transaction, 'request_id', p.request_id, 'currency', p.currency,
			'provider', p.provider, 'amount', p.amount, 'payment_dt', p.payment_dt,
			'bank', p.bank, 'delivery_cost', p.delivery_cost, 'goods_total', p.goods_total, 'custom_fee', p.custom_fee
		),
		'items', coalesce(
			jsonb_agg(jsonb_build_object(
			'chrt_id', i.chrt_id, 'track_number', i.track_number, 'price', i.price,
			'rid', i.rid, 'name', i.name, 'sale', i.sale, 'size', i.size,
			'total_price', i.total_price, 'nm_id', i.nm_id, 'brand', i.brand, 'status', i.status
			)) filter (where i.id is not null),
			'[]'::jsonb
		),
		'locale', o.locale,
		'internal_signature', o.internal_signature,
		'customer_id', o.customer_id,
		'delivery_service', o.delivery_service,
		'shardkey', o.shardkey,
		'sm_id', o.sm_id,
		'date_created', to_char(o.date_created, 'YYYY-MM-DD"T"HH24:MI:SS"Z"'),
		'oof_shard', o.oof_shard
		) as order_json
		from orders o
		left join delivery d on d.order_uid = o.order_uid
		left join payment  p on p.order_uid = o.order_uid
		left join items    i on i.order_uid = o.order_uid
		where o.order_uid = $1
		group by
		o.order_uid, o.track_number, o.entry, o.locale, o.internal_signature,
		o.customer_id, o.delivery_service, o.shardkey, o.sm_id, o.date_created, o.oof_shard,
		d.name, d.phone, d.zip, d.city, d.address, d.region, d.email,
		p.transaction, p.request_id, p.currency, p.provider, p.amount, p.payment_dt, p.bank, p.delivery_cost, p.goods_total, p.custom_fee;`

		var body []byte
		err := db.QueryRowContext(ctx, q, id).Scan(&body)
		if err == sql.ErrNoRows {
			http.Error(w, "order not found", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, "db error: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	})

	addr := ":8081"
	fmt.Println("HTTP server listening on", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
