package postgres

import (
	"bytes"
	"context"
	"image"
	"image/jpeg"
	"testing"

	"github.com/google/uuid"
)

func TestVersionTwoPhotoUpgradePreservesGarageAndReplaysContent(t *testing.T) {
	store, pool := testStore(t)
	ctx := context.Background()
	removeMVPAdditions(t, pool)
	if _, err := pool.Exec(ctx, `ALTER TABLE clubs DROP CONSTRAINT clubs_current_image_scope;
DROP TABLE club_images;
ALTER TABLE clubs DROP COLUMN current_image_id,DROP COLUMN image_version;
ALTER TABLE bikes DROP CONSTRAINT bikes_cover_photo_scope;
DROP TABLE bike_photos,photo_upload_attempts;
ALTER TABLE bikes DROP COLUMN cover_photo_id,DROP COLUMN photo_version;
DELETE FROM schema_migrations WHERE version>=3`); err != nil {
		t.Fatal(err)
	}
	owner, bike, entry := uuid.NewString(), uuid.NewString(), uuid.NewString()
	if _, err := pool.Exec(ctx, "INSERT INTO users(id,phone,display_name,created_at) VALUES($1,'+77011234631','Preserved owner',now())", owner); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "INSERT INTO bikes(id,owner_id,make,model,year,version,created_at,updated_at) VALUES($1,$2,'Honda','CB400',2005,7,now(),now())", bike, owner); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "INSERT INTO maintenance_entries(id,bike_id,service_date,mileage_km,work_performed,created_at,updated_at) VALUES($1,$2,'2026-11-01',5000,'Preserved private work',now(),now())", entry, bike); err != nil {
		t.Fatal(err)
	}
	if err := store.Ready(ctx); err == nil {
		t.Fatal("version2 accepted without photo migration")
	}
	if err := store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	var version, photoVersion int64
	var cover *string
	if err := pool.QueryRow(ctx, "SELECT version,photo_version,cover_photo_id::text FROM bikes WHERE id=$1", bike).Scan(&version, &photoVersion, &cover); err != nil || version != 7 || photoVersion != 1 || cover != nil {
		t.Fatalf("upgrade changed bike/default: %v %d %d %v", err, version, photoVersion, cover)
	}
	var work string
	if err := pool.QueryRow(ctx, "SELECT work_performed FROM maintenance_entries WHERE id=$1", entry).Scan(&work); err != nil || work != "Preserved private work" {
		t.Fatal("upgrade changed private maintenance")
	}
	var encoded bytes.Buffer
	if err := jpeg.Encode(&encoded, image.NewRGBA(image.Rect(0, 0, 4, 3)), nil); err != nil {
		t.Fatal(err)
	}
	photo := uuid.NewString()
	if _, err := pool.Exec(ctx, `INSERT INTO bike_photos(id,bike_id,owner_id,state,reserved_bytes,content,width,height,size_bytes,created_at,ready_at)
VALUES($1,$2,$3,'ready',$4,$5,4,3,$4,now(),now())`, photo, bike, owner, encoded.Len(), encoded.Bytes()); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "UPDATE bikes SET cover_photo_id=$1,photo_version=2 WHERE id=$2", photo, bike); err != nil {
		t.Fatal(err)
	}
	if err := store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err := store.Ready(ctx); err != nil {
		t.Fatal(err)
	}
	var data []byte
	if err := pool.QueryRow(ctx, "SELECT content FROM bike_photos WHERE id=$1", photo).Scan(&data); err != nil || !bytes.Equal(data, encoded.Bytes()) {
		t.Fatal("migration replay changed image bytes")
	}
	if err := pool.QueryRow(ctx, "SELECT cover_photo_id::text FROM bikes WHERE id=$1", bike).Scan(&cover); err != nil || cover == nil || *cover != photo {
		t.Fatal("migration replay lost selected cover")
	}
}
