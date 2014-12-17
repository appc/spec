package transformer

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/appc/spec/Godeps/_workspace/src/github.com/docker/docker/builder/parser"
	"github.com/appc/spec/schema"
	"github.com/appc/spec/schema/types"
)

const (
	DEFAULT_AC_KIND    = "AppManifest"
	DEFAULT_AC_VERSION = "1.0.0"
)

type ToRocket struct {
	BasePath string
	manifest schema.AppManifest
	aci      *ACIFile
	output   string
}

func NewToRocket(name, version, os, arch string) (*ToRocket, error) {
	t := &ToRocket{}
	if err := t.setBasicData(name, version, os, arch); err != nil {
		return nil, err
	}

	t.output = fmt.Sprintf("%s-v%s-%s-%s.aci", name, version, os, arch)
	t.aci = NewACIFile()

	return t, nil
}

func (t *ToRocket) setBasicData(name, version, os, arch string) error {
	t.manifest.Name = types.ACName(name)
	t.manifest.Version = types.ACName(version)
	t.manifest.OS = types.ACName(os)
	t.manifest.Arch = types.ACName(arch)

	t.manifest.ACKind = types.ACKind(DEFAULT_AC_KIND)

	if ver, err := types.NewSemVer(DEFAULT_AC_VERSION); err == nil {
		t.manifest.ACVersion = *ver
	} else {
		return err
	}

	return nil
}

func (t *ToRocket) Process(n *parser.Node) error {
	return t.processNode(n)
}

func (t *ToRocket) processNode(n *parser.Node) error {
	var err error
	switch n.Value {
	case "add":
		err = t.processAddOrCopyNode(n)
	case "copy":
		err = t.processAddOrCopyNode(n)
	case "entrypoint":
		err = t.processEntryPointOrCMDNode(n)
	case "cmd":
		err = t.processEntryPointOrCMDNode(n)
	case "volume":
		err = t.processVolumeNode(n)
	case "env":
		err = t.processEnvNode(n)
	case "expose":
		err = t.processExposeNode(n)
	}

	if err != nil {
		return err
	}

	if len(n.Children) != 0 {
		if err = t.iterateNodes(n.Children); err != nil {
			return err
		}
	}

	return nil
}

func (t *ToRocket) processAddOrCopyNode(n *parser.Node) error {
	add := n.Original[len(n.Value)+1:]
	files := strings.Split(add, " ")
	dst := files[len(files)-1]
	if dst[len(dst)-1] != '/' {
		dst += "/"
	}

	dst = filepath.Join("rootfs", dst)
	for _, file := range files[:len(files)-1] {
		file = filepath.Join(t.BasePath, file)
		if err := t.aci.AddFromFilesystem(file, dst); err != nil {
			return err
		}
	}

	return nil
}

func (t *ToRocket) processEntryPointOrCMDNode(n *parser.Node) error {
	cmd := n.Original[len(n.Value)+1:]
	if isJSON, ok := n.Attributes["json"]; ok && isJSON {
		var data []string
		json.Unmarshal([]byte(n.Original[4:]), &data)
		cmd = strings.Join(data, " ")
	}

	if len(t.manifest.Exec) == 1 {
		if n.Value == "cmd" {
			cmd = t.manifest.Exec[0] + " " + cmd
		} else {
			cmd += " " + t.manifest.Exec[0]
		}
	}

	t.manifest.Exec = []string{strings.Trim(cmd, " ")}

	return nil
}

func (t *ToRocket) processVolumeNode(n *parser.Node) error {
	var volumes []string

	if isJSON, ok := n.Attributes["json"]; ok && isJSON {
		json.Unmarshal([]byte(n.Original[7:]), &volumes)
	} else {
		volumes = []string{n.Original[7:]}
	}

	t.manifest.MountPoints = make([]types.MountPoint, len(volumes))
	for i, path := range volumes {
		pathS := strings.Split(path, "/")
		t.manifest.MountPoints[i] = types.MountPoint{
			Name: types.ACName(pathS[len(pathS)-1]),
			Path: path,
		}
	}

	return nil

}

func (t *ToRocket) processEnvNode(n *parser.Node) error {
	env := n.Original[4:]
	values := strings.Split(env, " ")

	if t.manifest.Environment == nil {
		t.manifest.Environment = make(map[string]string)
	}

	t.manifest.Environment[values[0]] = values[1]
	return nil
}

