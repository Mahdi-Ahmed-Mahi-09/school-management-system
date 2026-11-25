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
	sectionNameMax        = 30
	sectionRoomMax        = 40
	sectionDescriptionMax = 500
	sectionCapacityMax    = 1000
	// classSectionsLimit caps GET /classes/:id/sections (all years of one class).
	classSectionsLimit = 500
)

var errSectionNotFound = apperror.NotFound("Section not found.")

// SectionInput is the create/update body. Update replaces every field.
type SectionInput struct {
	AcademicYearID string `json:"academic_year_id"`
	ClassID        string `json:"class_id"`
	Name           string `json:"name"`
	Code           string `json:"code"`
	RoomNumber     string `json:"room_number"`
	Capacity       *int   `json:"capacity"` // null = no limit recorded
	Description    string `json:"description"`
	Status         string `json:"status"` // default active
}

// ListSectionsInput holds raw listing parameters.
type ListSectionsInput struct {
	ClassID, AcademicYearID, Search, Status, SortBy, SortOrder string
	Page, PageSize                                             int
}

// ListSectionsResult is a page of sections.
type ListSectionsResult struct {
	Sections []models.ClassSection
	Total    int
	Page     int
	PageSize int
}

// SectionService implements section management.
type SectionService struct {
	sections repository.SectionRepository
	classes  repository.ClassRepository
	activity repository.ActivityRepository
}

// NewSectionService creates a SectionService; activity may be nil.
func NewSectionService(sections repository.SectionRepository, classes repository.ClassRepository, activity repository.ActivityRepository) *SectionService {
	return &SectionService{sections: sections, classes: classes, activity: activity}
}

func mapSectionRepoError(err error) error {
	var appErr *apperror.Error
	switch {
	case errors.As(err, &appErr):
		return appErr
	case errors.Is(err, repository.ErrNotFound):
		return errSectionNotFound
	case errors.Is(err, repository.ErrClassNotFound):
		return apperror.Validation(map[string]string{"class_id": "That class doesn't exist."})
	case errors.Is(err, repository.ErrYearNotFound):
		return apperror.Validation(map[string]string{"academic_year_id": "That academic year doesn't exist."})
	case errors.Is(err, repository.ErrDuplicateSectionName):
		msg := "This class already has a section with this name in the selected academic year."
		return apperror.Conflict("SECTION_NAME_TAKEN", msg, map[string]string{"name": msg})
	case errors.Is(err, repository.ErrDuplicateSectionCode):
		msg := "Another section in this academic year already uses this code."
		return apperror.Conflict("SECTION_CODE_TAKEN", msg, map[string]string{"code": msg})
	case errors.Is(err, repository.ErrSectionHasEnrollments):
		return apperror.Conflict("SECTION_HAS_ENROLLMENTS",
			"Students are enrolled in this section, so it can't be moved to another class or academic year.", nil)
	case errors.Is(err, repository.ErrSectionHasAssignments):
		return apperror.Conflict("SECTION_HAS_TEACHER_ASSIGNMENTS",
			"Teachers are assigned to this section, so it can't be moved to another class or academic year.", nil)
	case errors.Is(err, repository.ErrSectionInUse):
		return apperror.Conflict("SECTION_IN_USE", "This section is used by other records (such as enrollments), so it can't be deleted.", nil)
	default:
		return apperror.Internal(err)
	}
}

func parseRequiredID(errs validation.Errors, field, raw, message string) uuid.UUID {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		errs.Add(field, message)
		return uuid.Nil
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		errs.Add(field, message)
	}
	return id
}

