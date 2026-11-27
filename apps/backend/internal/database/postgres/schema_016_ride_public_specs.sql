-- Public ride-fit attributes (style, surface, approximate distance) for structured discovery cards.
ALTER TABLE rides
  ADD COLUMN style text NOT NULL DEFAULT '' CHECK(style IN ('', 'chill', 'touring', 'sport', 'enduro', 'city')),
  ADD COLUMN surface text NOT NULL DEFAULT '' CHECK(surface IN ('', 'asphalt', 'mixed', 'unpaved')),
  ADD COLUMN distance_km integer NOT NULL DEFAULT 0 CHECK(distance_km >= 0 AND distance_km <= 50000),
  ADD COLUMN public_details_published boolean NOT NULL DEFAULT false;
