package matching

import (
	"encoding/json"
	"os"
	"testing"

	"blotato-comment-automation/internal/feature/automation"
)

func TestSharedEditorFixtures(t *testing.T) {
	data, err := os.ReadFile("../../../../frontend/src/feature/automation/model/testdata/matching.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []struct {
		Text, Term, Operator string
		CaseSensitive        bool `json:"case_sensitive"`
		Expected             bool
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, f := range fixtures {
		t.Run(f.Text+"/"+f.Operator, func(t *testing.T) {
			p, err := compile(automation.Predicate{Mode: "keywords", Operator: f.Operator, Logic: "any", Keywords: []string{f.Term}, CaseSensitive: f.CaseSensitive})
			if err != nil {
				t.Fatal(err)
			}
			if got := p.matches(normalize(f.Text, f.CaseSensitive)); got != f.Expected {
				t.Fatalf("got %v want %v", got, f.Expected)
			}
		})
	}
}
func TestLogicAndExclusions(t *testing.T) {
	p := automation.Predicate{Mode: "keywords", Operator: "whole_word", Logic: "all", Keywords: []string{"pricing", "guide"}, Exclusions: []string{"not interested"}}
	for _, tc := range []struct {
		text  string
		match bool
	}{{"pricing", false}, {"pricing guide", true}, {"pricing guide, not   interested", false}, {"pricing guide not interestedness", true}} {
		c, err := compile(p)
		if err != nil {
			t.Fatal(err)
		}
		if c.matches(normalize(tc.text, false)) != tc.match {
			t.Fatal(tc.text)
		}
	}
	p.Mode = "any"
	c, _ := compile(p)
	if c.matches("not interested") || !c.matches("hello") {
		t.Fatal("any mode must honor exclusions")
	}
}
func BenchmarkPredicate(b *testing.B) {
	p, _ := compile(automation.Predicate{Mode: "keywords", Operator: "whole_word", Logic: "any", Keywords: []string{"pricing", "price list"}})
	for b.Loop() {
		p.matches("thanks for sharing this helpful article!")
	}
}
