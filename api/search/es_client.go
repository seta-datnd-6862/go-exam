package search

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	es8 "github.com/elastic/go-elasticsearch/v8"
)

type ES struct {
	Client *es8.Client
	Index  string
}

func New(esURL, index string) (*ES, error) {
	es, err := es8.NewClient(es8.Config{Addresses: []string{esURL}, Transport: &http.Transport{}})
	if err != nil {
		return nil, err
	}
	return &ES{Client: es, Index: index}, nil
}

// Tạo index với mapping đơn giản; nếu đã tồn tại thì bỏ qua lỗi 400
func (e *ES) EnsureIndex(ctx context.Context) error {
	mapping := `{
	  "mappings": {
	    "properties": {
	      "title":   {"type":"text"},
	      "content": {"type":"text"},
	      "tags":    {"type":"keyword"}
	    }
	  }
	}`
	res, err := e.Client.Indices.Create(e.Index, e.Client.Indices.Create.WithBody(bytes.NewBufferString(mapping)))
	if err != nil {
		return err
	}
	defer res.Body.Close()
	return nil
}

func (e *ES) IndexDoc(ctx context.Context, id int, doc any) error {
	b, _ := json.Marshal(doc)
	res, err := e.Client.Index(e.Index, bytes.NewReader(b), e.Client.Index.WithDocumentID(fmt.Sprintf("%d", id)))
	if err != nil {
		return err
	}
	io.Copy(io.Discard, res.Body)
	res.Body.Close()
	return nil
}

func (e *ES) SearchMultiMatch(ctx context.Context, q string) (map[string]any, error) {
	body := map[string]any{
		"query": map[string]any{
			"multi_match": map[string]any{
				"query":  q,
				"fields": []string{"title", "content"},
			},
		},
	}
	b, _ := json.Marshal(body)
	res, err := e.Client.Search(e.Client.Search.WithIndex(e.Index), e.Client.Search.WithBody(bytes.NewReader(b)))
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	var out map[string]any
	json.NewDecoder(res.Body).Decode(&out)
	return out, nil
}

// Bonus: tìm liên quan theo tag, loại trừ chính bài viết đó
func (e *ES) SearchRelatedByTags(ctx context.Context, tags []string, excludeID int, size int) (map[string]any, error) {
	body := map[string]any{
		"size": size,
		"query": map[string]any{
			"bool": map[string]any{
				"must_not": []any{
					map[string]any{"term": map[string]any{"_id": excludeID}},
				},
				"should": []any{
					map[string]any{"terms": map[string]any{"tags": tags}},
				},
				"minimum_should_match": 1,
			},
		},
	}
	b, _ := json.Marshal(body)
	res, err := e.Client.Search(e.Client.Search.WithIndex(e.Index), e.Client.Search.WithBody(bytes.NewReader(b)))
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	var out map[string]any
	json.NewDecoder(res.Body).Decode(&out)
	return out, nil
}
