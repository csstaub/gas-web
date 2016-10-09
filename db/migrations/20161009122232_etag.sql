-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied
ALTER TABLE results ADD COLUMN etag VARCHAR(255);

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
ALTER TABLE RESULTS DROP COLUMN etag;
