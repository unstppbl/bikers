-- Exact meeting points and routes share the existing private full-ride view.
-- Existing text-only plans retain NULL and require no external map service.
ALTER TABLE rides ADD COLUMN map_plan jsonb;
ALTER TABLE rides ADD CONSTRAINT rides_map_plan_shape CHECK (
    map_plan IS NULL OR COALESCE((
        jsonb_typeof(map_plan) = 'object'
        AND octet_length(map_plan::text) <= 4194304
        AND jsonb_typeof(map_plan->'points') = 'array'
        AND jsonb_array_length(map_plan->'points') BETWEEN 1 AND 10
        AND jsonb_typeof(map_plan->'distance_meters') = 'number'
        AND jsonb_typeof(map_plan->'duration_seconds') = 'number'
    ), false)
);
