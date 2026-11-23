package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"github.com/google/uuid"

	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/apperror"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/models"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/repository"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/validation"
)

const (
	classNameMax        = 60
	classDescriptionMax = 500
	classMaxOrder       = 9999
	maxClassesToReorder = 500
)

var (
	// classCodePattern is shared by class and section codes, e.g. "C6" or "6-A".
	classCodePattern  = regexp.MustCompile(`^[A-Z0-9][A-Z0-9-]{0,19}$`)
	errClassNotFound  = apperror.NotFound("Class not found.")
	classCodeHelpText = "Use up to 20 letters, numbers or dashes."
)

// ClassInput is the create/update body. Status is only accepted on create;
// afterwards it changes through PATCH /classes/:id/status.
type ClassInput struct {
	Name         string `json:"name"`
	Code         string `json:"code"`
	DisplayOrder *int   `json:"display_order"` // null on create = after the last class; on update = unchanged
	Description  string `json:"description"`
}

// CreateClassInput adds the initial status to ClassInput.
type CreateClassInput struct {
	ClassInput
	Status string `json:"status"`
}

// ListClassesInput holds raw listing parameters.
type ListClassesInput struct {
	Search, Status, SortBy, SortOrder string
	Page, PageSize                    int
}

// ListClassesResult is a page of classes.
type ListClassesResult struct {
	Classes  []models.Class
	Total    int
	Page     int
	PageSize int
}

// ClassService implements class management.
type ClassService struct {
	classes  repository.ClassRepository
	activity repository.ActivityRepository
}

// NewClassService creates a ClassService; activity may be nil.
func NewClassService(classes repository.ClassRepository, activity repository.ActivityRepository) *ClassService {
	return &ClassService{classes: classes, activity: activity}
}

func mapClassRepoError(err error) error {
	var appErr *apperror.Error
	var inUse *repository.ClassInUseError
	switch {
	case errors.As(err, &appErr):
		return appErr
	case errors.Is(err, repository.ErrNotFound):
		return errClassNotFound
	case errors.Is(err, repository.ErrDuplicateClassName):
		return apperror.Conflict("CLASS_NAME_TAKEN", "A class with this name already exists.",
			map[string]string{"name": "A class with this name already exists."})
	case errors.Is(err, repository.ErrDuplicateClassCode):
		return apperror.Conflict("CLASS_CODE_TAKEN", "Another class already uses this code.",
			map[string]string{"code": "Another class already uses this code."})
	case errors.As(err, &inUse):
		msg := "This class is used by other records, so it can't be deleted."
		switch {
		case inUse.Sections > 0:
			msg = fmt.Sprintf("This class has %d section%s. Delete them first, or deactivate the class instead.",
				inUse.Sections, plural(inUse.Sections))
		case inUse.SubjectAssignments > 0:
			msg = fmt.Sprintf("This class has %d subject assignment%s. Remove them first, or deactivate the class instead.",
				inUse.SubjectAssignments, plural(inUse.SubjectAssignments))
		}
		return apperror.Conflict("CLASS_IN_USE", msg, nil)
	case errors.Is(err, repository.ErrClassOrderMismatch):
		return apperror.Conflict("CLASS_ORDER_CHANGED", "The class list changed since you opened it. Refresh and try again.", nil)
	default:
		return apperror.Internal(err)
	}
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// normalizeCode upper-cases an optional code and validates it.
func normalizeCode(errs validation.Errors, field, raw string) *string {
	code := strings.ToUpper(strings.TrimSpace(raw))
	if code == "" {
		return nil
	}
	if !classCodePattern.MatchString(code) {
		errs.Add(field, classCodeHelpText)
	}
	return &code
}

func validateClass(errs validation.Errors, in ClassInput) *models.Class {
	c := &models.Class{
		Name:        validation.NormalizeName(in.Name),
		Description: optional(validation.NormalizeText(in.Description)),
	}
	validation.PersonName(errs, "name", "Class name", c.Name, classNameMax)
	c.Code = normalizeCode(errs, "code", in.Code)
	if in.DisplayOrder != nil {
		if *in.DisplayOrder < 1 || *in.DisplayOrder > classMaxOrder {
			errs.Add("display_order", "Display order must be between 1 and 9999.")
		}
		c.DisplayOrder = *in.DisplayOrder
	}
	if c.Description != nil {
		validation.Text(errs, "description", "Description", *c.Description, classDescriptionMax, false, true)
	}
	return c
}

func (s *ClassService) record(ctx context.Context, actor *models.User, action string, c *models.Class, summary string, meta map[string]any) {
	if s.activity == nil {
		return
	}
	if err := s.activity.Record(ctx, models.ActivityEntry{
		ActorID: &actor.ID, Action: action, EntityType: "class", EntityID: &c.ID, Summary: summary, Metadata: meta,
	}); err != nil {
		slog.Warn("could not record activity", "action", action, "class", c.ID, "error", err)
	}
}

// List returns a filtered page of classes.
func (s *ClassService) List(ctx context.Context, in ListClassesInput) (*ListClassesResult, error) {
	errs := validation.Errors{}
	f := repository.ClassFilter{Search: strings.TrimSpace(in.Search), Page: in.Page, PageSize: in.PageSize, SortBy: in.SortBy}
	if len([]rune(f.Search)) > maxSearchLength {
		errs.Add("search", "Search must be at most 100 characters.")
	}
	if in.Status != "" {
		st, ok := models.ParseStatus(in.Status)
		if !ok {
			errs.Add("status", "Unknown status.")
		}
		f.Status = &st
	}
	switch f.SortBy {
	case "":
		f.SortBy = repository.ClassSortDisplayOrder
	case repository.ClassSortDisplayOrder, repository.ClassSortName, repository.ClassSortCreatedAt:
	default:
		errs.Add("sort", "Unsupported sort field.")
	}
	switch strings.ToLower(in.SortOrder) {
	case "":
		f.SortDesc = f.SortBy == repository.ClassSortCreatedAt
	case "asc":
	case "desc":
		f.SortDesc = true
	default:
		errs.Add("order", "Order must be asc or desc.")
	}
	f.Page, f.PageSize = normalizePaging(in.Page, in.PageSize)
	if errs.Any() {
		return nil, apperror.Validation(errs)
	}
	classes, total, err := s.classes.List(ctx, f)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return &ListClassesResult{Classes: classes, Total: total, Page: f.Page, PageSize: f.PageSize}, nil
}

// normalizePaging applies the default and maximum page size.
func normalizePaging(page, size int) (int, int) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = DefaultPageSize
	}
	if size > MaxPageSize {
		size = MaxPageSize
	}
	return page, size
}

