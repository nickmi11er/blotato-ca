package matching

import (
	"regexp"
	"strings"

	"blotato-comment-automation/internal/feature/automation"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"golang.org/x/text/unicode/norm"
)

// ECMAScript whitespace keeps backend matching aligned with the editor.
const whitespace = `[\t\n\v\f\r \x{00a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}\x{feff}]+`

var spaces = regexp.MustCompile(whitespace)

func jsSpace(r rune) bool {
	return r >= 9 && r <= 13 || r == 32 || r == 0xa0 || r == 0x1680 || r >= 0x2000 && r <= 0x200a || r == 0x2028 || r == 0x2029 || r == 0x202f || r == 0x205f || r == 0x3000 || r == 0xfeff
}
func normalize(s string, sensitive bool) string {
	s = strings.TrimFunc(norm.NFC.String(s), jsSpace)
	if !sensitive {
		s = cases.Lower(language.Und).String(s)
	}
	return s
}

type term struct {
	value string
	word  *regexp.Regexp
}
type predicate struct {
	mode, operator, logic string
	sensitive             bool
	keywords, exclusions  []term
}

func compileTerm(s, operator string, sensitive bool) term {
	s = normalize(s, sensitive)
	t := term{value: s}
	if operator == "whole_word" {
		parts := spaces.Split(s, -1)
		for i := range parts {
			parts[i] = regexp.QuoteMeta(parts[i])
		}
		t.word = regexp.MustCompile(`(?:^|[^\p{L}\p{N}\p{M}_])` + strings.Join(parts, whitespace) + `(?:$|[^\p{L}\p{N}\p{M}_])`)
	}
	return t
}
func compile(p automation.Predicate) (*predicate, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	result := &predicate{mode: p.Mode, operator: p.Operator, logic: p.Logic, sensitive: p.CaseSensitive}
	for _, s := range p.Keywords {
		result.keywords = append(result.keywords, compileTerm(s, p.Operator, p.CaseSensitive))
	}
	for _, s := range p.Exclusions {
		result.exclusions = append(result.exclusions, compileTerm(s, "whole_word", p.CaseSensitive))
	}
	return result, nil
}
func (t term) matches(text, operator string) bool {
	if t.value == "" {
		return false
	}
	switch operator {
	case "exact":
		return text == t.value
	case "contains":
		return strings.Contains(text, t.value)
	default:
		return t.word.MatchString(text)
	}
}
func (p *predicate) matches(text string) bool {
	for _, t := range p.exclusions {
		if t.matches(text, "whole_word") {
			return false
		}
	}
	if p.mode == "any" {
		return true
	}
	for _, t := range p.keywords {
		matched := t.matches(text, p.operator)
		if matched && p.logic == "any" {
			return true
		}
		if !matched && p.logic == "all" {
			return false
		}
	}
	return p.logic == "all"
}
