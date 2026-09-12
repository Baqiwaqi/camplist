package packing

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/data/azcosmos"
)

func TestOverviewQueriesSeparateDocumentTypes(t *testing.T) {
	ctx := context.Background()
	s, list, session := seedTrip(t)
	other := NewList("other", "Private", "")
	if err := s.SavePackingList(ctx, other); err != nil {
		t.Fatal(err)
	}
	deleted := NewList("camper", "Deleted", "")
	deleted.DeletedAt = &deleted.CreatedAt
	if err := s.SavePackingList(ctx, deleted); err != nil {
		t.Fatal(err)
	}
	lists, err := s.GetPackingLists(ctx, "camper")
	if err != nil || len(lists) != 1 || lists[0].ID != list.ID {
		t.Fatalf("visible lists: %+v, %v", lists, err)
	}
	sessions, err := s.ListPackingSession(ctx, "camper")
	if err != nil || len(sessions) != 1 || sessions[0].ID != session.ID {
		t.Fatalf("visible sessions: %+v, %v", sessions, err)
	}
}

type readContainer struct {
	containerClient
	document any
}

func (c *readContainer) ReadItem(context.Context, azcosmos.PartitionKey, string, *azcosmos.ItemOptions) (azcosmos.ItemResponse, error) {
	data, err := json.Marshal(c.document)
	return azcosmos.ItemResponse{Value: data}, err
}
func TestPointReadsRejectWrongDocumentTypeAndDeletedLists(t *testing.T) {
	ctx := context.Background()
	list := NewList("user", "Camping", "")
	c := &readContainer{document: list}
	s := &Store{container: c}
	if _, err := s.GetPackingSession(ctx, list.ID, "user"); err == nil {
		t.Error("list accepted as session")
	}
	c.document = NewPackingSession(list)
	if _, err := s.GetPackingList(ctx, "session", "user"); err == nil {
		t.Error("session accepted as list")
	}
	list.DeletedAt = &list.CreatedAt
	c.document = list
	if _, err := s.GetPackingList(ctx, list.ID, "user"); err == nil {
		t.Error("deleted list accepted")
	}
}

func TestSettingPackedStateCanBeRetried(t *testing.T) {
	s, list, session := seedTrip(t)
	for _, checked := range []bool{true, true, false, false} {
		got, err := s.SetSessionItem(context.Background(), session.ID, "camper", list.Items[0].ID, checked)
		if err != nil {
			t.Fatal(err)
		}
		if got.List.Items[0].Checked != checked {
			t.Fatal("saved state differs from requested state")
		}
	}
	unchanged, err := s.GetPackingList(context.Background(), list.ID, "camper")
	if err != nil {
		t.Fatal(err)
	}
	if unchanged.Items[0].Checked {
		t.Fatal("packing changed the reusable template")
	}
}

type concurrentContainer struct {
	readContainer
	expected azcore.ETag
	writes   int
}

func (c *concurrentContainer) ReadItem(ctx context.Context, pk azcosmos.PartitionKey, id string, options *azcosmos.ItemOptions) (azcosmos.ItemResponse, error) {
	res, err := c.readContainer.ReadItem(ctx, pk, id, options)
	res.ETag = `"version-1"`
	return res, err
}
func (c *concurrentContainer) ReplaceItem(_ context.Context, _ azcosmos.PartitionKey, _ string, _ []byte, options *azcosmos.ItemOptions) (azcosmos.ItemResponse, error) {
	if options == nil || options.IfMatchEtag == nil || *options.IfMatchEtag != c.expected {
		return azcosmos.ItemResponse{}, &azcore.ResponseError{StatusCode: 412}
	}
	c.writes++
	return azcosmos.ItemResponse{}, nil
}
func TestStaleListEditsDoNotOverwriteConcurrentChanges(t *testing.T) {
	list := NewList("user", "Camping", "")
	c := &concurrentContainer{readContainer: readContainer{document: list}, expected: `"version-2"`}
	s := &Store{container: c}
	loaded, err := s.GetPackingList(context.Background(), list.ID, "user")
	if err != nil {
		t.Fatal(err)
	}
	loaded.Name = "New name"
	err = s.SavePackingList(context.Background(), loaded)
	var response *azcore.ResponseError
	if !errors.As(err, &response) || response.StatusCode != 412 {
		t.Fatalf("expected stale write rejection, got %v", err)
	}
	if c.writes != 0 {
		t.Error("overwrote concurrent change")
	}
	c.expected = `"version-1"`
	if err = s.SavePackingList(context.Background(), loaded); err != nil {
		t.Fatal(err)
	}
	if c.writes != 1 {
		t.Error("matching version was not saved")
	}
}
