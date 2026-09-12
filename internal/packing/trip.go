package packing

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"strings"
)

// expandPersonalEntries runs in the same conditional write as membership approval.
// Sources come from this trip, preserving its snapshot when the template changes.
func (s *PackingSession) expandPersonalEntries() {
	people := map[string]string{s.UserID: "Trip owner"}
	for id, member := range s.Sharing.Members {
		people[id] = member.Name
	}
	items := append([]PackingItem{}, s.List.Items...)
	for i := range s.List.Items {
		item := &s.List.Items[i]
		if item.Scope == "person" && item.Assignee == "" {
			item.Assignee = s.UserID
			item.AssigneeName = "Trip owner"
			item.SourceID = item.ID
		}
	}
	for _, source := range items {
		if source.Scope != "person" {
			continue
		}
		sourceID := source.SourceID
		if sourceID == "" {
			sourceID = source.ID
		}
		for person, name := range people {
			found := false
			for _, item := range s.List.Items {
				if item.SourceID == sourceID && item.Assignee == person {
					found = true
					break
				}
			}
			if found {
				continue
			}
			copy := source
			copy.ID = uuid.NewString()
			copy.SourceID = sourceID
			copy.Assignee = person
			copy.AssigneeName = name
			copy.Checked = false
			copy.Revision = 0
			copy.ChangedBy = ""
			s.List.Items = append(s.List.Items, copy)
		}
	}
	tasks := append([]PreparationTask{}, s.List.Tasks...)
	for i := range s.List.Tasks {
		task := &s.List.Tasks[i]
		if task.Scope == "person" && task.Assignee == "" {
			task.Assignee = s.UserID
			task.AssigneeName = "Trip owner"
			task.SourceID = task.ID
		}
	}
	for _, source := range tasks {
		if source.Scope != "person" {
			continue
		}
		sourceID := source.SourceID
		if sourceID == "" {
			sourceID = source.ID
		}
		for person, name := range people {
			found := false
			for _, task := range s.List.Tasks {
				if task.SourceID == sourceID && task.Assignee == person {
					found = true
					break
				}
			}
			if found {
				continue
			}
			copy := source
			copy.ID = uuid.NewString()
			copy.SourceID = sourceID
			copy.Assignee = person
			copy.AssigneeName = name
			copy.Done = false
			copy.Revision = 0
			copy.ChangedBy = ""
			s.List.Tasks = append(s.List.Tasks, copy)
		}
	}
}

func (s PackingSession) DisplayName() string {
	if s.Name != "" {
		return s.Name
	}
	return s.List.Name
}

func validScope(scope string, personal bool) bool {
	return scope == "" || scope == "shared" || scope == "person" || personal && scope == "mine"
}
func (s PackingSession) participantName(actor string) string {
	if actor == s.UserID {
		return "Trip owner"
	}
	if m, ok := s.Sharing.Members[actor]; ok && m.Name != "" {
		return m.Name
	}
	return "Another camper"
}
func (s *PackingSession) recordOperation(key string, op PackingOperation) {
	if s.Operations == nil {
		s.Operations = map[string]PackingOperation{}
	}
	s.Operations[key] = op
}

// A future-list save is separately authorized and idempotent across a lost response.
// It is intentionally retried after the trip receipt, since Cosmos cannot atomically
// commit across independent owners' partitions.
func (s *Store) rememberTripEntry(ctx context.Context, trip PackingSession, actor string, op PackingOperation) error {
	for attempt := 0; attempt < 5; attempt++ {
		list, err := s.GetPackingList(ctx, trip.TemplateID(), actor)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrFutureSave, err)
		}
		id := uuid.NewSHA1(uuid.NameSpaceURL, []byte(trip.ID+"/"+op.ItemID)).String()
		scope := op.Scope
		if scope == "mine" {
			scope = "person"
		}
		if op.Kind == "task" {
			for _, task := range list.Tasks {
				if task.ID == id {
					return nil
				}
			}
			if len(list.Tasks) >= 200 {
				return ErrFutureSave
			}
			list.Tasks = append(list.Tasks, PreparationTask{ID: id, Name: op.Name, Scope: scope})
		} else {
			for _, item := range list.Items {
				if item.ID == id {
					return nil
				}
			}
			if len(list.Items) >= 2000 {
				return ErrFutureSave
			}
			item := NewItem(op.Name, op.Category)
			item.ID = id
			item.Scope = scope
			list.Items = append(list.Items, item)
		}
		if err = s.SavePackingList(ctx, list); err == ErrConflict {
			continue
		}
		if err != nil {
			return fmt.Errorf("%w: %v", ErrFutureSave, err)
		}
		return nil
	}
	return ErrFutureSave
}
func (s *Store) RenameTrip(ctx context.Context, id, actor, name string) (PackingSession, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 200 {
		return PackingSession{}, ErrInvalid
	}
	return s.updateSession(ctx, id, actor, func(trip *PackingSession) (bool, error) {
		if trip.UserID != actor {
			return false, ErrForbidden
		}
		trip.Name = name
		return true, nil
	})
}

var ErrFutureSave = errors.New("trip saved; saving to the reusable packing list needs attention")
