-- Add user_id column to stores table
ALTER TABLE stores ADD COLUMN user_id INTEGER;

-- Add foreign key constraint
ALTER TABLE stores ADD CONSTRAINT fk_stores_user_id 
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

-- Create index for faster lookups by user
CREATE INDEX idx_stores_user_id ON stores(user_id);

-- For existing stores, assign them to the first user (migration safety)
-- In production, you'd handle this more carefully
UPDATE stores SET user_id = (SELECT MIN(id) FROM users) WHERE user_id IS NULL;

-- Make user_id required going forward
ALTER TABLE stores ALTER COLUMN user_id SET NOT NULL;
