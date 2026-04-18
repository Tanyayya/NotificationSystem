package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	_ "github.com/lib/pq"
)

// FollowerDB wraps a PostgreSQL connection pool for follower lookups.
type FollowerDB struct {
	pool *sql.DB
}

// NewFollowerDB opens a connection pool to PostgreSQL using the given DSN.
func NewFollowerDB(dsn string) (*FollowerDB, error) {
	pool, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	if err := pool.Ping(); err != nil {
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	log.Printf("connected to PostgreSQL")
	return &FollowerDB{pool: pool}, nil
}

// Close releases the connection pool.
func (d *FollowerDB) Close() error {
	return d.pool.Close()
}

// GetFollowers returns the IDs of all users who follow followingID.
func (d *FollowerDB) GetFollowers(ctx context.Context, followingID string) ([]string, error) {
	rows, err := d.pool.QueryContext(ctx,
		`SELECT follower_id FROM followers WHERE following_id = $1`,
		followingID,
	)
	if err != nil {
		return nil, fmt.Errorf("query followers: %w", err)
	}
	defer rows.Close()

	var followers []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan follower: %w", err)
		}
		followers = append(followers, id)
	}
	return followers, rows.Err()
}
