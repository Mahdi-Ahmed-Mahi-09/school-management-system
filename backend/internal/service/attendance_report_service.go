package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/apperror"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/models"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/repository"
	"github.com/Mahdi-Ahmed-Mahi-09/school-management-system/backend/internal/validation"
)

// maxExportRows caps a CSV export of individual records.
const maxExportRows = 50000

// Section day states.
const (
	DayComplete   = "complete"    // every expected student has a record
	DayPartial    = "partial"     // some records, some students unmarked
	DayNotStarted = "not_started" // expected students, no records
	DayNoStudents = "no_students" // nobody expected and nothing recorded
)

// AttendanceClassOption is a class with the sections the caller can use.
type AttendanceClassOption struct {
	Class    models.ClassRef     `json:"class"`
	Sections []models.SectionRef `json:"sections"`
}

// AttendanceScopeInfo tells the client how far the caller's access reaches.
type AttendanceScopeInfo struct {
	All     bool             `json:"all"`
	Teacher *models.ActorRef `json:"teacher"`
	Note    string           `json:"note,omitempty"`
}

// AttendanceOptions feeds the attendance pickers.
type AttendanceOptions struct {
	AcademicYears []models.YearRef        `json:"academic_years"`
	AcademicYear  *models.YearRef         `json:"academic_year"`
	Today         string                  `json:"today"`
	DefaultDate   string                  `json:"default_date"`
	Classes       []AttendanceClassOption `json:"classes"`
	Scope         AttendanceScopeInfo     `json:"scope"`
	CanCorrect    bool                    `json:"can_correct"`
}

// SectionDayView is one section's attendance on a date.
type SectionDayView struct {
	Class    models.ClassRef   `json:"class"`
	Section  models.SectionRef `json:"section"`
	Enrolled int               `json:"enrolled"`
	models.AttendanceStats
	Unmarked    int        `json:"unmarked"`
	State       string     `json:"state"`
	LastSavedAt *time.Time `json:"last_saved_at"`
}

// ClassDayView totals a class's sections on a date.
type ClassDayView struct {
	Class    models.ClassRef `json:"class"`
	Sections int             `json:"sections"`
	Enrolled int             `json:"enrolled"`
	models.AttendanceStats
	Unmarked          int `json:"unmarked"`
	SectionsCompleted int `json:"sections_completed"`
}

// DayTotals totals every section of a date.
type DayTotals struct {
	Sections   int `json:"sections"`
	Enrolled   int `json:"enrolled"`
	Unmarked   int `json:"unmarked"`
	Completed  int `json:"sections_completed"`
	Partial    int `json:"sections_partial"`
	NotStarted int `json:"sections_not_started"`
	models.AttendanceStats
}

// DaySummary is the attendance of a date across sections.
type DaySummary struct {
	AcademicYear *models.YearRef      `json:"academic_year"`
	Date         string               `json:"date"`
	Today        string               `json:"today"`
	Totals       DayTotals            `json:"totals"`
	Classes      []ClassDayView       `json:"classes"`
	Sections     []SectionDayView     `json:"sections"`
	Students     []SessionStudentView `json:"students,omitempty"`
	Scope        AttendanceScopeInfo  `json:"scope"`
}

// GroupStatsView is one class or section over a range.
type GroupStatsView struct {
	Class    models.ClassRef    `json:"class"`
	Section  *models.SectionRef `json:"section"`
	Sections int                `json:"sections"`
	Enrolled int                `json:"enrolled"`
	Sessions int                `json:"sessions"`
	models.AttendanceStats
}

// DayStatsView is one date's tally.
type DayStatsView struct {
	Date string `json:"date"`
	models.AttendanceStats
}

// RangeReport compares classes (or a class's sections) over a date range.
type RangeReport struct {
	AcademicYear models.YearRef         `json:"academic_year"`
	Class        *models.ClassRef       `json:"class"`
	From         string                 `json:"from"`
	To           string                 `json:"to"`
	GroupBy      string                 `json:"group_by"` // class or section
	Totals       models.AttendanceStats `json:"totals"`
	Sessions     int                    `json:"sessions"` // distinct dates with records
	Groups       []GroupStatsView       `json:"groups"`
	Daily        []DayStatsView         `json:"daily"`
}

// StudentStatsView is one student's tally (monthly report row).
type StudentStatsView struct {
	Student    models.StudentRef `json:"student"`
	Class      models.ClassRef   `json:"class"`
	Section    models.SectionRef `json:"section"`
	RollNumber int               `json:"roll_number"`
	models.AttendanceStats
}

