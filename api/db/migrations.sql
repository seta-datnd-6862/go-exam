-- posts
CREATE TABLE IF NOT EXISTS posts (
  id SERIAL PRIMARY KEY,
  title VARCHAR(255) NOT NULL,
  content TEXT NOT NULL,
  tags TEXT[] NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- GIN index for tags
CREATE INDEX IF NOT EXISTS idx_posts_tags_gin ON posts USING GIN (tags);

-- activity_logs
CREATE TABLE IF NOT EXISTS activity_logs (
  id SERIAL PRIMARY KEY,
  action VARCHAR(64) NOT NULL,
  post_id INT NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
  logged_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- seed sample (optional)
INSERT INTO posts(title, content, tags)
SELECT 'Hello', 'First post', ARRAY['intro','hello']
WHERE NOT EXISTS (SELECT 1 FROM posts WHERE title='Hello');
