# High-Performance Blog API (Go + PostgreSQL + Redis + Elasticsearch)

## Architecture
- **API**: Go (Gin), pgx, go-redis, go-elasticsearch v8
- **DB**: PostgreSQL 15 (tags TEXT[] + GIN index)
- **Cache**: Redis 7 (cache-aside, TTL 5 minutes)
- **Search**: Elasticsearch 8 (index `posts`, multi_match title+content; related by tags)

## Run
```bash
docker compose up --build
```
Wait for:
- Postgres ready (healthcheck)
- Redis ready
- Elasticsearch ready (single-node)
- API running at `http://localhost:8080`

## Reset (if needed)
```bash
docker compose down -v
docker compose up --build
```

## Environment variables (set in compose)
- `DB_HOST=postgres`, `DB_PORT=5432`, `DB_USER=blog`, `DB_PASSWORD=blogpass`, `DB_NAME=blogdb`
- `REDIS_ADDR=redis:6379`, `CACHE_TTL_SECONDS=300`
- `ES_ADDR=http://elasticsearch:9200`, `ES_INDEX=posts`

## Endpoints & sample cURL

### Health
```bash
curl http://localhost:8080/healthz
```

### DB Health (count posts)
```bash
curl http://localhost:8080/db/health
```

### Create post (transaction: posts + activity_logs)
```bash
curl -X POST http://localhost:8080/posts   -H "Content-Type: application/json"   -d '{"title":"My Post","content":"Hello tx","tags":["intro","golang"]}'
```

### Search by tag (GIN index on TEXT[])
```bash
curl "http://localhost:8080/posts/search-by-tag?tag=intro"
```

### Get by ID (Redis cache-aside: first miss, second hit)
```bash
curl http://localhost:8080/posts/1
curl http://localhost:8080/posts/1
```

### Update (invalidate cache + re-index ES)
```bash
curl -X PUT http://localhost:8080/posts/1   -H "Content-Type: application/json"   -d '{"title":"My Post (updated)","tags":["intro","updated"]}'
```

### Full-text search (ES, multi_match: title + content)
```bash
curl "http://localhost:8080/posts/search?q=updated"
```

### Related posts (bonus: by tags, exclude self)
```bash
curl "http://localhost:8080/posts/1/related"
```

## Technical notes
- **GIN index**: `CREATE INDEX idx_posts_tags_gin ON posts USING GIN(tags);`
  Query: `WHERE tags @> ARRAY[$1]::text[]` uses the GIN index.
- **Transaction**: `POST /posts` writes `posts` and `activity_logs` in same `BEGIN ... COMMIT`.
- **Cache-aside**:
  - GET: lookup `post:<id>` → miss → read DB → SET TTL 300s.
  - PUT: update DB then **DEL** `post:<id>`.
- **Elasticsearch**:
  - Mapping: `title:text`, `content:text`, `tags:keyword`.
  - Index docs on **create/update**.
  - Search: `multi_match` on `title, content`.
  - Related: `bool` should `terms(tags)` + `must_not _id`.

## Debug tips
- Postgres shell:
  ```bash
  docker exec -it highperf-blog-api-postgres-1 psql -U blog -d blogdb
  \dt
  SELECT * FROM activity_logs ORDER BY id DESC LIMIT 5;
  ```
- Redis:
  ```bash
  docker exec -it highperf-blog-api-redis-1 redis-cli
  KEYS post:*
  GET post:1
  TTL post:1
  ```
- Elasticsearch:
  ```bash
  curl localhost:9200/_cat/indices?v
  curl localhost:9200/posts/_search?q=updated
  ```

## Checklist
- [ ] Complete source code (Go) + `docker-compose.yml`
- [ ] Run with `docker compose up --build`
- [ ] All endpoints work: POST / GET :id / PUT / search-by-tag / search / related
- [ ] README with instructions & sample cURL
