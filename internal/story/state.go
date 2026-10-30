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
	Assumptions []string          `json:"assumptions"`
	Facts       map[string]string `json:"facts"`
	Relations   map[string]int    `json:"relations"`
	Dead        []string          `json:"dead"`
	// SatisfiedAnchors tracks which canon anchors this branch has realised. It
	// belongs to the branch, not to the book: the same anchor may be met in one
	// world line and avoided in another.
	SatisfiedAnchors []string            `json:"satisfiedAnchors"`
	Items            map[string]string   `json:"items"`
	Knows            map[string][]string `json:"knows"`
	// Stats are the book-specific numbers derived from the text (money,
	// standing, injuries). Inventory maps item id to a quantity; an entry that
	// reaches zero is removed so "has the item" stays a single lookup.
	Stats     map[string]float64 `json:"stats"`
	Inventory map[string]int     `json:"inventory"`
}

// StatePatch is the change an applied intent or a finished scene produces.
type StatePatch struct {
	AddAssumptions []string            `json:"addAssumptions,omitempty"`
	SetFacts       map[string]string   `json:"setFacts,omitempty"`
	AddRelations   map[string]int      `json:"addRelations,omitempty"`
	Kill           []string            `json:"kill,omitempty"`
	SetItems       map[string]string   `json:"setItems,omitempty"`
	Learn          map[string][]string `json:"learn,omitempty"`
	SatisfyAnchors []string            `json:"satisfyAnchors,omitempty"`
	// StatDelta and ItemDelta are applied only after the world sheet has
	// validated them; State.Apply itself does not clamp.
	StatDelta map[string]float64 `json:"statDelta,omitempty"`
	ItemDelta map[string]int     `json:"itemDelta,omitempty"`
}

// NewState returns an empty ledger with every map allocated, so callers never
// have to nil-check before writing.
func NewState() *State {
	return &State{
		Facts:     map[string]string{},
		Relations: map[string]int{},
		Items:     map[string]string{},
		Knows:     map[string][]string{},
		Stats:     map[string]float64{},
		Inventory: map[string]int{},
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
	if s.Stats == nil {
		s.Stats = map[string]float64{}
	}
	if s.Inventory == nil {
		s.Inventory = map[string]int{}
	}
}

// Normalize allocates missing maps after JSON decoding so callers can write to
// the ledger without nil checks. Slices are left untouched.
func (s *State) Normalize() {
	if s == nil {
		return
	}
	s.ensureMaps()
}

// Clone copies a ledger. Slices and maps are deep-copied because a rollback
// after a rejected scene must not leak the rejected changes.
func (s *State) Clone() *State {
	if s == nil {
		return NewState()
	}
	out := &State{
		Assumptions:      append([]string(nil), s.Assumptions...),
		Dead:             append([]string(nil), s.Dead...),
		SatisfiedAnchors: append([]string(nil), s.SatisfiedAnchors...),
		Facts:            map[string]string{},
		Relations:        map[string]int{},
		Items:            map[string]string{},
		Knows:            map[string][]string{},
		Stats:            map[string]float64{},
		Inventory:        map[string]int{},
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
	for k, v := range s.Stats {
		out.Stats[k] = v
	}
	for k, v := range s.Inventory {
		out.Inventory[k] = v
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
	for _, id := range p.SatisfyAnchors {
		s.SatisfiedAnchors = appendUnique(s.SatisfiedAnchors, id)
	}
	for k, v := range p.SetItems {
		s.Items[k] = v
	}
	for who, facts := range p.Learn {
		for _, fact := range facts {
			s.Knows[who] = appendUnique(s.Knows[who], fact)
		}
	}
	// World deltas are applied raw here. Callers must run ApplyWorld first so
	// unknown items and out-of-range numbers are rejected or clamped.
	for k, v := range p.StatDelta {
		s.Stats[k] += v
	}
	for k, v := range p.ItemDelta {
		s.Inventory[k] += v
		if s.Inventory[k] <= 0 {
			delete(s.Inventory, k)
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
