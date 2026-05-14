package diagram

import (
	"encoding/json"
	"strings"
	"testing"
)

// A realistic Excalidraw export. Captured from the official excalidraw.com
// editor — keep it verbatim so the test exercises real-world JSON shapes.
const realExcalidrawJSON = `{
  "type": "excalidraw",
  "version": 2,
  "source": "https://excalidraw.com",
  "elements": [
    {
      "id": "rect-1",
      "type": "rectangle",
      "x": 100,
      "y": 100,
      "width": 200,
      "height": 100,
      "angle": 0,
      "strokeColor": "#1e1e1e",
      "backgroundColor": "#a5d8ff",
      "fillStyle": "solid",
      "strokeWidth": 2,
      "strokeStyle": "solid",
      "roughness": 1,
      "opacity": 100,
      "groupIds": [],
      "frameId": null,
      "roundness": {"type": 3},
      "seed": 12345,
      "version": 42,
      "versionNonce": 987654321,
      "isDeleted": false,
      "boundElements": [{"id": "txt-1", "type": "text"}],
      "updated": 1715000000000,
      "link": null,
      "locked": false,
      "customField": "preserved-through-roundtrip"
    },
    {
      "id": "txt-1",
      "type": "text",
      "x": 110,
      "y": 130,
      "width": 180,
      "height": 40,
      "angle": 0,
      "strokeColor": "#1e1e1e",
      "backgroundColor": "transparent",
      "fillStyle": "solid",
      "strokeWidth": 2,
      "strokeStyle": "solid",
      "roughness": 1,
      "opacity": 100,
      "groupIds": [],
      "frameId": null,
      "seed": 67890,
      "version": 12,
      "versionNonce": 123456789,
      "isDeleted": false,
      "boundElements": null,
      "updated": 1715000000000,
      "link": null,
      "locked": false,
      "text": "Hello",
      "fontSize": 20,
      "fontFamily": 5,
      "textAlign": "center",
      "verticalAlign": "middle",
      "containerId": "rect-1",
      "originalText": "Hello",
      "lineHeight": 1.25
    }
  ],
  "appState": {
    "gridSize": null,
    "viewBackgroundColor": "#ffffff"
  },
  "files": {}
}`

func TestParseScene_RoundTripPreservesUnknownFields(t *testing.T) {
	scene, err := ParseScene([]byte(realExcalidrawJSON))
	if err != nil {
		t.Fatalf("ParseScene: %v", err)
	}
	if got, want := len(scene.Elements), 2; got != want {
		t.Fatalf("elements: got %d, want %d", got, want)
	}
	// The customField on the first element must survive a round-trip — we
	// can't predict what Excalidraw will add in future versions, and the
	// agent must not silently drop fields the user's browser depends on.
	out, err := json.Marshal(scene)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(out), "preserved-through-roundtrip") {
		t.Errorf("custom field dropped on round-trip:\n%s", out)
	}
	// The container/text binding must survive too.
	if !strings.Contains(string(out), `"containerId":"rect-1"`) {
		t.Errorf("containerId binding lost:\n%s", out)
	}
}

func TestParseScene_RejectsMalformed(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "wrong type",
			in:   `{"type":"tldraw","version":2,"elements":[]}`,
			want: "is not",
		},
		{
			name: "element missing id",
			in: `{"type":"excalidraw","version":2,"elements":[{"type":"rectangle"}],
			      "appState":{}}`,
			want: "missing required field \"id\"",
		},
		{
			name: "element missing type",
			in: `{"type":"excalidraw","version":2,"elements":[{"id":"a"}],
			      "appState":{}}`,
			want: "missing required field \"type\"",
		},
		{
			name: "null elements",
			in:   `{"type":"excalidraw","version":2,"elements":null,"appState":{}}`,
			want: "elements must not be null",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseScene([]byte(tt.in))
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.want)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error %q does not contain %q", err.Error(), tt.want)
			}
		})
	}
}

func TestNewScene_IsValid(t *testing.T) {
	s := NewScene()
	if err := s.Validate(); err != nil {
		t.Fatalf("NewScene produced invalid scene: %v", err)
	}
	out, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	// Round-trip through ParseScene to confirm the produced JSON is
	// acceptable as input as well.
	if _, err := ParseScene(out); err != nil {
		t.Errorf("NewScene output failed re-parse: %v\n%s", err, out)
	}
}

func TestElementConstructors_ProduceValidScene(t *testing.T) {
	s := NewScene()
	rect := Rectangle(0, 0, 100, 60)
	label := Text(0, 0, "API")
	BindText(rect, label)
	other := Ellipse(200, 0, 100, 60)
	arrow := Arrow(100, 30, 200, 30)
	BindArrow(arrow, rect, other)

	s.Elements = append(s.Elements, rect, label, other, arrow)

	if err := s.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	// All ids should be unique.
	seen := map[string]bool{}
	for _, el := range s.Elements {
		id, _ := el["id"].(string)
		if id == "" {
			t.Fatalf("element with empty id: %v", el)
		}
		if seen[id] {
			t.Fatalf("duplicate element id %q", id)
		}
		seen[id] = true
	}
	// The rectangle must reference both the label and the arrow.
	bound := rect["boundElements"].([]interface{})
	if len(bound) != 2 {
		t.Errorf("rect.boundElements: got %d, want 2", len(bound))
	}
	// The arrow must have bindings on both ends.
	if arrow["startBinding"] == nil || arrow["endBinding"] == nil {
		t.Errorf("arrow bindings not set: %+v / %+v",
			arrow["startBinding"], arrow["endBinding"])
	}
	// And the whole thing must JSON-encode cleanly.
	if _, err := json.Marshal(s); err != nil {
		t.Errorf("marshal: %v", err)
	}
}

func TestSubmission_DecodeScreenshot(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []byte
	}{
		{"empty", "", nil},
		{"raw base64", "aGVsbG8=", []byte("hello")},
		{"data url", "data:image/png;base64,aGVsbG8=", []byte("hello")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sub := &Submission{ScreenshotPNG: tt.in}
			got, err := sub.DecodeScreenshot()
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			if string(got) != string(tt.want) {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
