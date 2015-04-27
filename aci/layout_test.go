package aci

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/appc/spec/schema"
)

func newValidateLayoutTest() (string, error) {
	td, err := ioutil.TempDir("", "")
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(path.Join(td, "rootfs"), 0755); err != nil {
		return "", err
	}

	if err := os.MkdirAll(path.Join(td, "rootfs", "dir", "rootfs"), 0755); err != nil {
		return "", err
	}

	evilManifestBody := "malformedManifest"
	manifestBody := fmt.Sprintf(`{"acKind":"ImageManifest","acVersion":"%s","name":"example.com/app"}`, schema.AppContainerVersion)

	evilManifestPath := "rootfs/manifest"
	evilManifestPath = path.Join(td, evilManifestPath)

	em, err := os.Create(evilManifestPath)
	if err != nil {
		return "", err
	}

	em.WriteString(evilManifestBody)
	em.Close()

	manifestPath := path.Join(td, "manifest")

	m, err := os.Create(manifestPath)
	if err != nil {
		return "", err
	}

	m.WriteString(manifestBody)
	m.Close()

	return td, nil
}

func TestValidateLayout(t *testing.T) {
	layoutPath, err := newValidateLayoutTest()
	if err != nil {
		t.Fatalf("newValidateLayoutTest: unexpected error: %v", err)
	}
	defer os.RemoveAll(layoutPath)

	err = ValidateLayout(layoutPath)
	if err != nil {
		t.Fatalf("ValidateLayout: unexpected error: %v", err)
	}
}
