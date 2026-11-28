package models

import (
	"errors"
	"strconv"
	"strings"
)

// Marks is an amount of marks in hundredths (45.5 marks = 4550). Using whole
// numbers keeps grade boundaries exact: no floating-point rounding can push a
// percentage across a boundary.
type Marks int64

// MaxMarks is the largest marks value stored (NUMERIC(6,2)).
const MaxMarks Marks = 999999

var (
	ErrMarksFormat   = errors.New("marks must be a number with at most two decimals")
	ErrMarksNegative = errors.New("marks can't be negative")
	ErrMarksTooLarge = errors.New("marks are too large")
)

// ParseMarks parses "45", "45.5" or "45.25" (no sign, at most two decimals).
func ParseMarks(s string) (Marks, error) {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "-") {
		if _, err := strconv.ParseFloat(s, 64); err == nil {
			return 0, ErrMarksNegative
		}
		return 0, ErrMarksFormat
	}
	whole, frac, hasDot := strings.Cut(s, ".")
	if whole == "" || len(frac) > 2 || (hasDot && frac == "") || len(whole) > 6 {
		if len(whole) > 6 {
			return 0, ErrMarksTooLarge
		}
		return 0, ErrMarksFormat
	}
	for _, r := range whole + frac {
		if r < '0' || r > '9' {
			return 0, ErrMarksFormat
		}
	}
	for len(frac) < 2 {
		frac += "0"
	}
	w, _ := strconv.ParseInt(whole, 10, 64)
	f, _ := strconv.ParseInt(frac, 10, 64)
	m := Marks(w*100 + f)
	if m > MaxMarks {
		return 0, ErrMarksTooLarge
	}
	return m, nil
}

// String formats marks without trailing zeros: "45", "45.5", "45.25".
func (m Marks) String() string {
	neg := m < 0
	if neg {
		m = -m
	}
	s := strconv.FormatInt(int64(m)/100, 10)
	if f := int64(m) % 100; f != 0 {
		fs := strconv.FormatInt(f, 10)
		if f < 10 {
			fs = "0" + fs
		}
		s += "." + strings.TrimRight(fs, "0")
	}
	if neg {
		s = "-" + s
	}
	return s
}

// MarshalJSON writes marks as a JSON number.
func (m Marks) MarshalJSON() ([]byte, error) { return []byte(m.String()), nil }

// Grade is a letter grade with its grade point (in hundredths: 5.00 = 500).
type Grade struct {
	Letter string
	Point  int64
}

// gradeBands are the Bangladesh grading boundaries, highest first: a
// percentage at or above MinPercent earns the grade.
var gradeBands = []struct {
	MinPercent int64
	Grade      Grade
}{
	{80, Grade{"A+", 500}},
	{70, Grade{"A", 400}},
	{60, Grade{"A-", 350}},
	{50, Grade{"B", 300}},
	{40, Grade{"C", 200}},
	{33, Grade{"D", 100}},
}

// GradeF is the failing grade.
var GradeF = Grade{"F", 0}

// GradeFor returns the grade for obtained out of full marks. The exact
// percentage is compared with each boundary (obtained × 100 ≥ boundary ×
// full), so 79.99% is an A, never rounded up to A+. Zero full marks give F.
func GradeFor(obtained, full Marks) Grade {
	if full <= 0 || obtained < 0 {
		return GradeF
	}
	for _, b := range gradeBands {
		if int64(obtained)*100 >= b.MinPercent*int64(full) {
			return b.Grade
		}
	}
	return GradeF
}

// PercentHundredths is obtained/full × 100 in hundredths of a percent,
// rounded half up (for display and storage only; grading uses GradeFor).
func PercentHundredths(obtained, full Marks) int64 {
	if full <= 0 {
		return 0
	}
	return (int64(obtained)*10000*2 + int64(full)) / (2 * int64(full))
}

// SubjectOutcome is one subject's result.
type SubjectOutcome struct {
	Grade  Grade
	Passed bool
}

// SubjectResultFor grades one subject. A subject fails when the student was
// absent, scored below the configured pass marks, or earned an F by
// percentage; a failed subject always gets F and 0.00.
func SubjectResultFor(obtained, full, pass Marks, absent bool) SubjectOutcome {
	if absent || full <= 0 || obtained < pass {
		return SubjectOutcome{Grade: GradeF}
	}
	g := GradeFor(obtained, full)
	if g.Point == 0 {
		return SubjectOutcome{Grade: GradeF}
	}
	return SubjectOutcome{Grade: g, Passed: true}
}

// GPAFor averages grade points (hundredths), rounding half up to two
// decimals: 4.375 → 4.38. If any subject failed the GPA is 0.00 and the
// overall grade F (the Classora fail rule). No subjects also gives 0.00/F.
func GPAFor(subjects []SubjectOutcome) (gpa int64, grade string, passed bool) {
	if len(subjects) == 0 {
		return 0, "F", false
	}
	var sum int64
	for _, s := range subjects {
		if !s.Passed {
			return 0, "F", false
		}
		sum += s.Grade.Point
	}
	n := int64(len(subjects))
	gpa = (2*sum + n) / (2 * n)
	return gpa, OverallGradeFor(gpa), true
}

// OverallGradeFor maps a GPA (hundredths) to a letter grade.
func OverallGradeFor(gpa int64) string {
	switch {
	case gpa >= 500:
		return "A+"
	case gpa >= 400:
		return "A"
	case gpa >= 350:
		return "A-"
	case gpa >= 300:
		return "B"
	case gpa >= 200:
		return "C"
	case gpa >= 100:
		return "D"
	}
	return "F"
}

// RankEntry is one student to rank.
type RankEntry struct {
	Passed bool
	Total  Marks
	GPA    int64
}

// Rank assigns merit positions using standard competition ranking
// ("1, 2, 2, 4"): passed students only, ordered by total marks then GPA;
// equal totals and GPAs share a position. Failed students get no position.
// The input must already be sorted with SortForMerit.
func Rank(entries []RankEntry) []int {
	pos := make([]int, len(entries))
	rank := 0
	for i, e := range entries {
		if !e.Passed {
			continue
		}
		if i == 0 || !entries[i-1].Passed || entries[i-1].Total != e.Total || entries[i-1].GPA != e.GPA {
			rank = i + 1
		}
		pos[i] = rank
	}
	return pos
}

// MeritLess orders passed before failed, then higher total, then higher GPA.
func MeritLess(a, b RankEntry) bool {
	if a.Passed != b.Passed {
		return a.Passed
	}
	if a.Total != b.Total {
		return a.Total > b.Total
	}
	return a.GPA > b.GPA
}
