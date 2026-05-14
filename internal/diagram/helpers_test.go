package diagram

import "testing"

// Worked example of every high-level helper. Doubles as docs.
func TestHelpers_RoundTrip(t *testing.T) {
	s := NewScene()

	// LabeledBox: rectangle + bound text in one call. Spread into Add.
	s.Add(LabeledBox(0, 0, 200, 60, "Hello",
		BoxStyle{Fill: "#fde68a", Stroke: "#1f2937", FontSize: 14, TextColor: "#0f172a"})...)

	// Caption: standalone text with style.
	s.Add(Caption(0, 80, "subtitle", TextStyle{FontSize: 12, Color: "#64748b"}))

	// Style: tweak a raw shape's visual attributes inline.
	s.Add(Style(Rectangle(0, 100, 200, 60), BoxStyle{Fill: "#bfdbfe"}))

	// Annotate: arrow + caption pointer for callouts.
	s.Add(Annotate(220, 30, 320, 30, "see this", "#be185d")...)

	if got := len(s.Elements); got != 6 {
		t.Fatalf("expected 6 elements (2 box, 1 caption, 1 rect, 1 arrow, 1 caption), got %d", got)
	}
	if err := s.Validate(); err != nil {
		t.Fatalf("scene built via helpers should validate: %v", err)
	}

	// Each labeled element pair should reference each other.
	box := s.Elements[0]
	label := s.Elements[1]
	if label["containerId"] != box["id"] {
		t.Fatalf("LabeledBox did not bind text container to box")
	}
}
