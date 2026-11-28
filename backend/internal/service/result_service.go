package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strings"

	"github.com/google/uuid"

	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/apperror"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/models"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/repository"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/validation"
)

const (
	componentNameMax  = 40
	maxComponents     = 10
	maxMarksRecords   = 500
	resultReasonMax   = 500
	maxReadinessItems = 20
)

var (
	errPaperNotFound  = apperror.NotFound("Examination paper not found.")
	errResultNotFound = apperror.NotFound("Result not found.")
)

// ComponentInput is one component of a marking scheme.
type ComponentInput struct {
	ID       string      `json:"id"` // set to keep an existing component
	Name     string      `json:"name"`
	MaxMarks json.Number `json:"max_marks"`
}

// SchemeInput is the body of PUT /examination-schedules/:id/mark-configuration.
type SchemeInput struct {
	FullMarks    json.Number      `json:"full_marks"`
	PassMarks    json.Number      `json:"pass_marks"`
	Instructions string           `json:"instructions"`
	Components   []ComponentInput `json:"components"`
	// Reason is required once marks have been entered.
	Reason string `json:"reason"`
}

// ComponentMarkInput is a student's marks in one component.
type ComponentMarkInput struct {
	ComponentID string      `json:"component_id"`
	Obtained    json.Number `json:"obtained"`
}

// MarkInput is one student's marks. Totals sent by clients are ignored: the
// server adds up the components itself.
type MarkInput struct {
	EnrollmentID string               `json:"enrollment_id"`
	Attendance   string               `json:"attendance"` // present (default) or absent
	Total        json.Number          `json:"total"`      // only for papers without components
	Components   []ComponentMarkInput `json:"components"`
	Version      int                  `json:"version"` // the record's version when loaded; 0 if none
}

// MarksInput is the body of POST /examination-schedules/:id/marks/bulk.
type MarksInput struct {
	SectionID string      `json:"section_id"`
	Records   []MarkInput `json:"records"`
	Reason    string      `json:"reason"`
}

// SingleMarkInput is the body of POST /examination-schedules/:id/marks.
type SingleMarkInput struct {
	SectionID string `json:"section_id"`
	MarkInput
}

// UpdateMarkInput is the body of PUT /examination-marks/:id.
type UpdateMarkInput struct {
	Attendance string               `json:"attendance"`
	Total      json.Number          `json:"total"`
	Components []ComponentMarkInput `json:"components"`
	Version    int                  `json:"version"`
	Reason     string               `json:"reason"`
}

// SheetActionInput is the body of the submit/verify/finalize/reopen endpoints.
type SheetActionInput struct {
	SectionID string `json:"section_id"`
	Reason    string `json:"reason"`
}

// ResultReasonInput carries a reason (result corrections).
type ResultReasonInput struct {
	Reason string `json:"reason"`
}

// SheetView is a mark sheet as the caller sees it.
type SheetView struct {
	Sheet   models.MarkSheet      `json:"sheet"`
	Scheme  models.MarkScheme     `json:"scheme"`
	Rows    []models.MarkEntryRow `json:"students"`
	Stage   string                `json:"result_stage"`
	CanEdit bool                  `json:"can_edit"`
	// CanSubmit / CanVerify / CanFinalize / CanReopen tell the client which
	// actions the caller may take now.
	CanSubmit   bool   `json:"can_submit"`
	CanVerify   bool   `json:"can_verify"`
	CanFinalize bool   `json:"can_finalize"`
	CanReopen   bool   `json:"can_reopen"`
	LockReason  string `json:"lock_reason,omitempty"`
}

// SaveMarksResult reports a marks save.
type SaveMarksResult struct {
	Sheet     *SheetView `json:"sheet"`
	Saved     int        `json:"saved"`
	Unchanged int        `json:"unchanged"`
}

// PaperSchemeView is a paper with its marking scheme and sheets summary.
type PaperSchemeView struct {
	Paper  models.PaperRef   `json:"paper"`
	Scheme models.MarkScheme `json:"scheme"`
}

// ReadinessIssue is one reason results can't be processed or published yet.
type ReadinessIssue struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ResultStatus is an examination's result processing state and readiness.
type ResultStatus struct {
	Examination  models.ExamRef             `json:"examination"`
	AcademicYear models.YearRef             `json:"academic_year"`
	Run          models.ResultRun           `json:"run"`
	Stats        models.ResultStats         `json:"stats"`
	Issues       []ReadinessIssue           `json:"issues"` // blocking processing
	Publications []models.ResultPublication `json:"publications"`
	// Next is the action that moves the results forward (empty when none).
	Next string `json:"next"`
}

// MeritEntry is one row of a merit list.
type MeritEntry struct {
	Position *int              `json:"position"`
	Result   models.ExamResult `json:"result"`
}

// ReportCard is one student's result in an examination.
type ReportCard struct {
	Examination models.Examination `json:"examination"`
	Result      models.ExamResult  `json:"result"`
	Published   bool               `json:"published"`
}

// ResultService manages marking schemes, marks, results and publication.
type ResultService struct {
	repo     repository.ResultRepository
	exams    repository.ExaminationRepository
	activity repository.ActivityRepository
}

// NewResultService creates a ResultService; activity may be nil.
func NewResultService(repo repository.ResultRepository, exams repository.ExaminationRepository, activity repository.ActivityRepository) *ResultService {
	return &ResultService{repo: repo, exams: exams, activity: activity}
}

func mapResultRepoError(err error) error {
	var appErr *apperror.Error
	switch {
	case errors.As(err, &appErr):
		return appErr
	case errors.Is(err, repository.ErrNotFound):
		return errPaperNotFound
	case errors.Is(err, repository.ErrSectionMismatch):
		return apperror.Validation(map[string]string{"section_id": "That section isn't part of this paper's class."})
	case errors.Is(err, repository.ErrMarksConflict):
		return apperror.Conflict("MARKS_CHANGED",
			"These marks were changed by someone else at the same time. Nothing was saved; reload and try again.", nil)
	case errors.Is(err, repository.ErrMarksInvalid):
		return apperror.Conflict("MARKS_INVALID", "The database rejected these marks: "+strings.TrimPrefix(err.Error(), repository.ErrMarksInvalid.Error()+": "), nil)
	default:
		return apperror.Internal(err)
	}
}

func (s *ResultService) record(ctx context.Context, actor *models.User, action string, examID uuid.UUID, summary string, meta map[string]any) {
	if s.activity == nil {
		return
	}
	if err := s.activity.Record(ctx, models.ActivityEntry{
		ActorID: &actor.ID, Action: action, EntityType: "examination", EntityID: &examID, Summary: summary, Metadata: meta,
	}); err != nil {
		slog.Warn("could not record activity", "action", action, "error", err)
	}
}

func paperLabel(p models.PaperRef) string {
	return fmt.Sprintf("%s – %s (%s)", p.Class.Name, p.Subject.Name, p.Examination.Name)
}

// ---- callers ----

type paperKey struct{ SectionID, SubjectID uuid.UUID }

type resultCaller struct {
	user   *models.User
	admin  bool
	papers map[paperKey]bool
	note   string
}

