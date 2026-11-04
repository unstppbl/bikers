package app

import (
	"bytes"
	"context"
	"testing"
	"time"
)

func TestSharedBikeClubProcessingSlotsRejectBeforeAnotherReservation(t *testing.T) {
	f := newFixture(t)
	bikeOwner := f.login("+77011234901", "Bike uploader")
	clubOwner := f.login("+77011234902", "Club uploader")
	bikeID := testBike(t, f, bikeOwner)["id"].(string)
	clubID := testClub(t, f, clubOwner)["id"].(string)
	otherClubID := testClub(t, f, clubOwner)["id"].(string)
	data := photoPNG(t)
	releaseBike, bikeFinished := startGatedImageUpload(t, f, "/api/v1/me/bikes/"+bikeID+"/photos", data, bikeOwner)
	waitImageReservation(t, f, "bike_photos", "bike_id", bikeID)
	releaseClub, clubFinished := startGatedImageUpload(t, f, "/api/v1/clubs/"+clubID+"/image?version=1", data, clubOwner)
	waitImageReservation(t, f, "club_images", "club_id", clubID)
	clubImageFailure(t, clubImageUpload(t, f, otherClubID, "1", data, clubOwner, true), 429, "photo_upload_busy")
	var pending, attempts int
	err := f.db.QueryRow(context.Background(), `SELECT
	 (SELECT count(*) FROM bike_photos WHERE state='pending')+(SELECT count(*) FROM club_images WHERE state='pending'),
	 (SELECT count(*) FROM photo_upload_attempts)`).Scan(&pending, &attempts)
	if err != nil || pending != 2 || attempts != 2 {
		t.Fatalf("busy third upload created persistent work: pending=%d attempts=%d err=%v", pending, attempts, err)
	}
	expect(t, finishImageUpload(t, releaseBike, bikeFinished), 201)
	expect(t, finishImageUpload(t, releaseClub, clubFinished), 201)
	gallery := photoGallery(t, f, bikeID, bikeOwner)
	if gallery["cover_photo_id"] != "" || len(gallery["photos"].([]any)) != 1 {
		t.Fatal("sharing the processor changed bike catalogue-default behavior")
	}
}

func TestSharedBikeClubFailedAttemptsRemainLimitedAcrossRestart(t *testing.T) {
	f := newFixture(t)
	owner := f.login("+77011234911", "Shared attempt owner")
	bikeID := testBike(t, f, owner)["id"].(string)
	clubID := testClub(t, f, owner)["id"].(string)
	malformed := []byte{0x89, 'P', 'N', 'G'}
	for range 6 {
		expect(t, photoUpload(t, f, bikeID, malformed, owner, true), 422)
		expect(t, clubImageUpload(t, f, clubID, "1", malformed, owner, true), 422)
	}
	for range 2 {
		clubImageFailure(t, clubImageUpload(t, f, clubID, "1", photoPNG(t), owner, true), 429, "photo_rate_limited")
		clubImageFailure(t, photoUpload(t, f, bikeID, photoPNG(t), owner, true), 429, "photo_rate_limited")
		f.startServer(f.gateway)
	}
	var attempts, reservations int
	err := f.db.QueryRow(context.Background(), `SELECT
	 (SELECT count(*) FROM photo_upload_attempts WHERE owner_id=$1),
	 (SELECT count(*) FROM bike_photos WHERE owner_id=$1)+(SELECT count(*) FROM club_images WHERE owner_id=$1)`, owner.id).Scan(&attempts, &reservations)
	if err != nil || attempts != 12 || reservations != 0 {
		t.Fatalf("cross-feature invalid uploads lost rate history or leaked reservations: attempts=%d reservations=%d err=%v", attempts, reservations, err)
	}
}

