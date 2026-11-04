ALTER TABLE clubs ADD COLUMN image_version bigint NOT NULL DEFAULT 1;
ALTER TABLE clubs ADD COLUMN current_image_id uuid;

CREATE TABLE club_images (
 id uuid PRIMARY KEY,
 club_id uuid NOT NULL REFERENCES clubs(id) ON DELETE CASCADE,
 owner_id uuid NOT NULL REFERENCES users(id),
 state text NOT NULL CHECK(state IN ('pending','ready')),
 reserved_bytes bigint NOT NULL CHECK(reserved_bytes>0),
 content bytea,
 width integer,
 height integer,
 size_bytes bigint,
 club_version bigint NOT NULL CHECK(club_version>0),
 image_version bigint NOT NULL CHECK(image_version>0),
 created_at timestamptz NOT NULL,
 ready_at timestamptz,
 expires_at timestamptz,
 UNIQUE(club_id,id),
 CHECK (
  (state='pending' AND reserved_bytes=4194304 AND content IS NULL AND width IS NULL AND height IS NULL AND size_bytes IS NULL AND ready_at IS NULL AND expires_at IS NOT NULL)
  OR
  (state='ready' AND content IS NOT NULL AND width IS NOT NULL AND height IS NOT NULL AND size_bytes IS NOT NULL AND width BETWEEN 1 AND 1920 AND height BETWEEN 1 AND 1920 AND size_bytes BETWEEN 1 AND 4194304 AND octet_length(content)=size_bytes AND reserved_bytes=size_bytes AND ready_at IS NOT NULL AND expires_at IS NULL)
 )
);
CREATE UNIQUE INDEX club_images_one_pending ON club_images(club_id) WHERE state='pending';
CREATE UNIQUE INDEX club_images_one_ready ON club_images(club_id) WHERE state='ready';
CREATE INDEX club_images_owner_state ON club_images(owner_id,state);
CREATE INDEX club_images_expiry ON club_images(expires_at) WHERE state='pending';
ALTER TABLE clubs ADD CONSTRAINT clubs_current_image_scope
 FOREIGN KEY(id,current_image_id) REFERENCES club_images(club_id,id)
 DEFERRABLE INITIALLY DEFERRED;
