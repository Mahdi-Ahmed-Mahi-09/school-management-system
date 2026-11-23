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
	// maxSubjectsPerAssign caps one POST /class-subjects request.
	maxSubjectsPerAssign = 100
	// relatedAssignmentsLimit caps GET /classes/:id/subjects and /subjects/:id/classes.
	relatedAssignmentsLimit = 500
)

var errClassSubjectNotFound = apperror.NotFound("Subject assignment not found.")

// AssignSubjectsInput is the body of POST /class-subjects: one or more
// subjects for one class in one academic year, saved all or nothing.
type AssignSubjectsInput struct {
	ClassID        string              `json:"class_id"`
	AcademicYearID string              `json:"academic_year_id"`
	Subjects       []AssignSubjectItem `json:"subjects"`
}

// AssignSubjectItem is one subject to assign. A null is_compulsory follows
// the subject's type.
type AssignSubjectItem struct {
	SubjectID    string `json:"subject_id"`
	IsCompulsory *bool  `json:"is_compulsory"`
}

// UpdateClassSubjectInput is the body of PUT /class-subjects/:id.
type UpdateClassSubjectInput struct {
	IsCompulsory *bool `json:"is_compulsory"`
}

// ListClassSubjectsInput holds raw listing parameters.
type ListClassSubjectsInput struct {
	ClassID, AcademicYearID, SubjectID, Type, Search string
	Page, PageSize                                   int
}

// ListClassSubjectsResult is a page of assignments.
type ListClassSubjectsResult struct {
	Assignments []models.ClassSubject
	Total       int
	Page        int
	PageSize    int
}

// ClassSubjectService manages which subjects each class takes in each
// academic year.
type ClassSubjectService struct {
	assignments repository.ClassSubjectRepository
	classes     repository.ClassRepository
	subjects    repository.SubjectRepository
	activity    repository.ActivityRepository
}

// NewClassSubjectService creates a ClassSubjectService; activity may be nil.
func NewClassSubjectService(assignments repository.ClassSubjectRepository, classes repository.ClassRepository,
	subjects repository.SubjectRepository, activity repository.ActivityRepository) *ClassSubjectService {
	return &ClassSubjectService{assignments: assignments, classes: classes, subjects: subjects, activity: activity}
}

// joinNames lists names as "A", "A and B" or "A, B and C".
func joinNames(names []string) string {
	switch len(names) {
	case 0:
		return ""
	case 1:
		return names[0]
	default:
		return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
	}
}

func subjectNames(subjects []models.SubjectRef) []string {
	names := make([]string, len(subjects))
	for i, s := range subjects {
		names[i] = s.Name
	}
	return names
}

func mapClassSubjectRepoError(err error) error {
	var appErr *apperror.Error
	var exists *repository.ClassSubjectExistsError
	switch {
	case errors.As(err, &appErr):
		return appErr
	case errors.Is(err, repository.ErrNotFound):
		return errClassSubjectNotFound
	case errors.Is(err, repository.ErrClassNotFound):
		return apperror.Validation(map[string]string{"class_id": "That class doesn't exist."})
	case errors.Is(err, repository.ErrYearNotFound):
		return apperror.Validation(map[string]string{"academic_year_id": "That academic year doesn't exist."})
	case errors.Is(err, repository.ErrSubjectNotFound):
		return apperror.Validation(map[string]string{"subjects": "One or more of the chosen subjects don't exist."})
	case errors.As(err, &exists):
		names := subjectNames(exists.Subjects)
		verb := "is"
		if len(names) > 1 {
			verb = "are"
		}
		msg := fmt.Sprintf("%s %s already assigned to this class for the selected academic year.", joinNames(names), verb)
		return apperror.Conflict("CLASS_SUBJECT_EXISTS", msg, map[string]string{"subjects": msg})
	case errors.Is(err, repository.ErrDuplicateClassSubject):
		msg := "One of these subjects was just assigned to this class for the selected academic year. Refresh and try again."
		return apperror.Conflict("CLASS_SUBJECT_EXISTS", msg, map[string]string{"subjects": msg})
	case errors.Is(err, repository.ErrClassSubjectInUse):
		return apperror.Conflict("CLASS_SUBJECT_IN_USE",
			"This subject assignment is used by other records (such as routines or exams), so it can't be removed.", nil)
	default:
		return apperror.Internal(err)
	}
}