// Stats returns class and section counts.
func (s *ClassService) Stats(ctx context.Context) (*models.ClassStats, error) {
	stats, err := s.classes.Stats(ctx)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return stats, nil
}

// Get returns one class.
func (s *ClassService) Get(ctx context.Context, id uuid.UUID) (*models.Class, error) {
	c, err := s.classes.GetByID(ctx, id)
	if err != nil {
		return nil, mapClassRepoError(err)
	}
	return c, nil
}

// Create adds a class.
func (s *ClassService) Create(ctx context.Context, actor *models.User, in CreateClassInput) (*models.Class, error) {
	errs := validation.Errors{}
	c := validateClass(errs, in.ClassInput)
	c.Status = models.StatusActive
	if strings.TrimSpace(in.Status) != "" {
		st, ok := models.ParseStatus(in.Status)
		if !ok {
			errs.Add("status", "Choose active or inactive.")
		}
		c.Status = st
	}
	if errs.Any() {
		return nil, apperror.Validation(errs)
	}
	created, err := s.classes.Create(ctx, c, actor.ID)
	if err != nil {
		return nil, mapClassRepoError(err)
	}
	s.record(ctx, actor, models.ActivityClassCreated, created, fmt.Sprintf("Added class %s", created.Name), nil)
	return created, nil
}

// Update replaces a class's name, code, description and (if given) display order.
func (s *ClassService) Update(ctx context.Context, actor *models.User, id uuid.UUID, in ClassInput) (*models.Class, error) {
	errs := validation.Errors{}
	next := validateClass(errs, in)
	if errs.Any() {
		return nil, apperror.Validation(errs)
	}
	var before string
	updated, err := s.classes.Update(ctx, id, actor.ID, func(c *models.Class) error {
		before = c.Name
		c.Name, c.Code, c.Description = next.Name, next.Code, next.Description
		if in.DisplayOrder != nil {
			c.DisplayOrder = next.DisplayOrder
		}
		return nil
	})
	if err != nil {
		return nil, mapClassRepoError(err)
	}
	meta := map[string]any{}
	if before != updated.Name {
		meta["name_from"], meta["name_to"] = before, updated.Name
	}
	s.record(ctx, actor, models.ActivityClassUpdated, updated, fmt.Sprintf("Updated class %s", updated.Name), meta)
	return updated, nil
}

// SetStatus activates or deactivates a class. Existing sections are kept;
// an inactive class can't receive new (or re-activated) sections.
func (s *ClassService) SetStatus(ctx context.Context, actor *models.User, id uuid.UUID, raw string) (*models.Class, error) {
	status, ok := models.ParseStatus(raw)
	if !ok || strings.TrimSpace(raw) == "" {
		return nil, apperror.Validation(map[string]string{"status": "Choose active or inactive."})
	}
	updated, err := s.classes.SetStatus(ctx, id, status, actor.ID)
	if err != nil {
		return nil, mapClassRepoError(err)
	}
	verb := "Activated"
	if status == models.StatusInactive {
		verb = "Deactivated"
	}
	s.record(ctx, actor, models.ActivityClassStatusChanged, updated, fmt.Sprintf("%s class %s", verb, updated.Name),
		map[string]any{"status_to": status})
	return updated, nil
}

// Reorder sets the display order of every class to the given sequence.
func (s *ClassService) Reorder(ctx context.Context, actor *models.User, rawIDs []string) error {
	if len(rawIDs) == 0 || len(rawIDs) > maxClassesToReorder {
		return apperror.Validation(map[string]string{"class_ids": "List every class in the new order."})
	}
	ids := make([]uuid.UUID, len(rawIDs))
	for i, raw := range rawIDs {
		id, err := uuid.Parse(raw)
		if err != nil {
			return apperror.Validation(map[string]string{"class_ids": "Contains an invalid class."})
		}
		ids[i] = id
	}
	if err := s.classes.Reorder(ctx, ids, actor.ID); err != nil {
		return mapClassRepoError(err)
	}
	if s.activity != nil {
		if err := s.activity.Record(ctx, models.ActivityEntry{
			ActorID: &actor.ID, Action: models.ActivityClassesReordered, EntityType: "class",
			Summary: "Changed the display order of classes", Metadata: map[string]any{"classes": len(ids)},
		}); err != nil {
			slog.Warn("could not record activity", "action", models.ActivityClassesReordered, "error", err)
		}
	}
	return nil
}

// Delete removes a class with no sections, subject assignments or other references.
func (s *ClassService) Delete(ctx context.Context, actor *models.User, id uuid.UUID) (*models.Class, error) {
	deleted, err := s.classes.Delete(ctx, id)
	if err != nil {
		return nil, mapClassRepoError(err)
	}
	s.record(ctx, actor, models.ActivityClassDeleted, deleted, fmt.Sprintf("Deleted class %s", deleted.Name), nil)
	return deleted, nil
}
