package workflow

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

type Definition struct {
	SchemaVersion int                 `json:"schema_version,omitempty"`
	Steps         []Step              `json:"steps"`
	Templates     map[string]Template `json:"templates,omitempty"`
}
type Template struct {
	Revision int64  `json:"revision"`
	Subject  string `json:"subject"`
	Body     string `json:"body"`
}
type Step struct {
	ID              string `json:"id"`
	Kind            string `json:"kind"`
	Text            string `json:"text,omitempty"`
	TemplateID      string `json:"templateId,omitempty"`
	Actions         []Step `json:"actions,omitempty"`
	Expected        string `json:"expected,omitempty"`
	TimeoutHours    int    `json:"timeoutHours,omitempty"`
	InvalidBehavior string `json:"invalidBehavior,omitempty"`
	RetryMessage    string `json:"retryMessage,omitempty"`
	MaxAttempts     int    `json:"maxAttempts,omitempty"`
	TimeoutBehavior string `json:"timeoutBehavior,omitempty"`
	TimeoutSteps    []Step `json:"timeoutSteps,omitempty"`
	TimeoutMessage  string `json:"timeoutMessage,omitempty"`
	PromptNodeID    string `json:"prompt_node_id,omitempty"`
	Minutes         int    `json:"minutes,omitempty"`
	At              string `json:"at,omitempty"`
	Timezone        string `json:"timezone,omitempty"`
	Mode            string `json:"mode,omitempty"`
	Time            string `json:"time,omitempty"`
}
type Node struct {
	Step        Step     `json:"step"`
	Next        string   `json:"next,omitempty"`
	TimeoutNext string   `json:"timeout_next,omitempty"`
	Parent      string   `json:"parent,omitempty"`
	Children    []string `json:"children,omitempty"`
	ArmWait     string   `json:"arm_wait,omitempty"`
}
type Plan struct {
	Version   int                 `json:"version"`
	First     string              `json:"first"`
	Nodes     map[string]Node     `json:"nodes"`
	Templates map[string]Template `json:"templates,omitempty"`
	HasReply  bool                `json:"has_reply"`
}

