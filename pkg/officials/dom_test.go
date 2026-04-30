package officials

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

// expectedTownWide is the full roster captured from the testdata fixture.
// Update alongside testdata/town_wide.html when refreshing the snapshot.
var expectedTownWide = []Committee{
	{Name: "Select Board & Surveyor of the Highway", Members: []string{
		"Winston Daley",
		"Meghan Haggerty",
		"John Driscoll",
		"Peter Obersheimer",
		"Megan Nolan",
	}},
	{Name: "Town Clerk", Members: []string{"Susan M. Galvin"}},
	{Name: "Board of Assessors", Members: []string{
		"William E. Bennett",
		"Brian Manning Cronin",
		"C. Robert Reetz",
	}},
	{Name: "School Committee", Members: []string{
		"Elizabeth Marshall Carroll",
		"Beverly Ross Denny",
		"Mark W. Loring",
		"Celina Miranda",
		"Bao Qiu",
		"Annamma Varghese",
	}},
	{Name: "Blue Hills Regional Technical School", Members: []string{"Marybeth Joyce"}},
	{Name: "Park Commissioners", Members: []string{
		"Theodore G. Carroll",
		"Robert Levash",
		"Carolyn Cahill",
	}},
	{Name: "Board of Health", Members: []string{
		"Roxanne F. Musto",
		"Mary F. Stenson",
		"Laura T. Richards",
	}},
	{Name: "Trustees of the Public Library", Members: []string{
		"Hyacinth Crichlow",
		"Sean Bentley",
		"John W. Folcarelli",
		"Paul Sitton Hays",
		"Kristine R. Hodlin",
		"Jaime Leigh Levash",
		"Sindu M. Meier",
		"Susan Doyle",
		"James C. Potter",
	}},
	{Name: "Constables", Members: []string{
		"Tamara A. Berton",
		"Eric Issner",
		"William J. Neville",
		"Kevin Chrisom Jr",
	}},
	{Name: "Trustees of the Cemetery", Members: []string{
		"James A. Coyne",
		"Jed Dolan",
		"Terence J. Driscoll",
		"Stephen J. Pender",
		"Joseph M. Reardon",
	}},
	{Name: "Town Moderator", Members: []string{"Elizabeth Dillon"}},
	{Name: "Housing Authority", Members: []string{
		"Lee B. Cary",
		"Joseph A. Duffy, Jr.",
		"June Elam-Mooers",
		"Marilynn Morgan",
		"Marilynn Morgan", // intentionally duplicated on the source page
		"Robert E. Powers, Jr.",
	}},
	{Name: "Planning Board", Members: []string{
		"Jim P. Davis",
		"Sean P. Fahy",
		"Meredith M. Hall",
		"Margaret Teresa Oldfield",
		"Cheryl Friedman Tougias",
	}},
}

func TestParseTownWideDOM_Fixture(t *testing.T) {
	f, err := os.Open("testdata/town_wide.html")
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer f.Close()

	got, err := ParseTownWideDOM(f)
	if err != nil {
		t.Fatalf("ParseTownWideDOM: %v", err)
	}

	if len(got) != len(expectedTownWide) {
		t.Fatalf("committee count = %d, want %d", len(got), len(expectedTownWide))
	}
	for i, want := range expectedTownWide {
		if got[i].Name != want.Name {
			t.Errorf("committee[%d].Name = %q, want %q", i, got[i].Name, want.Name)
		}
		if !reflect.DeepEqual(got[i].Members, want.Members) {
			t.Errorf("committee[%d] (%s) members = %v, want %v",
				i, want.Name, got[i].Members, want.Members)
		}
	}
}

func TestParseTownWideDOM_BlueHillsFallback(t *testing.T) {
	// Sanity check: the only committee using the <h3 class="subhead2">
	// fallback layout should still come through.
	f, err := os.Open("testdata/town_wide.html")
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer f.Close()
	got, err := ParseTownWideDOM(f)
	if err != nil {
		t.Fatalf("ParseTownWideDOM: %v", err)
	}
	for _, c := range got {
		if c.Name == "Blue Hills Regional Technical School" {
			if len(c.Members) != 1 || c.Members[0] != "Marybeth Joyce" {
				t.Errorf("Blue Hills members = %v, want [Marybeth Joyce]", c.Members)
			}
			return
		}
	}
	t.Errorf("Blue Hills committee not found")
}

func TestParseTownWideDOM_DoubleSpaceCollapsed(t *testing.T) {
	// "Meghan  Haggerty" appears with two spaces in the source.
	f, err := os.Open("testdata/town_wide.html")
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer f.Close()
	got, err := ParseTownWideDOM(f)
	if err != nil {
		t.Fatalf("ParseTownWideDOM: %v", err)
	}
	for _, m := range got[0].Members {
		if strings.Contains(m, "  ") {
			t.Errorf("member %q still contains a double space", m)
		}
		if m == "Meghan Haggerty" {
			return
		}
	}
	t.Errorf("Meghan Haggerty not found in first committee: %v", got[0].Members)
}

func TestNormalizeCommitteeName(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{" School COMMITTEE", "School Committee"},
		{"Board of Assessors", "Board of Assessors"},
		{"Select Board & Surveyor of the Highway", "Select Board & Surveyor of the Highway"},
		{" Blue Hills Regional Technical School", "Blue Hills Regional Technical School"},
		{"  TOWN   moderator  ", "Town Moderator"},
		{"trustees OF the cemetery", "Trustees of the Cemetery"},
		{"", ""},
	}
	for _, c := range cases {
		if got := normalizeCommitteeName(c.in); got != c.want {
			t.Errorf("normalizeCommitteeName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestCollapseWhitespace(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"Meghan  Haggerty", "Meghan Haggerty"},
		{"  leading and trailing  ", "leading and trailing"},
		{"a\tb\nc", "a b c"},
		{"", ""},
	}
	for _, c := range cases {
		if got := collapseWhitespace(c.in); got != c.want {
			t.Errorf("collapseWhitespace(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
