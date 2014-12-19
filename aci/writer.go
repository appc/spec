package aci

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/appc/spec/schema"
)

// ArchiveWriter writes App Container Images. Users wanting to create an ACI or
// should create an ArchiveWriter and add files to it; the ACI will be written
// to the underlying tar.Writer
type ArchiveWriter interface {
	AddFile(path string, hdr *tar.Header, r io.Reader) error
	Close() error
}

type imageArchiveWriter struct {
	*tar.Writer
	am         *schema.ImageManifest
	foundExecs map[string]bool
}

// NewImageWriter creates a new ArchiveWriter which will generate an App
// Container Image based on the given manifest and write it to the given
// tar.Writer
func NewImageWriter(am schema.ImageManifest, w *tar.Writer) ArchiveWriter {
	aw := &imageArchiveWriter{
		w,
		&am,
		map[string]bool{},
	}

	addExec := func(path string) {
		aw.foundExecs[filepath.Join("rootfs", path)] = false
	}
	addExec(am.App.Exec[0])
	for _, eh := range am.App.EventHandlers {
		addExec(eh.Exec[0])
	}

	return aw
}

func (aw *imageArchiveWriter) AddFile(path string, hdr *tar.Header, r io.Reader) error {
	err := aw.Writer.WriteHeader(hdr)
	if err != nil {
		return err
	}

	if r != nil {
		_, err := io.Copy(aw.Writer, r)
		if err != nil {
			return err
		}
	}

	if _, exec := aw.foundExecs[path]; exec {
		aw.foundExecs[path] = true
	}

	return nil
}

func (aw *imageArchiveWriter) addFileNow(path string, contents []byte) error {
	buf := bytes.NewBuffer(contents)
	now := time.Now()
	hdr := tar.Header{
		Name:       path,
		Mode:       0644,
		Uid:        0,
		Gid:        0,
		Size:       int64(buf.Len()),
		ModTime:    now,
		Typeflag:   tar.TypeReg,
		Uname:      "root",
		Gname:      "root",
		ChangeTime: now,
	}
	return aw.AddFile(path, &hdr, buf)
}

func (aw *imageArchiveWriter) addManifest(name string, m json.Marshaler) error {
	out, err := m.MarshalJSON()
	if err != nil {
		return err
	}
	return aw.addFileNow(name, out)
}

func (aw *imageArchiveWriter) Close() error {
	if err := aw.addManifest(ManifestFile, aw.am); err != nil {
		return err
	}

	for exec, present := range aw.foundExecs {
		if present == false {
			fmt.Fprintf(os.Stderr, "WARNING: Exec %q is absent from image\n", exec)
		}
	}
	return aw.Writer.Close()
}
