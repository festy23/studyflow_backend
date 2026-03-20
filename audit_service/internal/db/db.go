package db

import (
	"context"
	"errors"
	"log"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
)

// New creates a new pgxpool.Pool and optionally runs migrations.
func New(ctx context.Context, postgresURL string, maxConn, minConn int32, autoMigrate bool) (*pgxpool.Pool, error) {
	if autoMigrate {
		if err := runMigrations(postgresURL); err != nil {
			return nil, err
		}
	}

	pgxCfg, err := pgxpool.ParseConfig(postgresURL)
	if err != nil {
		return nil, err
	}

	pgxCfg.MaxConns = maxConn
	pgxCfg.MinConns = minConn

	pool, err := pgxpool.NewWithConfig(ctx, pgxCfg)
	if err != nil {
		return nil, err
	}

	if err = pool.Ping(ctx); err != nil {
		return nil, err
	}

	return pool, nil
}

func runMigrations(postgresURL string) error {
	m, err := migrate.New(
		"file://migrations",
		postgresURL,
	)
	if err != nil {
		return err
	}

	err = m.Up()
	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}
	log.Default().Println("Migrations successfully applied")
	return nil
}
