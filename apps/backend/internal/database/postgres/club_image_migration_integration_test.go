package postgres

import (
	"bytes"
	"context"
	"crypto/sha256"
	"image"
	"image/jpeg"
	"testing"

	"github.com/google/uuid"
)

func TestVersionThreeClubImageUpgradePreservesAccountsAndBikePhotos(t *testing.T) {
	store, pool := testStore(t)
	ctx := context.Background()
	removeMVPAdditions(t, pool)
	if _, err := pool.Exec(ctx, `ALTER TABLE clubs DROP CONSTRAINT clubs_current_image_scope;
DROP TABLE club_images;
ALTER TABLE clubs DROP COLUMN current_image_id,DROP COLUMN image_version;
DELETE FROM schema_migrations WHERE version=4`); err != nil {
		t.Fatal(err)
	}
	owner, bike, photo, club := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	if _, err := pool.Exec(ctx, "INSERT INTO users(id,phone,display_name,created_at) VALUES($1,'+77011234671','Preserved club owner',now())", owner); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte("isolated migration fixture session"))
	if _, err := pool.Exec(ctx, `INSERT INTO sessions(token_digest,user_id,csrf_token,created_at,last_seen_at,expires_at)
VALUES($1,$2,'isolated-csrf',now(),now(),now()+interval '1 day')`, digest[:], owner); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO bikes(id,owner_id,make,model,year,version,created_at,updated_at)
VALUES($1,$2,'Honda','CB400',2005,7,now(),now())`, bike, owner); err != nil {
		t.Fatal(err)
	}
	var encoded bytes.Buffer
	if err := jpeg.Encode(&encoded, image.NewRGBA(image.Rect(0, 0, 4, 3)), nil); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO bike_photos(id,bike_id,owner_id,state,reserved_bytes,content,width,height,size_bytes,created_at,ready_at)
VALUES($1,$2,$3,'ready',$4,$5,4,3,$4,now(),now())`, photo, bike, owner, encoded.Len(), encoded.Bytes()); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "UPDATE bikes SET cover_photo_id=$1,photo_version=2 WHERE id=$2", photo, bike); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO clubs(id,owner_id,name,city,description,status,version,created_at,updated_at)
VALUES($1,$2,'Preserved club','Almaty','Existing public description','active',5,now(),now())`, club, owner); err != nil {
		t.Fatal(err)
	}
	if err := store.Ready(ctx); err == nil {
		t.Fatal("version3 accepted without club image migration")
	}
	if err := store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err := store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err := store.Ready(ctx); err != nil {
		t.Fatal(err)
	}
	var version, imageVersion int64
	var imageID *string
	var name string
	if err := pool.QueryRow(ctx, "SELECT name,version,image_version,current_image_id::text FROM clubs WHERE id=$1", club).Scan(&name, &version, &imageVersion, &imageID); err != nil || name != "Preserved club" || version != 5 || imageVersion != 1 || imageID != nil {
		t.Fatalf("upgrade changed club/default: %v %q %d %d %v", err, name, version, imageVersion, imageID)
	}
	var content []byte
	var cover string
	var bikeVersion, galleryVersion int64
	if err := pool.QueryRow(ctx, `SELECT p.content,b.cover_photo_id::text,b.version,b.photo_version
FROM bikes b JOIN bike_photos p ON p.id=b.cover_photo_id WHERE b.id=$1`, bike).Scan(&content, &cover, &bikeVersion, &galleryVersion); err != nil || !bytes.Equal(content, encoded.Bytes()) || cover != photo || bikeVersion != 7 || galleryVersion != 2 {
		t.Fatal("upgrade/replay changed bike content or selected cover", err)
	}
	var sessions int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM sessions WHERE user_id=$1 AND token_digest=$2 AND revoked_at IS NULL", owner, digest[:]).Scan(&sessions); err != nil || sessions != 1 {
		t.Fatal("upgrade/replay changed existing session", err)
	}
	clubImage := uuid.NewString()
	if _, err := pool.Exec(ctx, `INSERT INTO club_images(id,club_id,owner_id,state,reserved_bytes,content,width,height,size_bytes,club_version,image_version,created_at,ready_at)
VALUES($1,$2,$3,'ready',$4,$5,4,3,$4,5,1,now(),now())`, clubImage, club, owner, encoded.Len(), encoded.Bytes()); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "UPDATE clubs SET current_image_id=$1,image_version=2 WHERE id=$2", clubImage, club); err != nil {
		t.Fatal(err)
	}
	if err := store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT i.content,c.current_image_id::text,c.version,c.image_version
FROM clubs c JOIN club_images i ON i.id=c.current_image_id WHERE c.id=$1`, club).Scan(&content, &cover, &version, &imageVersion); err != nil || !bytes.Equal(content, encoded.Bytes()) || cover != clubImage || version != 5 || imageVersion != 2 {
		t.Fatal("migration replay changed club image bytes or selection", err)
	}
}