func validateSection(in SectionInput) (*models.ClassSection, error) {
	errs := validation.Errors{}
	s := &models.ClassSection{
		AcademicYearID: parseRequiredID(errs, "academic_year_id", in.AcademicYearID, "Choose an academic year."),
		ClassID:        parseRequiredID(errs, "class_id", in.ClassID, "Choose a class."),
		Name:           validation.NormalizeName(in.Name),
		RoomNumber:     optional(validation.NormalizeName(in.RoomNumber)),
		Description:    optional(validation.NormalizeText(in.Description)),
		Capacity:       in.Capacity,
		Status:         models.StatusActive,
	}
	validation.PersonName(errs, "name", "Section name", s.Name, sectionNameMax)
	s.Code = normalizeCode(errs, "code", in.Code)
	if s.RoomNumber != nil {
		validation.Text(errs, "room_number", "Room number", *s.RoomNumber, sectionRoomMax, false, false)
	}
	if s.Capacity != nil && (*s.Capacity < 1 || *s.Capacity > sectionCapacityMax) {
		errs.Add("capacity", "Capacity must be a whole number between 1 and 1000.")
	}
	if s.Description != nil {
		validation.Text(errs, "description", "Description", *s.Description, sectionDescriptionMax, false, true)
	}
	if strings.TrimSpace(in.Status) != "" {
		st, ok := models.ParseStatus(in.Status)
		if !ok {
			errs.Add("status", "Choose active or inactive.")
		}
		s.Status = st
	}
	if errs.Any() {
		return nil, apperror.Validation(errs)
	}
	return s, nil
}

func classInactive(class models.ClassRef, action string) error {
	return apperror.Conflict("CLASS_INACTIVE",
		fmt.Sprintf("%s is inactive. Activate the class before %s.", class.Name, action),
		map[string]string{"class_id": fmt.Sprintf("%s is inactive.", class.Name)})
}

func yearCompleted(year models.YearRef) error {
	msg := fmt.Sprintf("%s is completed. Sections can only be added to the current or an upcoming academic year.", year.Name)
	return apperror.Conflict("ACADEMIC_YEAR_COMPLETED", msg, map[string]string{"academic_year_id": msg})
}

func (s *SectionService) record(ctx context.Context, actor *models.User, action string, sec *models.ClassSection, summary string, meta map[string]any) {
	if s.activity == nil {
		return
	}
	if err := s.activity.Record(ctx, models.ActivityEntry{
		ActorID: &actor.ID, Action: action, EntityType: "section", EntityID: &sec.ID, Summary: summary, Metadata: meta,
	}); err != nil {
		slog.Warn("could not record activity", "action", action, "section", sec.ID, "error", err)
	}
}

func sectionLabel(sec *models.ClassSection) string {
	return fmt.Sprintf("%s – Section %s (%s)", sec.Class.Name, sec.Name, sec.AcademicYear.Name)
}

func optionalID(errs validation.Errors, field, raw string) *uuid.UUID {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		errs.Add(field, "Invalid identifier.")
		return nil
	}
	return &id
}

// List returns a filtered page of sections.
func (s *SectionService) List(ctx context.Context, in ListSectionsInput) (*ListSectionsResult, error) {
	errs := validation.Errors{}
	f := repository.SectionFilter{
		ClassID:        optionalID(errs, "class_id", in.ClassID),
		AcademicYearID: optionalID(errs, "academic_year_id", in.AcademicYearID),
		Search:         strings.TrimSpace(in.Search),
		SortBy:         in.SortBy,
	}
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
		f.SortBy = repository.SectionSortClass
	case repository.SectionSortClass, repository.SectionSortName, repository.SectionSortCreatedAt, repository.SectionSortYear:
	default:
		errs.Add("sort", "Unsupported sort field.")
	}
	switch strings.ToLower(in.SortOrder) {
	case "":
		f.SortDesc = f.SortBy == repository.SectionSortCreatedAt || f.SortBy == repository.SectionSortYear
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
	sections, total, err := s.sections.List(ctx, f)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return &ListSectionsResult{Sections: sections, Total: total, Page: f.Page, PageSize: f.PageSize}, nil
}

// ForClass lists a class's sections, newest academic year first, optionally
// limited to one academic year.
func (s *SectionService) ForClass(ctx context.Context, classID uuid.UUID, rawYear string) ([]models.ClassSection, error) {
	errs := validation.Errors{}
	yearID := optionalID(errs, "academic_year_id", rawYear)
	if errs.Any() {
		return nil, apperror.Validation(errs)
	}
	if _, err := s.classes.GetByID(ctx, classID); err != nil {
		return nil, mapClassRepoError(err)
	}
	sections, _, err := s.sections.List(ctx, repository.SectionFilter{
		ClassID: &classID, AcademicYearID: yearID, Page: 1, PageSize: classSectionsLimit,
		SortBy: repository.SectionSortYear, SortDesc: true,
	})
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return sections, nil
}

