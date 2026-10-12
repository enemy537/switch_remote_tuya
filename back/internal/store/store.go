// Package store holds the Postgres queries for users and devices.
package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound = errors.New("not found")
	ErrConflict = errors.New("already exists")
)

type User struct {
	ID           int64
	Email        string
	PasswordHash string
	IsAdmin      bool
	TuyaUID      string
	DeviceCount  int // only filled by Users
}

type Device struct {
	ID        string
	UserID    int64
	Name      string
	UserEmail string // only filled by AllDevices
}

type Store struct {
	pool *pgxpool.Pool
}

// Open prepares a small pool. DATABASE_URL points at pgbouncer in transaction
// mode, where server-side prepared statements are unsafe, so they are disabled.
func Open(ctx context.Context, databaseURL string) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, err
	}
	cfg.MaxConns = 2
	cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeExec
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return &Store{pool: pool}, nil
}

const userColumns = `id, email, password_hash, is_admin, tuya_uid`

func (s *Store) UserByEmail(ctx context.Context, email string) (User, error) {
	return s.user(ctx, `select `+userColumns+` from users where email = $1`, email)
}

func (s *Store) UserByID(ctx context.Context, id int64) (User, error) {
	return s.user(ctx, `select `+userColumns+` from users where id = $1`, id)
}

func (s *Store) user(ctx context.Context, sql string, arg any) (User, error) {
	var u User
	err := s.pool.QueryRow(ctx, sql, arg).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.IsAdmin, &u.TuyaUID)
	if errors.Is(err, pgx.ErrNoRows) {
		return u, ErrNotFound
	}
	return u, err
}

func (s *Store) Users(ctx context.Context) ([]User, error) {
	rows, err := s.pool.Query(ctx, `
		select u.id, u.email, u.is_admin, u.tuya_uid, count(d.id)
		from users u left join devices d on d.user_id = u.id
		group by u.id order by u.email`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (User, error) {
		var u User
		err := row.Scan(&u.ID, &u.Email, &u.IsAdmin, &u.TuyaUID, &u.DeviceCount)
		return u, err
	})
}

// CreateUser inserts u and returns its new id.
func (s *Store) CreateUser(ctx context.Context, u User) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx, `
		insert into users (email, password_hash, is_admin, tuya_uid)
		values ($1, $2, $3, $4) returning id`,
		u.Email, u.PasswordHash, u.IsAdmin, u.TuyaUID).Scan(&id)
	return id, conflict(err)
}

func (s *Store) SetPassword(ctx context.Context, userID int64, hash string) error {
	return s.execOne(ctx, `update users set password_hash = $2 where id = $1`, userID, hash)
}

func (s *Store) DevicesByUser(ctx context.Context, userID int64) ([]Device, error) {
	rows, err := s.pool.Query(ctx,
		`select id, user_id, name from devices where user_id = $1 order by name, id`, userID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (Device, error) {
		var d Device
		err := row.Scan(&d.ID, &d.UserID, &d.Name)
		return d, err
	})
}

// RegisteredIDs returns which of ids are already registered, by any user.
func (s *Store) RegisteredIDs(ctx context.Context, ids []string) ([]string, error) {
	rows, err := s.pool.Query(ctx, `select id from devices where id = any($1)`, ids)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

func (s *Store) CreateDevice(ctx context.Context, d Device) error {
	_, err := s.pool.Exec(ctx,
		`insert into devices (id, user_id, name) values ($1, $2, $3)`, d.ID, d.UserID, d.Name)
	return conflict(err)
}

func (s *Store) DeleteDevice(ctx context.Context, id string, userID int64) error {
	return s.execOne(ctx, `delete from devices where id = $1 and user_id = $2`, id, userID)
}

func (s *Store) OwnsDevice(ctx context.Context, id string, userID int64) (bool, error) {
	var owns bool
	err := s.pool.QueryRow(ctx,
		`select exists (select 1 from devices where id = $1 and user_id = $2)`, id, userID).Scan(&owns)
	return owns, err
}

// AllDevices returns one page of every user's devices and the overall total.
func (s *Store) AllDevices(ctx context.Context, limit, offset int) ([]Device, int, error) {
	var total int
	if err := s.pool.QueryRow(ctx, `select count(*) from devices`).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.pool.Query(ctx, `
		select d.id, d.user_id, d.name, u.email
		from devices d join users u on u.id = d.user_id
		order by u.email, d.name, d.id limit $1 offset $2`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	devices, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Device, error) {
		var d Device
		err := row.Scan(&d.ID, &d.UserID, &d.Name, &d.UserEmail)
		return d, err
	})
	return devices, total, err
}

// execOne runs a statement that must touch exactly one row.
func (s *Store) execOne(ctx context.Context, sql string, args ...any) error {
	tag, err := s.pool.Exec(ctx, sql, args...)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

func conflict(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation
		return ErrConflict
	}
	return err
}
