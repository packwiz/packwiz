package github

import "testing"

func TestGhUpdater_ParseUpdate(t *testing.T) {
	in := map[string]interface{}{
		"slug":   "packwiz/packwiz",
		"tag":    "v1.0.0",
		"branch": "main",
		"regex":  `.*\.jar$`,
	}

	u := ghUpdater{}
	got, err := u.ParseUpdate(in)
	if err != nil {
		t.Fatalf("ParseUpdate: %v", err)
	}

	data, ok := got.(ghUpdateData)
	if !ok {
		t.Fatalf("ParseUpdate returned %T, want ghUpdateData", got)
	}

	if data.Slug != "packwiz/packwiz" || data.Tag != "v1.0.0" ||
		data.Branch != "main" || data.Regex != `.*\.jar$` {
		t.Errorf("got %+v, want all four fields populated from input map", data)
	}
}

func TestGhUpdater_ParseUpdate_MissingFieldsAreZero(t *testing.T) {
	// Partial input maps decode cleanly with the missing fields
	// defaulting to zero values — that's mapstructure's standard
	// behavior. Pinning so a future migration to a stricter decoder
	// (DisallowUnknownFields, ErrorUnused, etc.) surfaces here.
	in := map[string]interface{}{
		"slug": "packwiz/packwiz",
	}

	got, err := ghUpdater{}.ParseUpdate(in)
	if err != nil {
		t.Fatalf("ParseUpdate: %v", err)
	}

	data := got.(ghUpdateData)
	if data.Slug != "packwiz/packwiz" {
		t.Errorf("Slug = %q, want packwiz/packwiz", data.Slug)
	}

	if data.Tag != "" || data.Branch != "" || data.Regex != "" {
		t.Errorf("missing fields were not zero: %+v", data)
	}
}