func parseAssignInput(in AssignSubjectsInput) (uuid.UUID, uuid.UUID, []repository.NewClassSubject, error) {
	errs := validation.Errors{}
	classID := parseRequiredID(errs, "class_id", in.ClassID, "Choose a class.")
	yearID := parseRequiredID(errs, "academic_year_id", in.AcademicYearID, "Choose an academic year.")
	var items []repository.NewClassSubject
	switch {
	case len(in.Subjects) == 0:
		errs.Add("subjects", "Choose at least one subject.")
	case len(in.Subjects) > maxSubjectsPerAssign:
		errs.Add("subjects", fmt.Sprintf("Assign at most %d subjects at a time.", maxSubjectsPerAssign))
	default:
		seen := map[uuid.UUID]bool{}
		for _, it := range in.Subjects {
			id, err := uuid.Parse(strings.TrimSpace(it.SubjectID))
			if err != nil {
				errs.Add("subjects", "Contains an invalid subject.")
				break
			}
			if seen[id] {
				errs.Add("subjects", "Each subject can only be chosen once.")
				break
			}
			seen[id] = true
			items = append(items, repository.NewClassSubject{SubjectID: id, IsCompulsory: it.IsCompulsory})
		}
	}
	if errs.Any() {
		return uuid.Nil, uuid.Nil, nil, apperror.Validation(errs)
	}
	return classID, yearID, items, nil
}

func (s *ClassSubjectService) record(ctx context.Context, actor *models.User, action string, classID uuid.UUID, summary string, meta map[string]any) {
	if s.activity == nil {
		return
	}
	if err := s.activity.Record(ctx, models.ActivityEntry{
		ActorID: &actor.ID, Action: action, EntityType: "class", EntityID: &classID, Summary: summary, Metadata: meta,
	}); err != nil {
		slog.Warn("could not record activity", "action", action, "class", classID, "error", err)
	}
}

// List returns a filtered page of assignments.
func (s *ClassSubjectService) List(ctx context.Context, in ListClassSubjectsInput) (*ListClassSubjectsResult, error) {
	errs := validation.Errors{}
	f := repository.ClassSubjectFilter{
		ClassID:        optionalID(errs, "class_id", in.ClassID),
		AcademicYearID: optionalID(errs, "academic_year_id", in.AcademicYearID),
		SubjectID:      optionalID(errs, "subject_id", in.SubjectID),
		Search:         strings.TrimSpace(in.Search),
	}
	if len([]rune(f.Search)) > maxSearchLength {
		errs.Add("search", "Search must be at most 100 characters.")
	}
	if in.Type != "" {
		t, ok := models.ParseSubjectType(in.Type)
		if !ok {
			errs.Add("type", "Unknown subject type.")
		}
		compulsory := t == models.SubjectCompulsory
		f.IsCompulsory = &compulsory
	}
	f.Page, f.PageSize = normalizePaging(in.Page, in.PageSize)
	if errs.Any() {
		return nil, apperror.Validation(errs)
	}
	items, total, err := s.assignments.List(ctx, f)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return &ListClassSubjectsResult{Assignments: items, Total: total, Page: f.Page, PageSize: f.PageSize}, nil
}

// ForClass lists a class's subjects, newest academic year first, optionally
// limited to one academic year.
func (s *ClassSubjectService) ForClass(ctx context.Context, classID uuid.UUID, rawYear string) ([]models.ClassSubject, error) {
	errs := validation.Errors{}
	yearID := optionalID(errs, "academic_year_id", rawYear)
	if errs.Any() {
		return nil, apperror.Validation(errs)
	}
	if _, err := s.classes.GetByID(ctx, classID); err != nil {
		return nil, mapClassRepoError(err)
	}
	items, _, err := s.assignments.List(ctx, repository.ClassSubjectFilter{
		ClassID: &classID, AcademicYearID: yearID, Page: 1, PageSize: relatedAssignmentsLimit,
	})
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return items, nil
}

// ForSubject lists the classes a subject is assigned to, newest academic year
// first, optionally limited to one academic year.
func (s *ClassSubjectService) ForSubject(ctx context.Context, subjectID uuid.UUID, rawYear string) ([]models.ClassSubject, error) {
	errs := validation.Errors{}
	yearID := optionalID(errs, "academic_year_id", rawYear)
	if errs.Any() {
		return nil, apperror.Validation(errs)
	}
	if _, err := s.subjects.GetByID(ctx, subjectID); err != nil {
		return nil, mapSubjectRepoError(err)
	}
	items, _, err := s.assignments.List(ctx, repository.ClassSubjectFilter{
		SubjectID: &subjectID, AcademicYearID: yearID, Page: 1, PageSize: relatedAssignmentsLimit,
	})
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return items, nil
}

