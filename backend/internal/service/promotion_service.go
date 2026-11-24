package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/apperror"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/models"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/repository"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/validation"
)

const maxPromotionStudents = 2000

var errPromotionNotFound = apperror.NotFound("Promotion batch not found.")

// PromotionInput configures a promotion preview or execution.
type PromotionInput struct {
	SourceAcademicYearID string `json:"source_academic_year_id"`
	TargetAcademicYearID string `json:"target_academic_year_id"`
	// EnrollmentIDs are the selected source-year enrollments.
	EnrollmentIDs []string `json:"enrollment_ids"`
	// ClassMappings give each source class its destination class (and
	// optionally a default destination section) for promoted students.
	ClassMappings []ClassMappingInput `json:"class_mappings"`
	// Overrides set a per-student decision, section or roll number.
	Overrides    []PromotionOverride `json:"overrides"`
	RollStrategy string              `json:"roll_strategy"` // keep (default) or sequential
	// EnrollmentDate of the new enrollments; default: the target year's start.
	EnrollmentDate string `json:"enrollment_date"`
	Remarks        string `json:"remarks"`
	// RequestID makes execution idempotent (required to execute).
	RequestID string `json:"request_id"`
}

// ClassMappingInput maps a source class to its destination.
type ClassMappingInput struct {
	SourceClassID   string `json:"source_class_id"`
	TargetClassID   string `json:"target_class_id"`
	TargetSectionID string `json:"target_section_id"` // optional
}

// PromotionOverride is one student's exception.
type PromotionOverride struct {
	EnrollmentID    string `json:"enrollment_id"`
	Decision        string `json:"decision"`          // promote, repeat, complete, exclude
	TargetSectionID string `json:"target_section_id"` // optional
	RollNumber      *int   `json:"roll_number"`       // optional
}

// PromotionIssue is a problem that blocks execution until the admin fixes it.
type PromotionIssue struct {
	EnrollmentID *uuid.UUID `json:"enrollment_id,omitempty"`
	ClassID      *uuid.UUID `json:"class_id,omitempty"`
	Message      string     `json:"message"`
}

// PreviewPlacement is where a student is, or will be.
type PreviewPlacement struct {
	Class       models.ClassRef         `json:"class"`
	Section     models.SectionRef       `json:"section"`
	RollNumber  int                     `json:"roll_number"`
	Status      models.EnrollmentStatus `json:"status,omitempty"`
	RollChanged bool                    `json:"roll_changed,omitempty"`
}

// PreviewItem is one student's planned outcome.
type PreviewItem struct {
	EnrollmentID uuid.UUID         `json:"enrollment_id"`
	Student      models.StudentRef `json:"student"`
	Current      PreviewPlacement  `json:"current"`
	Decision     string            `json:"decision"`
	Outcome      string            `json:"outcome"` // processed, skipped, excluded
	Message      string            `json:"message,omitempty"`
	Target       *PreviewPlacement `json:"target"`
}

// PromotionSummary counts a preview's outcomes.
type PromotionSummary struct {
	Selected int `json:"selected"`
	Eligible int `json:"eligible"` // will be promoted, repeated or completed
	Promote  int `json:"promote"`
	Repeat   int `json:"repeat"`
	Complete int `json:"complete"`
	Excluded int `json:"excluded"`
	Skipped  int `json:"skipped"` // ineligible
}

// DestinationSummary counts students going to one target section.
type DestinationSummary struct {
	Class    models.ClassRef   `json:"class"`
	Section  models.SectionRef `json:"section"`
	Students int               `json:"students"`
	FromRoll int               `json:"from_roll"`
	ToRoll   int               `json:"to_roll"`
}

// PromotionPreview is the dry-run result of a promotion configuration.
type PromotionPreview struct {
	SourceYear     models.YearRef       `json:"source_academic_year"`
	TargetYear     models.YearRef       `json:"target_academic_year"`
	RollStrategy   string               `json:"roll_strategy"`
	EnrollmentDate string               `json:"enrollment_date"`
	Summary        PromotionSummary     `json:"summary"`
	Destinations   []DestinationSummary `json:"destinations"`
	Items          []PreviewItem        `json:"items"`
	Issues         []PromotionIssue     `json:"issues"`
	CanExecute     bool                 `json:"can_execute"`
}

