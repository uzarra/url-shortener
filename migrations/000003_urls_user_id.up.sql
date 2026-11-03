ALTER TABLE urls ADD COLUMN user_id uuid;
CREATE INDEX urls_user_id_idx ON urls (user_id);