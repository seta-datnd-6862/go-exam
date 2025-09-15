package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"highperf-blog-api/models"
)

type PostRepo struct {
	DB *pgxpool.Pool
}

func NewPostRepo(db *pgxpool.Pool) *PostRepo { return &PostRepo{DB: db} }

func (r *PostRepo) CreateWithLogTx(ctx context.Context, req models.CreatePostReq) (*models.Post, error) {
	tx, err := r.DB.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }() // no-op nếu đã Commit

	var id int
	err = tx.QueryRow(ctx,
		`INSERT INTO posts(title, content, tags) VALUES ($1,$2,$3) RETURNING id`,
		req.Title, req.Content, req.Tags,
	).Scan(&id)
	if err != nil {
		return nil, err
	}

	if _, err = tx.Exec(ctx,
		`INSERT INTO activity_logs(action, post_id) VALUES ($1,$2)`,
		"new_post", id,
	); err != nil {
		return nil, err
	}

	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}

	return r.GetByID(ctx, id)
}

func (r *PostRepo) GetByID(ctx context.Context, id int) (*models.Post, error) {
	row := r.DB.QueryRow(ctx, `SELECT id,title,content,tags,created_at FROM posts WHERE id=$1`, id)
	var p models.Post
	if err := row.Scan(&p.ID, &p.Title, &p.Content, &p.Tags, &p.CreatedAt); err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *PostRepo) SearchByTag(ctx context.Context, tag string) ([]models.Post, error) {
	// tags @> ARRAY[tag]::text[] tận dụng GIN index trên cột TEXT[]
	rows, err := r.DB.Query(ctx,
		`SELECT id,title,content,tags,created_at
		 FROM posts
		 WHERE tags @> ARRAY[$1]::text[]
		 ORDER BY id DESC`, tag)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.Post
	for rows.Next() {
		var p models.Post
		if err := rows.Scan(&p.ID, &p.Title, &p.Content, &p.Tags, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *PostRepo) Update(ctx context.Context, id int, req models.UpdatePostReq) (*models.Post, error) {
	set := ""
	args := []any{}
	arg := 1
	if req.Title != nil {
		set += fmt.Sprintf("title=$%d,", arg)
		args = append(args, *req.Title)
		arg++
	}
	if req.Content != nil {
		set += fmt.Sprintf("content=$%d,", arg)
		args = append(args, *req.Content)
		arg++
	}
	if req.Tags != nil {
		set += fmt.Sprintf("tags=$%d,", arg)
		args = append(args, *req.Tags)
		arg++
	}
	if set == "" {
		return nil, errors.New("nothing to update")
	}
	set = set[:len(set)-1]
	args = append(args, id)
	_, err := r.DB.Exec(ctx, fmt.Sprintf(`UPDATE posts SET %s WHERE id=$%d`, set, arg), args...)
	if err != nil {
		return nil, err
	}
	return r.GetByID(ctx, id)
}