// ExecutePromotionResult is the batch an execution created (or had created).
type ExecutePromotionResult struct {
	Batch *models.PromotionBatch
	// Replayed is true when the request id had already been executed.
	Replayed bool
}

// PromotionService previews and executes promotions.
type PromotionService struct {
	promotions repository.PromotionRepository
	activity   repository.ActivityRepository
}

// NewPromotionService creates a PromotionService; activity may be nil.
func NewPromotionService(promotions repository.PromotionRepository, activity repository.ActivityRepository) *PromotionService {
	return &PromotionService{promotions: promotions, activity: activity}
}

func mapPromotionRepoError(err error) error {
	var appErr *apperror.Error
	switch {
	case errors.As(err, &appErr):
		return appErr
	case errors.Is(err, repository.ErrNotFound):
		return errPromotionNotFound
	case errors.Is(err, repository.ErrSourceYearNotFound):
		return apperror.Validation(map[string]string{"source_academic_year_id": "That academic year doesn't exist."})
	case errors.Is(err, repository.ErrTargetYearNotFound):
		return apperror.Validation(map[string]string{"target_academic_year_id": "That academic year doesn't exist."})
	case errors.Is(err, repository.ErrPromotionConflict), errors.Is(err, repository.ErrEnrollmentConflict):
		return apperror.Conflict("PROMOTION_CONFLICT",
			"Enrollments changed while the promotion was running, so nothing was saved. Preview the promotion again.", nil)
	default:
		return apperror.Internal(err)
	}
}

// promotionRequest is a parsed PromotionInput.
type promotionRequest struct {
	sourceID, targetID uuid.UUID
	ids                []uuid.UUID
	mappings           map[uuid.UUID]classMapping
	overrides          map[uuid.UUID]PromotionOverride
	overrideSections   map[uuid.UUID]uuid.UUID
	rollStrategy       string
	date               *time.Time
	remarks            *string
	requestID          uuid.UUID
}

type classMapping struct {
	targetClass   uuid.UUID
	targetSection *uuid.UUID
}

