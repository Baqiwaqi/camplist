package testsupport

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/data/azcosmos"
	"regexp"
	"strings"
	"sync"
)

// Documents models the external database's atomic conditional replacement.
// Tests observe behavior only through the packing Store.
type Documents struct {
	mu       sync.Mutex
	docs     map[string][]byte
	versions map[string]int
}

func NewDocuments() *Documents {
	return &Documents{docs: map[string][]byte{}, versions: map[string]int{}}
}
func (m *Documents) ReadItem(_ context.Context, pk azcosmos.PartitionKey, id string, _ *azcosmos.ItemOptions) (azcosmos.ItemResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.docs[fmt.Sprint(pk)+id]
	if !ok {
		return azcosmos.ItemResponse{}, &azcore.ResponseError{StatusCode: 404}
	}
	return azcosmos.ItemResponse{Value: append([]byte(nil), b...), Response: azcosmos.Response{ETag: azcore.ETag(fmt.Sprint(m.versions[fmt.Sprint(pk)+id]))}}, nil
}
func (m *Documents) CreateItem(_ context.Context, pk azcosmos.PartitionKey, b []byte, _ *azcosmos.ItemOptions) (azcosmos.ItemResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var doc struct {
		ID string `json:"id"`
	}
	json.Unmarshal(b, &doc)
	if _, ok := m.docs[fmt.Sprint(pk)+doc.ID]; ok {
		return azcosmos.ItemResponse{}, &azcore.ResponseError{StatusCode: 409}
	}
	m.docs[fmt.Sprint(pk)+doc.ID] = append([]byte(nil), b...)
	m.versions[fmt.Sprint(pk)+doc.ID] = 1
	return azcosmos.ItemResponse{Value: b}, nil
}
func (m *Documents) ReplaceItem(_ context.Context, pk azcosmos.PartitionKey, id string, b []byte, opts *azcosmos.ItemOptions) (azcosmos.ItemResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.docs[fmt.Sprint(pk)+id]; !ok {
		return azcosmos.ItemResponse{}, &azcore.ResponseError{StatusCode: 404}
	}
	if opts == nil || opts.IfMatchEtag == nil || string(*opts.IfMatchEtag) != fmt.Sprint(m.versions[fmt.Sprint(pk)+id]) {
		return azcosmos.ItemResponse{}, &azcore.ResponseError{StatusCode: 412}
	}
	m.docs[fmt.Sprint(pk)+id] = append([]byte(nil), b...)
	m.versions[fmt.Sprint(pk)+id]++
	return azcosmos.ItemResponse{Value: b}, nil
}

// NewQueryItemsPager evaluates the query predicates used by the public overviews.
// It deliberately returns mixed types if the caller omits its type predicate.
func (m *Documents) NewQueryItemsPager(query string, pk azcosmos.PartitionKey, options *azcosmos.QueryOptions) *runtime.Pager[azcosmos.QueryItemsResponse] {
	return runtime.NewPager(runtime.PagingHandler[azcosmos.QueryItemsResponse]{
		More: func(azcosmos.QueryItemsResponse) bool { return false },
		Fetcher: func(context.Context, *azcosmos.QueryItemsResponse) (azcosmos.QueryItemsResponse, error) {
			m.mu.Lock()
			defer m.mu.Unlock()
			response := azcosmos.QueryItemsResponse{}
			typeMatch := regexp.MustCompile(`\b\w+\.type\s*=\s*'([^']*)'`).FindStringSubmatch(query)
			var owner string
			if options != nil {
				for _, param := range options.QueryParameters {
					if param.Name == "@userID" {
						owner, _ = param.Value.(string)
					}
				}
			}
			for _, data := range m.docs {
				var doc map[string]any
				if err := json.Unmarshal(data, &doc); err != nil {
					return response, err
				}
				if owner != "" && doc["userId"] != owner {
					continue
				}
				if len(typeMatch) > 0 && doc["type"] != typeMatch[1] {
					continue
				}
				if strings.Contains(query, "deletedAt") && doc["deletedAt"] != nil {
					continue
				}
				response.Items = append(response.Items, append([]byte(nil), data...))
			}
			return response, nil
		},
	})
}
func (m *Documents) DeleteItem(_ context.Context, pk azcosmos.PartitionKey, id string, _ *azcosmos.ItemOptions) (azcosmos.ItemResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.docs[fmt.Sprint(pk)+id]; !ok {
		return azcosmos.ItemResponse{}, &azcore.ResponseError{StatusCode: 404}
	}
	delete(m.docs, fmt.Sprint(pk)+id)
	delete(m.versions, fmt.Sprint(pk)+id)
	return azcosmos.ItemResponse{}, nil
}
func (m *Documents) PatchItem(context.Context, azcosmos.PartitionKey, string, azcosmos.PatchOperations, *azcosmos.ItemOptions) (azcosmos.ItemResponse, error) {
	return azcosmos.ItemResponse{}, fmt.Errorf("test Documents adapter does not implement Cosmos patches")
}