func Compile(raw []byte) (Plan, error) {
	var d Definition
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&d); err != nil {
		return Plan{}, fmt.Errorf("workflow: %w", err)
	}
	if dec.Decode(&struct{}{}) != io.EOF {
		return Plan{}, errors.New("workflow must contain one JSON object")
	}
	if d.SchemaVersion != 0 && d.SchemaVersion != 1 {
		return Plan{}, errors.New("unsupported workflow version")
	}
	if len(d.Steps) == 0 {
		return Plan{}, errors.New("workflow needs at least one step")
	}
	if len(d.Templates) > 100 {
		return Plan{}, errors.New("too many template snapshots")
	}
	for id, t := range d.Templates {
		if id == "" || t.Revision < 1 || strings.TrimSpace(t.Subject) == "" || len(t.Subject) > 200 || strings.TrimSpace(t.Body) == "" || len(t.Body) > 10000 || strings.ContainsAny(t.Subject, "\r\n") {
			return Plan{}, fmt.Errorf("invalid template %s", id)
		}
		if err := validateVariables(t.Subject + " " + t.Body); err != nil {
			return Plan{}, err
		}
	}
	p := Plan{Version: 1, Nodes: map[string]Node{}, Templates: d.Templates}
	usedPrompts := map[string]bool{}
	var sequence func([]Step, int, bool, []string) (string, error)
	add := func(s Step, parent string) error {
		if s.ID == "" || len(s.ID) > 100 || strings.ContainsRune(s.ID, 0) {
			return errors.New("invalid step ID")
		}
		if _, ok := p.Nodes[s.ID]; ok {
			return fmt.Errorf("duplicate step ID %s", s.ID)
		}
		if len(p.Nodes) >= 160 {
			return errors.New("too many workflow nodes")
		}
		p.Nodes[s.ID] = Node{Step: s, Parent: parent}
		return nil
	}
	action := func(s Step, hasEmail bool) error {
		switch s.Kind {
		case "reply", "dm":
			if strings.TrimSpace(s.Text) == "" || len(s.Text) > 4000 {
				return fmt.Errorf("%s needs message text", s.ID)
			}
			return validateVariables(s.Text)
		case "email":
			if !hasEmail {
				return fmt.Errorf("%s needs an earlier successful email capture", s.ID)
			}
			if _, ok := p.Templates[s.TemplateID]; !ok {
				return fmt.Errorf("%s needs a pinned email template", s.ID)
			}
			return nil
		default:
			return errors.New("parallel groups can contain only actions")
		}
	}
	sequence = func(steps []Step, depth int, hasEmail bool, prompts []string) (string, error) {
		if depth > 3 || len(steps) > 40 {
			return "", errors.New("workflow exceeds nesting or sequence limit")
		}
		first := ""
		previous := ""
		for _, s := range steps {
			if err := add(s, ""); err != nil {
				return "", err
			}
			if first == "" {
				first = s.ID
			}
			if previous != "" {
				n := p.Nodes[previous]
				n.Next = s.ID
				p.Nodes[previous] = n
			}
			previous = s.ID
			n := p.Nodes[s.ID]
			switch s.Kind {
			case "reply", "dm", "email":
				if err := action(s, hasEmail); err != nil {
					return "", err
				}
				if s.Kind == "dm" {
					prompts = []string{s.ID}
				}
			case "parallel":
				if len(s.Actions) < 2 || len(s.Actions) > 8 {
					return "", errors.New("parallel needs 2–8 actions")
				}
				var groupPrompts []string
				for _, a := range s.Actions {
					if err := add(a, s.ID); err != nil {
						return "", err
					}
					if err := action(a, hasEmail); err != nil {
						return "", err
					}
					n.Children = append(n.Children, a.ID)
					if a.Kind == "dm" {
						groupPrompts = append(groupPrompts, a.ID)
					}
				}
				if len(groupPrompts) > 0 {
					prompts = groupPrompts
				}
			case "wait_reply":
				p.HasReply = true
				if s.Expected != "email" && s.Expected != "text" {
					return "", errors.New("invalid reply expectation")
				}
				if s.TimeoutHours < 1 || s.TimeoutHours > 720 {
					return "", errors.New("reply timeout must be 1–720 hours")
				}
				if s.InvalidBehavior != "stop" && s.InvalidBehavior != "retry" {
					return "", errors.New("invalid reply policy")
				}
				if s.InvalidBehavior == "retry" && (s.MaxAttempts < 1 || s.MaxAttempts > 10 || strings.TrimSpace(s.RetryMessage) == "" || len(s.RetryMessage) > 4000) {
					return "", errors.New("invalid retry policy")
				}
				if err := validateVariables(s.RetryMessage + s.TimeoutMessage); err != nil {
					return "", err
				}
				if s.PromptNodeID == "" && len(prompts) == 1 {
					s.PromptNodeID = prompts[0]
				}
				if s.PromptNodeID == "" && len(prompts) > 1 {
					return "", errors.New("choose prompt_node_id for an ambiguous question")
				}
				if s.PromptNodeID != "" {
					found := false
					for _, id := range prompts {
						found = found || id == s.PromptNodeID
					}
					if !found || usedPrompts[s.PromptNodeID] {
						return "", errors.New("reply wait needs an unused preceding DM prompt")
					}
					usedPrompts[s.PromptNodeID] = true
					question := p.Nodes[s.PromptNodeID]
					question.ArmWait = s.ID
					p.Nodes[s.PromptNodeID] = question
				}
				n.Step.PromptNodeID = s.PromptNodeID
				switch s.TimeoutBehavior {
				case "stop":
				case "dm":
					if strings.TrimSpace(s.TimeoutMessage) == "" || len(s.TimeoutMessage) > 4000 {
						return "", errors.New("timeout DM text is required")
					}
				case "branch":
					if len(s.TimeoutSteps) == 0 {
						return "", errors.New("timeout branch cannot be empty")
					}
					branch, err := sequence(s.TimeoutSteps, depth+1, hasEmail, nil)
					if err != nil {
						return "", err
					}
					n.TimeoutNext = branch
				default:
					return "", errors.New("invalid timeout policy")
				}
				if s.Expected == "email" {
					hasEmail = true
				}
				prompts = nil
			case "delay":
				if s.Minutes < 1 || s.Minutes > 43200 {
					return "", errors.New("delay must be 1–43200 minutes")
				}
			case "schedule":
				if _, err := time.LoadLocation(s.Timezone); err != nil {
					return "", errors.New("invalid schedule timezone")
				}
				if s.Mode == "next_time" {
					if !wallTime.MatchString(s.Time) {
						return "", errors.New("invalid wall-clock time")
					}
				} else if s.Mode == "" || s.Mode == "date" {
					if _, err := time.Parse(time.RFC3339, s.At); err != nil {
						return "", errors.New("schedule needs an RFC3339 instant")
					}
				} else {
					return "", errors.New("invalid schedule mode")
				}
			default:
				return "", fmt.Errorf("unsupported step kind %s", s.Kind)
			}
			// Nested definitions live only once in the compiled node map.
			n.Step.Actions = nil
			n.Step.TimeoutSteps = nil
			p.Nodes[s.ID] = n
		}
		return first, nil
	}
	first, err := sequence(d.Steps, 0, false, nil)
	p.First = first
	return p, err
}
func validateVariables(s string) error {
	for _, variable := range variables.FindAllString(s, -1) {
		if variable != "{{email}}" && variable != "{{first_name}}" {
			return fmt.Errorf("unsupported variable %s", variable)
		}
	}
	return nil
}
func Render(s, email, firstName string) string {
	if firstName == "" {
		firstName = "there"
	}
	return strings.NewReplacer("{{email}}", email, "{{first_name}}", firstName).Replace(s)
}
