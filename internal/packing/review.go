package packing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/data/azcosmos"
	"github.com/google/uuid"
)

var ErrConflict = errors.New("the record changed; review the current version")
var ErrInvalid = errors.New("invalid packing input")

type PreparationTask struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Done bool   `json:"done"`
}

type ReviewAction string

const (
	ReviewObserve ReviewAction = "none"
	ReviewAdd     ReviewAction = "add"
	ReviewRemove  ReviewAction = "remove"
	ReviewTask    ReviewAction = "task"
)

type ReviewEntry struct {
	ID             string       `json:"id"`
	ItemID         string       `json:"itemId,omitempty"`
	Name           string       `json:"name"`
	Category       string       `json:"category,omitempty"`
	Forgotten      bool         `json:"forgotten"`
	Unused         bool         `json:"unused"`
	NeedsAttention bool         `json:"needsAttention"`
	Note           string       `json:"note,omitempty"`
	Action         ReviewAction `json:"action"`
	Task           string       `json:"task,omitempty"`
}

func (s *Store) updateSession(ctx context.Context, id, user string, change func(*PackingSession) (bool, error)) (PackingSession, error) {
	for attempt := 0; attempt < 5; attempt++ {
		session, err := s.GetPackingSession(ctx, id, user)
		if err != nil {
			return PackingSession{}, err
		}
		changed, err := change(&session)
		if err != nil {
			return session, err
		}
		if !changed {
			return session, nil
		}
		data, err := json.Marshal(session)
		if err != nil {
			return session, err
		}
		etag := azcore.ETag(session.etag)
		_, err = s.container.ReplaceItem(ctx, azcosmos.NewPartitionKeyString(user), id, data, &azcosmos.ItemOptions{IfMatchEtag: &etag})
		if preconditionFailed(err) {
			continue
		}
		return session, err
	}
	return PackingSession{}, ErrConflict
}
func preconditionFailed(err error) bool {
	var response *azcore.ResponseError
	return errors.As(err, &response) && response.StatusCode == 412
}

func (s *Store) AddReviewEntry(ctx context.Context, id, user string, entry ReviewEntry) (PackingSession, error) {
	entry.Name = strings.TrimSpace(entry.Name)
	entry.Task = strings.TrimSpace(entry.Task)
	if entry.ID == "" || len(entry.ID) > 100 || entry.Name == "" || len(entry.Name) > 200 || len(entry.Note) > 2000 || len(entry.Task) > 200 || len(entry.Category) > 100 || (!entry.Forgotten && !entry.Unused && !entry.NeedsAttention) {
		return PackingSession{}, ErrInvalid
	}
	if !slices.Contains([]ReviewAction{ReviewObserve, ReviewAdd, ReviewRemove, ReviewTask}, entry.Action) || (entry.Action == ReviewTask && entry.Task == "") {
		return PackingSession{}, ErrInvalid
	}
	return s.updateSession(ctx, id, user, func(session *PackingSession) (bool, error) {
		for _, existing := range session.Review {
			if existing.ID == entry.ID {
				if existing != entry {
					return false, ErrConflict
				}
				return false, nil
			}
		}
		if len(session.Review) >= 200 {
			return false, fmt.Errorf("%w: review limit reached", ErrInvalid)
		}
		if entry.ItemID != "" {
			if _, err := getItemIndexById(session.List, entry.ItemID); err != nil {
				return false, ErrInvalid
			}
		}
		if entry.Action == ReviewRemove && entry.ItemID == "" {
			return false, ErrInvalid
		}
		session.Review = append(session.Review, entry)
		return true, nil
	})
}

