-- One public cover photo per ride. Rides without a cover keep the city card.
ALTER TABLE rides ADD COLUMN cover_version bigint NOT NULL DEFAULT 1 CHECK(cover_version>0);
ALTER TABLE rides ADD COLUMN current_cover_id uuid;

CREATE TABLE ride_covers (
 id uuid PRIMARY KEY,
 ride_id uuid NOT NULL REFERENCES rides(id) ON DELETE CASCADE,
 owner_id uuid NOT NULL REFERENCES users(id),
 state text NOT NULL CHECK(state IN ('pending','ready')),
 reserved_bytes bigint NOT NULL CHECK(reserved_bytes>0),
 content bytea,
 width integer,
 height integer,
 size_bytes bigint,
 cover_version bigint NOT NULL CHECK(cover_version>0),
 created_at timestamptz NOT NULL,
 ready_at timestamptz,
 expires_at timestamptz,
 UNIQUE(ride_id,id),
 CHECK (
  (state='pending' AND reserved_bytes=4194304 AND content IS NULL AND width IS NULL AND height IS NULL AND size_bytes IS NULL AND ready_at IS NULL AND expires_at IS NOT NULL)
  OR
  (state='ready' AND content IS NOT NULL AND width IS NOT NULL AND height IS NOT NULL AND size_bytes IS NOT NULL AND width BETWEEN 1 AND 1920 AND height BETWEEN 1 AND 1920 AND size_bytes BETWEEN 1 AND 4194304 AND octet_length(content)=size_bytes AND reserved_bytes=size_bytes AND ready_at IS NOT NULL AND expires_at IS NULL)
 )
);
CREATE UNIQUE INDEX ride_covers_one_pending ON ride_covers(ride_id) WHERE state='pending';
CREATE UNIQUE INDEX ride_covers_one_ready ON ride_covers(ride_id) WHERE state='ready';
CREATE INDEX ride_covers_owner_state ON ride_covers(owner_id,state);
CREATE INDEX ride_covers_expiry ON ride_covers(expires_at) WHERE state='pending';
ALTER TABLE rides ADD CONSTRAINT rides_current_cover_scope
 FOREIGN KEY(id,current_cover_id) REFERENCES ride_covers(ride_id,id)
 DEFERRABLE INITIALLY DEFERRED;

ALTER TABLE moderation_hidden DROP CONSTRAINT moderation_hidden_target_type_check;
ALTER TABLE moderation_hidden ADD CONSTRAINT moderation_hidden_target_type_check
    CHECK(target_type IN ('user','ride','message','club','bike','bike_photo','club_image','avatar','ride_photo','ride_video','announcement','route','route_condition','ride_cover'));
ALTER TABLE trust_reports DROP CONSTRAINT trust_reports_target_type_check;
ALTER TABLE trust_reports ADD CONSTRAINT trust_reports_target_type_check
    CHECK(target_type IN ('user','ride','message','club','bike','bike_photo','club_image','avatar','ride_photo','ride_video','announcement','route','route_condition','ride_cover'));
