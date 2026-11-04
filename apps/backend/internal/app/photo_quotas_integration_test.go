package app

import (
	"bytes"
	"context"
	"sync"
	"testing"
	"time"
)

const quotaReservationBytes int64 = 4 << 20
const quotaOwnerBytes int64 = 100 << 20
const quotaGlobalBytes int64 = 1 << 30

func TestPhotoGlobalReservationRaceAndLazyExpiryPreserveReadyContent(t *testing.T) {
	f := newFixture(t)
	first := f.login("+77011234701", "First uploader")
	second := f.login("+77011234702", "Second uploader")
	firstBike := testBike(t, f, first)["id"].(string)
	secondBike := testBike(t, f, second)["id"].(string)
	ctx := context.Background()
	// Eleven owners have four bikes each. The 255 reservations occupy at most
	// six slots per bike and 96 MiB per owner, leaving one global reservation.
	_, err := f.db.Exec(ctx, `
	 WITH seeded_owners AS (
	  INSERT INTO users(id,phone,status,display_name,city,languages,locale,onboarding_complete,created_at)
	  SELECT ('f1000000-0000-0000-0000-'||lpad(n::text,12,'0'))::uuid,
	   '+770199'||lpad(n::text,5,'0'),'active','Quota fixture owner '||n,'Almaty',ARRAY['en'],'en',true,$1
	  FROM generate_series(1,11) AS source(n)
	  RETURNING id
	 ), seeded_bikes AS (
	  INSERT INTO bikes(id,owner_id,make,model,year,visibility,created_at,updated_at)
	  SELECT ('f2000000-0000-0000-0000-'||lpad(n::text,12,'0'))::uuid,
	   owner.id,'Fixture','Quota bike '||n,2020,'private',$1,$1
	  FROM generate_series(1,44) AS source(n)
	  JOIN seeded_owners AS owner ON owner.id=('f1000000-0000-0000-0000-'||lpad((((n-1)/4)+1)::text,12,'0'))::uuid
	  RETURNING id,owner_id
	 )
	 INSERT INTO bike_photos(id,bike_id,owner_id,state,reserved_bytes,created_at,expires_at)
	 SELECT ('f3000000-0000-0000-0000-'||lpad(n::text,12,'0'))::uuid,
	  bike.id,bike.owner_id,'pending',4194304,$1,$2
	 FROM generate_series(1,255) AS source(n)
	 JOIN seeded_bikes AS bike ON bike.id=('f2000000-0000-0000-0000-'||lpad((((n-1)/6)+1)::text,12,'0'))::uuid
	`, f.now, f.now.Add(10*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	var count, seededOwners, seededBikes, maximumBikeCount int
	var reserved, maximumOwnerBytes int64
	err = f.db.QueryRow(ctx, `SELECT
	 (SELECT count(*) FROM bike_photos),
	 (SELECT count(*) FROM users WHERE id::text LIKE 'f1000000-%'),
	 (SELECT count(*) FROM bikes WHERE id::text LIKE 'f2000000-%'),
	 (SELECT max(n) FROM (SELECT count(*) n FROM bike_photos GROUP BY bike_id) AS grouped),
	 (SELECT sum(reserved_bytes) FROM bike_photos),
	 (SELECT max(n) FROM (SELECT sum(reserved_bytes) n FROM bike_photos GROUP BY owner_id) AS grouped)
	`).Scan(&count, &seededOwners, &seededBikes, &maximumBikeCount, &reserved, &maximumOwnerBytes)
	if err != nil || count != 255 || seededOwners != 11 || seededBikes != 44 || maximumBikeCount > 6 || reserved != 255*quotaReservationBytes || maximumOwnerBytes > 96<<20 {
		t.Fatalf("invalid reservation fixture: photos=%d owners=%d bikes=%d max_bike=%d bytes=%d max_owner=%d err=%v", count, seededOwners, seededBikes, maximumBikeCount, reserved, maximumOwnerBytes, err)
	}
	type outcome struct {
		actor *actor
		bike  string
		res   response
	}
	data := photoPNG(t)
	start := make(chan struct{})
	results := make(chan outcome, 2)
	var uploads sync.WaitGroup
	for _, candidate := range []outcome{{actor: first, bike: firstBike}, {actor: second, bike: secondBike}} {
		uploads.Go(func() {
			<-start
			candidate.res = photoUpload(t, f, candidate.bike, data, candidate.actor, true)
			results <- candidate
		})
	}
	close(start)
	uploads.Wait()
	close(results)
	var winner, loser outcome
	created, rejected := 0, 0
	for result := range results {
		switch result.res.status {
		case 201:
			created++
			winner = result
		case 409:
			rejected++
			loser = result
			if result.res.body["error"].(map[string]any)["code"] != "photo_storage_limit" {
				t.Fatalf("quota race failed for another reason: %s", result.res.raw)
			}
		default:
			t.Fatalf("unexpected competing upload response: %d %s", result.res.status, result.res.raw)
		}
	}
	if created != 1 || rejected != 1 {
		t.Fatalf("global reservation race created=%d rejected=%d; want one of each", created, rejected)
	}
	if err = f.db.QueryRow(ctx, "SELECT sum(reserved_bytes) FROM bike_photos").Scan(&reserved); err != nil || reserved > quotaGlobalBytes || reserved <= 255*quotaReservationBytes {
		t.Fatalf("global reserved+ready bytes escaped quota: %d err=%v", reserved, err)
	}
	photo := winner.res.body["photo"].(map[string]any)
	contentURL := photo["url"].(string)
	beforeExpiry := photoRead(t, f, contentURL, winner.actor, "GET", 200)
	// Change the injected clock only after both HTTP requests have returned.
	f.now = f.now.Add(11 * time.Minute)
	expect(t, photoUpload(t, f, loser.bike, data, loser.actor, true), 201)
	var pending, ready int
	err = f.db.QueryRow(ctx, "SELECT count(*) FILTER(WHERE state='pending'),count(*) FILTER(WHERE state='ready'),sum(reserved_bytes) FROM bike_photos").Scan(&pending, &ready, &reserved)
	if err != nil || pending != 0 || ready != 2 || reserved > quotaGlobalBytes {
		t.Fatalf("lazy expiry did not release pending-only quota: pending=%d ready=%d bytes=%d err=%v", pending, ready, reserved, err)
	}
	if afterExpiry := photoRead(t, f, contentURL, winner.actor, "GET", 200); !bytes.Equal(beforeExpiry, afterExpiry) {
		t.Fatal("expiring pending reservations changed already-ready photo content")
	}
}

func TestPhotoOwnerReservationLimitSpansBikesWithoutBlockingAnotherOwner(t *testing.T) {
	f := newFixture(t)
	owner := f.login("+77011234711", "Full-quota owner")
	other := f.login("+77011234712", "Separate owner")
	ctx := context.Background()
	// Four bikes hold six reservations each: 96 MiB. A real upload can reserve
	// the final 4 MiB; its nonempty ready JPEG then blocks another reservation.
	_, err := f.db.Exec(ctx, `
	 WITH seeded_bikes AS (
	  INSERT INTO bikes(id,owner_id,make,model,year,visibility,created_at,updated_at)
	  SELECT ('f4000000-0000-0000-0000-'||lpad(n::text,12,'0'))::uuid,
	   $1::uuid,'Fixture','Owner quota bike '||n,2020,'private',$2,$2
	  FROM generate_series(1,4) AS source(n)
	  RETURNING id,owner_id
	 )
	 INSERT INTO bike_photos(id,bike_id,owner_id,state,reserved_bytes,created_at,expires_at)
	 SELECT ('f5000000-0000-0000-0000-'||lpad(n::text,12,'0'))::uuid,
	  bike.id,bike.owner_id,'pending',4194304,$2,$3
	 FROM generate_series(1,24) AS source(n)
	 JOIN seeded_bikes AS bike ON bike.id=('f4000000-0000-0000-0000-'||lpad((((n-1)/6)+1)::text,12,'0'))::uuid
	`, owner.id, f.now, f.now.Add(10*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	allowedBike := testBike(t, f, owner)["id"].(string)
	blockedBike := testBike(t, f, owner)["id"].(string)
	otherBike := testBike(t, f, other)["id"].(string)
	data := photoPNG(t)
	expect(t, photoUpload(t, f, allowedBike, data, owner, true), 201)
	blocked := photoUpload(t, f, blockedBike, data, owner, true)
	expect(t, blocked, 409)
	if blocked.body["error"].(map[string]any)["code"] != "photo_storage_limit" {
		t.Fatalf("cross-bike owner quota failed for another reason: %s", blocked.raw)
	}
	expect(t, photoUpload(t, f, otherBike, data, other, true), 201)
	var ownerBytes, globalBytes int64
	err = f.db.QueryRow(ctx, "SELECT sum(reserved_bytes) FILTER(WHERE owner_id=$1),sum(reserved_bytes) FROM bike_photos", owner.id).Scan(&ownerBytes, &globalBytes)
	if err != nil || ownerBytes <= 24*quotaReservationBytes || ownerBytes > quotaOwnerBytes || globalBytes > quotaGlobalBytes {
		t.Fatalf("owner/global accounting wrong: owner=%d global=%d err=%v", ownerBytes, globalBytes, err)
	}
	if len(photoGallery(t, f, blockedBike, owner)["photos"].([]any)) != 0 || len(photoGallery(t, f, otherBike, other)["photos"].([]any)) != 1 {
		t.Fatal("owner quota crossed another owner's bike boundary")
	}
}

func TestPhotoMalformedAttemptRateLimitPersistsAcrossAppRestart(t *testing.T) {
	f := newFixture(t)
	owner := f.login("+77011234721", "Rate-limited owner")
	bike := testBike(t, f, owner)["id"].(string)
	malformedPNG := []byte{0x89, 'P', 'N', 'G'}
	for range 12 {
		expect(t, photoUpload(t, f, bike, malformedPNG, owner, true), 422)
	}
	assertLimited := func() {
		t.Helper()
		limited := photoUpload(t, f, bike, malformedPNG, owner, true)
		expect(t, limited, 429)
		if limited.body["error"].(map[string]any)["code"] != "photo_rate_limited" {
			t.Fatalf("expected persistent attempt limit, got %s", limited.raw)
		}
	}
	assertLimited()
	f.startServer(f.gateway)
	assertLimited()
	var attempts, pending, ready int
	err := f.db.QueryRow(context.Background(), `SELECT
	 (SELECT count(*) FROM photo_upload_attempts WHERE owner_id=$1),
	 (SELECT count(*) FROM bike_photos WHERE owner_id=$1 AND state='pending'),
	 (SELECT count(*) FROM bike_photos WHERE owner_id=$1 AND state='ready')`, owner.id).Scan(&attempts, &pending, &ready)
	if err != nil || attempts != 12 || pending != 0 || ready != 0 {
		t.Fatalf("failed images lost attempt history or retained reservations: attempts=%d pending=%d ready=%d err=%v", attempts, pending, ready, err)
	}
	gallery := photoGallery(t, f, bike, owner)
	if gallery["version"] != float64(1) || gallery["cover_photo_id"] != "" || len(gallery["photos"].([]any)) != 0 {
		t.Fatal("invalid uploads changed the gallery or catalogue default")
	}
}