func TestSharedBikeClubOwnerQuotaCountsCurrentAndPendingReplacement(t *testing.T) {
	f := newFixture(t)
	owner := f.login("+77011234921", "Shared storage owner")
	other := f.login("+77011234922", "Separate storage owner")
	ctx := context.Background()
	_, err := f.db.Exec(ctx, `
	 WITH seeded_bikes AS (
	  INSERT INTO bikes(id,owner_id,make,model,year,visibility,created_at,updated_at)
	  SELECT ('c4000000-0000-0000-0000-'||lpad(n::text,12,'0'))::uuid,
	   $1::uuid,'Fixture','Shared quota bike '||n,2020,'private',$2,$2
	  FROM generate_series(1,4) AS source(n)
	  RETURNING id,owner_id
	 )
	 INSERT INTO bike_photos(id,bike_id,owner_id,state,reserved_bytes,created_at,expires_at)
	 SELECT ('c5000000-0000-0000-0000-'||lpad(n::text,12,'0'))::uuid,
	  bike.id,bike.owner_id,'pending',4194304,$2,$3
	 FROM generate_series(1,24) AS source(n)
	 JOIN seeded_bikes AS bike ON bike.id=('c4000000-0000-0000-0000-'||lpad((((n-1)/6)+1)::text,12,'0'))::uuid
	`, owner.id, f.now, f.now.Add(10*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	clubID := testClub(t, f, owner)["id"].(string)
	otherClubID := testClub(t, f, owner)["id"].(string)
	separateClubID := testClub(t, f, other)["id"].(string)
	bikeID := testBike(t, f, owner)["id"].(string)
	data := photoPNG(t)
	first := clubImageUpload(t, f, clubID, "1", data, owner, true)
	expect(t, first, 201)
	oldImage := first.body["image"].(map[string]any)
	clubImageFailure(t, photoUpload(t, f, bikeID, data, owner, true), 409, "photo_storage_limit")
	clubImageFailure(t, clubImageUpload(t, f, otherClubID, "1", data, owner, true), 409, "photo_storage_limit")
	clubImageFailure(t, clubImageUpload(t, f, clubID, "2", data, owner, true), 409, "photo_storage_limit")
	photoRead(t, f, oldImage["url"].(string), nil, "GET", 200)
	expect(t, clubImageUpload(t, f, separateClubID, "1", data, other, true), 201)
	var total int64
	err = f.db.QueryRow(ctx, `SELECT
	 COALESCE((SELECT sum(reserved_bytes) FROM bike_photos WHERE owner_id=$1),0)+
	 COALESCE((SELECT sum(reserved_bytes) FROM club_images WHERE owner_id=$1),0)`, owner.id).Scan(&total)
	if err != nil || total <= 96<<20 || total > 100<<20 {
		t.Fatalf("combined owner payload accounting=%d err=%v", total, err)
	}
	f.now = f.now.Add(11 * time.Minute)
	replaced := clubImageUpload(t, f, clubID, "2", data, owner, true)
	expect(t, replaced, 201)
	photoRead(t, f, oldImage["url"].(string), nil, "GET", 404)
	expect(t, photoUpload(t, f, bikeID, data, owner, true), 201)
	var pending int
	if err := f.db.QueryRow(ctx, "SELECT count(*) FROM bike_photos WHERE state='pending'").Scan(&pending); err != nil || pending != 0 {
		t.Fatalf("club upload did not expire pending bike reservations: %d %v", pending, err)
	}
}

func TestSharedBikeClubGlobalQuotaAndPendingExpiryWorkInBothDirections(t *testing.T) {
	f := newFixture(t)
	clubOwner := f.login("+77011234931", "Global club uploader")
	bikeOwner := f.login("+77011234932", "Global bike uploader")
	clubID := testClub(t, f, clubOwner)["id"].(string)
	bikeID := testBike(t, f, bikeOwner)["id"].(string)
	ctx := context.Background()
	// Fill the global budget with pending bike reservations; every synthetic
	// owner and bike stays below its own limit, isolating the global constraint.
	_, err := f.db.Exec(ctx, `
	 WITH seeded_owners AS (
	  INSERT INTO users(id,phone,status,display_name,city,languages,locale,onboarding_complete,created_at)
	  SELECT ('c1000000-0000-0000-0000-'||lpad(n::text,12,'0'))::uuid,
	   '+770198'||lpad(n::text,5,'0'),'active','Shared quota owner '||n,'Almaty',ARRAY['en'],'en',true,$1
	  FROM generate_series(1,11) AS source(n)
	  RETURNING id
	 ), seeded_bikes AS (
	  INSERT INTO bikes(id,owner_id,make,model,year,visibility,created_at,updated_at)
	  SELECT ('c2000000-0000-0000-0000-'||lpad(n::text,12,'0'))::uuid,
	   owner.id,'Fixture','Global quota bike '||n,2020,'private',$1,$1
	  FROM generate_series(1,44) AS source(n)
	  JOIN seeded_owners AS owner ON owner.id=('c1000000-0000-0000-0000-'||lpad((((n-1)/4)+1)::text,12,'0'))::uuid
	  RETURNING id,owner_id
	 )
	 INSERT INTO bike_photos(id,bike_id,owner_id,state,reserved_bytes,created_at,expires_at)
	 SELECT ('c3000000-0000-0000-0000-'||lpad(n::text,12,'0'))::uuid,
	  bike.id,bike.owner_id,'pending',4194304,$1,$2
	 FROM generate_series(1,256) AS source(n)
	 JOIN seeded_bikes AS bike ON bike.id=('c2000000-0000-0000-0000-'||lpad((((n-1)/6)+1)::text,12,'0'))::uuid
	`, f.now, f.now.Add(10*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	data := photoPNG(t)
	clubImageFailure(t, clubImageUpload(t, f, clubID, "1", data, clubOwner, true), 409, "photo_storage_limit")
	f.now = f.now.Add(11 * time.Minute)
	uploaded := clubImageUpload(t, f, clubID, "1", data, clubOwner, true)
	expect(t, uploaded, 201)
	imageURL := uploaded.body["image"].(map[string]any)["url"].(string)
	before := photoRead(t, f, imageURL, nil, "GET", 200)
	var bikePending int
	if err := f.db.QueryRow(ctx, "SELECT count(*) FROM bike_photos WHERE state='pending'").Scan(&bikePending); err != nil || bikePending != 0 {
		t.Fatalf("club reserve failed to expire bike reservations: %d %v", bikePending, err)
	}
	// Reverse the direction: 255 pending club images plus a nonempty ready club
	// image leave less than the 4 MiB needed for a new bike reservation.
	_, err = f.db.Exec(ctx, `
	 WITH seeded_clubs AS (
	  INSERT INTO clubs(id,owner_id,name,city,description,status,created_at,updated_at)
	  SELECT ('c6000000-0000-0000-0000-'||lpad(n::text,12,'0'))::uuid,
	   ('c1000000-0000-0000-0000-'||lpad((((n-1)/24)+1)::text,12,'0'))::uuid,
	   'Quota club '||n,'Almaty','Synthetic pending-image quota fixture','active',$1,$1
	  FROM generate_series(1,255) AS source(n)
	  RETURNING id,owner_id,version,image_version
	 )
	 INSERT INTO club_images(id,club_id,owner_id,state,reserved_bytes,club_version,image_version,created_at,expires_at)
	 SELECT ('c7000000-0000-0000-0000-'||lpad(n::text,12,'0'))::uuid,
	  club.id,club.owner_id,'pending',4194304,club.version,club.image_version,$1,$2
	 FROM generate_series(1,255) AS source(n)
	 JOIN seeded_clubs AS club ON club.id=('c6000000-0000-0000-0000-'||lpad(n::text,12,'0'))::uuid
	`, f.now, f.now.Add(10*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	clubImageFailure(t, photoUpload(t, f, bikeID, data, bikeOwner, true), 409, "photo_storage_limit")
	var total int64
	err = f.db.QueryRow(ctx, `SELECT COALESCE((SELECT sum(reserved_bytes) FROM bike_photos),0)+COALESCE((SELECT sum(reserved_bytes) FROM club_images),0)`).Scan(&total)
	if err != nil || total <= 255*(4<<20) || total > 1<<30 {
		t.Fatalf("combined global budget escaped bounds: %d %v", total, err)
	}
	f.now = f.now.Add(11 * time.Minute)
	expect(t, photoUpload(t, f, bikeID, data, bikeOwner, true), 201)
	var clubPending int
	err = f.db.QueryRow(ctx, "SELECT count(*) FROM club_images WHERE state='pending'").Scan(&clubPending)
	if err != nil || clubPending != 0 {
		t.Fatalf("bike reserve failed to expire club reservations: %d %v", clubPending, err)
	}
	if after := photoRead(t, f, imageURL, nil, "GET", 200); !bytes.Equal(before, after) {
		t.Fatal("cross-feature expiry removed or changed ready club content")
	}
}
