package packing

import (
	"context"
	"slices"
	"strings"
)

// EditPreparationTask returns the list as this edit saved it, carrying the
// revision of that write rather than whatever was stored after it.
func (s *Store) EditPreparationTask(ctx context.Context, id, actor string, task PreparationTask, remove bool, revision string) (PackingList, error) {
	list, err := s.GetPackingList(ctx, id, actor)
	if err != nil {
		return PackingList{}, err
	}
	if revision == "" || revision != list.Revision() {
		return PackingList{}, ErrConflict
	}
	if !validScope(task.Scope, false) {
		return PackingList{}, ErrInvalid
	}
	task.Name = strings.TrimSpace(task.Name)
	if task.ID == "" || len(task.ID) > 100 || !remove && (task.Name == "" || len(task.Name) > 200) {
		return PackingList{}, ErrInvalid
	}
	index := slices.IndexFunc(list.Tasks, func(current PreparationTask) bool { return current.ID == task.ID })
	if remove {
		if index < 0 {
			return PackingList{}, ErrNotFound
		}
		list.Tasks = append(list.Tasks[:index], list.Tasks[index+1:]...)
	} else if index < 0 {
		if len(list.Tasks) >= 200 {
			return PackingList{}, ErrInvalid
		}
		list.Tasks = append(list.Tasks, task)
	} else {
		list.Tasks[index] = task
	}
	etag, err := s.saveList(ctx, list)
	if err != nil {
		return PackingList{}, err
	}
	list.etag = etag
	return list, nil
}

// SetSessionPreparationTask records preparation for this trip only.
func (s *Store) SetSessionPreparationTask(ctx context.Context, id, actor, taskID string, done bool, expectedRevision int64) (PackingSession, error) {
	if expectedRevision < 0 {
		return PackingSession{}, ErrInvalid
	}
	return s.updateSession(ctx, id, actor, func(session *PackingSession) (bool, error) {
		index := slices.IndexFunc(session.List.Tasks, func(task PreparationTask) bool { return task.ID == taskID })
		if index < 0 {
			return false, ErrNotFound
		}
		task := &session.List.Tasks[index]
		return task.setCompletion(actor, session.participantName(actor), done, expectedRevision)
	})
}

func (task *PreparationTask) setCompletion(actor, name string, done bool, revision int64) (bool, error) {
	if task.Assignee != "" && task.Assignee != actor {
		return false, ErrForbidden
	}
	if task.Done == done {
		return false, nil
	}
	if task.Revision != revision {
		return false, ErrConflict
	}
	task.Done = done
	task.Revision++
	task.ChangedBy = name
	task.ChangedByID = actor
	return true, nil
}