func parsePromotion(in PromotionInput, execute bool) (*promotionRequest, error) {
	errs := validation.Errors{}
	r := &promotionRequest{
		mappings: map[uuid.UUID]classMapping{}, overrides: map[uuid.UUID]PromotionOverride{}, overrideSections: map[uuid.UUID]uuid.UUID{},
	}
	r.sourceID = parseRequiredID(errs, "source_academic_year_id", in.SourceAcademicYearID, "Choose the current (source) academic year.")
	r.targetID = parseRequiredID(errs, "target_academic_year_id", in.TargetAcademicYearID, "Choose the next (target) academic year.")
	if r.sourceID != uuid.Nil && r.sourceID == r.targetID {
		errs.Add("target_academic_year_id", "The target academic year must be different from the source year.")
	}
	switch {
	case len(in.EnrollmentIDs) == 0:
		errs.Add("enrollment_ids", "Choose at least one student.")
	case len(in.EnrollmentIDs) > maxPromotionStudents:
		errs.Add("enrollment_ids", fmt.Sprintf("Promote at most %d students at a time.", maxPromotionStudents))
	default:
		seen := map[uuid.UUID]bool{}
		for _, raw := range in.EnrollmentIDs {
			id, err := uuid.Parse(strings.TrimSpace(raw))
			if err != nil || seen[id] {
				errs.Add("enrollment_ids", "Contains an invalid or repeated enrollment.")
				break
			}
			seen[id] = true
			r.ids = append(r.ids, id)
		}
	}
	for _, m := range in.ClassMappings {
		src, err1 := uuid.Parse(strings.TrimSpace(m.SourceClassID))
		dst, err2 := uuid.Parse(strings.TrimSpace(m.TargetClassID))
		if err1 != nil || err2 != nil {
			errs.Add("class_mappings", "Each class mapping needs a source and a destination class.")
			break
		}
		if _, dup := r.mappings[src]; dup {
			errs.Add("class_mappings", "Each source class can only be mapped once.")
			break
		}
		cm := classMapping{targetClass: dst}
		if strings.TrimSpace(m.TargetSectionID) != "" {
			sec, err := uuid.Parse(strings.TrimSpace(m.TargetSectionID))
			if err != nil {
				errs.Add("class_mappings", "Contains an invalid section.")
				break
			}
			cm.targetSection = &sec
		}
		r.mappings[src] = cm
	}
	for _, o := range in.Overrides {
		id, err := uuid.Parse(strings.TrimSpace(o.EnrollmentID))
		if err != nil {
			errs.Add("overrides", "Contains an invalid enrollment.")
			break
		}
		if _, dup := r.overrides[id]; dup {
			errs.Add("overrides", "Each student can only have one decision.")
			break
		}
		o.Decision = strings.ToLower(strings.TrimSpace(o.Decision))
		if o.Decision == "" {
			o.Decision = models.DecisionPromote
		}
		switch o.Decision {
		case models.DecisionPromote, models.DecisionRepeat, models.DecisionComplete, models.DecisionExclude:
		default:
			errs.Add("overrides", "Decisions are promote, repeat, complete or exclude.")
		}
		if strings.TrimSpace(o.TargetSectionID) != "" {
			sec, err := uuid.Parse(strings.TrimSpace(o.TargetSectionID))
			if err != nil {
				errs.Add("overrides", "Contains an invalid section.")
				break
			}
			r.overrideSections[id] = sec
		}
		checkRoll(errs, "overrides", o.RollNumber)
		r.overrides[id] = o
	}
	switch strings.ToLower(strings.TrimSpace(in.RollStrategy)) {
	case "", models.RollKeep:
		r.rollStrategy = models.RollKeep
	case models.RollSequential:
		r.rollStrategy = models.RollSequential
	default:
		errs.Add("roll_strategy", "Choose keep or sequential roll numbers.")
	}
	if strings.TrimSpace(in.EnrollmentDate) != "" {
		if d, ok := validation.Date(errs, "enrollment_date", "Enrollment date", strings.TrimSpace(in.EnrollmentDate)); ok {
			r.date = &d
		}
	}
	r.remarks = optional(validation.NormalizeText(in.Remarks))
	if r.remarks != nil {
		validation.Text(errs, "remarks", "Remarks", *r.remarks, enrollRemarksMax, false, true)
	}
	if execute {
		r.requestID = parseRequiredID(errs, "request_id", in.RequestID, "A request id is required to execute a promotion.")
	}
	if errs.Any() {
		return nil, apperror.Validation(errs)
	}
	return r, nil
}

// planResult is a computed promotion.
type planResult struct {
	preview *PromotionPreview
	items   []repository.PromotionPlanItem
	date    time.Time
}

