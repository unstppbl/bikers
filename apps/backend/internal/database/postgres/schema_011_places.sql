CREATE TABLE places (
    id uuid PRIMARY KEY,
    name text NOT NULL CHECK(octet_length(name) BETWEEN 1 AND 200),
    category text NOT NULL CHECK(category IN ('fuel','workshop','tires','motorcycle_shop','parking','meeting_spot')),
    lat double precision NOT NULL CHECK(lat BETWEEN 40.5 AND 55.5),
    lon double precision NOT NULL CHECK(lon BETWEEN 46 AND 87.5),
    city text NOT NULL CHECK(octet_length(city)<=160),
    address text NOT NULL CHECK(octet_length(address)<=500),
    description text NOT NULL CHECK(octet_length(description)<=2000),
    website text NOT NULL CHECK(octet_length(website)<=2000),
    phone text NOT NULL CHECK(octet_length(phone)<=100),
    opening_hours text NOT NULL CHECK(octet_length(opening_hours)<=300),
    source text NOT NULL CHECK(source IN ('osm','community')),
    source_ref text UNIQUE,
    source_url text NOT NULL,
    source_date date,
    status text NOT NULL CHECK(status IN ('published','closed')),
    version bigint NOT NULL CHECK(version>0),
    updated_at timestamptz NOT NULL,
    CHECK(source_ref IS NULL OR source_ref ~ '^(node|way|relation)/[1-9][0-9]{0,18}$')
);
CREATE INDEX places_public_category ON places(category,id) WHERE status='published';
CREATE INDEX places_public_city ON places(lower(city),id) WHERE status='published';
CREATE INDEX places_public_coordinates ON places(lon,lat) WHERE status='published';

CREATE TABLE place_submissions (
    id uuid PRIMARY KEY,
    kind text NOT NULL CHECK(kind IN ('new','correction','closed')),
    place_id uuid REFERENCES places(id),
    base_version bigint,
    proposal jsonb,
    reason text NOT NULL CHECK(octet_length(reason) BETWEEN 1 AND 2000),
    submitter_id uuid NOT NULL REFERENCES users(id),
    reviewer_id uuid REFERENCES users(id),
    review_reason text NOT NULL DEFAULT '' CHECK(octet_length(review_reason)<=2000),
    status text NOT NULL CHECK(status IN ('pending','approved','rejected')),
    version bigint NOT NULL CHECK(version>0),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CHECK((kind='new' AND base_version IS NULL AND proposal IS NOT NULL) OR (kind='correction' AND place_id IS NOT NULL AND base_version IS NOT NULL AND base_version>0 AND proposal IS NOT NULL) OR (kind='closed' AND place_id IS NOT NULL AND base_version IS NOT NULL AND base_version>0 AND proposal IS NULL)),
    CHECK((status='pending' AND reviewer_id IS NULL AND review_reason='') OR (status<>'pending' AND reviewer_id IS NOT NULL AND octet_length(review_reason)>0))
);
CREATE INDEX place_submissions_queue ON place_submissions(status,id);
CREATE INDEX place_submissions_own ON place_submissions(submitter_id,id);
CREATE INDEX place_submissions_quota ON place_submissions(submitter_id,created_at);
