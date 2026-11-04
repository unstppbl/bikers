package postgres

import (
	"context"
	"github.com/google/uuid"
	"testing"
)

func TestVersionOneUpgradePreservesAccountsAndIsReplayable(t *testing.T) {
	store, pool := testStore(t)
	ctx := context.Background()
	removeMVPAdditions(t, pool)
	// Reconstruct the prior schema in this isolated disposable test namespace.
	if _, err := pool.Exec(ctx, "ALTER TABLE clubs DROP CONSTRAINT clubs_current_image_scope; ALTER TABLE bikes DROP CONSTRAINT bikes_cover_photo_scope; DROP TABLE club_images,photo_upload_attempts,bike_photos,club_rides,club_memberships,clubs,maintenance_entries,bikes; DELETE FROM schema_migrations WHERE version>1"); err != nil {
		t.Fatal(err)
	}
	id := uuid.NewString()
	if _, err := pool.Exec(ctx, "INSERT INTO users(id,phone,display_name,created_at) VALUES($1,'+77011234567','Preserved rider',now())", id); err != nil {
		t.Fatal(err)
	}
	if err := store.Ready(ctx); err == nil {
		t.Fatal("version1 incorrectly ready for current executable")
	}
	if err := store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err := store.Ready(ctx); err != nil {
		t.Fatal(err)
	}
	var name string
	if err := pool.QueryRow(ctx, "SELECT display_name FROM users WHERE id=$1", id).Scan(&name); err != nil || name != "Preserved rider" {
		t.Fatal("upgrade changed existing account")
	}
	bike := uuid.NewString()
	if _, err := pool.Exec(ctx, "INSERT INTO bikes(id,owner_id,make,model,year,created_at,updated_at) VALUES($1,$2,'Honda','CB400',2005,now(),now())", bike, id); err != nil {
		t.Fatal(err)
	}
	if err := store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM bikes WHERE id=$1", bike).Scan(&count); err != nil || count != 1 {
		t.Fatal("migration replay changed garage data")
	}
}

func TestMigrationReceiptDriftAndUnknownVersionsFailSafely(t *testing.T) {
	store, pool := testStore(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, "UPDATE schema_migrations SET checksum='changed' WHERE version=2"); err != nil {
		t.Fatal(err)
	}
	if err := store.Ready(ctx); err == nil {
		t.Fatal("changed receipt accepted by readiness")
	}
	if err := store.Migrate(ctx); err == nil {
		t.Fatal("changed receipt accepted by migrate")
	}
	if _, err := pool.Exec(ctx, "UPDATE schema_migrations SET checksum=$1 WHERE version=2", migrationChecksum(garageClubsSQL)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "INSERT INTO schema_migrations(version,checksum) VALUES($1,'future')", len(migrations())+1); err != nil {
		t.Fatal(err)
	}
	if err := store.Ready(ctx); err == nil {
		t.Fatal("unknown future schema accepted")
	}
	if err := store.Migrate(ctx); err == nil {
		t.Fatal("unknown future schema migrated")
	}
}
