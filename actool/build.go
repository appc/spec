package main

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/appc/spec/aci"
	"github.com/appc/spec/pkg/tarheader"
	"github.com/appc/spec/schema"
)

var (
	buildNocompress bool
	buildOverwrite  bool
	cmdBuild        = &Command{
		Name: "build",
		Description: `Build an ACI from a given directory. The directory should
contain an Image Layout. The Image Layout will be validated
before the ACI is created. The produced ACI will be
gzip-compressed by default.`,
		Summary: "Build an ACI from an Image Layout (experimental)",
		Usage:   `[--overwrite] [--no-compression] DIRECTORY OUTPUT_FILE`,
		Run:     runBuild,
	}
)

func init() {
	cmdBuild.Flags.BoolVar(&buildOverwrite, "overwrite", false, "Overwrite target file if it already exists")
	cmdBuild.Flags.BoolVar(&buildNocompress, "no-compression", false, "Do not gzip-compress the produced ACI")
}

func buildWalker(root string, aw aci.ArchiveWriter) filepath.WalkFunc {
	// cache of inode -> filepath, used to leverage hard links in the archive
	inos := map[uint64]string{}
	return func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relpath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if relpath == "." {
			return nil
		}
		if relpath == aci.ManifestFile {
			// ignore; this will be written by the archive writer
			// TODO(jonboulle): does this make sense? maybe just remove from archivewriter?
			return nil
		}

		link := ""
		var r io.Reader
		switch info.Mode() & os.ModeType {
		case os.ModeCharDevice:
		case os.ModeDevice:
		case os.ModeDir:
		case os.ModeSymlink:
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			link = target
		default:
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()
			r = file
		}

		hdr, err := tar.FileInfoHeader(info, link)
		if err != nil {
			panic(err)
		}
		// Because os.FileInfo's Name method returns only the base
		// name of the file it describes, it may be necessary to
		// modify the Name field of the returned header to provide the
		// full path name of the file.
		hdr.Name = relpath
		tarheader.Populate(hdr, info, inos)
		// If the file is a hard link to a file we've already seen, we
		// don't need the contents
		if hdr.Typeflag == tar.TypeLink {
			hdr.Size = 0
			r = nil
		}
		if err := aw.AddFile(relpath, hdr, r); err != nil {
			return err
		}

		return nil
	}
}

func runBuild(args []string) (exit int) {
	if len(args) != 2 {
		stderr("build: Must provide directory and output file")
		return 1
	}

	root := args[0]
	tgt := args[1]
	ext := filepath.Ext(tgt)
	if ext != schema.ACIExtension {
		stderr("build: Extension must be %s (given %s)", schema.ACIExtension, ext)
		return 1
	}

	mode := os.O_CREATE | os.O_WRONLY
	if buildOverwrite {
		mode |= os.O_TRUNC
	} else {
		mode |= os.O_EXCL
	}
	fh, err := os.OpenFile(tgt, mode, 0644)
	if err != nil {
		if os.IsExist(err) {
			stderr("build: Target file exists (try --overwrite)")
		} else {
			stderr("build: Unable to open target %s: %v", tgt, err)
		}
		return 1
	}

	var gw *gzip.Writer
	var r io.WriteCloser = fh
	if !buildNocompress {
		gw = gzip.NewWriter(fh)
		r = gw
	}
	tr := tar.NewWriter(r)

	defer func() {
		tr.Close()
		if !buildNocompress {
			gw.Close()
		}
		fh.Close()
		if exit != 0 && !buildOverwrite {
			os.Remove(tgt)
		}
	}()

	// TODO(jonboulle): stream the validation so we don't have to walk the rootfs twice
	if err := aci.ValidateLayout(root); err != nil {
		stderr("build: Layout failed validation: %v", err)
		return 1
	}
	mpath := filepath.Join(root, aci.ManifestFile)
	b, err := ioutil.ReadFile(mpath)
	if err != nil {
		stderr("build: Unable to read Image Manifest: %v", err)
		return 1
	}
	var im schema.ImageManifest
	if err := im.UnmarshalJSON(b); err != nil {
		stderr("build: Unable to load Image Manifest: %v", err)
		return 1
	}
	iw := aci.NewImageWriter(im, tr)

	err = filepath.Walk(root, buildWalker(root, iw))
	if err != nil {
		stderr("build: Error walking rootfs: %v", err)
		return 1
	}

	err = iw.Close()
	if err != nil {
		stderr("build: Unable to close image %s: %v", tgt, err)
		return 1
	}

	return
}
