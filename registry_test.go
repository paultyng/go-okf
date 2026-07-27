package okf

import "testing"

type testReview struct {
	Type   string `yaml:"type"`
	Title  string `yaml:"title"`
	Status string `yaml:"status"`
}

func TestAs(t *testing.T) {
	src := []byte("---\ntype: Review\ntitle: Q3 review\nstatus: draft\n---\n\nbody\n")
	c, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	r, err := As[testReview](c)
	if err != nil {
		t.Fatalf("As: %v", err)
	}
	if r.Title != "Q3 review" || r.Status != "draft" {
		t.Errorf("As[testReview] = %+v", r)
	}
}

func TestRegistryDispatch(t *testing.T) {
	Register("TestReview", func(c *Concept) (any, error) {
		return As[testReview](c)
	})

	src := []byte("---\ntype: TestReview\ntitle: Hello\n---\n\nbody\n")
	c, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	v, ok := c.Typed()
	if !ok {
		t.Fatal("expected Typed() to dispatch for a registered type")
	}
	r, ok := v.(*testReview)
	if !ok {
		t.Fatalf("Typed() returned %T, want *testReview", v)
	}
	if r.Title != "Hello" {
		t.Errorf("Title = %q", r.Title)
	}

	unknown := &Concept{Type: "SomethingUnregistered"}
	if _, ok := unknown.Typed(); ok {
		t.Error("expected Typed() to return false for an unregistered type")
	}
}
