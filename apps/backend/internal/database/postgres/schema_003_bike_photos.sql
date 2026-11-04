ALTER TABLE bikes ADD COLUMN photo_version bigint NOT NULL DEFAULT 1;
ALTER TABLE bikes ADD COLUMN cover_photo_id uuid;

CREATE TABLE bike_photos (
 id uuid PRIMARY KEY,
 bike_id uuid NOT NULL REFERENCES bikes(id) ON DELETE CASCADE,
 owner_id uuid NOT NULL REFERENCES users(id),
 state text NOT NULL CHECK(state IN ('pending','ready')),
 reserved_bytes bigint NOT NULL CHECK(reserved_bytes>0),
 content bytea,
 width integer,
 height integer,
 size_bytes bigint,
 created_at timestamptz NOT NULL,
 ready_at timestamptz,
 expires_at timestamptz,
 UNIQUE(bike_id,id),
 CHECK (
  (state='pending' AND reserved_bytes=4194304 AND content IS NULL AND width IS NULL AND height IS NULL AND size_bytes IS NULL AND ready_at IS NULL AND expires_at IS NOT NULL)
  OR
  (state='ready' AND content IS NOT NULL AND width IS NOT NULL AND height IS NOT NULL AND size_bytes IS NOT NULL AND width BETWEEN 1 AND 1920 AND height BETWEEN 1 AND 1920 AND size_bytes BETWEEN 1 AND 4194304 AND octet_length(content)=size_bytes AND reserved_bytes=size_bytes AND ready_at IS NOT NULL AND expires_at IS NULL)
 )
);
CREATE INDEX bike_photos_owner_state ON bike_photos(owner_id,state);
CREATE INDEX bike_photos_expiry ON bike_photos(expires_at) WHERE state='pending';
ALTER TABLE bikes ADD CONSTRAINT bikes_cover_photo_scope
 FOREIGN KEY(id,cover_photo_id) REFERENCES bike_photos(bike_id,id)
 DEFERRABLE INITIALLY DEFERRED;

CREATE TABLE photo_upload_attempts (
 owner_id uuid NOT NULL REFERENCES users(id),
 requested_at timestamptz NOT NULL
);
CREATE INDEX photo_upload_attempts_owner_time ON photo_upload_attempts(owner_id,requested_at);
