package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	// pgx как драйвер для database/sql
	_ "github.com/jackc/pgx/v5/stdlib"

	"wb-orders/internal/models" // поправь импорт под свой module path
)

type Repo struct {
	DB *sql.DB
}

func New(db *sql.DB) *Repo { return &Repo{DB: db} }

// GetOrderByID читает заказ + delivery + payment + items.
// Сделаем это в два шага: (1) шапка заказа с delivery/payment; (2) items.
func (r *Repo) GetOrderByID(ctx context.Context, id string) (models.Order, error) {
	var o models.Order

	// 1) Шапка заказа: берём по order_uid
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
		&o.ID,
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

	// 2) Товары заказа — тоже по order_uid
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
			&it.Sale, &it.Size, &it.TotalPrice, &it.NmID, &it.Brand, &it.Status,
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
