// Package diagram defines the data model exchanged between the AI agent,
// the local HTTP server, and the Excalidraw frontend.
//
// We do not attempt to model the entire Excalidraw schema. Instead, elements
// and app state are passed through as opaque map[string]interface{}. This
// guarantees round-trip preservation of any field the user's browser produces.
// Typed constructors in elements.go cover the common shapes the agent will
// want to author programmatically.
package diagram

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Scene is the Excalidraw scene format (the JSON produced by File > Save and
// consumed by File > Open). It is what the agent sends as initial content and
// what is returned after the user clicks Send.
type Scene struct {
	Type     string                   `json:"type"`
	Version  int                      `json:"version"`
	Source   string                   `json:"source,omitempty"`
	Elements []map[string]interface{} `json:"elements"`
	AppState map[string]interface{}   `json:"appState,omitempty"`
	Files    map[string]interface{}   `json:"files,omitempty"`
}

// NewScene returns an empty but valid Excalidraw scene.
func NewScene() *Scene {
	return &Scene{
		Type:     "excalidraw",
		Version:  2,
		Source:   "draw_interface",
		Elements: []map[string]interface{}{},
		AppState: map[string]interface{}{
			"viewBackgroundColor": "#ffffff",
			"gridSize":            nil,
		},
		Files: map[string]interface{}{},
	}
}

// Validate performs minimal sanity checks. It accepts anything Excalidraw
// would accept and only rejects obviously malformed payloads.
func (s *Scene) Validate() error {
	if s == nil {
		return fmt.Errorf("scene is nil")
	}
	if s.Type != "" && s.Type != "excalidraw" {
		return fmt.Errorf("scene type %q is not %q", s.Type, "excalidraw")
	}
	if s.Elements == nil {
		return fmt.Errorf("scene.elements must not be null (use [] for empty)")
	}
	for i, el := range s.Elements {
		if _, ok := el["type"]; !ok {
			return fmt.Errorf("element[%d] missing required field \"type\"", i)
		}
		if _, ok := el["id"]; !ok {
			return fmt.Errorf("element[%d] missing required field \"id\"", i)
		}
	}
	return nil
}

// InitialPayload is what the agent passes in at start-up to seed the UI.
type InitialPayload struct {
	// Prompt is shown to the user as a banner above the canvas. Use it to ask
	// a specific question ("Does this match what you had in mind?").
	Prompt string `json:"prompt,omitempty"`
	// Scene is the initial Excalidraw content. Nil means a blank canvas.
	Scene *Scene `json:"scene,omitempty"`
}

// Result is what the CLI writes back to the agent after the user clicks Send
// (or cancels). It is the structured info promised in the task description.
type Result struct {
	// Scene is the final state of the diagram. Nil iff Cancelled.
	Scene *Scene `json:"scene,omitempty"`
	// Comment is free-text the user typed in the comment box. May be empty.
	Comment string `json:"comment,omitempty"`
	// ScreenshotPath is the absolute path to the PNG export of the diagram,
	// suitable for the agent to read back as an image. Empty iff Cancelled
	// or screenshot capture failed.
	ScreenshotPath string `json:"screenshotPath,omitempty"`
	// Cancelled is true if the user closed the window without submitting.
	Cancelled bool `json:"cancelled"`
}

// Submission is the raw payload posted by the frontend on Send. The server
// turns it into a Result by writing the screenshot to disk.
type Submission struct {
	Scene   *Scene `json:"scene"`
	Comment string `json:"comment,omitempty"`
	// ScreenshotPNG is the base64-encoded PNG export of the diagram, with or
	// without a "data:image/png;base64," prefix.
	ScreenshotPNG string `json:"screenshotPng,omitempty"`
}

// DecodeScreenshot decodes the screenshot payload into raw PNG bytes,
// tolerating both bare base64 and data-URL prefixed forms.
func (s *Submission) DecodeScreenshot() ([]byte, error) {
	if s == nil || s.ScreenshotPNG == "" {
		return nil, nil
	}
	payload := s.ScreenshotPNG
	if i := strings.Index(payload, ","); strings.HasPrefix(payload, "data:") && i > 0 {
		payload = payload[i+1:]
	}
	return base64Decode(payload)
}

// ParseScene unmarshals a JSON document into a Scene with validation.
func ParseScene(data []byte) (*Scene, error) {
	var s Scene
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.UseNumber() // preserve numeric precision through round-trip
	if err := dec.Decode(&s); err != nil {
		return nil, fmt.Errorf("decode scene: %w", err)
	}
	if err := s.Validate(); err != nil {
		return nil, err
	}
	return &s, nil
}
