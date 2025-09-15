package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"highperf-blog-api/cache"
	"highperf-blog-api/models"
	"highperf-blog-api/repository"
	"highperf-blog-api/search"
)

func mustEnv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func mustEnvInt(k string, def int) int {
	if v := os.Getenv(k); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return def
}

func main() {
	// ---- Load config
	port := mustEnv("APP_PORT", "8080")
	dbHost := mustEnv("DB_HOST", "postgres")
	dbPort := mustEnv("DB_PORT", "5432")
	dbUser := mustEnv("DB_USER", "blog")
	dbPass := mustEnv("DB_PASSWORD", "blogpass")
	dbName := mustEnv("DB_NAME", "blogdb")
	dbSSL := mustEnv("DB_SSLMODE", "disable")

	// ---- DB
	connStr := fmt.Sprintf("postgresql://%s:%s@%s:%s/%s?sslmode=%s",
		dbUser, dbPass, dbHost, dbPort, dbName, dbSSL)

	pool, err := pgxpool.New(context.Background(), connStr)
	if err != nil {
		log.Fatalf("db open error: %v", err)
	}
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("db ping error: %v", err)
	}
	log.Println("DB connected")

	repo := repository.NewPostRepo(pool)

	esURL := mustEnv("ES_ADDR", "http://elasticsearch:9200")
	esIndex := mustEnv("ES_INDEX", "posts")
	es, err := search.New(esURL, esIndex)
	if err != nil {
		log.Fatalf("es init error: %v", err)
	}
	// best-effort tạo index (nếu đã tồn tại, ES trả 400 — có thể bỏ qua)
	_ = es.EnsureIndex(context.Background())

	redisAddr := mustEnv("REDIS_ADDR", "redis:6379")
	redisDB := mustEnvInt("REDIS_DB", 0)
	ttl := mustEnvInt("CACHE_TTL_SECONDS", 300)
	rc := cache.New(redisAddr, redisDB, ttl)

	r := gin.Default()

	// Health
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true, "time": time.Now()})
	})
	r.GET("/db/health", func(c *gin.Context) {
		var cnt int
		if err := pool.QueryRow(c, "SELECT count(*) FROM posts").Scan(&cnt); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"db_ok": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"db_ok": true, "posts_count": cnt})
	})

	// --------- Bước 3 Endpoints ----------
	// 1) POST /posts (transaction: posts + activity_logs)
	r.POST("/posts", func(c *gin.Context) {
		var req models.CreatePostReq
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		post, err := repo.CreateWithLogTx(c, req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		_ = es.IndexDoc(c, post.ID, map[string]any{
			"id":      post.ID,
			"title":   post.Title,
			"content": post.Content,
			"tags":    post.Tags,
		})

		c.JSON(http.StatusCreated, post)
	})

	// 2) GET /posts/search-by-tag?tag=...
	r.GET("/posts/search-by-tag", func(c *gin.Context) {
		tag := c.Query("tag")
		if tag == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "tag is required"})
			return
		}
		posts, err := repo.SearchByTag(c, tag)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, posts)
	})

	// Cache-aside: GET /posts/:id
	r.GET("/posts/:id", func(c *gin.Context) {
		idStr := c.Param("id")
		id, err := strconv.Atoi(idStr)
		if err != nil || id <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
		key := "post:" + idStr

		// 1) Try cache
		if val, err := rc.Get(c, key); err == nil && val != "" {
			var p models.Post
			if jsonErr := json.Unmarshal([]byte(val), &p); jsonErr == nil {
				c.JSON(http.StatusOK, gin.H{"cached": true, "data": p})
				return
			}
		}

		// 2) Miss → DB
		p, err := repo.GetByID(c, id)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}

		// 3) Set cache (best-effort)
		b, _ := json.Marshal(p)
		_ = rc.Set(c, key, string(b))

		// 4) Return
		c.JSON(http.StatusOK, gin.H{"cached": false, "data": p})
	})

	// Update + Invalidate: PUT /posts/:id
	r.PUT("/posts/:id", func(c *gin.Context) {
		idStr := c.Param("id")
		id, err := strconv.Atoi(idStr)
		if err != nil || id <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
		var req models.UpdatePostReq
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		p, err := repo.Update(c, id, req)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Invalidate cache
		_ = rc.Del(c, "post:"+idStr)

		_ = es.IndexDoc(c, p.ID, map[string]any{
			"id":      p.ID,
			"title":   p.Title,
			"content": p.Content,
			"tags":    p.Tags,
		})

		c.JSON(http.StatusOK, p)
	})

	r.GET("/posts/search", func(c *gin.Context) {
		q := c.Query("q")
		if q == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "q is required"})
			return
		}
		res, err := es.SearchMultiMatch(c, q)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, res)
	})

	r.GET("/posts/:id/related", func(c *gin.Context) {
		idStr := c.Param("id")
		id, err := strconv.Atoi(idStr)
		if err != nil || id <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}
		p, err := repo.GetByID(c, id)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		res, err := es.SearchRelatedByTags(c, p.Tags, p.ID, 5)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, res)
	})

	addr := ":" + port
	log.Printf("Listening on %s ...", addr)
	if err := r.Run(addr); err != nil {
		log.Fatal(err)
	}
}
