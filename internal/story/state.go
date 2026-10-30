package story

// State is the assumption ledger.
//
// Each ledger exists because it answers a specific class of continuity failure
// that a language model makes routinely: forgetting what a character knows,
// resurrecting the dead, teleporting an item, flattening a relationship, or
// dropping the counterfactual the whole branch is built on.
type State struct {
	// Assumptions are the branch's counterfactuals, e.g. "真相未被揭露".
	// They must be injected into every scene, which is why they are first.
	Assumptions []string            `json:"assumptions"`
	Facts       map[string]string   `json:"facts"`
	Relations   map[string]int      `json:"relations"`
	Dead        []string            `json:"dead"`
	Items       map[string]string   `json:"items"`
	Knows       map[string][]string `json:"knows"`
}

// StatePatch is the change an applied intent or a finished scene produces.
type StatePatch struct {
	AddAssumptions []string            `json:"addAssumptions,omitempty"`
	SetFacts       map[string]string   `json:"setFacts,omitempty"`
	AddRelations   map[string]int      `json:"addRelations,omitempty"`
	Kill           []string            `json:"kill,omitempty"`
	SetItems       map[string]string   `json:"setItems,omitempty"`
	Learn          map[string][]string `json:"learn,omitempty"`
}

// NewState returns an empty ledger with every map allocated, so callers never
// have to nil-check before writing.
func NewState() *State {
	return &State{
		Facts:     map[string]string{},
		Relations: map[string]int{},
		Items:     map[string]string{},
		Knows:     map[string][]string{},
	}
}

func (s *State) ensureMaps() {
	if s.Facts == nil {
		s.Facts = map[string]string{}
	}
	if s.Relations == nil {
		s.Relations = map[string]int{}
	}
	if s.Items == nil {
		s.Items = map[string]string{}
	}
	if s.Knows == nil {
		s.Knows = map[string][]string{}
	}
}

// Clone copies a ledger. Slices and maps are deep-copied because a rollback
// after a rejected scene must not leak the rejected changes.
func (s *State) Clone() *State {
	if s == nil {
		return NewState()
	}
	out := &State{
		Assumptions: append([]string(nil), s.Assumptions...),
		Dead:        append([]string(nil), s.Dead...),
		Facts:       map[string]string{},
		Relations:   map[string]int{},
		Items:       map[string]string{},
		Knows:       map[string][]string{},
	}
	for k, v := range s.Facts {
		out.Facts[k] = v
	}
	for k, v := range s.Relations {
		out.Relations[k] = v
	}
	for k, v := range s.Items {
		out.Items[k] = v
	}
	for k, v := range s.Knows {
		out.Knows[k] = append([]string(nil), v...)
	}
	return out
}

// Apply merges a patch. Duplicates are dropped so re-applying the same intent
// is idempotent, which matters when a scene is retried.
func (s *State) Apply(p StatePatch) {
	s.ensureMaps()
	for _, a := range p.AddAssumptions {
		s.Assumptions = appendUnique(s.Assumptions, a)
	}
	for k, v := range p.SetFacts {
		s.Facts[k] = v
	}
	for k, v := range p.AddRelations {
		s.Relations[k] += v
	}
	for _, name := range p.Kill {
		s.Dead = appendUnique(s.Dead, name)
	}
	for k, v := range p.SetItems {
		s.Items[k] = v
	}
	for who, facts := range p.Learn {
		for _, fact := range facts {
			s.Knows[who] = appendUnique(s.Knows[who], fact)
		}
	}
}

// IsDead reports whether a character has died in this branch.
func (s *State) IsDead(name string) bool {
	if s == nil {
		return false
	}
	for _, d := range s.Dead {
		if d == name {
			return true
		}
	}
	return false
}

// KnowsFact reports whether a character has learned a fact.
func (s *State) KnowsFact(who, fact string) bool {
	if s == nil {
		return false
	}
	for _, f := range s.Knows[who] {
		if f == fact {
			return true
		}
	}
	return false
}

func appendUnique(list []string, v string) []string {
	if v == "" {
		return list
	}
	for _, existing := range list {
		if existing == v {
			return list
		}
	}
	return append(list, v)
}
