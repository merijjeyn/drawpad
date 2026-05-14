package diagram

// High-level helpers that collapse the most repetitive scene-building
// patterns (build rectangle → set fill → build text → BindText → append)
// into a single call. Use these from generator programs in cmd/.

// BoxStyle bundles the common visual attributes for a shape + its label.
// Zero-value fields are left at the underlying element's default.
type BoxStyle struct {
	Fill        string  // backgroundColor; setting this also flips fillStyle to "solid"
	Stroke      string  // strokeColor
	StrokeWidth float64 // strokeWidth
	FontSize    float64 // text size when the style is applied with a label
	TextColor   string  // text strokeColor when applied with a label
}

// TextStyle bundles the visual attributes for a standalone text element.
type TextStyle struct {
	FontSize float64
	Color    string // strokeColor
}

// Add appends one or more elements to the scene and returns the scene so
// calls can be chained.
func (s *Scene) Add(els ...map[string]interface{}) *Scene {
	s.Elements = append(s.Elements, els...)
	return s
}

// Style applies a BoxStyle to a shape element in place and returns it, so
// the call can be inlined inside Scene.Add. Only the shape fields of the
// style (Fill/Stroke/StrokeWidth) are used here — FontSize/TextColor are
// for label-bearing helpers like LabeledBox.
func Style(el map[string]interface{}, st BoxStyle) map[string]interface{} {
	if st.Fill != "" {
		el["backgroundColor"] = st.Fill
		el["fillStyle"] = "solid"
	}
	if st.Stroke != "" {
		el["strokeColor"] = st.Stroke
	}
	if st.StrokeWidth > 0 {
		el["strokeWidth"] = st.StrokeWidth
	}
	return el
}

// LabeledBox returns a rectangle with a centered text label bound inside.
// The two elements come back as a slice ordered [box, label] so they can
// be spread straight into Scene.Add: s.Add(LabeledBox(...)...).
func LabeledBox(x, y, w, h float64, label string, st BoxStyle) []map[string]interface{} {
	box := Style(Rectangle(x, y, w, h), st)
	text := Text(0, 0, label)
	if st.FontSize > 0 {
		text["fontSize"] = st.FontSize
	}
	if st.TextColor != "" {
		text["strokeColor"] = st.TextColor
	}
	BindText(box, text)
	return []map[string]interface{}{box, text}
}

// Caption returns a standalone text element with optional styling applied
// in one call instead of three.
func Caption(x, y float64, content string, st TextStyle) map[string]interface{} {
	t := Text(x, y, content)
	if st.FontSize > 0 {
		t["fontSize"] = st.FontSize
	}
	if st.Color != "" {
		t["strokeColor"] = st.Color
	}
	return t
}

// Annotate draws a colored arrow from (x1,y1) to (x2,y2) together with a
// caption placed just above the arrow's start. Returns [arrow, caption]
// for spreading into Scene.Add. Use this for explanatory pointers; for
// arrows that should follow two shapes when dragged, build the arrow with
// Arrow(...) and call BindArrow(...) instead.
func Annotate(x1, y1, x2, y2 float64, caption, color string) []map[string]interface{} {
	arr := Arrow(x1, y1, x2, y2)
	if color != "" {
		arr["strokeColor"] = color
	}
	cap := Caption(x1, y1-18, caption, TextStyle{FontSize: 12, Color: color})
	return []map[string]interface{}{arr, cap}
}
