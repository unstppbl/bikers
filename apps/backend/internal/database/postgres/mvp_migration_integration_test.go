package postgres

import (
	"bytes"
	"context"
	"crypto/sha256"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// removeMVPAdditions reconstructs version four only inside disposable test schemas.
func removeMVPAdditions(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `DROP TABLE push_deliveries,push_subscriptions,notifications,announcements;
ALTER TABLE users DROP COLUMN country_code,DROP COLUMN currency_code;
DROP TABLE trust_audit,trust_reports,community_badges,moderation_hidden,user_blocks,trust_admins;
ALTER TABLE users DROP CONSTRAINT users_current_avatar_scope;
DROP TABLE avatars,ride_photos,ride_video_links;
ALTER TABLE users DROP COLUMN avatar_version,DROP COLUMN current_avatar_id;
ALTER TABLE rides DROP COLUMN album_version;
ALTER TABLE participation DROP COLUMN media_version;
DELETE FROM schema_migrations WHERE version>=5`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestVersionFourMVPUpgradePreservesSessionsAndClubImage(t *testing.T) {
	store, pool := testStore(t)
	ctx := context.Background()
	removeMVPAdditions(t, pool)
	owner, club, image := uuid.NewString(), uuid.NewString(), uuid.NewString()
	digest := sha256.Sum256([]byte("isolated version four session"))
	content := []byte("opaque existing processed image bytes")
	if _, err := pool.Exec(ctx, `INSERT INTO users(id,phone,display_name,locale,created_at) VALUES($1,'+77011234781','Existing owner','kk',now());`, owner); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO sessions(token_digest,user_id,csrf_token,created_at,last_seen_at,expires_at) VALUES($1,$2,'existing-csrf',now(),now(),now()+interval '1 day')`, digest[:], owner); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO clubs(id,owner_id,name,city,description,status,version,created_at,updated_at) VALUES($1,$2,'Existing club','Almaty','Existing description','active',9,now(),now())`, club, owner); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO club_images(id,club_id,owner_id,state,reserved_bytes,content,width,height,size_bytes,club_version,image_version,created_at,ready_at) VALUES($1,$2,$3,'ready',$4,$5,4,3,$4,9,1,now(),now())`, image, club, owner, len(content), content); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE clubs SET current_image_id=$1,image_version=2 WHERE id=$2`, image, club); err != nil {
		t.Fatal(err)
	}
	if err := store.Ready(ctx); err == nil {
		t.Fatal("version four reported ready before MVP upgrade")
	}
	for range 2 {
		if err := store.Migrate(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.Ready(ctx); err != nil {
		t.Fatal(err)
	}
	var country, currency, locale, csrf string
	var data []byte
	var clubVersion, imageVersion, avatarVersion int64
	if err := pool.QueryRow(ctx, `SELECT u.country_code,u.currency_code,u.locale,u.avatar_version,s.csrf_token,i.content,c.version,c.image_version FROM users u JOIN sessions s ON s.user_id=u.id JOIN clubs c ON c.owner_id=u.id JOIN club_images i ON i.id=c.current_image_id WHERE u.id=$1 AND s.token_digest=$2 AND s.revoked_at IS NULL`, owner, digest[:]).Scan(&country, &currency, &locale, &avatarVersion, &csrf, &data, &clubVersion, &imageVersion); err != nil {
		t.Fatal(err)
	}
	if country != "KZ" || currency != "KZT" || locale != "kk" || avatarVersion != 1 || csrf != "existing-csrf" || !bytes.Equal(data, content) || clubVersion != 9 || imageVersion != 2 {
		t.Fatal("MVP upgrade changed existing account, session, locale, image or version")
	}
	var admins, notices int
	if err := pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM trust_admins),(SELECT count(*) FROM notifications)`).Scan(&admins, &notices); err != nil || admins != 0 || notices != 0 {
		t.Fatal("upgrade synthesized admin roles or notices", err)
	}
}