func (s *ResultService) caller(ctx context.Context, u *models.User) (*resultCaller, error) {
	c := &resultCaller{user: u, admin: u.Role == models.RoleAdmin, papers: map[paperKey]bool{}}
	if c.admin {
		return c, nil
	}
	t, papers, err := s.repo.TeacherPapers(ctx, u.ID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	switch {
	case t == nil:
		c.note = "Your account isn't linked to a teacher record yet. Ask an administrator to link it."
	case t.Status != models.StatusActive:
		c.note = "Your teacher record is inactive."
	case len(papers) == 0:
		c.note = "You aren't assigned as a subject teacher yet, so there are no marks to enter."
	}
	for _, p := range papers {
		c.papers[paperKey{p.SectionID, p.SubjectID}] = true
	}
	return c, nil
}

func (c *resultCaller) mayEnter(subjectID, sectionID uuid.UUID) bool {
	return c.admin || c.papers[paperKey{sectionID, subjectID}]
}

func notYourPaper() error {
	return &apperror.Error{Status: http.StatusForbidden, Code: "PAPER_NOT_ASSIGNED",
		Message: "You can only enter marks for the subjects and sections you teach."}
}

func examNotOpen(p models.PaperRef) error {
	switch p.Examination.Status {
	case models.ExamDraft:
		return apperror.Conflict("EXAM_NOT_PUBLISHED", fmt.Sprintf("%s isn't published yet. Marks can be entered once its timetable is published.", p.Examination.Name), nil)
	case models.ExamCancelled:
		return apperror.Conflict("EXAM_CANCELLED", fmt.Sprintf("%s was cancelled, so it has no marks.", p.Examination.Name), nil)
	}
	return nil
}

func resultsPublished(name string) error {
	return apperror.Conflict("RESULTS_PUBLISHED",
		fmt.Sprintf("The results of %s are published. Start a correction to change anything.", name), nil)
}

func parseMarksField(errs validation.Errors, field, label string, raw json.Number, required bool) (models.Marks, bool) {
	str := strings.TrimSpace(string(raw))
	if str == "" {
		if required {
			errs.Add(field, label+" is required.")
		}
		return 0, false
	}
	m, err := models.ParseMarks(str)
	switch {
	case errors.Is(err, models.ErrMarksNegative):
		errs.Add(field, label+" can't be negative.")
	case errors.Is(err, models.ErrMarksTooLarge):
		errs.Add(field, label+" is too large.")
	case err != nil:
		errs.Add(field, label+" must be a number with at most two decimals.")
	default:
		return m, true
	}
	return 0, false
}

// ---- marking schemes ----

// Schemes lists an examination's papers with their schemes (or defaults).
func (s *ResultService) Schemes(ctx context.Context, examID uuid.UUID) ([]PaperSchemeView, error) {
	if _, err := s.exams.GetByID(ctx, examID); err != nil {
		return nil, mapExamRepoError(err)
	}
	papers, err := s.repo.Schemes(ctx, examID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	out := []PaperSchemeView{}
	for _, p := range papers {
		sch := models.MarkScheme{}
		if p.Scheme != nil {
			sch = *p.Scheme
		} else {
			_, d, err := s.repo.Scheme(ctx, p.Paper.ScheduleID)
			if err != nil {
				return nil, apperror.Internal(err)
			}
			sch = *d
		}
		out = append(out, PaperSchemeView{Paper: p.Paper, Scheme: sch})
	}
	return out, nil
}

// Scheme returns one paper's scheme (saved, or the subject's defaults).
func (s *ResultService) Scheme(ctx context.Context, actor *models.User, scheduleID uuid.UUID) (*PaperSchemeView, error) {
	p, sch, err := s.repo.Scheme(ctx, scheduleID)
	if err != nil {
		return nil, mapResultRepoError(err)
	}
	if actor.Role != models.RoleAdmin {
		c, err := s.caller(ctx, actor)
		if err != nil {
			return nil, err
		}
		allowed := false
		for k := range c.papers {
			if k.SubjectID == p.Subject.ID {
				allowed = true
			}
		}
		if !allowed || p.Examination.Status == models.ExamDraft {
			return nil, errPaperNotFound
		}
	}
	return &PaperSchemeView{Paper: *p, Scheme: *sch}, nil
}

type schemePlan struct {
	full, pass   models.Marks
	instructions *string
	components   []repository.ComponentWrite
	reason       *string
}

func parseScheme(in SchemeInput) (*schemePlan, error) {
	errs := validation.Errors{}
	p := &schemePlan{}
	var okFull, okPass bool
	p.full, okFull = parseMarksField(errs, "full_marks", "Full marks", in.FullMarks, true)
	p.pass, okPass = parseMarksField(errs, "pass_marks", "Pass marks", in.PassMarks, true)
	if okFull && p.full == 0 {
		errs.Add("full_marks", "Full marks must be more than 0.")
	}
	if okFull && okPass && p.pass > p.full {
		errs.Add("pass_marks", "Pass marks can't exceed the full marks.")
	}
	p.instructions = optional(validation.NormalizeText(in.Instructions))
	if p.instructions != nil {
		validation.Text(errs, "instructions", "Instructions", *p.instructions, 500, false, true)
	}
	p.reason = optional(validation.NormalizeText(in.Reason))
	if p.reason != nil {
		validation.Text(errs, "reason", "Reason", *p.reason, resultReasonMax, false, true)
	}
	if len(in.Components) > maxComponents {
		errs.Add("components", fmt.Sprintf("A paper can have at most %d components.", maxComponents))
	}
	names := map[string]bool{}
	var sum models.Marks
	for i, c := range in.Components {
		prefix := fmt.Sprintf("components[%d].", i)
		cw := repository.ComponentWrite{Name: validation.NormalizeName(c.Name), Order: i + 1}
		validation.Text(errs, prefix+"name", "Component name", cw.Name, componentNameMax, true, false)
		if names[strings.ToLower(cw.Name)] && cw.Name != "" {
			errs.Add(prefix+"name", "Each component needs a different name.")
		}
		names[strings.ToLower(cw.Name)] = true
		if max, ok := parseMarksField(errs, prefix+"max_marks", "Maximum marks", c.MaxMarks, true); ok {
			if max == 0 {
				errs.Add(prefix+"max_marks", "Maximum marks must be more than 0.")
			}
			cw.Max = max
			sum += max
		}
		if strings.TrimSpace(c.ID) != "" {
			id, err := uuid.Parse(strings.TrimSpace(c.ID))
			if err != nil {
				errs.Add(prefix+"id", "Invalid component.")
			}
			cw.ID = &id
		}
		p.components = append(p.components, cw)
	}
	if len(in.Components) > 0 && okFull && !errs.Any() && sum != p.full {
		errs.Add("components", fmt.Sprintf("The components add up to %s but the full marks are %s. They must be equal.", sum, p.full))
	}
	if errs.Any() {
		return nil, apperror.Validation(errs)
	}
	return p, nil
}

// planScheme decides whether a scheme change is allowed. Once marks exist
// the change needs a reason, every sheet must be back in draft, components
// can't be added or removed, and maximums can't drop below marks already given.
func planScheme(sc *repository.SchemeContext, p *schemePlan) (*repository.SchemeWrite, error) {
	if err := examNotOpen(sc.Paper); err != nil && sc.Paper.Examination.Status == models.ExamCancelled {
		return nil, err
	}
	if sc.Stage == models.StagePublished {
		return nil, resultsPublished(sc.Paper.Examination.Name)
	}
	current := map[uuid.UUID]models.MarkComponent{}
	for _, c := range sc.Scheme.Components {
		current[c.ID] = c
	}
	for _, c := range p.components {
		if c.ID != nil {
			if _, ok := current[*c.ID]; !ok || !sc.Scheme.Saved {
				return nil, apperror.Validation(map[string]string{"components": "A component doesn't belong to this paper's scheme."})
			}
		}
	}
	if sc.MarksCount > 0 {
		if p.reason == nil {
			msg := "Marks have already been entered for this paper. Give a reason for changing its marking scheme."
			return nil, &apperror.Error{Status: http.StatusUnprocessableEntity, Code: "REASON_REQUIRED", Message: msg, Details: map[string]string{"reason": msg}}
		}
		if sc.SheetsBeyondDraft > 0 {
			return nil, apperror.Conflict("SCHEME_LOCKED", "Marks for this paper have been submitted. Reopen every section's marks before changing the scheme.", nil)
		}
		kept := 0
		for _, c := range p.components {
			if c.ID == nil {
				return nil, apperror.Conflict("SCHEME_LOCKED", "Marks have been entered, so components can't be added or removed — only renamed or resized.", nil)
			}
			kept++
			if used, ok := sc.MaxByComponent[*c.ID]; ok && c.Max < used {
				msg := fmt.Sprintf("%s already has marks up to %s; its maximum can't be lower.", c.Name, used)
				return nil, apperror.Conflict("SCHEME_LOCKED", msg, map[string]string{"components": msg})
			}
		}
		if kept != len(sc.Scheme.Components) {
			return nil, apperror.Conflict("SCHEME_LOCKED", "Marks have been entered, so components can't be added or removed — only renamed or resized.", nil)
		}
		if p.full < sc.MaxTotal {
			msg := fmt.Sprintf("A student already has %s marks; the full marks can't be lower.", sc.MaxTotal)
			return nil, apperror.Conflict("SCHEME_LOCKED", msg, map[string]string{"full_marks": msg})
		}
	}
	return &repository.SchemeWrite{Full: p.full, Pass: p.pass, Instructions: p.instructions, Components: p.components}, nil
}

// SaveScheme saves a paper's marking scheme (administrators).
func (s *ResultService) SaveScheme(ctx context.Context, actor *models.User, scheduleID uuid.UUID, in SchemeInput) (*PaperSchemeView, error) {
	p, err := parseScheme(in)
	if err != nil {
		return nil, err
	}
	var paper models.PaperRef
	var hadMarks int
	saved, err := s.repo.SaveScheme(ctx, scheduleID, actor.ID, func(sc *repository.SchemeContext) (*repository.SchemeWrite, error) {
		paper, hadMarks = sc.Paper, sc.MarksCount
		return planScheme(sc, p)
	})
	if err != nil {
		return nil, mapResultRepoError(err)
	}
	comps := []string{}
	for _, c := range saved.Components {
		comps = append(comps, fmt.Sprintf("%s %s", c.Name, c.MaxMarks))
	}
	meta := map[string]any{"full_marks": saved.FullMarks.String(), "pass_marks": saved.PassMarks.String(), "components": comps, "version": saved.Version}
	if p.reason != nil {
		meta["reason"] = *p.reason
	}
	if hadMarks > 0 {
		meta["marks_entered"] = hadMarks
	}
	s.record(ctx, actor, models.ActivityMarksSchemeSaved, paper.Examination.ID, "Saved the marking scheme of "+paperLabel(paper), meta)
	return &PaperSchemeView{Paper: paper, Scheme: *saved}, nil
}

// ---- mark sheets ----

func (s *ResultService) view(c *resultCaller, sc *repository.SheetContext) *SheetView {
	v := &SheetView{Sheet: sc.Sheet, Scheme: sc.Scheme, Rows: sc.Rows, Stage: sc.Stage}
	if v.Rows == nil {
		v.Rows = []models.MarkEntryRow{}
	}
	if !c.admin {
		for i := range v.Rows {
			v.Rows[i].Student.PhotoURL = nil // photos are served to administrators only
		}
	}
	open := examNotOpen(sc.Paper) == nil && sc.Stage != models.StagePublished
	mine := c.mayEnter(sc.Paper.Subject.ID, sc.Sheet.Section.ID)
	st := sc.Sheet.Status
	v.CanEdit = open && mine && st == models.SheetDraft
	v.CanSubmit = v.CanEdit && sc.Sheet.Marked > 0
	v.CanVerify = open && c.admin && st == models.SheetSubmitted
	v.CanFinalize = open && c.admin && st == models.SheetVerified
	v.CanReopen = c.admin && sc.Stage != models.StagePublished && st != models.SheetDraft
	switch {
	case !mine:
		v.LockReason = "You don't teach this subject in this section."
	case sc.Stage == models.StagePublished:
		v.LockReason = "The results are published. An administrator must start a correction to change marks."
	case examNotOpen(sc.Paper) != nil:
		var appErr *apperror.Error
		if errors.As(examNotOpen(sc.Paper), &appErr) {
			v.LockReason = appErr.Message
		}
	case st == models.SheetSubmitted:
		v.LockReason = "These marks are submitted for verification. An administrator can return them for correction."
	case st == models.SheetVerified:
		v.LockReason = "These marks are verified and waiting to be finalized."
	case st == models.SheetFinalized:
		v.LockReason = "These marks are finalized. Corrections need an administrator to reopen them with a reason."
	}
	return v
}

func parseSheetKey(errs validation.Errors, raw string) uuid.UUID {
	return parseRequiredID(errs, "section_id", raw, "Choose a section.")
}

// Sheet returns a paper's marks for one section.
func (s *ResultService) Sheet(ctx context.Context, actor *models.User, scheduleID uuid.UUID, rawSection string) (*SheetView, error) {
	errs := validation.Errors{}
	sectionID := parseSheetKey(errs, rawSection)
	if errs.Any() {
		return nil, apperror.Validation(errs)
	}
	c, err := s.caller(ctx, actor)
	if err != nil {
		return nil, err
	}
	sc, err := s.repo.Sheet(ctx, scheduleID, sectionID)
	if err != nil {
		return nil, mapResultRepoError(err)
	}
	if !c.mayEnter(sc.Paper.Subject.ID, sectionID) {
		return nil, notYourPaper()
	}
	if !c.admin && sc.Paper.Examination.Status == models.ExamDraft {
		return nil, errPaperNotFound
	}
	return s.view(c, sc), nil
}

// parsedMark is a validated marks record (components keyed by id).
type parsedMark struct {
	index        int
	enrollmentID uuid.UUID
	attendance   string
	total        *models.Marks
	components   map[uuid.UUID]models.Marks
	version      int
}

func parseMarkInput(errs validation.Errors, prefix string, i int, in MarkInput) parsedMark {
	p := parsedMark{index: i, version: in.Version, components: map[uuid.UUID]models.Marks{}}
	p.enrollmentID = parseRequiredID(errs, prefix+"enrollment_id", in.EnrollmentID, "Choose a student.")
	switch strings.ToLower(strings.TrimSpace(in.Attendance)) {
	case "", models.ExamPresent:
		p.attendance = models.ExamPresent
	case models.ExamAbsent:
		p.attendance = models.ExamAbsent
	default:
		errs.Add(prefix+"attendance", "Choose present or absent.")
	}
	if in.Version < 0 {
		errs.Add(prefix+"version", "Invalid version.")
	}
	if p.attendance == models.ExamAbsent {
		if strings.TrimSpace(string(in.Total)) != "" || len(in.Components) > 0 {
			errs.Add(prefix+"attendance", "An absent student can't have marks. Mark them present to enter marks.")
		}
		return p
	}
	if m, ok := parseMarksField(errs, prefix+"total", "Marks", in.Total, false); ok {
		p.total = &m
	}
	for j, c := range in.Components {
		cp := fmt.Sprintf("%scomponents[%d].", prefix, j)
		id, err := uuid.Parse(strings.TrimSpace(c.ComponentID))
		if err != nil {
			errs.Add(cp+"component_id", "Invalid component.")
			continue
		}
		if _, dup := p.components[id]; dup {
			errs.Add(cp+"component_id", "A component is listed twice.")
			continue
		}
		if m, ok := parseMarksField(errs, cp+"obtained", "Marks", c.Obtained, true); ok {
			p.components[id] = m
		}
	}
	return p
}

func markSnapshot(attendance string, total models.Marks, comps []models.ComponentMark) map[string]any {
	c := map[string]string{}
	for _, x := range comps {
		c[x.ComponentID.String()] = x.Obtained.String()
	}
	return map[string]any{"attendance": attendance, "total": total.String(), "components": c}
}

// planMarks validates marks against the scheme and decides what to save.
// Totals are always computed here from the components.
func planMarks(c *resultCaller, sc *repository.SheetContext, marks []parsedMark, reason *string, saved, unchanged *int) (*repository.MarksWrite, error) {
	if err := examNotOpen(sc.Paper); err != nil {
		return nil, err
	}
	if sc.Stage == models.StagePublished {
		return nil, resultsPublished(sc.Paper.Examination.Name)
	}
	if !c.mayEnter(sc.Paper.Subject.ID, sc.Sheet.Section.ID) {
		return nil, notYourPaper()
	}
	if sc.Sheet.Status != models.SheetDraft {
		msg := fmt.Sprintf("These marks are %s. ", sc.Sheet.Status)
		if c.admin {
			msg += "Reopen them with a reason to make changes."
		} else {
			msg += "Ask an administrator to return them for correction."
		}
		return nil, apperror.Conflict("SHEET_LOCKED", msg, nil)
	}
	w := &repository.MarksWrite{}
	if !sc.Scheme.Saved {
		w.CreateScheme = &repository.SchemeWrite{Full: sc.Scheme.FullMarks, Pass: sc.Scheme.PassMarks}
	}
	rows := map[uuid.UUID]*models.MarkEntryRow{}
	for i := range sc.Rows {
		rows[sc.Rows[i].EnrollmentID] = &sc.Rows[i]
	}
	comps := sc.Scheme.Components
	errs := validation.Errors{}
	for _, m := range marks {
		prefix := fmt.Sprintf("records[%d].", m.index)
		row := rows[m.enrollmentID]
		if row == nil || (!row.Eligible && row.Mark == nil) {
			errs.Add(prefix+"enrollment_id", fmt.Sprintf("That student isn't enrolled in %s – %s.", sc.Paper.Class.Name, sc.Sheet.Section.Name))
			continue
		}
		var total models.Marks
		stored := []models.ComponentMark{}
		if m.attendance == models.ExamPresent {
			if len(comps) > 0 {
				if m.total != nil && len(m.components) == 0 {
					errs.Add(prefix+"components", "Enter marks for each component.")
					continue
				}
				for _, comp := range comps {
					v, ok := m.components[comp.ID]
					if !ok {
						errs.Add(prefix+"components", fmt.Sprintf("Enter the %s marks.", comp.Name))
						continue
					}
					if v > comp.MaxMarks {
						errs.Add(prefix+"components", fmt.Sprintf("%s marks can't exceed %s.", comp.Name, comp.MaxMarks))
					}
					total += v
					stored = append(stored, models.ComponentMark{ComponentID: comp.ID, Obtained: v})
				}
				if len(m.components) > len(comps) {
					errs.Add(prefix+"components", "A component doesn't belong to this paper.")
				}
			} else {
				if len(m.components) > 0 {
					errs.Add(prefix+"components", "This paper has no components; enter the total marks.")
					continue
				}
				if m.total == nil {
					errs.Add(prefix+"total", "Enter the marks, or mark the student absent.")
					continue
				}
				total = *m.total
			}
			if total > sc.Scheme.FullMarks {
				errs.Add(prefix+"total", fmt.Sprintf("Marks can't exceed the full marks (%s).", sc.Scheme.FullMarks))
			}
		}
		mw := repository.MarkWrite{EnrollmentID: m.enrollmentID, StudentID: row.Student.ID, Attendance: m.attendance, Total: total, Components: stored}
		if cur := row.Mark; cur != nil {
			after := markSnapshot(m.attendance, total, stored)
			before := markSnapshot(cur.Attendance, cur.Total, cur.Components)
			if fmt.Sprint(before) == fmt.Sprint(after) {
				*unchanged++
				continue
			}
			if m.version != cur.Version {
				return nil, mapResultRepoError(repository.ErrMarksConflict)
			}
			id := cur.ID
			mw.ExistingID, mw.Version, mw.Before, mw.After, mw.Reason = &id, cur.Version, before, after, reason
		} else if m.version != 0 {
			return nil, mapResultRepoError(repository.ErrMarksConflict)
		}
		w.Writes = append(w.Writes, mw)
		*saved++
	}
	if errs.Any() {
		e := apperror.Validation(errs)
		e.Message = fmt.Sprintf("%d mark%s need%s attention. Nothing was saved.", len(errs), plural(len(errs)), map[bool]string{true: "s", false: ""}[len(errs) == 1])
		return nil, e
	}
	return w, nil
}

func (s *ResultService) saveMarks(ctx context.Context, actor *models.User, scheduleID, sectionID uuid.UUID, marks []parsedMark, reason *string) (*SaveMarksResult, error) {
	c, err := s.caller(ctx, actor)
	if err != nil {
		return nil, err
	}
	saved, unchanged := 0, 0
	var schemeCreated bool
	sc, err := s.repo.SaveMarks(ctx, scheduleID, sectionID, actor.ID, func(sc *repository.SheetContext) (*repository.MarksWrite, error) {
		saved, unchanged = 0, 0
		w, err := planMarks(c, sc, marks, reason, &saved, &unchanged)
		if w != nil {
			schemeCreated = w.CreateScheme != nil
		}
		return w, err
	})
	if err != nil {
		return nil, mapResultRepoError(err)
	}
	if saved > 0 {
		meta := map[string]any{"section": sc.Sheet.Section.Name, "saved": saved}
		if schemeCreated {
			meta["scheme"] = "created from the subject's defaults"
		}
		if reason != nil {
			meta["reason"] = *reason
		}
		s.record(ctx, actor, models.ActivityMarksSaved, sc.Paper.Examination.ID,
			fmt.Sprintf("Saved %d mark%s for %s, Section %s", saved, plural(saved), paperLabel(sc.Paper), sc.Sheet.Section.Name), meta)
	}
	return &SaveMarksResult{Sheet: s.view(c, sc), Saved: saved, Unchanged: unchanged}, nil
}

// SaveMarks saves a whole section's marks (bulk entry) in one transaction.
// Every record is validated; if any fails nothing is saved and each failure
// is reported by record.
func (s *ResultService) SaveMarks(ctx context.Context, actor *models.User, scheduleID uuid.UUID, in MarksInput) (*SaveMarksResult, error) {
	errs := validation.Errors{}
	sectionID := parseSheetKey(errs, in.SectionID)
	switch {
	case len(in.Records) == 0:
		errs.Add("records", "Enter marks for at least one student.")
	case len(in.Records) > maxMarksRecords:
		errs.Add("records", fmt.Sprintf("At most %d students at once.", maxMarksRecords))
	}
	reason := parseExamReason(errs, in.Reason)
	marks := []parsedMark{}
	seen := map[uuid.UUID]bool{}
	for i, r := range in.Records {
		m := parseMarkInput(errs, fmt.Sprintf("records[%d].", i), i, r)
		if seen[m.enrollmentID] && m.enrollmentID != uuid.Nil {
			errs.Add(fmt.Sprintf("records[%d].enrollment_id", i), "This student is listed twice.")
		}
		seen[m.enrollmentID] = true
		marks = append(marks, m)
	}
	if errs.Any() {
		e := apperror.Validation(errs)
		e.Message = fmt.Sprintf("%d mark%s need attention. Nothing was saved.", len(errs), plural(len(errs)))
		return nil, e
	}
	return s.saveMarks(ctx, actor, scheduleID, sectionID, marks, reason)
}

// SaveMark saves one student's marks.
func (s *ResultService) SaveMark(ctx context.Context, actor *models.User, scheduleID uuid.UUID, in SingleMarkInput) (*SaveMarksResult, error) {
	return s.SaveMarks(ctx, actor, scheduleID, MarksInput{SectionID: in.SectionID, Records: []MarkInput{in.MarkInput}})
}

// UpdateMark changes one saved marks record.
func (s *ResultService) UpdateMark(ctx context.Context, actor *models.User, markID uuid.UUID, in UpdateMarkInput) (*SaveMarksResult, error) {
	errs := validation.Errors{}
	if in.Version < 1 {
		errs.Add("version", "Reload the marks and try again.")
	}
	reason := parseExamReason(errs, in.Reason)
	if errs.Any() {
		return nil, apperror.Validation(errs)
	}
	scheduleID, sectionID, err := s.repo.MarkPlace(ctx, markID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, apperror.NotFound("Marks record not found.")
	}
	if err != nil {
		return nil, apperror.Internal(err)
	}
	sc, err := s.repo.Sheet(ctx, scheduleID, sectionID)
	if err != nil {
		return nil, mapResultRepoError(err)
	}
	var enrollment uuid.UUID
	for _, r := range sc.Rows {
		if r.Mark != nil && r.Mark.ID == markID {
			enrollment = r.EnrollmentID
		}
	}
	m := parseMarkInput(errs, "", 0, MarkInput{EnrollmentID: enrollment.String(), Attendance: in.Attendance, Total: in.Total, Components: in.Components, Version: in.Version})
	if errs.Any() {
		return nil, apperror.Validation(errs)
	}
	res, err := s.saveMarks(ctx, actor, scheduleID, sectionID, []parsedMark{m}, reason)
	if err != nil {
		var appErr *apperror.Error
		if errors.As(err, &appErr) && appErr.Details != nil {
			d := map[string]string{}
			for k, v := range appErr.Details {
				d[strings.TrimPrefix(k, "records[0].")] = v
			}
			cp := *appErr
			cp.Details = d
			return nil, &cp
		}
		return nil, err
	}
	return res, nil
}

// MarkHistory lists a marks record's audited changes.
func (s *ResultService) MarkHistory(ctx context.Context, actor *models.User, markID uuid.UUID) ([]models.MarkChange, error) {
	scheduleID, sectionID, err := s.repo.MarkPlace(ctx, markID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, apperror.NotFound("Marks record not found.")
	}
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if _, err := s.Sheet(ctx, actor, scheduleID, sectionID.String()); err != nil {
		return nil, err
	}
	items, err := s.repo.MarkChanges(ctx, markID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return items, nil
}

// ---- sheet workflow ----

func unmarkedCount(sc *repository.SheetContext) int {
	n := 0
	for _, r := range sc.Rows {
		if r.Eligible && r.Mark == nil {
			n++
		}
	}
	return n
}

func unmarkedError(n int) error {
	msg := fmt.Sprintf("%d student%s %s no marks yet. Enter marks or mark them absent first.", n, plural(n), map[bool]string{true: "has", false: "have"}[n == 1])
	return &apperror.Error{Status: http.StatusUnprocessableEntity, Code: "UNMARKED_STUDENTS", Message: msg}
}

func invalidSheetMove(from models.MarkSheetStatus, to string) error {
	return apperror.Conflict("INVALID_STATUS_CHANGE", fmt.Sprintf("These marks are %s, so they can't be %s.", from, to), nil)
}

// sheetAction runs a workflow step; admins only, except submit.
func (s *ResultService) sheetAction(ctx context.Context, actor *models.User, scheduleID uuid.UUID, in SheetActionInput, action string) (*SheetView, error) {
	errs := validation.Errors{}
	sectionID := parseSheetKey(errs, in.SectionID)
	reason := parseExamReason(errs, in.Reason)
	if action == "reopen" && reason == nil {
		errs.Add("reason", "Give a reason for returning or reopening these marks.")
	}
	if errs.Any() {
		return nil, apperror.Validation(errs)
	}
	c, err := s.caller(ctx, actor)
	if err != nil {
		return nil, err
	}
	var from models.MarkSheetStatus
	sc, err := s.repo.SetSheetStatus(ctx, scheduleID, sectionID, actor.ID, func(sc *repository.SheetContext) (*repository.SheetTransition, error) {
		from = sc.Sheet.Status
		if !c.mayEnter(sc.Paper.Subject.ID, sc.Sheet.Section.ID) {
			return nil, notYourPaper()
		}
		if action != "submit" && !c.admin {
			return nil, apperror.Forbidden("Only administrators can verify, finalize or reopen marks.")
		}
		if action != "reopen" {
			if err := examNotOpen(sc.Paper); err != nil {
				return nil, err
			}
		}
		if sc.Stage == models.StagePublished {
			return nil, resultsPublished(sc.Paper.Examination.Name)
		}
		switch action {
		case "submit":
			if sc.Sheet.Status != models.SheetDraft {
				return nil, invalidSheetMove(sc.Sheet.Status, "submitted")
			}
			if n := unmarkedCount(sc); n > 0 {
				return nil, unmarkedError(n)
			}
			if sc.Sheet.Marked == 0 {
				return nil, &apperror.Error{Status: http.StatusUnprocessableEntity, Code: "NO_MARKS", Message: "There are no marks to submit."}
			}
			return &repository.SheetTransition{To: models.SheetSubmitted}, nil
		case "verify":
			if sc.Sheet.Status != models.SheetSubmitted {
				return nil, invalidSheetMove(sc.Sheet.Status, "verified")
			}
			if n := unmarkedCount(sc); n > 0 {
				return nil, unmarkedError(n)
			}
			return &repository.SheetTransition{To: models.SheetVerified}, nil
		case "finalize":
			if sc.Sheet.Status != models.SheetVerified {
				return nil, invalidSheetMove(sc.Sheet.Status, "finalized")
			}
			if n := unmarkedCount(sc); n > 0 {
				return nil, unmarkedError(n)
			}
			return &repository.SheetTransition{To: models.SheetFinalized}, nil
		default: // reopen
			if sc.Sheet.Status == models.SheetDraft {
				return nil, invalidSheetMove(sc.Sheet.Status, "reopened")
			}
			reset := sc.Stage != models.StagePending
			return &repository.SheetTransition{To: models.SheetDraft, Reason: reason, ResetResults: reset}, nil
		}
	})
	if err != nil {
		return nil, mapResultRepoError(err)
	}
	verbs := map[string]string{"submit": "Submitted", "verify": "Verified", "finalize": "Finalized", "reopen": "Reopened"}
	meta := map[string]any{"section": sc.Sheet.Section.Name, "from": from, "to": sc.Sheet.Status}
	if reason != nil {
		meta["reason"] = *reason
	}
	s.record(ctx, actor, models.ActivityMarksStatus, sc.Paper.Examination.ID,
		fmt.Sprintf("%s marks of %s, Section %s", verbs[action], paperLabel(sc.Paper), sc.Sheet.Section.Name), meta)
	return s.view(c, sc), nil
}

// Submit sends a section's marks for verification (teachers and admins).
func (s *ResultService) Submit(ctx context.Context, actor *models.User, scheduleID uuid.UUID, in SheetActionInput) (*SheetView, error) {
	return s.sheetAction(ctx, actor, scheduleID, in, "submit")
}

// Verify accepts submitted marks (administrators).
func (s *ResultService) Verify(ctx context.Context, actor *models.User, scheduleID uuid.UUID, in SheetActionInput) (*SheetView, error) {
	return s.sheetAction(ctx, actor, scheduleID, in, "verify")
}

// Finalize locks verified marks for result processing (administrators).
func (s *ResultService) Finalize(ctx context.Context, actor *models.User, scheduleID uuid.UUID, in SheetActionInput) (*SheetView, error) {
	return s.sheetAction(ctx, actor, scheduleID, in, "finalize")
}

// Reopen returns marks to draft with a reason (administrators). Unpublished
// processed results of the examination are discarded.
func (s *ResultService) Reopen(ctx context.Context, actor *models.User, scheduleID uuid.UUID, in SheetActionInput) (*SheetView, error) {
	return s.sheetAction(ctx, actor, scheduleID, in, "reopen")
}

// ---- sheets overview ----

// MarkExams lists examinations with marks to enter (teachers: those with
// papers they teach).
func (s *ResultService) MarkExams(ctx context.Context, actor *models.User, rawYear string) ([]models.Examination, string, error) {
	errs := validation.Errors{}
	year := optionalID(errs, "academic_year_id", rawYear)
	if errs.Any() {
		return nil, "", apperror.Validation(errs)
	}
	c, err := s.caller(ctx, actor)
	if err != nil {
		return nil, "", err
	}
	exams, err := s.repo.MarkExams(ctx, year)
	if err != nil {
		return nil, "", apperror.Internal(err)
	}
	if c.admin {
		return exams, "", nil
	}
	if len(c.papers) == 0 {
		return []models.Examination{}, c.note, nil
	}
	out := []models.Examination{}
	for _, x := range exams {
		sheets, err := s.sheetsFor(ctx, c, x.ID)
		if err != nil {
			return nil, "", err
		}
		if len(sheets) > 0 {
			out = append(out, x)
		}
	}
	return out, "", nil
}

func (s *ResultService) sheetsFor(ctx context.Context, c *resultCaller, examID uuid.UUID) ([]models.MarkSheet, error) {
	rc, err := s.repo.ExamContext(ctx, examID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, errExamNotFound
	}
	if err != nil {
		return nil, apperror.Internal(err)
	}
	out := []models.MarkSheet{}
	for _, ps := range rc.Papers {
		p := ps.Paper
		for _, sec := range examSections(rc, p.Class.ID) {
			if !c.mayEnter(p.Subject.ID, sec.ID) {
				continue
			}
			sh := models.MarkSheet{Paper: p, Section: sec, Status: models.SheetDraft}
			if st, ok := rc.Sheets[repository.SheetKey{ScheduleID: p.ScheduleID, SectionID: sec.ID}]; ok {
				sh.Status, sh.Started = st, true
			}
			for _, es := range rc.Students {
				if es.Section.ID != sec.ID {
					continue
				}
				sh.Students++
				if m, ok := rc.Marks[repository.MarkKey{ScheduleID: p.ScheduleID, EnrollmentID: es.EnrollmentID}]; ok {
					sh.Marked++
					if m.Attendance == models.ExamAbsent {
						sh.Absent++
					}
				}
			}
			out = append(out, sh)
		}
	}
	return out, nil
}

// examSections lists the sections of a class that have students or a sheet.
func examSections(rc *repository.ExamResultContext, classID uuid.UUID) []models.SectionRef {
	out := []models.SectionRef{}
	for _, sec := range rc.Sections {
		if sec.ClassID == classID {
			out = append(out, sec)
		}
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	return out
}

// Sheets lists an examination's mark sheets (paper × section) the caller can
// see, with entry progress and status (the marks review).
func (s *ResultService) Sheets(ctx context.Context, actor *models.User, examID uuid.UUID) ([]models.MarkSheet, error) {
	c, err := s.caller(ctx, actor)
	if err != nil {
		return nil, err
	}
	return s.sheetsFor(ctx, c, examID)
}

// ---- result processing ----

// readiness lists what stops subject results from being processed.
func readiness(rc *repository.ExamResultContext) []ReadinessIssue {
	issues := []ReadinessIssue{}
	add := func(code, msg string) {
		if len(issues) < maxReadinessItems {
			issues = append(issues, ReadinessIssue{Code: code, Message: msg})
		}
	}
	switch rc.Exam.Status {
	case models.ExamDraft:
		add("EXAM_NOT_PUBLISHED", "The examination timetable isn't published yet.")
	case models.ExamCancelled:
		add("EXAM_CANCELLED", "The examination was cancelled.")
	}
	if len(rc.Papers) == 0 {
		add("NO_PAPERS", "The examination has no scheduled papers.")
	}
	if len(rc.Students) == 0 {
		add("NO_STUDENTS", "No students are enrolled in the examination's classes.")
	}
	for _, ps := range rc.Papers {
		p := ps.Paper
		bySection := map[uuid.UUID]int{}
		for _, es := range rc.Students {
			if es.ClassID != p.Class.ID {
				continue
			}
			if _, ok := rc.Marks[repository.MarkKey{ScheduleID: p.ScheduleID, EnrollmentID: es.EnrollmentID}]; !ok {
				bySection[es.Section.ID]++
			}
		}
		if ps.Scheme == nil {
			add("NO_SCHEME", fmt.Sprintf("%s – %s has no marks yet.", p.Class.Name, p.Subject.Name))
			continue
		}
		for _, sec := range examSections(rc, p.Class.ID) {
			hasStudents := false
			for _, es := range rc.Students {
				if es.Section.ID == sec.ID {
					hasStudents = true
					break
				}
			}
			if !hasStudents {
				continue
			}
			label := fmt.Sprintf("%s – %s, Section %s", p.Class.Name, p.Subject.Name, sec.Name)
			if n := bySection[sec.ID]; n > 0 {
				add("UNMARKED_STUDENTS", fmt.Sprintf("%s: %d student%s without marks.", label, n, plural(n)))
				continue
			}
			if st := rc.Sheets[repository.SheetKey{ScheduleID: p.ScheduleID, SectionID: sec.ID}]; st != models.SheetFinalized {
				if st == "" {
					st = models.SheetDraft
				}
				add("NOT_FINALIZED", fmt.Sprintf("%s: marks are %s, not finalized.", label, st))
			}
		}
	}
	return issues
}

func notReady(issues []ReadinessIssue) error {
	details := map[string]string{}
	for i, is := range issues {
		details[fmt.Sprintf("issues[%d]", i)] = is.Message
	}
	return &apperror.Error{Status: http.StatusConflict, Code: "RESULTS_NOT_READY",
		Message: "Results can't be processed yet: " + issues[0].Message, Details: details}
}

// computeSubjectResults grades every eligible student's papers.
func computeSubjectResults(rc *repository.ExamResultContext) []repository.NewSubjectResult {
	out := []repository.NewSubjectResult{}
	for _, ps := range rc.Papers {
		p, sch := ps.Paper, ps.Scheme
		for _, es := range rc.Students {
			if es.ClassID != p.Class.ID {
				continue
			}
			m := rc.Marks[repository.MarkKey{ScheduleID: p.ScheduleID, EnrollmentID: es.EnrollmentID}]
			absent := m.Attendance == models.ExamAbsent
			obtained := m.Total
			if absent {
				obtained = 0
			}
			r := models.SubjectResultFor(obtained, sch.FullMarks, sch.PassMarks, absent)
			out = append(out, repository.NewSubjectResult{
				ScheduleID: p.ScheduleID, EnrollmentID: es.EnrollmentID, StudentID: es.Student.ID, ClassID: es.ClassID, SectionID: es.Section.ID,
				SubjectID: p.Subject.ID, SubjectName: p.Subject.Name, SubjectCode: p.Subject.Code,
				Full: sch.FullMarks, Pass: sch.PassMarks, Obtained: obtained,
				Percentage: models.Marks(models.PercentHundredths(obtained, sch.FullMarks)),
				Letter:     r.Grade.Letter, GradePoint: models.Marks(r.Grade.Point), Absent: absent, Passed: r.Passed,
			})
		}
	}
	return out
}

// computeOverall totals each student's subject results, applies the fail
// rule and assigns class and section positions.
func computeOverall(rc *repository.ExamResultContext) ([]repository.NewExamResult, error) {
	papersByClass := map[uuid.UUID]int{}
	for _, ps := range rc.Papers {
		papersByClass[ps.Paper.Class.ID]++
	}
	out := []repository.NewExamResult{}
	for _, es := range rc.Students {
		subjects := rc.SubjectResults[es.EnrollmentID]
		if want := papersByClass[es.ClassID]; len(subjects) != want || want == 0 {
			return nil, apperror.Conflict("SUBJECT_RESULTS_INCOMPLETE",
				fmt.Sprintf("%s has %d of %d subject results. Process subject results again.", es.Student.FullName, len(subjects), want), nil)
		}
		var total, full models.Marks
		outcomes := make([]models.SubjectOutcome, 0, len(subjects))
		failed := 0
		for _, sr := range subjects {
			total += sr.Obtained
			full += sr.FullMarks
			outcomes = append(outcomes, models.SubjectOutcome{Grade: models.Grade{Letter: sr.Letter, Point: int64(sr.GradePoint)}, Passed: sr.Passed})
			if !sr.Passed {
				failed++
			}
		}
		gpa, grade, passed := models.GPAFor(outcomes)
		out = append(out, repository.NewExamResult{
			EnrollmentID: es.EnrollmentID, StudentID: es.Student.ID, ClassID: es.ClassID, SectionID: es.Section.ID, RollNumber: es.RollNumber,
			Subjects: len(subjects), Failed: failed, TotalObtained: total, TotalFull: full,
			Percentage: models.Marks(models.PercentHundredths(total, full)), GPA: models.Marks(gpa), Grade: grade, Passed: passed,
		})
	}
	assignPositions(out, func(r *repository.NewExamResult) uuid.UUID { return r.ClassID }, func(r *repository.NewExamResult, p *int) { r.ClassPosition = p })
	assignPositions(out, func(r *repository.NewExamResult) uuid.UUID { return r.SectionID }, func(r *repository.NewExamResult, p *int) { r.SectionPosition = p })
	return out, nil
}

// assignPositions ranks results within each group (class or section).
func assignPositions(rs []repository.NewExamResult, group func(*repository.NewExamResult) uuid.UUID, set func(*repository.NewExamResult, *int)) {
	groups := map[uuid.UUID][]int{}
	for i := range rs {
		g := group(&rs[i])
		groups[g] = append(groups[g], i)
	}
	for _, idx := range groups {
		sort.SliceStable(idx, func(a, b int) bool {
			ra, rb := rs[idx[a]], rs[idx[b]]
			return models.MeritLess(models.RankEntry{Passed: ra.Passed, Total: ra.TotalObtained, GPA: int64(ra.GPA)},
				models.RankEntry{Passed: rb.Passed, Total: rb.TotalObtained, GPA: int64(rb.GPA)})
		})
		entries := make([]models.RankEntry, len(idx))
		for k, i := range idx {
			entries[k] = models.RankEntry{Passed: rs[i].Passed, Total: rs[i].TotalObtained, GPA: int64(rs[i].GPA)}
		}
		for k, pos := range models.Rank(entries) {
			if pos > 0 {
				p := pos
				set(&rs[idx[k]], &p)
			}
		}
	}
}

func (s *ResultService) process(ctx context.Context, actor *models.User, examID uuid.UUID, step string, reason *string) (*ResultStatus, error) {
	var summary string
	var meta map[string]any
	_, err := s.repo.ProcessResults(ctx, examID, actor.ID, func(rc *repository.ExamResultContext) (*repository.ResultWrite, error) {
		stage := rc.Run.Stage
		if step != "correction" && stage == models.StagePublished {
			return nil, resultsPublished(rc.Exam.Name)
		}
		switch step {
		case "subjects":
			if issues := readiness(rc); len(issues) > 0 {
				return nil, notReady(issues)
			}
			subjects := computeSubjectResults(rc)
			failed := 0
			for _, s := range subjects {
				if !s.Passed {
					failed++
				}
			}
			summary = fmt.Sprintf("Processed %d subject results of %s", len(subjects), rc.Exam.Name)
			meta = map[string]any{"subject_results": len(subjects), "failed_subjects": failed}
			return &repository.ResultWrite{Stage: models.StageSubjectsProcessed, Subjects: subjects}, nil
		case "overall":
			if stage != models.StageSubjectsProcessed && stage != models.StageProcessed && stage != models.StageVerified {
				return nil, apperror.Conflict("SUBJECT_RESULTS_REQUIRED", "Process the subject results first.", nil)
			}
			if issues := readiness(rc); len(issues) > 0 {
				return nil, notReady(issues)
			}
			overall, err := computeOverall(rc)
			if err != nil {
				return nil, err
			}
			passed := 0
			for _, o := range overall {
				if o.Passed {
					passed++
				}
			}
			summary = fmt.Sprintf("Processed overall results of %s: %d passed, %d failed", rc.Exam.Name, passed, len(overall)-passed)
			meta = map[string]any{"students": len(overall), "passed": passed}
			return &repository.ResultWrite{Stage: models.StageProcessed, Overall: overall}, nil
		case "verify":
			if stage != models.StageProcessed {
				return nil, apperror.Conflict("RESULTS_NOT_PROCESSED", "Process the overall results before verifying them.", nil)
			}
			summary = "Verified the results of " + rc.Exam.Name
			return &repository.ResultWrite{Stage: models.StageVerified}, nil
		case "publish":
			if stage != models.StageVerified {
				return nil, apperror.Conflict("RESULTS_NOT_VERIFIED", "Verify the processed results before publishing them.", nil)
			}
			if issues := publishIssues(rc); len(issues) > 0 {
				return nil, notReady(issues)
			}
			passed := 0
			for _, r := range rc.Results {
				if r.Passed {
					passed++
				}
			}
			summary = fmt.Sprintf("Published the results of %s (%d students, %d passed)", rc.Exam.Name, len(rc.Results), passed)
			meta = map[string]any{"students": len(rc.Results), "passed": passed, "publication": rc.Run.Publications + 1}
			if rc.Run.CorrectionReason != nil {
				meta["correction_reason"] = *rc.Run.CorrectionReason
			}
			return &repository.ResultWrite{Stage: models.StagePublished, Publish: true}, nil
		default: // correction
			if stage != models.StagePublished {
				return nil, apperror.Conflict("RESULTS_NOT_PUBLISHED", "Only published results need a correction; unpublished results can be reprocessed directly.", nil)
			}
			summary = "Started a correction of the published results of " + rc.Exam.Name
			meta = map[string]any{"reason": *reason}
			return &repository.ResultWrite{Stage: models.StageCorrection, CorrectionReason: reason}, nil
		}
	})
	if err != nil {
		var appErr *apperror.Error
		if errors.As(err, &appErr) {
			return nil, appErr
		}
		if errors.Is(err, repository.ErrNotFound) {
			return nil, errExamNotFound
		}
		return nil, mapResultRepoError(err)
	}
	actions := map[string]string{"subjects": models.ActivityResultsProcessed, "overall": models.ActivityResultsProcessed,
		"verify": models.ActivityResultsVerified, "publish": models.ActivityResultsPublished, "correction": models.ActivityResultsCorrection}
	s.record(ctx, actor, actions[step], examID, summary, meta)
	return s.Status(ctx, examID)
}

// publishIssues re-checks processed results before publication.
func publishIssues(rc *repository.ExamResultContext) []ReadinessIssue {
	issues := readiness(rc)
	byEnrollment := map[uuid.UUID]models.ExamResult{}
	for _, r := range rc.Results {
		byEnrollment[r.EnrollmentID] = r
	}
	papersByClass := map[uuid.UUID]int{}
	for _, ps := range rc.Papers {
		papersByClass[ps.Paper.Class.ID]++
	}
	for _, es := range rc.Students {
		r, ok := byEnrollment[es.EnrollmentID]
		switch {
		case !ok:
			issues = append(issues, ReadinessIssue{"RESULT_MISSING", fmt.Sprintf("%s has no overall result. Process the results again.", es.Student.FullName)})
		case len(rc.SubjectResults[es.EnrollmentID]) != papersByClass[es.ClassID]:
			issues = append(issues, ReadinessIssue{"SUBJECT_RESULTS_INCOMPLETE", fmt.Sprintf("%s's subject results are incomplete.", es.Student.FullName)})
		case r.FailedSubjects > 0 && (r.GPA != 0 || r.Grade != "F" || r.Passed):
			issues = append(issues, ReadinessIssue{"FAIL_RULE", fmt.Sprintf("%s failed a subject but doesn't have GPA 0.00 / F.", es.Student.FullName)})
		}
		if len(issues) >= maxReadinessItems {
			break
		}
	}
	if len(rc.Results) != len(rc.Students) && len(issues) == 0 {
		issues = append(issues, ReadinessIssue{"RESULTS_STALE", "The enrolled students changed since processing. Process the results again."})
	}
	return issues
}

// ProcessSubjectResults grades every paper of every eligible student.
func (s *ResultService) ProcessSubjectResults(ctx context.Context, actor *models.User, examID uuid.UUID) (*ResultStatus, error) {
	return s.process(ctx, actor, examID, "subjects", nil)
}

// ProcessOverallResults computes GPAs, overall grades and positions.
func (s *ResultService) ProcessOverallResults(ctx context.Context, actor *models.User, examID uuid.UUID) (*ResultStatus, error) {
	return s.process(ctx, actor, examID, "overall", nil)
}

// VerifyResults confirms processed results are ready to publish.
func (s *ResultService) VerifyResults(ctx context.Context, actor *models.User, examID uuid.UUID) (*ResultStatus, error) {
	return s.process(ctx, actor, examID, "verify", nil)
}

// PublishResults publishes verified results and stores a snapshot.
func (s *ResultService) PublishResults(ctx context.Context, actor *models.User, examID uuid.UUID) (*ResultStatus, error) {
	return s.process(ctx, actor, examID, "publish", nil)
}

// CorrectPublishedResults reopens published results for an audited correction.
func (s *ResultService) CorrectPublishedResults(ctx context.Context, actor *models.User, examID uuid.UUID, in ResultReasonInput) (*ResultStatus, error) {
	errs := validation.Errors{}
	reason := parseExamReason(errs, in.Reason)
	if reason == nil {
		errs.Add("reason", "Give a reason for correcting published results.")
	}
	if errs.Any() {
		return nil, apperror.Validation(errs)
	}
	return s.process(ctx, actor, examID, "correction", reason)
}

// ---- reads ----

func statsFor(rc *repository.ExamResultContext, classID, sectionID *uuid.UUID) models.ResultStats {
	st := models.ResultStats{}
	in := func(class, section uuid.UUID) bool {
		return (classID == nil || *classID == class) && (sectionID == nil || *sectionID == section)
	}
	for _, es := range rc.Students {
		if !in(es.ClassID, es.Section.ID) {
			continue
		}
		st.Students++
		for _, ps := range rc.Papers {
			if ps.Paper.Class.ID != es.ClassID {
				continue
			}
			st.PapersExpected++
			if _, ok := rc.Marks[repository.MarkKey{ScheduleID: ps.Paper.ScheduleID, EnrollmentID: es.EnrollmentID}]; ok {
				st.PapersMarked++
				if rc.Sheets[repository.SheetKey{ScheduleID: ps.Paper.ScheduleID, SectionID: es.Section.ID}] == models.SheetFinalized {
					st.PapersFinalized++
				}
			}
		}
	}
	for _, ps := range rc.Papers {
		for _, sec := range examSections(rc, ps.Paper.Class.ID) {
			if !in(ps.Paper.Class.ID, sec.ID) {
				continue
			}
			st.SheetsTotal++
			if rc.Sheets[repository.SheetKey{ScheduleID: ps.Paper.ScheduleID, SectionID: sec.ID}] == models.SheetFinalized {
				st.SheetsFinalized++
			}
		}
	}
	var gpaSum int64
	for _, r := range rc.Results {
		if !in(r.Class.ID, r.Section.ID) {
			continue
		}
		st.Processed++
		if r.Passed {
			st.Passed++
		} else {
			st.Failed++
		}
		if r.PublishedAt != nil && rc.Run.Stage == models.StagePublished {
			st.Published++
		}
		gpaSum += int64(r.GPA)
		if r.TotalObtained > st.HighestTotal {
			st.HighestTotal = r.TotalObtained
		}
		if r.GPA > st.HighestGPA {
			st.HighestGPA = r.GPA
		}
	}
	if st.Processed > 0 {
		st.PassPercentage = models.Marks(models.PercentHundredths(models.Marks(st.Passed), models.Marks(st.Processed)))
		st.AverageGPA = models.Marks((2*gpaSum + int64(st.Processed)) / (2 * int64(st.Processed)))
	}
	return st
}

func nextStep(rc *repository.ExamResultContext, issues []ReadinessIssue) string {
	switch rc.Run.Stage {
	case models.StagePending, models.StageCorrection:
		if len(issues) == 0 {
			return "process_subjects"
		}
	case models.StageSubjectsProcessed:
		return "process_overall"
	case models.StageProcessed:
		return "verify"
	case models.StageVerified:
		return "publish"
	}
	return ""
}

// Status returns an examination's result stage, readiness and statistics.
func (s *ResultService) Status(ctx context.Context, examID uuid.UUID) (*ResultStatus, error) {
	return s.StatusFor(ctx, examID, "", "")
}

// StatusFor is Status with statistics limited to a class and/or section.
func (s *ResultService) StatusFor(ctx context.Context, examID uuid.UUID, rawClass, rawSection string) (*ResultStatus, error) {
	errs := validation.Errors{}
	classID := optionalID(errs, "class_id", rawClass)
	sectionID := optionalID(errs, "section_id", rawSection)
	if errs.Any() {
		return nil, apperror.Validation(errs)
	}
	rc, err := s.repo.ExamContext(ctx, examID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, errExamNotFound
	}
	if err != nil {
		return nil, apperror.Internal(err)
	}
	pubs, err := s.repo.Publications(ctx, examID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	issues := readiness(rc)
	return &ResultStatus{Examination: rc.Exam, AcademicYear: rc.Year, Run: rc.Run, Stats: statsFor(rc, classID, sectionID),
		Issues: issues, Publications: pubs, Next: nextStep(rc, issues)}, nil
}

// ListResultsInput holds raw result listing parameters.
type ListResultsInput struct {
	ClassID, SectionID, Status, Search string
	Page, PageSize                     int
}

// Results lists an examination's overall results.
func (s *ResultService) Results(ctx context.Context, examID uuid.UUID, in ListResultsInput) ([]models.ExamResult, int, int, int, error) {
	errs := validation.Errors{}
	f := repository.ResultFilter{ExaminationID: examID, ClassID: optionalID(errs, "class_id", in.ClassID), SectionID: optionalID(errs, "section_id", in.SectionID),
		Search: strings.TrimSpace(in.Search)}
	switch in.Status {
	case "":
	case "passed", "failed":
		v := in.Status == "passed"
		f.Passed = &v
	default:
		errs.Add("status", "Choose passed or failed.")
	}
	if len([]rune(f.Search)) > maxSearchLength {
		errs.Add("search", "Search must be at most 100 characters.")
	}
	f.Page, f.PageSize = normalizePaging(in.Page, in.PageSize)
	if errs.Any() {
		return nil, 0, 0, 0, apperror.Validation(errs)
	}
	if _, err := s.exams.GetByID(ctx, examID); err != nil {
		return nil, 0, 0, 0, mapExamRepoError(err)
	}
	items, total, err := s.repo.Results(ctx, f)
	if err != nil {
		return nil, 0, 0, 0, apperror.Internal(err)
	}
	return items, total, f.Page, f.PageSize, nil
}

// Result returns one overall result with its subject results.
func (s *ResultService) Result(ctx context.Context, id uuid.UUID) (*models.ExamResult, error) {
	r, err := s.repo.Result(ctx, id)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, errResultNotFound
	}
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return r, nil
}

// MeritList ranks an examination's results (optionally one class or section).
// Positions are the stored class/section positions; across classes they are
// computed the same way (passed first, total marks, then GPA; ties share).
func (s *ResultService) MeritList(ctx context.Context, examID uuid.UUID, rawClass, rawSection string) ([]MeritEntry, error) {
	errs := validation.Errors{}
	f := repository.ResultFilter{ExaminationID: examID, ClassID: optionalID(errs, "class_id", rawClass), SectionID: optionalID(errs, "section_id", rawSection), All: true}
	if errs.Any() {
		return nil, apperror.Validation(errs)
	}
	if _, err := s.exams.GetByID(ctx, examID); err != nil {
		return nil, mapExamRepoError(err)
	}
	items, _, err := s.repo.Results(ctx, f)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	entry := func(r models.ExamResult) models.RankEntry {
		return models.RankEntry{Passed: r.Passed, Total: r.TotalObtained, GPA: int64(r.GPA)}
	}
	sort.SliceStable(items, func(i, j int) bool { return models.MeritLess(entry(items[i]), entry(items[j])) })
	out := make([]MeritEntry, len(items))
	var ranks []int
	if f.ClassID == nil && f.SectionID == nil {
		entries := make([]models.RankEntry, len(items))
		for i := range items {
			entries[i] = entry(items[i])
		}
		ranks = models.Rank(entries)
	}
	for i, r := range items {
		out[i] = MeritEntry{Result: r}
		switch {
		case ranks != nil && ranks[i] > 0:
			p := ranks[i]
			out[i].Position = &p
		case ranks == nil && f.SectionID != nil:
			out[i].Position = r.SectionPosition
		case ranks == nil:
			out[i].Position = r.ClassPosition
		}
	}
	return out, nil
}

// ReportCard returns a student's result in an examination.
func (s *ResultService) ReportCard(ctx context.Context, examID, studentID uuid.UUID) (*ReportCard, error) {
	x, err := s.exams.GetByID(ctx, examID)
	if err != nil {
		return nil, mapExamRepoError(err)
	}
	items, _, err := s.repo.Results(ctx, repository.ResultFilter{ExaminationID: examID, All: true})
	if err != nil {
		return nil, apperror.Internal(err)
	}
	for _, r := range items {
		if r.Student.ID == studentID {
			full, err := s.Result(ctx, r.ID)
			if err != nil {
				return nil, err
			}
			return &ReportCard{Examination: *x, Result: *full, Published: full.PublishedAt != nil}, nil
		}
	}
	return nil, apperror.NotFound("This student has no processed result in this examination.")
}

// StudentResults lists a student's examination results (history).
func (s *ResultService) StudentResults(ctx context.Context, studentID uuid.UUID) ([]models.ExamResult, error) {
	items, err := s.repo.StudentResults(ctx, studentID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return items, nil
}