// Get returns one section.
func (s *SectionService) Get(ctx context.Context, id uuid.UUID) (*models.ClassSection, error) {
	sec, err := s.sections.GetByID(ctx, id)
	if err != nil {
		return nil, mapSectionRepoError(err)
	}
	return sec, nil
}

// Create adds a section to an active class in the current or an upcoming
// academic year.
func (s *SectionService) Create(ctx context.Context, actor *models.User, in SectionInput) (*models.ClassSection, error) {
	sec, err := validateSection(in)
	if err != nil {
		return nil, err
	}
	created, err := s.sections.Create(ctx, sec, actor.ID, func(_ *models.ClassSection, class models.ClassRef, year models.YearRef) error {
		if class.Status != models.StatusActive {
			return classInactive(class, "adding sections")
		}
		if year.Status == models.YearCompleted {
			return yearCompleted(year)
		}
		return nil
	})
	if err != nil {
		return nil, mapSectionRepoError(err)
	}
	s.record(ctx, actor, models.ActivitySectionCreated, created, "Added "+sectionLabel(created),
		map[string]any{"class": created.Class.Name, "academic_year": created.AcademicYear.Name})
	return created, nil
}

// Update replaces a section's details. Moving it to another class or academic
// year follows the same rules as creating it there; sections of a completed
// year can still be corrected in place.
func (s *SectionService) Update(ctx context.Context, actor *models.User, id uuid.UUID, in SectionInput) (*models.ClassSection, error) {
	next, err := validateSection(in)
	if err != nil {
		return nil, err
	}
	var before models.ClassSection
	updated, err := s.sections.Update(ctx, id, actor.ID, next, func(cur *models.ClassSection, class models.ClassRef, year models.YearRef) error {
		before = *cur
		moved := class.ID != cur.ClassID || year.ID != cur.AcademicYearID
		if moved && year.Status == models.YearCompleted {
			return yearCompleted(year)
		}
		if class.Status != models.StatusActive && (moved || (next.Status == models.StatusActive && cur.Status != models.StatusActive)) {
			if moved {
				return classInactive(class, "moving sections into it")
			}
			return classInactive(class, "activating its sections")
		}
		return nil
	})
	if err != nil {
		return nil, mapSectionRepoError(err)
	}
	meta := map[string]any{}
	if before.ClassID != updated.ClassID {
		meta["class_from"], meta["class_to"] = before.Class.Name, updated.Class.Name
	}
	if before.AcademicYearID != updated.AcademicYearID {
		meta["academic_year_from"], meta["academic_year_to"] = before.AcademicYear.Name, updated.AcademicYear.Name
	}
	s.record(ctx, actor, models.ActivitySectionUpdated, updated, "Updated "+sectionLabel(updated), meta)
	return updated, nil
}

// SetStatus activates or deactivates a section. A section can't be activated
// while its class is inactive.
func (s *SectionService) SetStatus(ctx context.Context, actor *models.User, id uuid.UUID, raw string) (*models.ClassSection, error) {
	status, ok := models.ParseStatus(raw)
	if !ok || strings.TrimSpace(raw) == "" {
		return nil, apperror.Validation(map[string]string{"status": "Choose active or inactive."})
	}
	updated, err := s.sections.SetStatus(ctx, id, status, actor.ID, func(cur *models.ClassSection, class models.ClassRef, _ models.YearRef) error {
		if status == models.StatusActive && cur.Status != models.StatusActive && class.Status != models.StatusActive {
			return classInactive(class, "activating its sections")
		}
		return nil
	})
	if err != nil {
		return nil, mapSectionRepoError(err)
	}
	verb := "Activated"
	if status == models.StatusInactive {
		verb = "Deactivated"
	}
	s.record(ctx, actor, models.ActivitySectionStatusChanged, updated, fmt.Sprintf("%s %s", verb, sectionLabel(updated)),
		map[string]any{"status_to": status})
	return updated, nil
}

// Delete removes a section that no other record references.
func (s *SectionService) Delete(ctx context.Context, actor *models.User, id uuid.UUID) (*models.ClassSection, error) {
	deleted, err := s.sections.Delete(ctx, id)
	if err != nil {
		return nil, mapSectionRepoError(err)
	}
	s.record(ctx, actor, models.ActivitySectionDeleted, deleted, "Deleted "+sectionLabel(deleted), nil)
	return deleted, nil
}
