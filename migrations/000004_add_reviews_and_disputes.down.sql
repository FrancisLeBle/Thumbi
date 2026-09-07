-- 000004_add_reviews_and_disputes.down.sql

DROP TRIGGER IF EXISTS trg_reviews_update_reputation ON reviews;
DROP FUNCTION IF EXISTS fn_update_user_reputation();

DROP TABLE IF EXISTS disputes CASCADE;
DROP TABLE IF EXISTS reviews CASCADE;

DROP INDEX IF EXISTS idx_users_rating_avg;

ALTER TABLE users 
    DROP COLUMN IF EXISTS rating_avg,
    DROP COLUMN IF EXISTS rating_count;

DROP TYPE IF EXISTS dispute_status_enum CASCADE;
