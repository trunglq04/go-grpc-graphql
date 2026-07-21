package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/elastic/go-elasticsearch/v9"
	"github.com/elastic/go-elasticsearch/v9/typedapi/core/search"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types"
)

const (
	productIndex = "catalog"
	documentType = "products"
)

var ErrNotFound = errors.New("Entity not found")

type Repository interface {
	Close()
	PutProduct(ctx context.Context, p Product) error
	GetProductByID(ctx context.Context, id string) (*Product, error)
	ListProducts(ctx context.Context, skip, take uint64) ([]Product, error)
	ListProductsWithIDs(ctx context.Context, ids []string) ([]Product, error)
	searchProducts(ctx context.Context, query string, skip, take uint64) ([]Product, error)
}

type elasticRepository struct {
	client *elasticsearch.TypedClient
}

type productDocument struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Price       float64 `json:"price"`
}

func NewElasticRepository(url string) (Repository, error) {
	client, err := elasticsearch.NewTyped(
		elasticsearch.WithAddresses(url),
	)
	if err != nil {
		return nil, err
	}
	return &elasticRepository{client}, nil
}

func (r *elasticRepository) Close() {
	if r.client != nil {
		_ = r.client.Close(context.Background())
	}
}

func (r *elasticRepository) PutProduct(ctx context.Context, p Product) error {
	doc := productDocument{
		Name:        p.Name,
		Description: p.Description,
		Price:       p.Price,
	}

	_, err := r.client.Index(productIndex).
		Id(p.ID).
		Request(doc).
		Do(ctx)
	if err != nil {
		return err
	}

	return nil
}

func (r *elasticRepository) GetProductByID(ctx context.Context, id string) (*Product, error) {
	res, err := r.client.Get(productIndex, id).Do(ctx)
	if err != nil {
		return nil, err
	}

	if !res.Found {
		return nil, ErrNotFound
	}

	var source productDocument
	if err := json.Unmarshal(res.Source_, &source); err != nil {
		return nil, err
	}

	return &Product{
		ID:          id,
		Name:        source.Name,
		Description: source.Description,
		Price:       source.Price,
	}, nil
}

func (r *elasticRepository) ListProducts(ctx context.Context, skip, take uint64) ([]Product, error) {
	res, err := r.client.Search().
		Index(productIndex).
		From(int(skip)).
		Size(int(take)).
		Request(&search.Request{
			Query: &types.Query{
				MatchAll: &types.MatchAllQuery{},
			},
		}).
		Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("list products: %w", err)
	}
	if len(res.Hits.Hits) == 0 {
		return nil, ErrNotFound
	}

	products := make([]Product, 0, len(res.Hits.Hits))
	for _, hit := range res.Hits.Hits {
		if hit.Id_ == nil {
			return nil, errors.New("search result missing _id")
		}
		var doc productDocument
		if err := json.Unmarshal(hit.Source_, &doc); err == nil {
			products = append(products, Product{
				ID:          *hit.Id_,
				Name:        doc.Name,
				Description: doc.Description,
				Price:       doc.Price,
			})
		} else {
			return nil, fmt.Errorf("decode product %q: %w", *hit.Id_, err)
		}

	}
	return products, nil
}

func (r *elasticRepository) ListProductsWithIDs(ctx context.Context, ids []string) ([]Product, error) {
	if len(ids) == 0 {
		return []Product{}, fmt.Errorf("no ids input")
	}

	res, err := r.client.
		Search().
		Index(productIndex).
		Size(len(ids)).
		Request(&search.Request{
			Query: &types.Query{
				Ids: &types.IdsQuery{
					Values: ids,
				},
			},
		}).
		Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("list products by IDs: %w", err)
	}
	if len(res.Hits.Hits) == 0 {
		return nil, ErrNotFound
	}

	products := make([]Product, 0, len(res.Hits.Hits))
	for _, hit := range res.Hits.Hits {
		if hit.Id_ == nil {
			return nil, errors.New("search result missing _id")
		}
		var doc productDocument
		if err := json.Unmarshal(hit.Source_, &doc); err == nil {
			products = append(products, Product{
				ID:          *hit.Id_,
				Name:        doc.Name,
				Description: doc.Description,
				Price:       doc.Price,
			})
		} else {
			return nil, fmt.Errorf("decode product %q: %w", *hit.Id_, err)
		}
	}
	return products, nil
}

func (r *elasticRepository) searchProducts(ctx context.Context, query string, skip uint64, take uint64) ([]Product, error) {
	res, err := r.client.Search().
		Index(productIndex).
		From(int(skip)).
		Size(int(take)).
		Request(&search.Request{
			Query: &types.Query{
				MultiMatch: &types.MultiMatchQuery{
					Query:  query,
					Fields: []string{"name^2", "description"},
				},
			},
		}).
		Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("search products: %w", err)
	}
	if len(res.Hits.Hits) == 0 {
		return nil, ErrNotFound
	}

	products := make([]Product, 0, len(res.Hits.Hits))
	for _, hit := range res.Hits.Hits {
		if hit.Id_ == nil {
			return nil, errors.New("search result missing _id")
		}
		var doc productDocument
		if err = json.Unmarshal(hit.Source_, &doc); err == nil {
			products = append(products, Product{
				ID:          *hit.Id_,
				Name:        doc.Name,
				Description: doc.Description,
				Price:       doc.Price,
			})
		} else {
			return nil, fmt.Errorf("decode product %q: %w", *hit.Id_, err)
		}
	}
	return products, nil
}