// MonthlyReport tallies each student of a class or section for one month.
type MonthlyReport struct {
	AcademicYear models.YearRef         `json:"academic_year"`
	Month        string                 `json:"month"` // YYYY-MM
	From         string                 `json:"from"`
	To           string                 `json:"to"`
	Sessions     int                    `json:"sessions"`
	Totals       models.AttendanceStats `json:"totals"`
	Students     []StudentStatsView     `json:"students"`
}

// MonthStatsView is one month of a student's attendance.
type MonthStatsView struct {
	Month string `json:"month"`
	models.AttendanceStats
}

// StudentAttendanceSummary is a student's attendance in one academic year.
type StudentAttendanceSummary struct {
	AcademicYears []models.YearRef       `json:"academic_years"`
	AcademicYear  *models.YearRef        `json:"academic_year"`
	Stats         models.AttendanceStats `json:"stats"`
	Months        []MonthStatsView       `json:"months"`
	Recent        []models.Attendance    `json:"recent"`
}

// ReportQuery holds raw report parameters.
type ReportQuery struct {
	AcademicYearID, ClassID, SectionID, StudentID, Date, Month, From, To string
}

// CSVExport is a ready-to-write CSV file.
type CSVExport struct {
	Filename string
	Header   []string
	Rows     [][]string
}

// ---- helpers ----

func (s *AttendanceService) scopeInfo(c *attendanceCaller) AttendanceScopeInfo {
	info := AttendanceScopeInfo{All: c.scope.All, Note: c.note}
	if c.teacher != nil {
		info.Teacher = &models.ActorRef{ID: c.teacher.ID, Name: c.teacher.FullName}
	}
	return info
}

// pickYear resolves the requested year among the caller's years, defaulting
// to the current year, else the newest. Nil when the caller has none.
func pickYear(years []models.YearRef, requested *uuid.UUID) (*models.YearRef, error) {
	if requested != nil {
		for i := range years {
			if years[i].ID == *requested {
				return &years[i], nil
			}
		}
		return nil, apperror.Validation(map[string]string{"academic_year_id": "That academic year doesn't exist or isn't available to you."})
	}
	for i := range years {
		if years[i].IsCurrent {
			return &years[i], nil
		}
	}
	if len(years) > 0 {
		return &years[0], nil
	}
	return nil, nil
}

// defaultDay is today when it falls within the year, else the nearest end.
func (s *AttendanceService) defaultDay(year models.YearRef) time.Time {
	today := s.today()
	start, end := parseDate(year.StartDate), parseDate(year.EndDate)
	switch {
	case today.Before(start):
		return start
	case today.After(end):
		return end
	}
	return today
}

// yearFor resolves a year the caller can use.
func (s *AttendanceService) yearFor(ctx context.Context, c *attendanceCaller, raw string) (*models.YearRef, []models.YearRef, error) {
	errs := validation.Errors{}
	requested := optionalID(errs, "academic_year_id", raw)
	if errs.Any() {
		return nil, nil, apperror.Validation(errs)
	}
	years, err := s.repo.Years(ctx, c.scope)
	if err != nil {
		return nil, nil, apperror.Internal(err)
	}
	year, err := pickYear(years, requested)
	return year, years, err
}

// clampRange limits a range to the academic year and today.
func (s *AttendanceService) clampRange(year models.YearRef, from, to *time.Time) (time.Time, time.Time) {
	start, end := parseDate(year.StartDate), parseDate(year.EndDate)
	if today := s.today(); today.Before(end) {
		end = today
	}
	f, t := start, end
	if from != nil && from.After(f) {
		f = *from
	}
	if to != nil && to.Before(t) {
		t = *to
	}
	return f, t
}

// ---- options ----

// Options lists the years and sections the caller can take attendance for.
func (s *AttendanceService) Options(ctx context.Context, actor *models.User, rawYear string) (*AttendanceOptions, error) {
	c, err := s.caller(ctx, actor)
	if err != nil {
		return nil, err
	}
	year, years, err := s.yearFor(ctx, c, rawYear)
	if err != nil {
		return nil, err
	}
	out := &AttendanceOptions{AcademicYears: years, AcademicYear: year, Today: s.today().Format(models.DateLayout),
		Classes: []AttendanceClassOption{}, Scope: s.scopeInfo(c), CanCorrect: c.canCorrect}
	if year == nil {
		return out, nil
	}
	out.DefaultDate = s.defaultDay(*year).Format(models.DateLayout)
	sections, err := s.repo.Sections(ctx, year.ID, c.scope)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	for _, o := range sections {
		n := len(out.Classes)
		if n == 0 || out.Classes[n-1].Class.ID != o.Class.ID {
			out.Classes = append(out.Classes, AttendanceClassOption{Class: o.Class, Sections: []models.SectionRef{}})
			n++
		}
		out.Classes[n-1].Sections = append(out.Classes[n-1].Sections, o.Section)
	}
	return out, nil
}

