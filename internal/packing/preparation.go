package packing

import (
	"context"
	"slices"
	"strings"
)

func (s *Store) EditPreparationTask(ctx context.Context, id, actor string, task PreparationTask, remove bool, revision string) error {
	list, err := s.GetPackingList(ctx, id, actor)
	if err != nil {
		return err
	}
	if revision == "" || revision != list.Revision() {
		return ErrConflict
	}
	task.Name = strings.TrimSpace(task.Name)
	if task.ID == "" || len(task.ID) > 100 || !remove && (task.Name == "" || len(task.Name) > 200) {
		return ErrInvalid
	}
	index := slices.IndexFunc(list.Tasks, func(current PreparationTask) bool { return current.ID == task.ID })
	if remove {
		if index < 0 {
			return ErrNotFound
		}
		list.Tasks = append(list.Tasks[:index], list.Tasks[index+1:]...)
	} else if index < 0 {
		if len(list.Tasks) >= 200 {
			return ErrInvalid
		}
		list.Tasks = append(list.Tasks, task)
	} else {
		list.Tasks[index] = task
	}
	return s.SavePackingList(ctx, list)
}
