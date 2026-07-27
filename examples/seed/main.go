package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
)

func main() {
	host := flag.String("host", "localhost", "host")
	port := flag.Int("port", 5432, "port")
	user := flag.String("user", "postgres", "user")
	password := flag.String("password", "postgres", "password")
	database := flag.String("database", "demo", "database")
	flag.Parse()

	dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable", *user, *password, *host, *port, *database)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	var conn *pgx.Conn
	var err error
	for i := 0; i < 30; i++ {
		conn, err = pgx.Connect(ctx, dsn)
		if err == nil {
			break
		}
		time.Sleep(time.Second)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close(ctx)

	stmts := []string{
		`CREATE SCHEMA IF NOT EXISTS billing`,
		`CREATE SCHEMA IF NOT EXISTS analytics`,
		`CREATE TABLE IF NOT EXISTS users (
			id SERIAL PRIMARY KEY,
			email TEXT NOT NULL UNIQUE,
			name TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
		`CREATE TABLE IF NOT EXISTS product_categories (
			id SERIAL PRIMARY KEY,
			slug TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS products (
			id SERIAL PRIMARY KEY,
			sku TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL,
			price_cents INT NOT NULL,
			category_id INT REFERENCES product_categories(id),
			metadata JSONB NOT NULL DEFAULT '{}'::jsonb
		)`,
		`CREATE TABLE IF NOT EXISTS orders (
			id SERIAL PRIMARY KEY,
			user_id INT NOT NULL REFERENCES users(id),
			total_cents INT NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
		`CREATE TABLE IF NOT EXISTS order_items (
			id SERIAL PRIMARY KEY,
			order_id INT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
			product_id INT NOT NULL REFERENCES products(id),
			quantity INT NOT NULL DEFAULT 1,
			unit_price_cents INT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS billing.invoices (
			id SERIAL PRIMARY KEY,
			user_id INT NOT NULL,
			amount_cents INT NOT NULL,
			status TEXT NOT NULL DEFAULT 'open',
			issued_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
		`CREATE TABLE IF NOT EXISTS analytics.events (
			id BIGSERIAL PRIMARY KEY,
			user_id INT,
			event_type TEXT NOT NULL,
			payload JSONB NOT NULL DEFAULT '{}'::jsonb,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
		`CREATE OR REPLACE VIEW active_users AS
			SELECT id, email, name, created_at FROM users ORDER BY created_at DESC`,
		`CREATE SEQUENCE IF NOT EXISTS order_number_seq`,
		`ALTER TABLE products ADD COLUMN IF NOT EXISTS category_id INT REFERENCES product_categories(id)`,
		`ALTER TABLE billing.invoices ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'open'`,
		`TRUNCATE analytics.events, billing.invoices, order_items, orders, products, product_categories, users RESTART IDENTITY CASCADE`,
		`INSERT INTO product_categories (slug, name) VALUES
			('peripherals', 'Peripherals'),
			('displays', 'Displays'),
			('cables', 'Cables & Adapters'),
			('audio', 'Audio'),
			('storage', 'Storage'),
			('networking', 'Networking'),
			('power', 'Power'),
			('accessories', 'Accessories')`,
		`INSERT INTO users (email, name, created_at)
		 SELECT
			'user' || g || '@example.com',
			(ARRAY[
				'Alice','Bob','Carol','Dave','Erin','Frank','Grace','Hank','Ivy','Jack',
				'Karen','Leo','Mia','Noah','Olivia','Paul','Quinn','Rita','Sam','Tina',
				'Uma','Vince','Wendy','Xander','Yara','Zane'
			])[1 + ((g - 1) % 26)] || ' ' ||
			(ARRAY[
				'Smith','Johnson','Williams','Brown','Jones','Garcia','Miller','Davis',
				'Rodriguez','Martinez','Hernandez','Lopez','Gonzalez','Wilson','Anderson',
				'Thomas','Taylor','Moore','Jackson','Martin'
			])[1 + ((g - 1) % 20)],
			now() - ((random() * 730)::int || ' days')::interval - ((random() * 86400)::int || ' seconds')::interval
		 FROM generate_series(1, 1000) AS g`,
		`INSERT INTO products (sku, name, price_cents, category_id, metadata)
		 SELECT
			'SKU-' || lpad(g::text, 4, '0'),
			(ARRAY[
				'Keyboard','Mouse','Monitor','Cable','Headset','SSD','Router','Dock',
				'Webcam','Microphone','Speaker','Charger','Hub','Adapter','Stand',
				'Case','Mat','Lamp','Fan','Battery'
			])[1 + ((g - 1) % 20)] || ' ' ||
			(ARRAY[
				'Pro','Lite','Max','Mini','Ultra','Plus','Air','Edge','Core','Prime'
			])[1 + ((g - 1) % 10)] || ' ' || g,
			500 + (g * 137) % 49900,
			1 + ((g - 1) % 8),
			jsonb_build_object(
				'color', (ARRAY['black','white','silver','blue','red','gray'])[1 + ((g - 1) % 6)],
				'weight_g', 50 + (g * 17) % 2500,
				'in_stock', (g % 7) <> 0,
				'tags', to_jsonb(ARRAY[
					(ARRAY['new','sale','bundle','popular','limited'])[1 + ((g - 1) % 5)],
					(ARRAY['office','gaming','travel','studio','home'])[1 + ((g - 1) % 5)]
				])
			)
		 FROM generate_series(1, 250) AS g`,
		`INSERT INTO orders (user_id, total_cents, status, created_at)
		 SELECT
			1 + ((g - 1) % 1000),
			1000 + (g * 913) % 99000,
			(ARRAY['pending','paid','shipped','cancelled','refunded'])[1 + ((g - 1) % 5)],
			now() - ((random() * 365)::int || ' days')::interval - ((random() * 86400)::int || ' seconds')::interval
		 FROM generate_series(1, 5000) AS g`,
		`INSERT INTO order_items (order_id, product_id, quantity, unit_price_cents)
		 SELECT
			o.id,
			1 + ((o.id * n + n) % 250),
			1 + ((o.id + n) % 5),
			500 + ((o.id * n * 37) % 49900)
		 FROM orders o
		 CROSS JOIN generate_series(1, 2 + (o.id % 4)) AS n`,
		`INSERT INTO billing.invoices (user_id, amount_cents, status, issued_at)
		 SELECT
			1 + ((g - 1) % 1000),
			1500 + (g * 641) % 75000,
			(ARRAY['open','paid','void','past_due'])[1 + ((g - 1) % 4)],
			now() - ((random() * 400)::int || ' days')::interval
		 FROM generate_series(1, 600) AS g`,
		`INSERT INTO analytics.events (user_id, event_type, payload, created_at)
		 SELECT
			1 + ((g - 1) % 1000),
			(ARRAY[
				'page_view','click','add_to_cart','checkout_start','purchase',
				'search','login','logout','signup','error'
			])[1 + ((g - 1) % 10)],
			jsonb_build_object(
				'session_id', 'sess_' || ((g * 7919) % 50000),
				'path', (ARRAY['/','/products','/cart','/checkout','/account','/search'])[1 + ((g - 1) % 6)],
				'referer', (ARRAY['direct','google','twitter','email','ads','github'])[1 + ((g - 1) % 6)],
				'device', (ARRAY['desktop','mobile','tablet'])[1 + ((g - 1) % 3)],
				'value', (g * 13) % 1000
			),
			now() - ((random() * 90)::int || ' days')::interval - ((random() * 86400)::int || ' seconds')::interval
		 FROM generate_series(1, 10000) AS g`,
		`ANALYZE users`,
		`ANALYZE product_categories`,
		`ANALYZE products`,
		`ANALYZE orders`,
		`ANALYZE order_items`,
		`ANALYZE billing.invoices`,
		`ANALYZE analytics.events`,
	}

	for _, s := range stmts {
		if _, err := conn.Exec(ctx, s); err != nil {
			fmt.Fprintf(os.Stderr, "seed: %v\n  sql: %s\n", err, s)
			os.Exit(1)
		}
	}

	counts := []struct {
		label string
		sql   string
	}{
		{"users", `SELECT count(*) FROM users`},
		{"product_categories", `SELECT count(*) FROM product_categories`},
		{"products", `SELECT count(*) FROM products`},
		{"orders", `SELECT count(*) FROM orders`},
		{"order_items", `SELECT count(*) FROM order_items`},
		{"billing.invoices", `SELECT count(*) FROM billing.invoices`},
		{"analytics.events", `SELECT count(*) FROM analytics.events`},
	}
	fmt.Println("Seeded demo database successfully")
	for _, c := range counts {
		var n int64
		if err := conn.QueryRow(ctx, c.sql).Scan(&n); err != nil {
			fmt.Fprintf(os.Stderr, "count %s: %v\n", c.label, err)
			os.Exit(1)
		}
		fmt.Printf("  %-22s %d\n", c.label+":", n)
	}
}
