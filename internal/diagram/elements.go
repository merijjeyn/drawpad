package diagram

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

// Defaults applied to every element constructed via the helpers below. They
// match Excalidraw's own defaults so the output looks natural in the UI.
const (
	defaultStrokeColor     = "#1e1e1e"
	defaultBackgroundColor = "transparent"
	defaultFillStyle       = "solid"
	defaultStrokeWidth     = 2.0
	defaultStrokeStyle     = "solid"
	defaultRoughness       = 1.0
	defaultOpacity         = 100.0
	defaultFontSize        = 20.0
	defaultFontFamily      = 5 // Excaliifont-compatible sans
	defaultTextAlign       = "center"
	defaultVerticalAlign   = "middle"
	defaultLineHeight      = 1.25
)

// newElementID returns a random ID compatible with Excalidraw's nanoid format.
func newElementID() string {
	b := make([]byte, 10)
	if _, err := rand.Read(b); err != nil {
		// Fall back to a deterministic-ish id; collisions inside a single
		// session are still vanishingly unlikely.
		return hex.EncodeToString([]byte(time.Now().Format("150405.000000")))
	}
	return hex.EncodeToString(b)
}

// baseElement returns a map populated with the fields every element shares.
func baseElement(kind string, x, y, w, h float64) map[string]interface{} {
	return map[string]interface{}{
		"id":              newElementID(),
		"type":            kind,
		"x":               x,
		"y":               y,
		"width":           w,
		"height":          h,
		"angle":           0,
		"strokeColor":     defaultStrokeColor,
		"backgroundColor": defaultBackgroundColor,
		"fillStyle":       defaultFillStyle,
		"strokeWidth":     defaultStrokeWidth,
		"strokeStyle":     defaultStrokeStyle,
		"roughness":       defaultRoughness,
		"opacity":         defaultOpacity,
		"groupIds":        []string{},
		"frameId":         nil,
		"roundness":       map[string]interface{}{"type": 3},
		"seed":            randomSeed(),
		"version":         1,
		"versionNonce":    randomSeed(),
		"isDeleted":       false,
		"boundElements":   []interface{}{},
		"updated":         time.Now().UnixMilli(),
		"link":            nil,
		"locked":          false,
	}
}

func randomSeed() int64 {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	var v int64
	for _, x := range b {
		v = (v << 8) | int64(x)
	}
	if v < 0 {
		v = -v
	}
	return v
}

// Rectangle creates a rectangle element. Strings are written as a separate
// text element bound to this one — call BindText for that.
func Rectangle(x, y, w, h float64) map[string]interface{} {
	return baseElement("rectangle", x, y, w, h)
}

// Ellipse creates an ellipse element.
func Ellipse(x, y, w, h float64) map[string]interface{} {
	return baseElement("ellipse", x, y, w, h)
}

// Diamond creates a diamond element.
func Diamond(x, y, w, h float64) map[string]interface{} {
	return baseElement("diamond", x, y, w, h)
}

// Text creates a standalone text element. For text inside a shape, create the
// shape then call BindText(shape, Text(...)).
func Text(x, y float64, content string) map[string]interface{} {
	// Rough size estimate; Excalidraw will recompute on render.
	w := float64(len(content)) * defaultFontSize * 0.55
	if w < 40 {
		w = 40
	}
	h := defaultFontSize * defaultLineHeight
	el := baseElement("text", x, y, w, h)
	el["text"] = content
	el["originalText"] = content
	el["fontSize"] = defaultFontSize
	el["fontFamily"] = defaultFontFamily
	el["textAlign"] = "left"
	el["verticalAlign"] = "top"
	el["containerId"] = nil
	el["lineHeight"] = defaultLineHeight
	el["baseline"] = defaultFontSize
	delete(el, "fillStyle")
	delete(el, "roundness")
	return el
}

// Arrow creates an arrow from (x1,y1) to (x2,y2).
func Arrow(x1, y1, x2, y2 float64) map[string]interface{} {
	el := baseElement("arrow", x1, y1, x2-x1, y2-y1)
	el["points"] = [][]float64{{0, 0}, {x2 - x1, y2 - y1}}
	el["lastCommittedPoint"] = nil
	el["startBinding"] = nil
	el["endBinding"] = nil
	el["startArrowhead"] = nil
	el["endArrowhead"] = "arrow"
	el["elbowed"] = false
	delete(el, "roundness")
	return el
}

// Line creates a plain line from (x1,y1) to (x2,y2).
func Line(x1, y1, x2, y2 float64) map[string]interface{} {
	el := baseElement("line", x1, y1, x2-x1, y2-y1)
	el["points"] = [][]float64{{0, 0}, {x2 - x1, y2 - y1}}
	el["lastCommittedPoint"] = nil
	el["startBinding"] = nil
	el["endBinding"] = nil
	el["startArrowhead"] = nil
	el["endArrowhead"] = nil
	delete(el, "roundness")
	return el
}

// BindText attaches a text element to a container shape so the text renders
// centered inside it. Both elements should be added to the scene afterwards.
func BindText(container, text map[string]interface{}) {
	cid, _ := container["id"].(string)
	tid, _ := text["id"].(string)
	text["containerId"] = cid
	text["textAlign"] = defaultTextAlign
	text["verticalAlign"] = defaultVerticalAlign
	// Center the text inside the container.
	if cw, ok := numericField(container, "width"); ok {
		text["x"], _ = numericField(container, "x")
		text["width"] = cw
	}
	if ch, ok := numericField(container, "height"); ok {
		text["y"], _ = numericField(container, "y")
		text["height"] = ch
	}
	container["boundElements"] = append(
		toInterfaceSlice(container["boundElements"]),
		map[string]interface{}{"id": tid, "type": "text"},
	)
}

// BindArrow binds an arrow's endpoints to two shapes so it follows them when
// the user drags them around.
func BindArrow(arrow, from, to map[string]interface{}) {
	fid, _ := from["id"].(string)
	tid, _ := to["id"].(string)
	arrow["startBinding"] = map[string]interface{}{
		"elementId": fid,
		"focus":     0,
		"gap":       1,
	}
	arrow["endBinding"] = map[string]interface{}{
		"elementId": tid,
		"focus":     0,
		"gap":       1,
	}
	aid, _ := arrow["id"].(string)
	from["boundElements"] = append(
		toInterfaceSlice(from["boundElements"]),
		map[string]interface{}{"id": aid, "type": "arrow"},
	)
	to["boundElements"] = append(
		toInterfaceSlice(to["boundElements"]),
		map[string]interface{}{"id": aid, "type": "arrow"},
	)
}

func toInterfaceSlice(v interface{}) []interface{} {
	if v == nil {
		return nil
	}
	if s, ok := v.([]interface{}); ok {
		return s
	}
	return nil
}

func numericField(m map[string]interface{}, k string) (float64, bool) {
	v, ok := m[k]
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	}
	return 0, false
}
