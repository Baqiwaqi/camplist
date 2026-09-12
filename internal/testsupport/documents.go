package testsupport

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/data/azcosmos"
	"sync"
)

// Documents models the external database's atomic conditional replacement.
// Tests observe behavior only through the packing Store.
type Documents struct {
	*azcosmos.ContainerClient
	mu       sync.Mutex
	docs     map[string][]byte
	versions map[string]int
}

func NewDocuments() *Documents {
	return &Documents{docs: map[string][]byte{}, versions: map[string]int{}}
}
func (m *Documents) ReadItem(_ context.Context, _ azcosmos.PartitionKey, id string, _ *azcosmos.ItemOptions) (azcosmos.ItemResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.docs[id]
	if !ok {
		return azcosmos.ItemResponse{}, &azcore.ResponseError{StatusCode: 404}
	}
	return azcosmos.ItemResponse{Value: append([]byte(nil), b...), Response: azcosmos.Response{ETag: azcore.ETag(fmt.Sprint(m.versions[id]))}}, nil
}
func (m *Documents) CreateItem(_ context.Context, _ azcosmos.PartitionKey, b []byte, _ *azcosmos.ItemOptions) (azcosmos.ItemResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var doc struct {
		ID string `json:"id"`
	}
	json.Unmarshal(b, &doc)
	if _, ok := m.docs[doc.ID]; ok {
		return azcosmos.ItemResponse{}, &azcore.ResponseError{StatusCode: 409}
	}
	m.docs[doc.ID] = append([]byte(nil), b...)
	m.versions[doc.ID] = 1
	return azcosmos.ItemResponse{Value: b}, nil
}
func (m *Documents) ReplaceItem(_ context.Context, _ azcosmos.PartitionKey, id string, b []byte, opts *azcosmos.ItemOptions) (azcosmos.ItemResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.docs[id]; !ok {
		return azcosmos.ItemResponse{}, &azcore.ResponseError{StatusCode: 404}
	}
	if opts == nil || opts.IfMatchEtag == nil || string(*opts.IfMatchEtag) != fmt.Sprint(m.versions[id]) {
		return azcosmos.ItemResponse{}, &azcore.ResponseError{StatusCode: 412}
	}
	m.docs[id] = append([]byte(nil), b...)
	m.versions[id]++
	return azcosmos.ItemResponse{Value: b}, nil
}
