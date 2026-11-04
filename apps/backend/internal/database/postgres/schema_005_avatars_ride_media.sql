ALTER TABLE users ADD COLUMN avatar_version bigint NOT NULL DEFAULT 1;
ALTER TABLE users ADD COLUMN current_avatar_id uuid;
ALTER TABLE rides ADD COLUMN album_version bigint NOT NULL DEFAULT 1;
ALTER TABLE participation ADD COLUMN media_version bigint NOT NULL DEFAULT 1;

CREATE TABLE avatars (
 id uuid PRIMARY KEY,
 owner_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 state text NOT NULL CHECK(state IN ('pending','ready')),
 reserved_bytes bigint NOT NULL CHECK(reserved_bytes>0),
 content bytea,
 width integer,
 height integer,
 size_bytes bigint,
 avatar_version bigint NOT NULL CHECK(avatar_version>0),
 created_at timestamptz NOT NULL,
 ready_at timestamptz,
 expires_at timestamptz,
 UNIQUE(owner_id,id),
 CHECK (
  (state='pending' AND reserved_bytes=4194304 AND content IS NULL AND width IS NULL AND height IS NULL AND size_bytes IS NULL AND ready_at IS NULL AND expires_at IS NOT NULL)
  OR
  (state='ready' AND content IS NOT NULL AND width IS NOT NULL AND height IS NOT NULL AND size_bytes IS NOT NULL AND width BETWEEN 1 AND 1920 AND height BETWEEN 1 AND 1920 AND size_bytes BETWEEN 1 AND 4194304 AND octet_length(content)=size_bytes AND reserved_bytes=size_bytes AND ready_at IS NOT NULL AND expires_at IS NULL)
 )
);
CREATE UNIQUE INDEX avatars_one_pending ON avatars(owner_id) WHERE state='pending';
CREATE UNIQUE INDEX avatars_one_ready ON avatars(owner_id) WHERE state='ready';
CREATE INDEX avatars_expiry ON avatars(expires_at) WHERE state='pending';
ALTER TABLE users ADD CONSTRAINT users_current_avatar_scope
 FOREIGN KEY(id,current_avatar_id) REFERENCES avatars(owner_id,id)
 DEFERRABLE INITIALLY DEFERRED;

CREATE TABLE ride_photos (
 id uuid PRIMARY KEY,
 ride_id uuid NOT NULL REFERENCES rides(id) ON DELETE CASCADE,
 owner_id uuid NOT NULL REFERENCES users(id),
 state text NOT NULL CHECK(state IN ('pending','ready')),
 reserved_bytes bigint NOT NULL CHECK(reserved_bytes>0),
 content bytea,
 width integer,
 height integer,
 size_bytes bigint,
 participation_version bigint NOT NULL CHECK(participation_version>0),
 ride_plan_version bigint NOT NULL CHECK(ride_plan_version>0),
 created_at timestamptz NOT NULL,
 ready_at timestamptz,
 expires_at timestamptz,
 CHECK (
  (state='pending' AND reserved_bytes=4194304 AND content IS NULL AND width IS NULL AND height IS NULL AND size_bytes IS NULL AND ready_at IS NULL AND expires_at IS NOT NULL)
  OR
  (state='ready' AND content IS NOT NULL AND width IS NOT NULL AND height IS NOT NULL AND size_bytes IS NOT NULL AND width BETWEEN 1 AND 1920 AND height BETWEEN 1 AND 1920 AND size_bytes BETWEEN 1 AND 4194304 AND octet_length(content)=size_bytes AND reserved_bytes=size_bytes AND ready_at IS NOT NULL AND expires_at IS NULL)
 )
);
CREATE INDEX ride_photos_ride ON ride_photos(ride_id,created_at,id);
CREATE INDEX ride_photos_owner_state ON ride_photos(owner_id,state);
CREATE INDEX ride_photos_expiry ON ride_photos(expires_at) WHERE state='pending';

CREATE TABLE ride_video_links (
 id uuid PRIMARY KEY,
 ride_id uuid NOT NULL REFERENCES rides(id) ON DELETE CASCADE,
 owner_id uuid NOT NULL REFERENCES users(id),
 url text NOT NULL,
 title text NOT NULL,
 created_at timestamptz NOT NULL
);
CREATE INDEX ride_video_links_ride ON ride_video_links(ride_id,created_at,id);
CREATE INDEX ride_video_links_owner ON ride_video_links(owner_id,ride_id);
