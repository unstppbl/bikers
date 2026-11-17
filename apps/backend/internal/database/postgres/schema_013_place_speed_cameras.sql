ALTER TABLE places DROP CONSTRAINT places_category_check;
ALTER TABLE places ADD CONSTRAINT places_category_check
    CHECK(category IN ('fuel','workshop','tires','motorcycle_shop','parking','meeting_spot','speed_camera'));
