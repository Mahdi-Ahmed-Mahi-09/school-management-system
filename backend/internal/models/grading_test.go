package models

import (
	"sort"
	"testing"
)

func mk(s string) Marks {
	m, err := ParseMarks(s)
	if err != nil {
		panic(err)
	}
	return m
}

func TestParseMarks(t *testing.T) {
	ok := map[string]Marks{"0": 0, "45": 4500, "45.5": 4550, "45.25": 4525, " 7.05 ": 705, "100.00": 10000}
	for in, want := range ok {
		if got, err := ParseMarks(in); err != nil || got != want {
			t.Errorf("ParseMarks(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	bad := map[string]error{"-1": ErrMarksNegative, "-0.5": ErrMarksNegative, "1.234": ErrMarksFormat, "abc": ErrMarksFormat, "": ErrMarksFormat,
		"1e2": ErrMarksFormat, "1.": ErrMarksFormat, ".5": ErrMarksFormat, "10000000": ErrMarksTooLarge}
	for in, want := range bad {
		if _, err := ParseMarks(in); err != want {
			t.Errorf("ParseMarks(%q) error = %v, want %v", in, err, want)
		}
	}
	for m, want := range map[Marks]string{4500: "45", 4550: "45.5", 4525: "45.25", 705: "7.05", 0: "0"} {
		if m.String() != want {
			t.Errorf("%d.String() = %q, want %q", m, m.String(), want)
		}
	}
}

func TestGradeBoundaries(t *testing.T) {
	full := mk("100")
	cases := []struct {
		obtained string
		letter   string
		point    int64
	}{
		{"100", "A+", 500}, {"80", "A+", 500}, {"79.99", "A", 400}, {"70", "A", 400}, {"69.99", "A-", 350},
		{"60", "A-", 350}, {"59.99", "B", 300}, {"50", "B", 300}, {"49.99", "C", 200}, {"40", "C", 200},
		{"39.99", "D", 100}, {"33", "D", 100}, {"32.99", "F", 0}, {"0", "F", 0},
	}
	for _, c := range cases {
		if g := GradeFor(mk(c.obtained), full); g.Letter != c.letter || g.Point != c.point {
			t.Errorf("%s/100 = %+v, want %s %d", c.obtained, g, c.letter, c.point)
		}
	}
	// Non-100 full marks: 39.5 of 50 is 79% (A), 40/50 is exactly 80% (A+).
	if g := GradeFor(mk("39.5"), mk("50")); g.Letter != "A" {
		t.Errorf("39.5/50 = %s, want A", g.Letter)
	}
	if g := GradeFor(mk("40"), mk("50")); g.Letter != "A+" {
		t.Errorf("40/50 = %s, want A+", g.Letter)
	}
	// 59.99% must not round up to 60%: 23.99/40 = 59.975%.
	if g := GradeFor(mk("23.99"), mk("40")); g.Letter != "B" {
		t.Errorf("23.99/40 = %s, want B", g.Letter)
	}
	// 33% boundary with odd full marks: 24.75/75 = 33% exactly, 24.74 below.
	if GradeFor(mk("24.75"), mk("75")).Letter != "D" || GradeFor(mk("24.74"), mk("75")).Letter != "F" {
		t.Error("33% boundary on 75 full marks")
	}
	if g := GradeFor(mk("10"), 0); g != GradeF {
		t.Error("zero full marks must be F, not divide by zero")
	}
	if p := PercentHundredths(mk("2"), mk("3")); p != 6667 {
		t.Errorf("2/3 = %d hundredths, want 6667", p)
	}
	if PercentHundredths(mk("1"), 0) != 0 {
		t.Error("percentage with zero full marks")
	}
}

func TestSubjectPassRules(t *testing.T) {
	full := mk("100")
	if r := SubjectResultFor(mk("90"), full, mk("33"), true); r.Passed || r.Grade != GradeF {
		t.Errorf("absent must fail: %+v", r)
	}
	// Stricter pass marks: 38% is a D by percentage but fails a 40-mark pass line.
	if r := SubjectResultFor(mk("38"), full, mk("40"), false); r.Passed || r.Grade != GradeF {
		t.Errorf("below pass marks must fail with F/0.00: %+v", r)
	}
	if r := SubjectResultFor(mk("40"), full, mk("40"), false); !r.Passed || r.Grade.Letter != "C" {
		t.Errorf("at pass marks: %+v", r)
	}
	// Lenient pass marks still fail below 33%.
	if r := SubjectResultFor(mk("30"), full, mk("25"), false); r.Passed {
		t.Errorf("F by percentage must fail: %+v", r)
	}
}

func TestGPA(t *testing.T) {
	pass := func(letter string, pt int64) SubjectOutcome {
		return SubjectOutcome{Grade: Grade{letter, pt}, Passed: true}
	}
	gpa, grade, ok := GPAFor([]SubjectOutcome{pass("A+", 500), pass("A", 400), pass("A+", 500), pass("A-", 350)})
	if gpa != 438 || grade != "A" || !ok {
		t.Errorf("4.375 → %d %s %v, want 438 A true", gpa, grade, ok)
	}
	gpa, grade, ok = GPAFor([]SubjectOutcome{pass("A+", 500), pass("A", 400), {Grade: GradeF}, pass("A", 400)})
	if gpa != 0 || grade != "F" || ok {
		t.Errorf("one failed subject must give 0.00 F fail, got %d %s %v", gpa, grade, ok)
	}
	if gpa, grade, _ := GPAFor([]SubjectOutcome{pass("A+", 500), pass("A+", 500)}); gpa != 500 || grade != "A+" {
		t.Errorf("all A+ = %d %s", gpa, grade)
	}
	// 3.4999… rounds to 3.50: (350+350+349.97)… use points 350, 350, 300, 400 = 3.50.
	if gpa, grade, _ := GPAFor([]SubjectOutcome{pass("A-", 350), pass("A-", 350), pass("B", 300), pass("A", 400)}); gpa != 350 || grade != "A-" {
		t.Errorf("3.50 = %d %s", gpa, grade)
	}
	// 3.3333… rounds down to 3.33 → B.
	if gpa, grade, _ := GPAFor([]SubjectOutcome{pass("A", 400), pass("B", 300), pass("B", 300)}); gpa != 333 || grade != "B" {
		t.Errorf("3.333 = %d %s", gpa, grade)
	}
	if gpa, grade, ok := GPAFor(nil); gpa != 0 || grade != "F" || ok {
		t.Error("no subjects must not divide by zero")
	}
	for gpa, want := range map[int64]string{500: "A+", 499: "A", 400: "A", 399: "A-", 350: "A-", 349: "B", 300: "B", 299: "C", 200: "C", 199: "D", 100: "D", 99: "F"} {
		if OverallGradeFor(gpa) != want {
			t.Errorf("OverallGradeFor(%d) = %s, want %s", gpa, OverallGradeFor(gpa), want)
		}
	}
}

func TestRank(t *testing.T) {
	entries := []RankEntry{
		{Passed: false, Total: 9000, GPA: 0}, // failed, highest marks: must not rank
		{Passed: true, Total: 7000, GPA: 400},
		{Passed: true, Total: 8000, GPA: 450},
		{Passed: true, Total: 7000, GPA: 400}, // tie with the other 7000/4.00
		{Passed: true, Total: 7000, GPA: 350}, // same total, lower GPA
		{Passed: true, Total: 6000, GPA: 300},
	}
	sort.SliceStable(entries, func(i, j int) bool { return MeritLess(entries[i], entries[j]) })
	pos := Rank(entries)
	want := []int{1, 2, 2, 4, 5, 0}
	for i := range want {
		if pos[i] != want[i] {
			t.Fatalf("positions = %v, want %v (entries %+v)", pos, want, entries)
		}
	}
	if entries[5].Passed {
		t.Error("failed student must be last")
	}
}