// planPromotion decides every selected student's outcome. It never trusts the
// client: eligibility, destinations and roll numbers are all derived from the
// context read from the database.
func planPromotion(pc *repository.PromotionContext, r *promotionRequest) *planResult {
	pv := &PromotionPreview{SourceYear: pc.Source, TargetYear: pc.Target, RollStrategy: r.rollStrategy,
		Items: []PreviewItem{}, Issues: []PromotionIssue{}, Destinations: []DestinationSummary{}}
	issueSeen := map[string]bool{}
	issue := func(key string, enrollment, class *uuid.UUID, msg string) {
		if issueSeen[key] {
			return
		}
		issueSeen[key] = true
		pv.Issues = append(pv.Issues, PromotionIssue{EnrollmentID: enrollment, ClassID: class, Message: msg})
	}

	// Academic years.
	if pc.Target.Status == models.YearCompleted {
		issue("target-completed", nil, nil, fmt.Sprintf("%s is completed. Choose the current or an upcoming year as the target.", pc.Target.Name))
	}
	if pc.Target.StartDate <= pc.Source.StartDate {
		issue("target-order", nil, nil, fmt.Sprintf("%s starts before %s. Promote into a later academic year.", pc.Target.Name, pc.Source.Name))
	}
	date := parseDate(pc.Target.StartDate)
	if r.date != nil {
		date = *r.date
		if err := checkEnrollmentDate(date, pc.Target); err != nil {
			var appErr *apperror.Error
			if errors.As(err, &appErr) {
				issue("date", nil, nil, appErr.Details["enrollment_date"])
			}
		}
	}
	pv.EnrollmentDate = date.Format(models.DateLayout)

	missing := 0
	for _, id := range r.ids {
		if _, ok := pc.Enrollments[id]; !ok {
			missing++
		}
	}
	if missing > 0 {
		verb := "are"
		if missing == 1 {
			verb = "is"
		}
		issue("missing", nil, nil, fmt.Sprintf("%d selected enrollment%s %s not in %s any more. Reload the student list.",
			missing, plural(missing), verb, pc.Source.Name))
	}

	// Target sections by class and by id.
	sectionsByClass := map[uuid.UUID][]repository.TargetSection{}
	sectionByID := map[uuid.UUID]repository.TargetSection{}
	for _, s := range pc.TargetSections {
		sectionsByClass[s.ClassID] = append(sectionsByClass[s.ClassID], s)
		sectionByID[s.ID] = s
	}

	// Students in class order, then section, then roll.
	selected := make([]models.Enrollment, 0, len(pc.Enrollments))
	for _, e := range pc.Enrollments {
		selected = append(selected, e)
	}
	sort.Slice(selected, func(i, j int) bool {
		a, b := selected[i], selected[j]
		if a.Class.DisplayOrder != b.Class.DisplayOrder {
			return a.Class.DisplayOrder < b.Class.DisplayOrder
		}
		if !strings.EqualFold(a.Class.Name, b.Class.Name) {
			return strings.ToLower(a.Class.Name) < strings.ToLower(b.Class.Name)
		}
		if !strings.EqualFold(a.Section.Name, b.Section.Name) {
			return strings.ToLower(a.Section.Name) < strings.ToLower(b.Section.Name)
		}
		if a.RollNumber != b.RollNumber {
			return a.RollNumber < b.RollNumber
		}
		return a.ID.String() < b.ID.String()
	})

	// pickSection finds the destination section in a target class.
	pickSection := func(e models.Enrollment, class models.ClassRef, explicit *uuid.UUID) (*repository.TargetSection, bool) {
		if explicit != nil {
			s, ok := sectionByID[*explicit]
			if !ok {
				issue("section:"+explicit.String(), &e.ID, nil, fmt.Sprintf("A chosen section isn't part of %s.", pc.Target.Name))
				return nil, false
			}
			if s.ClassID != class.ID {
				issue("section-class:"+explicit.String()+class.ID.String(), &e.ID, &class.ID,
					fmt.Sprintf("Section %s belongs to %s, not %s.", s.Name, s.Class.Name, class.Name))
				return nil, false
			}
			return &s, true
		}
		options := sectionsByClass[class.ID]
		if len(options) == 0 {
			issue("no-sections:"+class.ID.String(), nil, &class.ID,
				fmt.Sprintf("%s has no sections in %s. Add sections for it first.", class.Name, pc.Target.Name))
			return nil, false
		}
		for i := range options {
			if strings.EqualFold(options[i].Name, e.Section.Name) {
				return &options[i], true
			}
		}
		if len(options) == 1 {
			return &options[0], true
		}
		issue("choose-section:"+class.ID.String()+":"+strings.ToLower(e.Section.Name), nil, &class.ID,
			fmt.Sprintf("Choose a section of %s in %s for students from %s (there is no Section %s).",
				class.Name, pc.Target.Name, placementLabel(e.Class, e.Section), e.Section.Name))
		return nil, false
	}

	type pending struct {
		idx     int
		section repository.TargetSection
		roll    *int // explicit override
	}
	var destined []pending
	plan := make([]repository.PromotionPlanItem, 0, len(selected))

	for _, e := range selected {
		o, hasOverride := r.overrides[e.ID]
		decision := models.DecisionPromote
		if hasOverride {
			decision = o.Decision
		}
		item := PreviewItem{EnrollmentID: e.ID, Student: e.Student, Decision: decision, Outcome: models.ItemProcessed,
			Current: PreviewPlacement{Class: e.Class, Section: e.Section, RollNumber: e.RollNumber, Status: e.Status}}
		existing, inTarget := pc.TargetEnrollments[e.StudentID]
		switch {
		case decision == models.DecisionExclude:
			item.Outcome, item.Message = models.ItemExcluded, "Excluded from this promotion."
		case e.Status != models.EnrollmentEnrolled:
			item.Outcome, item.Message = models.ItemSkipped, fmt.Sprintf("Not eligible: this enrollment is %s.", e.Status)
		case inTarget:
			item.Outcome, item.Message = models.ItemSkipped, fmt.Sprintf("Already enrolled in %s (%s).",
				pc.Target.Name, placementLabel(existing.Class, existing.Section))
		case decision != models.DecisionComplete && e.Student.Status != models.StatusActive:
			item.Outcome, item.Message = models.ItemSkipped, "Not eligible: the student record is inactive."
		case decision == models.DecisionComplete:
			item.Message = "Studies completed; no enrollment in " + pc.Target.Name + "."
		default:
			var class models.ClassRef
			var explicit *uuid.UUID
			if sec, ok := r.overrideSections[e.ID]; ok {
				explicit = &sec
			}
			ok := true
			if decision == models.DecisionRepeat {
				class = e.Class
			} else if explicit != nil {
				if s, found := sectionByID[*explicit]; found {
					class = pc.Classes[s.ClassID]
				} else {
					issue("section:"+explicit.String(), &e.ID, nil, fmt.Sprintf("A chosen section isn't part of %s.", pc.Target.Name))
					ok = false
				}
			} else if m, mapped := r.mappings[e.Class.ID]; !mapped {
				issue("mapping:"+e.Class.ID.String(), nil, &e.Class.ID, fmt.Sprintf("Choose a destination class for %s.", e.Class.Name))
				ok = false
			} else if c, exists := pc.Classes[m.targetClass]; !exists {
				issue("mapping-class:"+e.Class.ID.String(), nil, &e.Class.ID, fmt.Sprintf("The destination class for %s doesn't exist.", e.Class.Name))
				ok = false
			} else {
				class, explicit = c, m.targetSection
			}
			if ok && class.Status != models.StatusActive {
				issue("class-inactive:"+class.ID.String(), nil, &class.ID, fmt.Sprintf("%s is inactive. Activate it or choose another class.", class.Name))
				ok = false
			}
			var sec *repository.TargetSection
			if ok {
				sec, ok = pickSection(e, class, explicit)
			}
			if ok && sec.Status != models.StatusActive {
				issue("section-inactive:"+sec.ID.String(), nil, &class.ID,
					fmt.Sprintf("%s is inactive in %s.", placementLabel(sec.Class, sec.SectionRef), pc.Target.Name))
				ok = false
			}
			if ok {
				item.Target = &PreviewPlacement{Class: sec.Class, Section: sec.SectionRef}
				destined = append(destined, pending{idx: len(pv.Items), section: *sec, roll: o.RollNumber})
			}
		}
		pv.Items = append(pv.Items, item)
		plan = append(plan, repository.PromotionPlanItem{Source: e, Decision: decision, Status: item.Outcome, Message: item.Message})
	}

	// Roll numbers per destination section: explicit overrides first, then
	// current rolls (keep) where free, then the smallest free numbers.
	taken := map[uuid.UUID]map[int]bool{}
	takenIn := func(sec uuid.UUID) map[int]bool {
		if taken[sec] == nil {
			taken[sec] = map[int]bool{}
			for n := range pc.TakenRolls[sec] {
				taken[sec][n] = true
			}
		}
		return taken[sec]
	}
	assign := func(p pending, n int) {
		t := pv.Items[p.idx].Target
		t.RollNumber = n
		takenIn(p.section.ID)[n] = true
	}
	var rest []pending
	for _, p := range destined {
		if p.roll == nil {
			rest = append(rest, p)
			continue
		}
		if takenIn(p.section.ID)[*p.roll] {
			issue(fmt.Sprintf("roll:%s:%d", p.section.ID, *p.roll), &pv.Items[p.idx].EnrollmentID, nil,
				fmt.Sprintf("Roll %d is already used in %s (%s).", *p.roll, placementLabel(p.section.Class, p.section.SectionRef), pc.Target.Name))
			continue
		}
		assign(p, *p.roll)
	}
	var fallback []pending
	if r.rollStrategy == models.RollKeep {
		for _, p := range rest {
			cur := pv.Items[p.idx].Current.RollNumber
			if !takenIn(p.section.ID)[cur] {
				assign(p, cur)
			} else {
				fallback = append(fallback, p)
			}
		}
	} else {
		fallback = rest
	}
	for _, p := range fallback {
		n := nextFreeRoll(takenIn(p.section.ID), 1)
		if n == 0 {
			issue("rolls-full:"+p.section.ID.String(), nil, nil,
				fmt.Sprintf("No roll numbers are free in %s.", placementLabel(p.section.Class, p.section.SectionRef)))
			continue
		}
		assign(p, n)
		if r.rollStrategy == models.RollKeep {
			pv.Items[p.idx].Target.RollChanged = true
			pv.Items[p.idx].Message = fmt.Sprintf("Roll %d is taken in the new section; assigned %d.", pv.Items[p.idx].Current.RollNumber, n)
		}
	}

	// Summary, destinations and the plan to apply.
	dest := map[uuid.UUID]*DestinationSummary{}
	var destOrder []uuid.UUID
	for i := range pv.Items {
		it := &pv.Items[i]
		plan[i].Message = it.Message // includes roll-number notes made above
		pv.Summary.Selected++
		switch it.Outcome {
		case models.ItemExcluded:
			pv.Summary.Excluded++
			continue
		case models.ItemSkipped:
			pv.Summary.Skipped++
			continue
		}
		pv.Summary.Eligible++
		switch it.Decision {
		case models.DecisionPromote:
			pv.Summary.Promote++
		case models.DecisionRepeat:
			pv.Summary.Repeat++
		case models.DecisionComplete:
			pv.Summary.Complete++
		}
		if it.Target == nil || it.Target.RollNumber == 0 {
			continue
		}
		t := it.Target
		d, ok := dest[t.Section.ID]
		if !ok {
			d = &DestinationSummary{Class: t.Class, Section: t.Section, FromRoll: t.RollNumber, ToRoll: t.RollNumber}
			dest[t.Section.ID] = d
			destOrder = append(destOrder, t.Section.ID)
		}
		d.Students++
		d.FromRoll, d.ToRoll = min(d.FromRoll, t.RollNumber), max(d.ToRoll, t.RollNumber)
		classID, sectionID, roll := t.Class.ID, t.Section.ID, t.RollNumber
		plan[i].TargetClassID, plan[i].TargetSectionID, plan[i].TargetRoll = &classID, &sectionID, &roll
	}
	for _, id := range destOrder {
		pv.Destinations = append(pv.Destinations, *dest[id])
	}
	sort.SliceStable(pv.Destinations, func(i, j int) bool {
		a, b := pv.Destinations[i], pv.Destinations[j]
		if a.Class.DisplayOrder != b.Class.DisplayOrder {
			return a.Class.DisplayOrder < b.Class.DisplayOrder
		}
		return strings.ToLower(a.Section.Name) < strings.ToLower(b.Section.Name)
	})
	if pv.Summary.Eligible == 0 && len(pv.Items) > 0 {
		issue("nothing", nil, nil, "Nothing to do: every selected student is excluded or not eligible.")
	}
	pv.CanExecute = len(pv.Issues) == 0 && pv.Summary.Eligible > 0
	return &planResult{preview: pv, items: plan, date: date}
}

