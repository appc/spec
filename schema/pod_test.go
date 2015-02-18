package schema

import "testing"

func TestPodManifestMerge(t *testing.T) {
	pmj := `{}`
	pm := &PodManifest{}

	if pm.UnmarshalJSON([]byte(pmj)) == nil {
		t.Fatal("Manifest JSON without acKind and acVersion unmarshalled successfully")
	}

	pm = BlankPodManifest()

	err := pm.UnmarshalJSON([]byte(pmj))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