// ---- daily summary ----

func dayState(d repository.SectionDay) string {
	switch {
	case d.Counts.Marked() == 0 && d.Unmarked == 0:
		return DayNoStudents
	case d.Counts.Marked() == 0:
		return DayNotStarted
	case d.Unmarked > 0:
		return DayPartial
	}
	return DayComplete
}

// Day summarises a date across the caller's sections (optionally one class
// or section): the dashboard and the daily report.
func (s *AttendanceService) Day(ctx context.Context, actor *models.User, q ReportQuery) (*DaySummary, error) {
	c, err := s.caller(ctx, actor)
	if err != nil {
		return nil, err
	}
	errs := validation.Errors{}
	classID := optionalID(errs, "class_id", q.ClassID)
	sectionID := optionalID(errs, "section_id", q.SectionID)
	date := optionalDate(errs, "date", q.Date)
	if errs.Any() {
		return nil, apperror.Validation(errs)
	}
	year, _, err := s.yearFor(ctx, c, q.AcademicYearID)
	if err != nil {
		return nil, err
	}
	out := &DaySummary{AcademicYear: year, Today: s.today().Format(models.DateLayout), Classes: []ClassDayView{}, Sections: []SectionDayView{}, Scope: s.scopeInfo(c)}
	if year == nil {
		out.Date = out.Today
		return out, nil
	}
	day := s.defaultDay(*year)
	if date != nil {
		day = *date
		if err := s.checkSessionDate(day, *year); err != nil {
			return nil, err
		}
	}
	out.Date = day.Format(models.DateLayout)
	if sectionID != nil && !c.scope.Allows(*sectionID) {
		return nil, notAssigned()
	}
	days, err := s.repo.Day(ctx, year.ID, day, c.scope, classID, sectionID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	var total models.AttendanceCounts
	for _, d := range days {
		v := SectionDayView{Class: d.Class, Section: d.Section, Enrolled: d.Enrolled, AttendanceStats: d.Counts.Stats(),
			Unmarked: d.Unmarked, State: dayState(d), LastSavedAt: d.LastSavedAt}
		out.Sections = append(out.Sections, v)
		total.Present, total.Absent, total.Late, total.Leave = total.Present+d.Counts.Present, total.Absent+d.Counts.Absent, total.Late+d.Counts.Late, total.Leave+d.Counts.Leave
		out.Totals.Sections++
		out.Totals.Enrolled += d.Enrolled
		out.Totals.Unmarked += d.Unmarked
		switch v.State {
		case DayComplete:
			out.Totals.Completed++
		case DayPartial:
			out.Totals.Partial++
		case DayNotStarted:
			out.Totals.NotStarted++
		}
		n := len(out.Classes)
		if n == 0 || out.Classes[n-1].Class.ID != d.Class.ID {
			out.Classes = append(out.Classes, ClassDayView{Class: d.Class})
			n++
		}
		cl := &out.Classes[n-1]
		cl.Sections++
		cl.Enrolled += d.Enrolled
		cl.Unmarked += d.Unmarked
		cl.Present += d.Counts.Present
		cl.Absent += d.Counts.Absent
		cl.Late += d.Counts.Late
		cl.Leave += d.Counts.Leave
		if v.State == DayComplete {
			cl.SectionsCompleted++
		}
	}
	for i := range out.Classes {
		out.Classes[i].AttendanceStats = out.Classes[i].AttendanceCounts.Stats()
	}
	out.Totals.AttendanceStats = total.Stats()
	if sectionID != nil {
		sc, err := s.repo.LoadSession(ctx, *sectionID, day)
		if err != nil {
			return nil, mapAttendanceRepoError(err)
		}
		if sc.Year.ID != year.ID {
			return nil, checkYearMatches(&year.ID, sc)
		}
		out.Students = s.view(c, sc).Students
	}
	return out, nil
}

// ---- range reports ----

func (s *AttendanceService) rangeQuery(ctx context.Context, c *attendanceCaller, q ReportQuery, needClassOrSection bool) (*repository.RangeQuery, *models.YearRef, error) {
	errs := validation.Errors{}
	classID := optionalID(errs, "class_id", q.ClassID)
	sectionID := optionalID(errs, "section_id", q.SectionID)
	from := optionalDate(errs, "from", q.From)
	to := optionalDate(errs, "to", q.To)
	if from != nil && to != nil && to.Before(*from) {
		errs.Add("to", "The end date must be on or after the start date.")
	}
	if needClassOrSection && classID == nil && sectionID == nil {
		errs.Add("class_id", "Choose a class.")
	}
	if errs.Any() {
		return nil, nil, apperror.Validation(errs)
	}
	year, _, err := s.yearFor(ctx, c, q.AcademicYearID)
	if err != nil {
		return nil, nil, err
	}
	if year == nil {
		return nil, nil, apperror.Validation(map[string]string{"academic_year_id": "Add an academic year first."})
	}
	f, t := s.clampRange(*year, from, to)
	return &repository.RangeQuery{Scope: c.scope, AcademicYearID: year.ID, From: f, To: t, ClassID: classID, SectionID: sectionID}, year, nil
}

// ClassWise compares classes (or, with class_id, a class's sections) over a
// date range — the class-wise and date range reports.
func (s *AttendanceService) ClassWise(ctx context.Context, actor *models.User, q ReportQuery) (*RangeReport, error) {
	c, err := s.caller(ctx, actor)
	if err != nil {
		return nil, err
	}
	rq, year, err := s.rangeQuery(ctx, c, q, false)
	if err != nil {
		return nil, err
	}
	rep := &RangeReport{AcademicYear: *year, From: rq.From.Format(models.DateLayout), To: rq.To.Format(models.DateLayout),
		GroupBy: "class", Groups: []GroupStatsView{}, Daily: []DayStatsView{}}
	if rq.To.Before(rq.From) {
		rep.Totals = models.AttendanceCounts{}.Stats()
		return rep, nil // the year hasn't started yet
	}
	bySection := rq.ClassID != nil || rq.SectionID != nil
	if bySection {
		rep.GroupBy = "section"
	}
	groups, err := s.repo.GroupRange(ctx, *rq, bySection)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	daily, err := s.repo.DailyRange(ctx, *rq)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	var total models.AttendanceCounts
	for _, g := range groups {
		if rq.ClassID != nil && rep.Class == nil {
			cl := g.Class
			rep.Class = &cl
		}
		rep.Groups = append(rep.Groups, GroupStatsView{Class: g.Class, Section: g.Section, Sections: g.Sections, Enrolled: g.Enrolled,
			Sessions: g.Sessions, AttendanceStats: g.Counts.Stats()})
	}
	for _, d := range daily {
		rep.Daily = append(rep.Daily, DayStatsView{Date: d.Date, AttendanceStats: d.Counts.Stats()})
		total.Present += d.Counts.Present
		total.Absent += d.Counts.Absent
		total.Late += d.Counts.Late
		total.Leave += d.Counts.Leave
	}
	rep.Sessions = len(daily)
	rep.Totals = total.Stats()
	return rep, nil
}

// Monthly tallies each student of a class or section for one month.
func (s *AttendanceService) Monthly(ctx context.Context, actor *models.User, q ReportQuery) (*MonthlyReport, error) {
	c, err := s.caller(ctx, actor)
	if err != nil {
		return nil, err
	}
	month, err := time.Parse("2006-01", strings.TrimSpace(q.Month))
	if strings.TrimSpace(q.Month) == "" || err != nil {
		return nil, apperror.Validation(map[string]string{"month": "Choose a month (YYYY-MM)."})
	}
	first, last := month, month.AddDate(0, 1, -1)
	q.From, q.To = first.Format(models.DateLayout), last.Format(models.DateLayout)
	rq, year, err := s.rangeQuery(ctx, c, q, true)
	if err != nil {
		return nil, err
	}
	rep := &MonthlyReport{AcademicYear: *year, Month: month.Format("2006-01"), From: rq.From.Format(models.DateLayout),
		To: rq.To.Format(models.DateLayout), Students: []StudentStatsView{}}
	if rq.To.Before(rq.From) || last.Before(parseDate(year.StartDate)) || first.After(parseDate(year.EndDate)) {
		msg := fmt.Sprintf("%s has no school days of %s up to today.", month.Format("January 2006"), year.Name)
		return nil, apperror.Validation(map[string]string{"month": msg})
	}
	students, err := s.repo.StudentRange(ctx, *rq)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	daily, err := s.repo.DailyRange(ctx, *rq)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	var total models.AttendanceCounts
	for _, st := range students {
		c.redact(&st.Student)
		rep.Students = append(rep.Students, StudentStatsView{Student: st.Student, Class: st.Class, Section: st.Section,
			RollNumber: st.RollNumber, AttendanceStats: st.Counts.Stats()})
		total.Present += st.Counts.Present
		total.Absent += st.Counts.Absent
		total.Late += st.Counts.Late
		total.Leave += st.Counts.Leave
	}
	rep.Sessions = len(daily)
	rep.Totals = total.Stats()
	return rep, nil
}

// ---- students ----

// StudentSummary is a student's attendance in one academic year (default:
// the current year if they were enrolled in it, else their latest year).
func (s *AttendanceService) StudentSummary(ctx context.Context, studentID uuid.UUID, rawYear string) (*StudentAttendanceSummary, error) {
	errs := validation.Errors{}
	requested := optionalID(errs, "academic_year_id", rawYear)
	if errs.Any() {
		return nil, apperror.Validation(errs)
	}
	years, err := s.repo.StudentYears(ctx, studentID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, errStudentNotFound
	}
	if err != nil {
		return nil, apperror.Internal(err)
	}
	out := &StudentAttendanceSummary{AcademicYears: years, Months: []MonthStatsView{}, Recent: []models.Attendance{}, Stats: models.AttendanceCounts{}.Stats()}
	year, err := pickYear(years, requested)
	if err != nil || year == nil {
		return out, err
	}
	out.AcademicYear = year
	months, err := s.repo.StudentMonths(ctx, studentID, year.ID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	var total models.AttendanceCounts
	for _, m := range months {
		out.Months = append(out.Months, MonthStatsView{Month: m.Month, AttendanceStats: m.Counts.Stats()})
		total.Present += m.Counts.Present
		total.Absent += m.Counts.Absent
		total.Late += m.Counts.Late
		total.Leave += m.Counts.Leave
	}
	out.Stats = total.Stats()
	recent, _, err := s.repo.List(ctx, repository.AttendanceFilter{Scope: repository.AttendanceScope{All: true}, StudentID: &studentID,
		AcademicYearID: &year.ID, Page: 1, PageSize: 10})
	if err != nil {
		return nil, apperror.Internal(err)
	}
	out.Recent = recent
	return out, nil
}

// StudentHistory lists a student's records (administrators only).
func (s *AttendanceService) StudentHistory(ctx context.Context, actor *models.User, studentID uuid.UUID, in ListAttendanceInput) (*ListAttendanceResult, error) {
	if _, err := s.repo.StudentYears(ctx, studentID); errors.Is(err, repository.ErrNotFound) {
		return nil, errStudentNotFound
	} else if err != nil {
		return nil, apperror.Internal(err)
	}
	in.StudentID = studentID.String()
	return s.List(ctx, actor, in)
}

// ---- CSV exports ----

func pct(p *float64) string {
	if p == nil {
		return "N/A"
	}
	return strconv.FormatFloat(*p, 'f', 1, 64)
}

func statCells(st models.AttendanceStats) []string {
	return []string{strconv.Itoa(st.Present), strconv.Itoa(st.Absent), strconv.Itoa(st.Late), strconv.Itoa(st.Leave),
		strconv.Itoa(st.Marked), pct(st.Percentage)}
}

var statHeader = []string{"Present", "Absent", "Late", "Leave", "Marked", "Attendance %"}

func remarksCell(r *string) string {
	if r == nil {
		return ""
	}
	return *r
}

func actorName(a *models.ActorRef) string {
	if a == nil {
		return ""
	}
	return a.Name
}

func slug(parts ...string) string {
	out := strings.ToLower(strings.Join(parts, "-"))
	var b strings.Builder
	for _, r := range out {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-':
			b.WriteRune(r)
		case r == ' ' || r == '_' || r == '/':
			b.WriteRune('-')
		}
	}
	return strings.Trim(b.String(), "-")
}

// Export builds a CSV of a report with the same filters as the screen.
// kind: daily, monthly, class-wise, student or records.
func (s *AttendanceService) Export(ctx context.Context, actor *models.User, kind string, q ReportQuery, list ListAttendanceInput) (*CSVExport, error) {
	switch kind {
	case "daily":
		d, err := s.Day(ctx, actor, q)
		if err != nil {
			return nil, err
		}
		if d.Students != nil {
			exp := &CSVExport{Filename: slug("attendance", d.Date, "daily-section"),
				Header: []string{"Date", "Roll", "Admission no.", "Student", "Status", "Remarks", "Marked by", "Last updated"}}
			for _, st := range d.Students {
				row := []string{d.Date, strconv.Itoa(st.RollNumber), st.Student.AdmissionNumber, st.Student.FullName, "Unmarked", "", "", ""}
				if a := st.Attendance; a != nil {
					row[4], row[5], row[6], row[7] = strings.ToUpper(string(a.Status)), remarksCell(a.Remarks), actorName(a.UpdatedBy), a.UpdatedAt.UTC().Format(time.RFC3339)
				}
				if st.State == StudentElsewhere {
					row[4] = "Recorded elsewhere"
				}
				exp.Rows = append(exp.Rows, row)
			}
			return exp, nil
		}
		exp := &CSVExport{Filename: slug("attendance", d.Date, "daily"),
			Header: append([]string{"Date", "Class", "Section", "Enrolled"}, append(statHeader, "Unmarked", "State")...)}
		for _, sec := range d.Sections {
			row := append([]string{d.Date, sec.Class.Name, sec.Section.Name, strconv.Itoa(sec.Enrolled)}, statCells(sec.AttendanceStats)...)
			exp.Rows = append(exp.Rows, append(row, strconv.Itoa(sec.Unmarked), sec.State))
		}
		return exp, nil
	case "monthly":
		m, err := s.Monthly(ctx, actor, q)
		if err != nil {
			return nil, err
		}
		exp := &CSVExport{Filename: slug("attendance", m.Month, "monthly"),
			Header: append([]string{"Month", "Class", "Section", "Roll", "Admission no.", "Student"}, statHeader...)}
		for _, st := range m.Students {
			row := []string{m.Month, st.Class.Name, st.Section.Name, strconv.Itoa(st.RollNumber), st.Student.AdmissionNumber, st.Student.FullName}
			exp.Rows = append(exp.Rows, append(row, statCells(st.AttendanceStats)...))
		}
		return exp, nil
	case "class-wise":
		r, err := s.ClassWise(ctx, actor, q)
		if err != nil {
			return nil, err
		}
		exp := &CSVExport{Filename: slug("attendance", r.From, "to", r.To, "class-wise"),
			Header: append([]string{"From", "To", "Class", "Section", "Enrolled", "Days recorded"}, statHeader...)}
		for _, g := range r.Groups {
			section := "All sections"
			if g.Section != nil {
				section = g.Section.Name
			}
			row := []string{r.From, r.To, g.Class.Name, section, strconv.Itoa(g.Enrolled), strconv.Itoa(g.Sessions)}
			exp.Rows = append(exp.Rows, append(row, statCells(g.AttendanceStats)...))
		}
		return exp, nil
	case "student", "records":
		f, err := attendanceFilter(list)
		if err != nil {
			return nil, err
		}
		if kind == "student" && f.StudentID == nil {
			return nil, apperror.Validation(map[string]string{"student_id": "Choose a student."})
		}
		c, err := s.caller(ctx, actor)
		if err != nil {
			return nil, err
		}
		f.Scope, f.Limit = c.scope, maxExportRows
		items, total, err := s.repo.List(ctx, f)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		if total > maxExportRows {
			return nil, apperror.Validation(map[string]string{"date": fmt.Sprintf("This export has %d records. Narrow the filters to at most %d.", total, maxExportRows)})
		}
		name := "attendance-records"
		if kind == "student" && len(items) > 0 {
			name = slug("attendance", items[0].Student.AdmissionNumber, "history")
		}
		exp := &CSVExport{Filename: name,
			Header: []string{"Date", "Academic year", "Class", "Section", "Roll", "Admission no.", "Student", "Status", "Remarks", "Marked by", "Last updated by", "Last updated", "Corrections"}}
		for _, a := range items {
			exp.Rows = append(exp.Rows, []string{a.Date, a.AcademicYear.Name, a.Class.Name, a.Section.Name, strconv.Itoa(a.RollNumber),
				a.Student.AdmissionNumber, a.Student.FullName, strings.ToUpper(string(a.Status)), remarksCell(a.Remarks),
				actorName(a.MarkedBy), actorName(a.UpdatedBy), a.UpdatedAt.UTC().Format(time.RFC3339), strconv.Itoa(a.Changes)})
		}
		return exp, nil
	}
	return nil, apperror.Validation(map[string]string{"report": "Choose daily, monthly, class-wise, student or records."})
}