// Preview computes a promotion without saving anything.
func (s *PromotionService) Preview(ctx context.Context, in PromotionInput) (*PromotionPreview, error) {
	r, err := parsePromotion(in, false)
	if err != nil {
		return nil, err
	}
	pc, err := s.promotions.Context(ctx, r.sourceID, r.targetID, r.ids)
	if err != nil {
		return nil, mapPromotionRepoError(err)
	}
	return planPromotion(pc, r).preview, nil
}

// Execute re-validates the promotion under lock and applies it in one
// transaction. Any blocking issue aborts with nothing saved. Re-submitting the
// same request id returns the batch it created.
func (s *PromotionService) Execute(ctx context.Context, actor *models.User, in PromotionInput) (*ExecutePromotionResult, error) {
	r, err := parsePromotion(in, true)
	if err != nil {
		return nil, err
	}
	batch, replayed, err := s.promotions.Execute(ctx, r.sourceID, r.targetID, r.ids, r.requestID, actor.ID,
		func(pc *repository.PromotionContext) (*repository.PromotionPlan, error) {
			res := planPromotion(pc, r)
			if !res.preview.CanExecute {
				msgs := make([]string, 0, 3)
				for i, is := range res.preview.Issues {
					if i == 3 {
						break
					}
					msgs = append(msgs, is.Message)
				}
				msg := "The promotion can't run: " + strings.Join(msgs, " ")
				if n := len(res.preview.Issues); n > 3 {
					msg += fmt.Sprintf(" (and %d more)", n-3)
				}
				return nil, &apperror.Error{Status: http.StatusUnprocessableEntity, Code: "PROMOTION_INVALID", Message: msg}
			}
			return &repository.PromotionPlan{RequestID: r.requestID, RollStrategy: r.rollStrategy, EnrollmentDate: res.date,
				Remarks: r.remarks, Items: res.items}, nil
		})
	if err != nil {
		return nil, mapPromotionRepoError(err)
	}
	if !replayed && s.activity != nil {
		summary := fmt.Sprintf("Promotion %s → %s: %d promoted, %d repeating, %d completed",
			batch.SourceYear.Name, batch.TargetYear.Name, batch.PromotedCount, batch.RepeatedCount, batch.CompletedCount)
		if err := s.activity.Record(ctx, models.ActivityEntry{
			ActorID: &actor.ID, Action: models.ActivityPromotionExecuted, EntityType: "promotion", EntityID: &batch.ID, Summary: summary,
			Metadata: map[string]any{"excluded": batch.ExcludedCount, "skipped": batch.SkippedCount, "total": batch.TotalStudents},
		}); err != nil {
			slog.Warn("could not record activity", "action", models.ActivityPromotionExecuted, "error", err)
		}
	}
	return &ExecutePromotionResult{Batch: batch, Replayed: replayed}, nil
}

// List returns a page of promotion batches, newest first.
func (s *PromotionService) List(ctx context.Context, rawYear string, page, size int) ([]models.PromotionBatch, int, int, int, error) {
	errs := validation.Errors{}
	year := optionalID(errs, "academic_year_id", rawYear)
	if errs.Any() {
		return nil, 0, 0, 0, apperror.Validation(errs)
	}
	page, size = normalizePaging(page, size)
	items, total, err := s.promotions.List(ctx, repository.PromotionFilter{AcademicYearID: year, Page: page, PageSize: size})
	if err != nil {
		return nil, 0, 0, 0, apperror.Internal(err)
	}
	return items, total, page, size, nil
}

// Get returns one batch with every student's outcome.
func (s *PromotionService) Get(ctx context.Context, id uuid.UUID) (*models.PromotionBatch, error) {
	b, err := s.promotions.Get(ctx, id)
	if err != nil {
		return nil, mapPromotionRepoError(err)
	}
	return b, nil
}
