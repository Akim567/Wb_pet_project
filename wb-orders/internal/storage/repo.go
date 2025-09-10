// internal/storage/repo.go
package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	// pgx как драйвер для database/sql
	_ "github.com/jackc/pgx/v5/stdlib"

	"wb-orders/internal/models"
)

type Repo struct {
	DB *sql.DB
}

func New(db *sql.DB) *Repo { return &Repo{DB: db} }

// -------------------- READ: GetOrderByID --------------------
// Читает заказ + delivery + payment + items.
func (r *Repo) GetOrderByID(ctx context.Context, id string) (models.Order, error) {
	var o models.Order

	// 1) Шапка заказа
	const headSQL = `
	SELECT
		o.order_uid,
		d.name, d.phone, d.zip, d.city, d.address, d.region, d.email,
		p.transaction, p.request_id, p.currency, p.provider, p.amount, p.payment_dt,
		p.bank, p.delivery_cost, p.goods_total, p.custom_fee
	FROM orders o
	LEFT JOIN delivery d ON d.order_uid = o.order_uid
	LEFT JOIN payment  p ON p.order_uid = o.order_uid
	WHERE o.order_uid = $1
	`
	row := r.DB.QueryRowContext(ctx, headSQL, id)
	if err := row.Scan(
		&o.OrderUID,
		&o.Delivery.Name, &o.Delivery.Phone, &o.Delivery.Zip, &o.Delivery.City,
		&o.Delivery.Address, &o.Delivery.Region, &o.Delivery.Email,
		&o.Payment.Transaction, &o.Payment.RequestID, &o.Payment.Currency,
		&o.Payment.Provider, &o.Payment.Amount, &o.Payment.PaymentDT,
		&o.Payment.Bank, &o.Payment.DeliveryCost, &o.Payment.GoodsTotal,
		&o.Payment.CustomFee,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.Order{}, fmt.Errorf("order not found: %w", err)
		}
		return models.Order{}, err
	}

	// 2) Items
	const itemsSQL = `
	SELECT chrt_id, track_number, price, rid, name, sale, size,
		total_price, nm_id, brand, status
	FROM items
	WHERE order_uid = $1
	ORDER BY chrt_id
	`
	rows, err := r.DB.QueryContext(ctx, itemsSQL, id)
	if err != nil {
		return models.Order{}, err
	}
	defer rows.Close()

	for rows.Next() {
		var it models.Item
		if err := rows.Scan(
			&it.ChrtID, &it.TrackNumber, &it.Price, &it.RID, &it.Name,
			&it.Sale, &it.Size, &it.TotalPrice, &it.NmID,
			&it.Brand, &it.Status,
		); err != nil {
			return models.Order{}, err
		}
		o.Items = append(o.Items, it)
	}
	if err := rows.Err(); err != nil {
		return models.Order{}, err
	}

	return o, nil
}

// -------------------- WRITE: UpsertOrder --------------------
// Идемпотентное сохранение заказа.
// 1) upsert в orders, delivery, payment
// 2) удаление старых items этого заказа + вставка новых
func (r *Repo) UpsertOrder(ctx context.Context, o models.Order) error {
	tx, err := r.DB.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		// На случай паники — откат
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
	}()

	// ----- 1) orders (вставляем все нужные NOT NULL поля)
	{
		const q = `
			INSERT INTO orders (
				order_uid,
				track_number,
				entry,
				locale,
				internal_signature,
				customer_id,
				delivery_service,
				shardkey,
				sm_id,
				date_created,
				oof_shard
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
			ON CONFLICT (order_uid) DO UPDATE SET
				track_number       = EXCLUDED.track_number,
				entry              = EXCLUDED.entry,
				locale             = EXCLUDED.locale,
				internal_signature = EXCLUDED.internal_signature,
				customer_id        = EXCLUDED.customer_id,
				delivery_service   = EXCLUDED.delivery_service,
				shardkey           = EXCLUDED.shardkey,
				sm_id              = EXCLUDED.sm_id,
				date_created       = EXCLUDED.date_created,
				oof_shard          = EXCLUDED.oof_shard
		`
		if _, err := tx.ExecContext(ctx, q,
			o.OrderUID,
			o.TrackNumber,
			o.Entry,
			o.Locale,
			o.InternalSignature,
			o.CustomerID,
			o.DeliveryService,
			o.ShardKey,
			o.SmID,
			o.DateCreated, // time.Time -> timestamp/timestamptz
			o.OofShard,
		); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("upsert orders: %w", err)
		}
	}

	// ----- 2) delivery
	{
		const q = `
			INSERT INTO delivery (
				order_uid, name, phone, zip, city, address, region, email
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
			ON CONFLICT (order_uid) DO UPDATE SET
				name    = EXCLUDED.name,
				phone   = EXCLUDED.phone,
				zip     = EXCLUDED.zip,
				city    = EXCLUDED.city,
				address = EXCLUDED.address,
				region  = EXCLUDED.region,
				email   = EXCLUDED.email
		`
		d := o.Delivery
		if _, err := tx.ExecContext(ctx, q,
			o.OrderUID, d.Name, d.Phone, d.Zip, d.City, d.Address, d.Region, d.Email,
		); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("upsert delivery: %w", err)
		}
	}

	// ----- 3) payment
	{
		const q = `
			INSERT INTO payment (
				order_uid, transaction, request_id, currency, provider, amount, payment_dt, bank, delivery_cost, goods_total, custom_fee
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
			ON CONFLICT (order_uid) DO UPDATE SET
				transaction   = EXCLUDED.transaction,
				request_id    = EXCLUDED.request_id,
				currency      = EXCLUDED.currency,
				provider      = EXCLUDED.provider,
				amount        = EXCLUDED.amount,
				payment_dt    = EXCLUDED.payment_dt,
				bank          = EXCLUDED.bank,
				delivery_cost = EXCLUDED.delivery_cost,
				goods_total   = EXCLUDED.goods_total,
				custom_fee    = EXCLUDED.custom_fee
		`
		p := o.Payment
		if _, err := tx.ExecContext(ctx, q,
			o.OrderUID, p.Transaction, p.RequestID, p.Currency, p.Provider, p.Amount, p.PaymentDT, p.Bank, p.DeliveryCost, p.GoodsTotal, p.CustomFee,
		); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("upsert payment: %w", err)
		}
	}

	// ----- 4) items: сначала удаляем, затем вставляем заново
	{
		if _, err := tx.ExecContext(ctx, `DELETE FROM items WHERE order_uid = $1`, o.OrderUID); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("delete items: %w", err)
		}

		if len(o.Items) > 0 {
			const q = `
				INSERT INTO items (
					order_uid, chrt_id, track_number, price, rid, name, sale, size, total_price, nm_id, brand, status
				) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
			`
			for _, it := range o.Items {
				if _, err := tx.ExecContext(ctx, q,
					o.OrderUID, it.ChrtID, it.TrackNumber, it.Price, it.RID, it.Name,
					it.Sale, it.Size, it.TotalPrice, it.NmID,
					it.Brand, it.Status,
				); err != nil {
					_ = tx.Rollback()
					return fmt.Errorf("insert item chrt_id=%d: %w", it.ChrtID, err)
				}
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// -------------------- ValidateOrder (опционально) --------------------
func ValidateOrder(o models.Order) error {
	if o.OrderUID == "" {
		return errors.New("empty order_uid")
	}

	if o.TrackNumber == "" {
		return errors.New("empty track_number")
	}

	if o.DateCreated.IsZero() {
		return errors.New("empty date_created")
	}

	// Добавляй доп. проверки по необходимости
	return nil
}
