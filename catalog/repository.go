package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"github.com/elastic/go-elasticsearch/v9"
	"github.com/elastic/go-elasticsearch/v9/typedapi/core/mget"
	"github.com/elastic/go-elasticsearch/v9/typedapi/core/search"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types"
)

var (
	productIndex = "catalog"
	ErrNotFound  = errors.New("Entity not found")
)

type Repository interface {
	Close()
	PutProduct(ctx context.Context, p Product) error
	GetProductByID(ctx context.Context, id string) (*Product, error)
	ListProducts(ctx context.Context, skip, take uint64) ([]Product, error)
	ListProductsWithIDs(ctx context.Context, ids []string) ([]Product, error)
	SearchProducts(ctx context.Context, query string, skip, take uint64) ([]Product, error)
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

	_, err := r.client.
		Index(productIndex).
		Id(p.ID).
		Request(doc).
		Do(ctx)
	if err != nil {
		return err
	}

	return nil
}

func (r *elasticRepository) GetProductByID(ctx context.Context, id string) (*Product, error) {
	res, err := r.client.
		Get(productIndex, id).
		Do(ctx)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	if !res.Found {
		log.Println(err)
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
		log.Println(err)
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

	mGetDocs := make([]types.MgetOperation, 0, len(ids))
	for _, id := range ids {
		mGetDocs = append(mGetDocs, types.MgetOperation{
			Index_: &productIndex,
			Id_:    id,
		})
	}
	res, err := r.client.Mget().
		Request(&mget.Request{
			Docs: mGetDocs,
		}).
		Do(ctx)
	if err != nil {
		log.Println(err)
		return nil, fmt.Errorf("list products by IDs: %w", err)
	}

	products := make([]Product, 0, len(res.Docs))
	for _, doc := range res.Docs {
		if gr, ok := doc.(*types.GetResult); ok && gr.Found {
			var p productDocument
			if err := json.Unmarshal(gr.Source_, &p); err == nil {
				products = append(products, Product{
					ID:          gr.Id_,
					Name:        p.Name,
					Description: p.Description,
					Price:       p.Price,
				})
			}
		}
	}
	return products, nil
}

func (r *elasticRepository) SearchProducts(ctx context.Context, query string, skip uint64, take uint64) ([]Product, error) {
	res, err := r.client.Search().
		Index(productIndex).
		Request(&search.Request{
			Query: &types.Query{
				MultiMatch: &types.MultiMatchQuery{
					Query:  query,
					Fields: []string{"name^2", "description"}, // name score 2, description score 1
				},
			},
		}).
		From(int(skip)).
		Size(int(take)).
		Do(ctx)
	if err != nil {
		log.Println(err)
		return nil, fmt.Errorf("search products: %w", err)
	}
	if len(res.Hits.Hits) == 0 {
		log.Println(err)
		return nil, ErrNotFound
	}

	products := make([]Product, 0, res.Hits.Total.Value)
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