func (t *ToRocket) processExposeNode(n *parser.Node) error {
	expose := n.Original[7:]
	port := strings.Split(expose, "/")
	portInt, _ := strconv.Atoi(port[0])

	proto := "tcp"
	if len(port) == 2 {
		proto = port[1]
	}

	t.manifest.Ports = append(t.manifest.Ports, types.Port{
		Name:     types.ACName(port[0]),
		Protocol: proto,
		Port:     uint(portInt),
	})

	return nil
}

func (t *ToRocket) iterateNodes(nodes []*parser.Node) error {
	for _, n := range nodes {
		if err := t.Process(n); err != nil {
			return err
		}
	}

	return nil
}

func (t *ToRocket) Print() {
	json, _ := t.manifest.MarshalJSON()
	fmt.Printf("%s", json)
}

func (t *ToRocket) SaveToFile(filename string) (string, error) {
	if filename == "" {
		filename = t.output
	}

	json, err := t.manifest.MarshalJSON()
	if err != nil {
		return "", err
	}

	if err := t.aci.AddFromBytes("app", json); err != nil {
		return "", err
	}

	if err := t.aci.SaveToFile(filename); err != nil {
		return "", err
	}

	return t.output, nil
}

type ACIFile struct {
	contents []*content
}

type content struct {
	header *tar.Header
	raw    []byte
}

func NewACIFile() *ACIFile {
	aci := &ACIFile{make([]*content, 0)}
	aci.BuildDir("rootfs")

	return aci
}

func (a *ACIFile) BuildDir(path string) {
	fmt.Println(path)
	c := &content{}

	if path[len(path)-1] != '/' {
		path += "/"
	}

	time := time.Now()
	c.header = &tar.Header{
		Name:       path,
		Size:       int64(0),
		ModTime:    time,
		AccessTime: time,
		ChangeTime: time,
		Typeflag:   tar.TypeDir,
		Mode:       int64(0660),
	}

	a.contents = append(a.contents, c)
}

func (a *ACIFile) AddFromBytes(filename string, raw []byte) error {
	c := &content{}

	time := time.Now()

	c.header = &tar.Header{
		Name:       filename,
		Size:       int64(len(raw)),
		ModTime:    time,
		AccessTime: time,
		ChangeTime: time,
		Mode:       int64(0660),
	}

	c.raw = raw
	a.contents = append(a.contents, c)

	return nil
}

func (a *ACIFile) AddFromFilesystem(src string, dst string) error {
	matches, err := filepath.Glob(src)
	if err != nil {
		return err
	}

	for _, file := range matches {
		if err := a.addFile(file, dst); err != nil {
			return err
		}
	}

	return nil
}

func (a *ACIFile) addFolder(src string, dst string) error {
	return filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if p == src {
			return nil
		}

		newDst := filepath.Join(dst, strings.Replace(p, path.Dir(src), "", -1))
		if err := a.addFile(p, newDst); err != nil {
			return err
		}

		return nil
	})
}

func (a *ACIFile) addFile(src string, dst string) error {
	fInfo, err := os.Lstat(src)
	if err != nil {
		return err
	}

	if fInfo.IsDir() {
		return a.addFolder(src, dst)
	}

	c := &content{}

	c.raw, err = ioutil.ReadFile(src)
	if err != nil {
		return err
	}

	c.header, err = tar.FileInfoHeader(fInfo, "")

	c.header.Name = dst
	if dst[len(dst)-1] == '/' {
		c.header.Name = filepath.Join(dst, path.Base(src))
	}

	a.BuildDir(path.Dir(dst))

	if err != nil {
		return err
	}

	a.contents = append(a.contents, c)

	return nil
}

func (a *ACIFile) SaveToFile(filename string) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}

	g := gzip.NewWriter(f)
	t := tar.NewWriter(g)

	defer func() {
		t.Close()
		g.Close()
		f.Close()
	}()

	for _, c := range a.contents {
		if err := t.WriteHeader(c.header); err != nil {
			return err
		}

		if _, err := t.Write(c.raw); err != nil {
			return err
		}
	}

	return nil
}
