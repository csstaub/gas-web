-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied
CREATE TABLE locks (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  hash BINARY(32) UNIQUE NOT NULL,
  description VARCHAR(255) NOT NULL,
  holder VARCHAR(255) NOT NULL,
  timestamp BIGINT NOT NULL,
  lifetime BIGINT NOT NULL
) ENGINE=InnoDB, CHARACTER SET=utf8, COLLATE=utf8_unicode_ci;

CREATE TABLE results (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  hash BINARY(32) UNIQUE NOT NULL,
  timestamp BIGINT NOT NULL,
  results TEXT NOT NULL
) ENGINE=InnoDB, CHARACTER SET=utf8, COLLATE=utf8_unicode_ci;

-- +goose Down
-- SQL section 'Down' is executed when this migration is rolled back
DROP TABLE IF EXISTS locks;
DROP TABLE IF EXISTS results;