// Get returns one assignment.
func (s *ClassSubjectService) Get(ctx context.Context, id uuid.UUID) (*models.ClassSubject, error) {
	a, err := s.assignments.GetByID(ctx, id)
	if err != nil {
		return nil, mapClassSubjectRepoError(err)
	}
	return a, nil
}

// Assign gives a class one or more active subjects for the current or an
// upcoming academic year. Nothing is saved if any subject can't be assigned.
func (s *ClassSubjectService) Assign(ctx context.Context, actor *models.User, in AssignSubjectsInput) ([]models.ClassSubject, error) {
	classID, yearID, items, err := parseAssignInput(in)
	if err != nil {
		return nil, err
	}
	created, err := s.assignments.Assign(ctx, classID, yearID, items, actor.ID,
		func(class models.ClassRef, year models.YearRef, subjects []models.SubjectRef, items []repository.NewClassSubject) error {
			if class.Status != models.StatusActive {
				return classInactive(class, "assigning subjects")
			}
			if year.Status == models.YearCompleted {
				msg := fmt.Sprintf("%s is completed. Subjects can only be assigned for the current or an upcoming academic year.", year.Name)
				return apperror.Conflict("ACADEMIC_YEAR_COMPLETED", msg, map[string]string{"academic_year_id": msg})
			}
			var inactive []string
			for i, sub := range subjects {
				if sub.Status != models.StatusActive {
					inactive = append(inactive, sub.Name)
				}
				if items[i].IsCompulsory == nil {
					compulsory := sub.Type == models.SubjectCompulsory
					items[i].IsCompulsory = &compulsory
				}
			}
			if len(inactive) > 0 {
				verb, pronoun := "is", "it"
				if len(inactive) > 1 {
					verb, pronoun = "are", "them"
				}
				msg := fmt.Sprintf("%s %s inactive. Activate %s before assigning.", joinNames(inactive), verb, pronoun)
				return apperror.Conflict("SUBJECT_INACTIVE", msg, map[string]string{"subjects": msg})
			}
			return nil
		})
	if err != nil {
		return nil, mapClassSubjectRepoError(err)
	}
	if len(created) > 0 {
		first := created[0]
		names := make([]string, len(created))
		for i, a := range created {
			names[i] = a.Subject.Name
		}
		s.record(ctx, actor, models.ActivityClassSubjectsAssigned, first.ClassID,
			fmt.Sprintf("Assigned %s to %s (%s)", joinNames(names), first.Class.Name, first.AcademicYear.Name),
			map[string]any{"class": first.Class.Name, "academic_year": first.AcademicYear.Name, "subjects": len(created)})
	}
	return created, nil
}

// Update changes whether a subject is compulsory for a class in that year.
func (s *ClassSubjectService) Update(ctx context.Context, actor *models.User, id uuid.UUID, in UpdateClassSubjectInput) (*models.ClassSubject, error) {
	if in.IsCompulsory == nil {
		return nil, apperror.Validation(map[string]string{"is_compulsory": "Choose compulsory or optional."})
	}
	updated, err := s.assignments.SetCompulsory(ctx, id, *in.IsCompulsory, actor.ID)
	if err != nil {
		return nil, mapClassSubjectRepoError(err)
	}
	kind := "optional"
	if updated.IsCompulsory {
		kind = "compulsory"
	}
	s.record(ctx, actor, models.ActivityClassSubjectUpdated, updated.ClassID,
		fmt.Sprintf("Made %s %s for %s (%s)", updated.Subject.Name, kind, updated.Class.Name, updated.AcademicYear.Name),
		map[string]any{"subject": updated.Subject.Name, "is_compulsory": updated.IsCompulsory})
	return updated, nil
}

// Delete removes a subject from a class for that academic year, unless other
// records reference the assignment.
func (s *ClassSubjectService) Delete(ctx context.Context, actor *models.User, id uuid.UUID) (*models.ClassSubject, error) {
	deleted, err := s.assignments.Delete(ctx, id)
	if err != nil {
		return nil, mapClassSubjectRepoError(err)
	}
	s.record(ctx, actor, models.ActivityClassSubjectRemoved, deleted.ClassID,
		fmt.Sprintf("Removed %s from %s (%s)", deleted.Subject.Name, deleted.Class.Name, deleted.AcademicYear.Name),
		map[string]any{"subject": deleted.Subject.Name})
	return deleted, nil
}
