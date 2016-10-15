-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied
ALTER TABLE results MODIFY results MEDIUMTEXT;

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
ALTER TABLE results MODIFY results TEXT;
