package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/apperror"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/models"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/repository"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/validation"
)

const (
	academicYearNameMax        = 50
	academicYearDescriptionMax = 500
	// academicYearMaxYears caps a year's length to catch typos such as 2062.
	academicYearMaxYears = 2
	academicYearMinYear  = 1900
	academicYearMaxYear  = 2200
	humanDate            = "2 Jan 2006"
)

var errAcademicYearNotFound = apperror.NotFound("Academic year not found.")

// AcademicYearInput is the create/update body. Update replaces every field;
// status changes only through the activate and complete actions.
type AcademicYearInput struct {
	Name        string `json:"name"`
	StartDate   string `json:"start_date"`
	EndDate     string `json:"end_date"`
	Description string `json:"description"`
}

// ListAcademicYearsInput holds raw listing parameters.
type ListAcademicYearsInput struct {
	Search    string
	Status    string
	SortBy    string
	SortOrder string
	Page      int
	PageSize  int
}

// ListAcademicYearsResult is a page of academic years.
type ListAcademicYearsResult struct {
	Years    []models.AcademicYear
	Total    int
	Page     int
	PageSize int
}

// AcademicYearService implements academic year management.
type AcademicYearService struct {
	years    repository.AcademicYearRepository
	activity repository.ActivityRepository
	loc      *time.Location
	now      func() time.Time
}

// NewAcademicYearService creates an AcademicYearService. loc is the school's
// timezone, which decides what "today" is; activity may be nil.
func NewAcademicYearService(years repository.AcademicYearRepository, activity repository.ActivityRepository, loc *time.Location) *AcademicYearService {
	if loc == nil {
		loc = time.UTC
	}
	return &AcademicYearService{years: years, activity: activity, loc: loc, now: time.Now}
}

