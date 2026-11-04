package postgres

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
)

// Existing migration bytes are immutable once applied.
//
//go:embed schema.sql
var schemaSQL string

//go:embed schema_002_garage_clubs.sql
var garageClubsSQL string

//go:embed schema_003_bike_photos.sql
var bikePhotosSQL string

//go:embed schema_004_club_images.sql
var clubImagesSQL string

//go:embed schema_005_avatars_ride_media.sql
var mediaSQL string

//go:embed schema_006_trust.sql
var trustSQL string

//go:embed schema_007_notifications_preferences.sql
var notificationsSQL string

//go:embed schema_008_club_notifications.sql
var clubNotificationsSQL string

//go:embed schema_009_notification_preferences_reminders.sql
var notificationPreferencesRemindersSQL string

type migration struct {
	version int
	sql     string
}

func migrations() []migration {
	return []migration{{1, schemaSQL}, {2, garageClubsSQL}, {3, bikePhotosSQL}, {4, clubImagesSQL}, {5, mediaSQL}, {6, trustSQL}, {7, notificationsSQL}, {8, clubNotificationsSQL}, {9, notificationPreferencesRemindersSQL}}
}
func migrationChecksum(sql string) string { return fmt.Sprintf("%x", sha256.Sum256([]byte(sql))) }
func schemaChecksum() string              { return migrationChecksum(schemaSQL) }

func (s *Store) Migrate(ctx context.Context) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return repositoryError(err)
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(775021613)"); err != nil {
		return repositoryError(err)
	}
	if _, err = tx.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations(version integer PRIMARY KEY,checksum text NOT NULL)`); err != nil {
		return repositoryError(err)
	}
	all := migrations()
	var unknown int
	if err = tx.QueryRow(ctx, "SELECT count(*) FROM schema_migrations WHERE version<1 OR version>$1", all[len(all)-1].version).Scan(&unknown); err != nil {
		return repositoryError(err)
	}
	if unknown != 0 {
		return errors.New("database has migrations unsupported by this executable")
	}
	for _, m := range all {
		var got string
		err = tx.QueryRow(ctx, "SELECT checksum FROM schema_migrations WHERE version=$1", m.version).Scan(&got)
		if err == nil {
			if got != migrationChecksum(m.sql) {
				return errors.New("migration checksum differs; do not modify an applied migration")
			}
			continue
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return repositoryError(err)
		}
		if _, err = tx.Exec(ctx, m.sql); err != nil {
			return repositoryError(err)
		}
		if _, err = tx.Exec(ctx, "INSERT INTO schema_migrations(version,checksum) VALUES($1,$2)", m.version, migrationChecksum(m.sql)); err != nil {
			return repositoryError(err)
		}
	}
	return repositoryError(tx.Commit(ctx))
}
func (s *Store) Ready(ctx context.Context) error {
	rows, err := s.pool.Query(ctx, "SELECT version,checksum FROM schema_migrations ORDER BY version")
	if err != nil {
		return repositoryError(err)
	}
	defer rows.Close()
	all := migrations()
	seen := 0
	for rows.Next() {
		var version int
		var checksum string
		if err = rows.Scan(&version, &checksum); err != nil {
			return repositoryError(err)
		}
		if seen >= len(all) || version != all[seen].version || checksum != migrationChecksum(all[seen].sql) {
			return errors.New("schema migration required")
		}
		seen++
	}
	if err = rows.Err(); err != nil {
		return repositoryError(err)
	}
	if seen != len(all) {
		return errors.New("schema migration required")
	}
	return nil
}
