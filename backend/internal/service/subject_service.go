package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"

	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/apperror"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/models"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/repository"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/validation"
)

const (
	subjectNameMax        = 100
	subjectDescriptionMax = 500
	subjectMarksMax       = 1000
)

var errSubjectNotFound = apperror.NotFound("Subject not found.")

// SubjectInput is the create/update body. On update, an empty subject_type
// or status keeps the current value; every other field is replaced.
type SubjectInput struct {
	Name        string `json:"name"`
	Code        string `json:"code"`
	Description string `json:"description"`
	SubjectType string `json:"subject_type"` // compulsory (default) or optional
	FullMarks   *int   `json:"full_marks"`   // null = not set
	PassMarks   *int   `json:"pass_marks"`   // null = not set; needs full_marks
	Status      string `json:"status"`       // active (default) or inactive
}

// ListSubjectsInput holds raw listing parameters.
type ListSubjectsInput struct {
	Search, Status, Type, SortBy, SortOrder string
	Page, PageSize                          int
}

// ListSubjectsResult is a page of subjects.
type ListSubjectsResult struct {
	Subjects []models.Subject
	Total    int
	Page     int
	PageSize int
}

// SubjectService implements the subject catalogue.
type SubjectService struct {
	subjects repository.SubjectRepository
	activity repository.ActivityRepository
}

// NewSubjectService creates a SubjectService; activity may be nil.
func NewSubjectService(subjects repository.SubjectRepository, activity repository.ActivityRepository) *SubjectService {
	return &SubjectService{subjects: subjects, activity: activity}
}

func mapSubjectRepoError(err error) error {
	var appErr *apperror.Error
	var inUse *repository.SubjectInUseError
	switch {
	case errors.As(err, &appErr):
		return appErr
	case errors.Is(err, repository.ErrNotFound):
		return errSubjectNotFound
	case errors.Is(err, repository.ErrDuplicateSubjectName):
		msg := "A subject with this name already exists."
		return apperror.Conflict("SUBJECT_NAME_TAKEN", msg, map[string]string{"name": msg})
	case errors.Is(err, repository.ErrDuplicateSubjectCode):
		msg := "Another subject already uses this code."
		return apperror.Conflict("SUBJECT_CODE_TAKEN", msg, map[string]string{"code": msg})
	case errors.As(err, &inUse):
		msg := "This subject is used by other records, so it can't be deleted."
		if inUse.Assignments > 0 {
			msg = fmt.Sprintf("This subject is assigned to %d class%s. Remove those assignments first, or deactivate the subject instead.",
				inUse.Assignments, pluralES(inUse.Assignments))
		}
		return apperror.Conflict("SUBJECT_IN_USE", msg, nil)
	default:
		return apperror.Internal(err)
	}
}

func pluralES(n int) string {
	if n == 1 {
		return ""
	}
	return "es"
}

// validateSubject validates the fields shared by create and update. Type and
// status are left zero when not given.
func validateSubject(in SubjectInput) (*models.Subject, error) {
	errs := validation.Errors{}
	s := &models.Subject{
		Name:        validation.NormalizeName(in.Name),
		Description: optional(validation.NormalizeText(in.Description)),
		FullMarks:   in.FullMarks,
		PassMarks:   in.PassMarks,
	}
	validation.PersonName(errs, "name", "Subject name", s.Name, subjectNameMax)
	if code := normalizeCode(errs, "code", in.Code); code == nil {
		errs.Add("code", "Subject code is required.")
	} else {
		s.Code = *code
	}
	if s.Description != nil {
		validation.Text(errs, "description", "Description", *s.Description, subjectDescriptionMax, false, true)
	}
	if strings.TrimSpace(in.SubjectType) != "" {
		t, ok := models.ParseSubjectType(in.SubjectType)
		if !ok {
			errs.Add("subject_type", "Choose compulsory or optional.")
		}
		s.Type = t
	}
	if strings.TrimSpace(in.Status) != "" {
		st, ok := models.ParseStatus(in.Status)
		if !ok {
			errs.Add("status", "Choose active or inactive.")
		}
		s.Status = st
	}
	if s.FullMarks != nil && (*s.FullMarks < 1 || *s.FullMarks > subjectMarksMax) {
		errs.Add("full_marks", "Full marks must be a whole number from 1 to 1000.")
	}
	if s.PassMarks != nil {
		switch {
		case *s.PassMarks < 0:
			errs.Add("pass_marks", "Pass marks can't be negative.")
		case s.FullMarks == nil:
			errs.Add("pass_marks", "Enter the full marks to set pass marks.")
		case *s.PassMarks > *s.FullMarks:
			errs.Add("pass_marks", "Pass marks can't be more than the full marks.")
		}
	}
	if errs.Any() {
		return nil, apperror.Validation(errs)
	}
	return s, nil
}

func (s *SubjectService) record(ctx context.Context, actor *models.User, action string, sub *models.Subject, summary string, meta map[string]any) {
	if s.activity == nil {
		return
	}
	if err := s.activity.Record(ctx, models.ActivityEntry{
		ActorID: &actor.ID, Action: action, EntityType: "subject", EntityID: &sub.ID, Summary: summary, Metadata: meta,
	}); err != nil {
		slog.Warn("could not record activity", "action", action, "subject", sub.ID, "error", err)
	}
}

