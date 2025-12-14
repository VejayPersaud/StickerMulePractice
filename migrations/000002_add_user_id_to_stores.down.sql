-- Remove the foreign key constraint
ALTER TABLE stores DROP CONSTRAINT IF EXISTS fk_stores_user_id;

-- Remove the index
DROP INDEX IF EXISTS idx_stores_user_id;

-- Remove the column
ALTER TABLE stores DROP COLUMN IF EXISTS user_id;