// Today is the current calendar date in the school's timezone (UTC midnight).
func (s *AcademicYearService) Today() time.Time {
	y, m, d := s.now().In(s.loc).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// DTO converts a year relative to today.
func (s *AcademicYearService) DTO(y *models.AcademicYear) models.AcademicYearDTO {
	return y.ToDTO(s.Today())
}

// DTOs converts a slice of years.
func (s *AcademicYearService) DTOs(years []models.AcademicYear) []models.AcademicYearDTO {
	today := s.Today()
	out := make([]models.AcademicYearDTO, len(years))
	for i := range years {
		out[i] = years[i].ToDTO(today)
	}
	return out
}

func blocked(b *models.Blocker) error {
	return apperror.Conflict(b.Code, b.Message, nil)
}

func mapAcademicYearRepoError(err error) error {
	var appErr *apperror.Error
	var overlap *repository.OverlapError
	switch {
	case errors.As(err, &appErr):
		return appErr
	case errors.Is(err, repository.ErrNotFound):
		return errAcademicYearNotFound
	case errors.As(err, &overlap):
		msg := "These dates overlap with another academic year."
		if overlap.Year != nil {
			msg = fmt.Sprintf("These dates overlap with %s (%s – %s).", overlap.Year.Name,
				overlap.Year.StartDate.Format(humanDate), overlap.Year.EndDate.Format(humanDate))
		}
		return apperror.Conflict("ACADEMIC_YEAR_OVERLAP", msg, map[string]string{"start_date": msg, "end_date": msg})
	case errors.Is(err, repository.ErrDuplicateAcademicYearName):
		return apperror.Conflict("ACADEMIC_YEAR_NAME_TAKEN", "An academic year with this name already exists.",
			map[string]string{"name": "An academic year with this name already exists."})
	case errors.Is(err, repository.ErrAcademicYearInUse):
		return apperror.Conflict("ACADEMIC_YEAR_IN_USE",
			"This academic year is used by other records (such as sections or subject assignments), so it can't be deleted.", nil)
	case errors.Is(err, repository.ErrAnotherYearCurrent):
		return apperror.Conflict("ACADEMIC_YEAR_CURRENT_CHANGED",
			"Another academic year was made current at the same time. Refresh and try again.", nil)
	default:
		return apperror.Internal(err)
	}
}

// yearFields are validated, normalised create/update fields.
type yearFields struct {
	name        string
	start, end  time.Time
	description *string
}

func validateAcademicYear(in AcademicYearInput) (*yearFields, error) {
	errs := validation.Errors{}
	f := &yearFields{
		name:        validation.NormalizeName(in.Name),
		description: optional(validation.NormalizeText(in.Description)),
	}
	validation.PersonName(errs, "name", "Academic year name", f.name, academicYearNameMax)
	if f.description != nil {
		validation.Text(errs, "description", "Description", *f.description, academicYearDescriptionMax, false, true)
	}

	start, startOK := validation.Date(errs, "start_date", "Start date", strings.TrimSpace(in.StartDate))
	end, endOK := validation.Date(errs, "end_date", "End date", strings.TrimSpace(in.EndDate))
	if startOK && (start.Year() < academicYearMinYear || start.Year() > academicYearMaxYear) {
		errs.Add("start_date", "Enter a realistic start date.")
		startOK = false
	}
	if endOK && (end.Year() < academicYearMinYear || end.Year() > academicYearMaxYear) {
		errs.Add("end_date", "Enter a realistic end date.")
		endOK = false
	}
	if startOK && endOK {
		switch {
		case !start.Before(end):
			errs.Add("end_date", "End date must be after the start date.")
		case end.After(start.AddDate(academicYearMaxYears, 0, 0)):
			errs.Add("end_date", "An academic year can be at most 2 years long.")
		}
	}
	f.start, f.end = start, end
	if errs.Any() {
		return nil, apperror.Validation(errs)
	}
	return f, nil
}

func (s *AcademicYearService) record(ctx context.Context, actor *models.User, action string, y *models.AcademicYear, summary string, metadata map[string]any) {
	if s.activity == nil {
		return
	}
	if err := s.activity.Record(ctx, models.ActivityEntry{
		ActorID: &actor.ID, Action: action, EntityType: "academic_year", EntityID: &y.ID, Summary: summary, Metadata: metadata,
	}); err != nil {
		slog.Warn("could not record activity", "action", action, "academic_year", y.ID, "error", err)
	}
}

// List returns a filtered page of academic years.
func (s *AcademicYearService) List(ctx context.Context, in ListAcademicYearsInput) (*ListAcademicYearsResult, error) {
	errs := validation.Errors{}
	f := repository.AcademicYearFilter{Search: strings.TrimSpace(in.Search), Page: in.Page, PageSize: in.PageSize, SortBy: in.SortBy}
	if len([]rune(f.Search)) > maxSearchLength {
		errs.Add("search", "Search must be at most 100 characters.")
	}
	if in.Status != "" {
		st, ok := models.ParseAcademicYearStatus(in.Status)
		if !ok {
			errs.Add("status", "Unknown status.")
		}
		f.Status = &st
	}
	switch f.SortBy {
	case "":
		f.SortBy = repository.AcademicYearSortStartDate
	case repository.AcademicYearSortStartDate, repository.AcademicYearSortName, repository.AcademicYearSortCreatedAt:
	default:
		errs.Add("sort", "Unsupported sort field.")
	}
	switch strings.ToLower(in.SortOrder) {
	case "":
		f.SortDesc = f.SortBy != repository.AcademicYearSortName
	case "asc":
	case "desc":
		f.SortDesc = true
	default:
		errs.Add("order", "Order must be asc or desc.")
	}
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 {
		f.PageSize = DefaultPageSize
	}
	if f.PageSize > MaxPageSize {
		f.PageSize = MaxPageSize
	}
	if errs.Any() {
		return nil, apperror.Validation(errs)
	}
	years, total, err := s.years.List(ctx, f)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return &ListAcademicYearsResult{Years: years, Total: total, Page: f.Page, PageSize: f.PageSize}, nil
}

// Stats returns per-status counts and the current year.
func (s *AcademicYearService) Stats(ctx context.Context) (*models.AcademicYearStats, error) {
	counts, current, err := s.years.Stats(ctx)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	stats := &models.AcademicYearStats{
		Upcoming:  counts[models.YearUpcoming],
		Active:    counts[models.YearActive],
		Completed: counts[models.YearCompleted],
	}
	stats.Total = stats.Upcoming + stats.Active + stats.Completed
	if current != nil {
		dto := s.DTO(current)
		stats.Current = &dto
	}
	return stats, nil
}

// Get returns one academic year.
func (s *AcademicYearService) Get(ctx context.Context, id uuid.UUID) (*models.AcademicYear, error) {
	y, err := s.years.GetByID(ctx, id)
	if err != nil {
		return nil, mapAcademicYearRepoError(err)
	}
	return y, nil
}

// Create adds an academic year. It is never activated automatically: a year
// whose dates have already passed is recorded as completed, any other year
// starts as upcoming until an admin makes it current.
func (s *AcademicYearService) Create(ctx context.Context, actor *models.User, in AcademicYearInput) (*models.AcademicYear, error) {
	f, err := validateAcademicYear(in)
	if err != nil {
		return nil, err
	}
	y := &models.AcademicYear{Name: f.name, StartDate: f.start, EndDate: f.end, Description: f.description, Status: models.YearUpcoming}
	if s.Today().After(f.end) {
		y.Status = models.YearCompleted
	}
	created, err := s.years.Create(ctx, y, actor.ID)
	if err != nil {
		return nil, mapAcademicYearRepoError(err)
	}
	s.record(ctx, actor, models.ActivityAcademicYearCreated, created,
		fmt.Sprintf("Added academic year %s", created.Name),
		map[string]any{"status": created.Status, "start_date": created.StartDate.Format(models.DateLayout), "end_date": created.EndDate.Format(models.DateLayout)})
	return created, nil
}

// Update replaces a year's name, dates and description. Dates must stay
// consistent with the status: an upcoming or current year can't be moved to
// end in the past (it should be completed instead).
func (s *AcademicYearService) Update(ctx context.Context, actor *models.User, id uuid.UUID, in AcademicYearInput) (*models.AcademicYear, error) {
	f, err := validateAcademicYear(in)
	if err != nil {
		return nil, err
	}
	today := s.Today()
	var before models.AcademicYear
	updated, err := s.years.Update(ctx, id, actor.ID, func(y *models.AcademicYear) error {
		before = *y
		if !f.end.Equal(y.EndDate) && f.end.Before(today) && y.Status != models.YearCompleted {
			msg := "An upcoming academic year can't end in the past."
			if y.Status == models.YearActive {
				msg = "The current academic year can't end in the past. Mark it completed instead."
			}
			return apperror.Validation(map[string]string{"end_date": msg})
		}
		y.Name, y.StartDate, y.EndDate, y.Description = f.name, f.start, f.end, f.description
		return nil
	})
	if err != nil {
		return nil, mapAcademicYearRepoError(err)
	}
	meta := map[string]any{}
	if before.Name != updated.Name {
		meta["name_from"], meta["name_to"] = before.Name, updated.Name
	}
	if !before.StartDate.Equal(updated.StartDate) || !before.EndDate.Equal(updated.EndDate) {
		meta["dates_changed"] = true
	}
	s.record(ctx, actor, models.ActivityAcademicYearUpdated, updated, fmt.Sprintf("Updated academic year %s", updated.Name), meta)
	return updated, nil
}

// Activate makes a year the current academic year. The previously current
// year, if any, is completed (when it lies before this one) or moved back to
// upcoming (when it lies after it), in the same transaction.
func (s *AcademicYearService) Activate(ctx context.Context, actor *models.User, id uuid.UUID) (*repository.Activation, error) {
	today := s.Today()
	result, err := s.years.Activate(ctx, id, actor.ID, func(y *models.AcademicYear) error {
		if b := y.ActivateBlocker(today); b != nil {
			return blocked(b)
		}
		return nil
	})
	if err != nil {
		return nil, mapAcademicYearRepoError(err)
	}
	meta := map[string]any{}
	if result.Previous != nil {
		meta["previous"], meta["previous_status"] = result.Previous.Name, result.Previous.Status
	}
	s.record(ctx, actor, models.ActivityAcademicYearActivated, result.Activated,
		fmt.Sprintf("Set %s as the current academic year", result.Activated.Name), meta)
	return result, nil
}

// Complete marks a year as completed.
func (s *AcademicYearService) Complete(ctx context.Context, actor *models.User, id uuid.UUID) (*models.AcademicYear, error) {
	today := s.Today()
	var wasCurrent bool
	completed, err := s.years.Complete(ctx, id, actor.ID, func(y *models.AcademicYear) error {
		if b := y.CompleteBlocker(today); b != nil {
			return blocked(b)
		}
		wasCurrent = y.IsCurrent()
		return nil
	})
	if err != nil {
		return nil, mapAcademicYearRepoError(err)
	}
	s.record(ctx, actor, models.ActivityAcademicYearCompleted, completed,
		fmt.Sprintf("Marked academic year %s as completed", completed.Name), map[string]any{"was_current": wasCurrent})
	return completed, nil
}

// Delete removes an academic year that is not current and not referenced.
func (s *AcademicYearService) Delete(ctx context.Context, actor *models.User, id uuid.UUID) (*models.AcademicYear, error) {
	deleted, err := s.years.Delete(ctx, id, func(y *models.AcademicYear) error {
		if b := y.DeleteBlocker(); b != nil {
			return blocked(b)
		}
		return nil
	})
	if err != nil {
		return nil, mapAcademicYearRepoError(err)
	}
	s.record(ctx, actor, models.ActivityAcademicYearDeleted, deleted, fmt.Sprintf("Deleted academic year %s", deleted.Name),
		map[string]any{"status": deleted.Status})
	return deleted, nil
}