func (s *Store) ApplyReview(ctx context.Context, sessionID, user, revision string, selected []string) (PackingList, error) {
	session, err := s.GetPackingSession(ctx, sessionID, user)
	if err != nil {
		return PackingList{}, err
	}
	list, err := s.GetPackingList(ctx, session.TemplateID(), user)
	if err != nil {
		return PackingList{}, err
	}
	var pending []ReviewEntry
	for _, id := range selected {
		index := slices.IndexFunc(session.Review, func(e ReviewEntry) bool { return e.ID == id })
		if index < 0 {
			return list, ErrInvalid
		}
		entry := session.Review[index]
		if entry.Action == ReviewObserve {
			return list, ErrInvalid
		}
		key := sessionID + ":" + entry.ID
		if !slices.Contains(list.AppliedReviews, key) && !slices.ContainsFunc(pending, func(e ReviewEntry) bool { return e.ID == id }) {
			pending = append(pending, entry)
		}
	}
	if len(pending) == 0 {
		return list, nil
	}
	if revision == "" || revision != list.Revision() {
		return list, ErrConflict
	}
	for _, entry := range pending {
		key := sessionID + ":" + entry.ID
		switch entry.Action {
		case ReviewAdd:
			item := NewItem(entry.Name, entry.Category)
			item.ID = uuid.NewSHA1(uuid.NameSpaceURL, []byte(key)).String()
			list.Items = append(list.Items, item)
			list.Changes = append(list.Changes, "Added "+entry.Name)
		case ReviewRemove:
			index := slices.IndexFunc(list.Items, func(i PackingItem) bool { return i.ID == entry.ItemID })
			if index < 0 {
				return list, ErrConflict
			}
			list.Items = append(list.Items[:index], list.Items[index+1:]...)
			list.Changes = append(list.Changes, "Removed "+entry.Name)
		case ReviewTask:
			list.Tasks = append(list.Tasks, PreparationTask{ID: uuid.NewSHA1(uuid.NameSpaceURL, []byte(key)).String(), Name: entry.Task})
			list.Changes = append(list.Changes, "Prepare: "+entry.Task)
		}
		list.AppliedReviews = append(list.AppliedReviews, key)
	}
	if err = s.SavePackingList(ctx, list); preconditionFailed(err) {
		return list, ErrConflict
	}
	if err != nil {
		return list, err
	}
	return s.GetPackingList(ctx, list.ID, user)
}

func (s *Store) SetPreparationTask(ctx context.Context, id, user, taskID string, done bool, revision string) (PackingList, error) {
	list, err := s.GetPackingList(ctx, id, user)
	if err != nil {
		return list, err
	}
	index := slices.IndexFunc(list.Tasks, func(task PreparationTask) bool { return task.ID == taskID })
	if index < 0 {
		return list, ErrNotFound
	}
	if revision != list.Revision() {
		return list, ErrConflict
	}
	list.Tasks[index].Done = done
	if err = s.SavePackingList(ctx, list); preconditionFailed(err) {
		return list, ErrConflict
	}
	if err != nil {
		return list, err
	}
	return s.GetPackingList(ctx, id, user)
}

func isMissing(err error) bool {
	if errors.Is(err, ErrNotFound) {
		return true
	}
	var response *azcore.ResponseError
	return errors.As(err, &response) && response.StatusCode == 404
}

func (s *Store) RecoverReviewTemplate(ctx context.Context, sessionID, user string) (PackingList, error) {
	session, err := s.GetPackingSession(ctx, sessionID, user)
	if err != nil {
		return PackingList{}, err
	}
	if session.ReviewTargetID != "" {
		return s.GetPackingList(ctx, session.ReviewTargetID, user)
	}
	if _, err = s.GetPackingList(ctx, session.List.ID, user); err == nil {
		return PackingList{}, ErrConflict
	} else if !isMissing(err) {
		return PackingList{}, err
	}
	// The deterministic identity makes a retry after either write safe.
	recoveredID := uuid.NewSHA1(uuid.NameSpaceURL, []byte("recover:"+user+":"+sessionID)).String()
	list, err := s.GetPackingList(ctx, recoveredID, user)
	if isMissing(err) {
		list = NewList(user, session.List.Name+" (recovered)", session.List.Description)
		list.ID = recoveredID
		list.Items = append([]PackingItem{}, session.List.Items...)
		for i := range list.Items {
			list.Items[i].Checked = false
		}
		list.Tasks = append([]PreparationTask{}, session.List.Tasks...)
		if err = s.SavePackingList(ctx, list); err != nil {
			var response *azcore.ResponseError
			if !errors.As(err, &response) || response.StatusCode != 409 {
				return list, err
			}
		}
		list, err = s.GetPackingList(ctx, recoveredID, user)
	}
	if err != nil {
		return list, err
	}
	_, err = s.updateSession(ctx, sessionID, user, func(ses *PackingSession) (bool, error) { ses.ReviewTargetID = recoveredID; return true, nil })
	return list, err
}
