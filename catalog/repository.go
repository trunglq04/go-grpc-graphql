package catalog

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	elasticsearch "github.com/elastic/go-elasticsearch"
)

var (
	IndexName    = "catalog"
	DocumentType = "products"
	ErrNotFound  = errors.New("Entity not found")
)

type Repository interface {
	Close()
	PutProduct(ctx context.Context, p Product) error
	GetProductByID(ctx context.Context, id string) (*Product, error)
	ListProducts(ctx context.Context, skip, take uint64) ([]Product, error)
	ListProductsWithIDs(ctx context.Context, ids []string) ([]Product, error)
	searchProducts(ctx context.Context, query string, skip, take uint64) ([]Product, error)
}

type elasticRepository struct {
	client *elasticsearch.Client
}

type productDocument struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Price       float64 `json:"price"`
}

func NewElasticRepository(url string) (Repository, error) {
	client, err := elasticsearch.NewClient(elasticsearch.Config{
		Addresses: []string{url},
	})
	if err != nil {
		return nil, err
	}
	return &elasticRepository{client}, nil
}

func (r *elasticRepository) Close() {}

func (r *elasticRepository) PutProduct(ctx context.Context, p Product) error {
	doc := productDocument{
		Name:        p.Name,
		Description: p.Description,
		Price:       p.Price,
	}

	body, err := json.Marshal(doc)
	if err != nil {
		return err
	}

	res, err := r.client.Index(
		IndexName,
		bytes.NewReader(body),
		r.client.Index.WithContext(ctx),
		r.client.Index.WithDocumentID(p.ID),
	)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("index request failed: %s", res.String())
	}

	return nil
}

func (r *elasticRepository) GetProductByID(ctx context.Context, id string) (*Product, error) {
	res, err := r.client.Get(
		IndexName,
		id,
		r.client.Get.WithContext(ctx),
	)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("get request failed: %s", res.String())
	}

	var esResponse struct {
		Source productDocument `json:"_source"`
	}
	err = json.NewDecoder(res.Body).Decode(&esResponse)
	if err != nil {
		return nil, err

	}
	return &Product{
		ID:          id,
		Name:        esResponse.Source.Name,
		Description: esResponse.Source.Description,
		Price:       esResponse.Source.Price,
	}, nil
}

func (r *elasticRepository) ListProducts(ctx context.Context, skip, take uint64) ([]Product, error) {
	res, err := r.client.Search(
		r.client.Search.WithContext(ctx),
		r.client.Search.WithIndex(IndexName),
		r.client.Search.WithFrom(int(skip)),
		r.client.Search.WithSize(int(take)),
	)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("list products request failed: %s", res.String())
	}

	var esResponse struct {
		Hits struct {
			Hits []struct {
				ID     string          `json:"_id"`
				Source productDocument `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.NewDecoder(res.Body).Decode(&esResponse); err != nil {
		return nil, err
	}

	products := make([]Product, 0, len(esResponse.Hits.Hits))
	for _, hit := range esResponse.Hits.Hits {
		products = append(products, Product{
			ID:          hit.ID,
			Name:        hit.Source.Name,
			Description: hit.Source.Description,
			Price:       hit.Source.Price,
		})
	}
	return products, nil
}

func (r *elasticRepository) ListProductsWithIDs(ctx context.Context, ids []string) ([]Product, error) {
	payload := map[string]interface{}{
		"ids": ids,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	res, err := r.client.Mget(
		bytes.NewReader(body),
		r.client.Mget.WithIndex(IndexName),
		r.client.Mget.WithContext(ctx),
	)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("mget request failed: %s", res.String())
	}

	var esResponse struct {
		Docs []struct {
			ID     string          `json:"_id"`
			Found  bool            `json:"found"`
			Source productDocument `json:"_source"`
		} `json:"docs"`
	}

	if err := json.NewDecoder(res.Body).Decode(&esResponse); err != nil {
		return nil, err
	}

	products := make([]Product, 0, len(esResponse.Docs))
	for _, doc := range esResponse.Docs {
		if doc.Found {
			products = append(products, Product{
				ID:          doc.ID,
				Name:        doc.Source.Name,
				Description: doc.Source.Description,
				Price:       doc.Source.Price,
			})
		}
	}
	return products, nil
}

func (r *elasticRepository) searchProducts(ctx context.Context, query string, skip uint64, take uint64) ([]Product, error) {
	res, err := r.client.Search(
		r.client.Search.WithContext(ctx),
		r.client.Search.WithQuery(query),
		r.client.Search.WithIndex(IndexName),
		r.client.Search.WithFrom(int(skip)),
		r.client.Search.WithSize(int(take)),
	)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("list products request failed: %s", res.String())
	}

	var esResponse struct {
		Hits struct {
			Hits []struct {
				ID     string          `json:"_id"`
				Source productDocument `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.NewDecoder(res.Body).Decode(&esResponse); err != nil {
		return nil, err
	}

	products := make([]Product, 0, len(esResponse.Hits.Hits))
	for _, hit := range esResponse.Hits.Hits {
		products = append(products, Product{
			ID:          hit.ID,
			Name:        hit.Source.Name,
			Description: hit.Source.Description,
			Price:       hit.Source.Price,
		})
	}
	return products, nil
}