func subjectLabel(sub *models.Subject) string { return fmt.Sprintf("%s (%s)", sub.Name, sub.Code) }

// List returns a filtered page of subjects.
func (s *SubjectService) List(ctx context.Context, in ListSubjectsInput) (*ListSubjectsResult, error) {
	errs := validation.Errors{}
	f := repository.SubjectFilter{Search: strings.TrimSpace(in.Search), SortBy: in.SortBy}
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
	if in.Type != "" {
		t, ok := models.ParseSubjectType(in.Type)
		if !ok {
			errs.Add("type", "Unknown subject type.")
		}
		f.Type = &t
	}
	switch f.SortBy {
	case "":
		f.SortBy = repository.SubjectSortName
	case repository.SubjectSortName, repository.SubjectSortCode, repository.SubjectSortCreatedAt:
	default:
		errs.Add("sort", "Unsupported sort field.")
	}
	switch strings.ToLower(in.SortOrder) {
	case "":
		f.SortDesc = f.SortBy == repository.SubjectSortCreatedAt
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
	subjects, total, err := s.subjects.List(ctx, f)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return &ListSubjectsResult{Subjects: subjects, Total: total, Page: f.Page, PageSize: f.PageSize}, nil
}

// Stats returns subject and assignment counts.
func (s *SubjectService) Stats(ctx context.Context) (*models.SubjectStats, error) {
	stats, err := s.subjects.Stats(ctx)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return stats, nil
}

// Get returns one subject.
func (s *SubjectService) Get(ctx context.Context, id uuid.UUID) (*models.Subject, error) {
	sub, err := s.subjects.GetByID(ctx, id)
	if err != nil {
		return nil, mapSubjectRepoError(err)
	}
	return sub, nil
}

// Create adds a subject to the catalogue.
func (s *SubjectService) Create(ctx context.Context, actor *models.User, in SubjectInput) (*models.Subject, error) {
	sub, err := validateSubject(in)
	if err != nil {
		return nil, err
	}
	if sub.Type == "" {
		sub.Type = models.SubjectCompulsory
	}
	if sub.Status == "" {
		sub.Status = models.StatusActive
	}
	created, err := s.subjects.Create(ctx, sub, actor.ID)
	if err != nil {
		return nil, mapSubjectRepoError(err)
	}
	s.record(ctx, actor, models.ActivitySubjectCreated, created, "Added subject "+subjectLabel(created), nil)
	return created, nil
}

// Update replaces a subject's details. Changing the subject type does not
// change existing class assignments, which keep their own compulsory flag.
func (s *SubjectService) Update(ctx context.Context, actor *models.User, id uuid.UUID, in SubjectInput) (*models.Subject, error) {
	next, err := validateSubject(in)
	if err != nil {
		return nil, err
	}
	var before models.Subject
	updated, err := s.subjects.Update(ctx, id, actor.ID, func(cur *models.Subject) error {
		before = *cur
		cur.Name, cur.Code, cur.Description = next.Name, next.Code, next.Description
		cur.FullMarks, cur.PassMarks = next.FullMarks, next.PassMarks
		if next.Type != "" {
			cur.Type = next.Type
		}
		if next.Status != "" {
			cur.Status = next.Status
		}
		return nil
	})
	if err != nil {
		return nil, mapSubjectRepoError(err)
	}
	meta := map[string]any{}
	if before.Name != updated.Name {
		meta["name_from"], meta["name_to"] = before.Name, updated.Name
	}
	if before.Code != updated.Code {
		meta["code_from"], meta["code_to"] = before.Code, updated.Code
	}
	if before.Status != updated.Status {
		meta["status_to"] = updated.Status
	}
	s.record(ctx, actor, models.ActivitySubjectUpdated, updated, "Updated subject "+subjectLabel(updated), meta)
	return updated, nil
}

// SetStatus activates or deactivates a subject. Existing class assignments are
// kept; an inactive subject can't be assigned to more classes.
func (s *SubjectService) SetStatus(ctx context.Context, actor *models.User, id uuid.UUID, raw string) (*models.Subject, error) {
	status, ok := models.ParseStatus(raw)
	if !ok || strings.TrimSpace(raw) == "" {
		return nil, apperror.Validation(map[string]string{"status": "Choose active or inactive."})
	}
	updated, err := s.subjects.SetStatus(ctx, id, status, actor.ID)
	if err != nil {
		return nil, mapSubjectRepoError(err)
	}
	verb := "Activated"
	if status == models.StatusInactive {
		verb = "Deactivated"
	}
	s.record(ctx, actor, models.ActivitySubjectStatusChanged, updated, fmt.Sprintf("%s subject %s", verb, subjectLabel(updated)),
		map[string]any{"status_to": status})
	return updated, nil
}

// Delete removes a subject with no class assignments and no other references.
func (s *SubjectService) Delete(ctx context.Context, actor *models.User, id uuid.UUID) (*models.Subject, error) {
	deleted, err := s.subjects.Delete(ctx, id)
	if err != nil {
		return nil, mapSubjectRepoError(err)
	}
	s.record(ctx, actor, models.ActivitySubjectDeleted, deleted, "Deleted subject "+subjectLabel(deleted), nil)
	return deleted, nil
}
