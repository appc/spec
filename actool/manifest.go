// Copyright 2015 The appc Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/appc/spec/aci"
	"github.com/appc/spec/schema"
	"github.com/appc/spec/schema/types"
)

var (
	inputFile  string
	outputFile string

	patchNocompress        bool
	patchOverwrite         bool
	patchReplace           bool
	patchManifestFile      string
	patchName              string
	patchExec              string
	patchUser              string
	patchGroup             string
	patchSupplementaryGIDs string
	patchCaps              string
	patchMounts            string
	patchPorts             string
	patchIsolators         string
	patchDependencies      string

	catPrettyPrint bool

	cmdPatchManifest = &Command{
		Name:        "patch-manifest",
		Description: `Copy an ACI and patch its manifest.`,
		Summary:     "Copy an ACI and patch its manifest (experimental)",
		Usage: `
		  [--manifest=MANIFEST_FILE]
		  [--name=example.com/app]
		  [--exec="/app --debug"]
		  [--user=uid] [--group=gid]
		  [--capability=CAP_SYS_ADMIN,CAP_NET_ADMIN]
		  [--mounts=work,path=/opt,readOnly=true[:work2,...]]
		  [--ports=query,protocol=tcp,port=8080[:query2,...]]
		  [--supplementary-groups=gid1,gid2,...]
		  [--isolators=resource/cpu,request=50m,limit=100m[:resource/memory,...]]
		  [--dependencies=example.com/app1[:example.com/app2:...]]
		  [--replace]
		  INPUT_ACI_FILE
		  [OUTPUT_ACI_FILE]`,
		Run: runPatchManifest,
	}
	cmdCatManifest = &Command{
		Name:        "cat-manifest",
		Description: `Print the manifest from an ACI.`,
		Summary:     "Print the manifest from an ACI",
		Usage:       ` [--pretty-print] ACI_FILE`,
		Run:         runCatManifest,
	}
)

func init() {
	cmdPatchManifest.Flags.BoolVar(&patchOverwrite, "overwrite", false, "Overwrite target file if it already exists")
	cmdPatchManifest.Flags.BoolVar(&patchNocompress, "no-compression", false, "Do not gzip-compress the produced ACI")
	cmdPatchManifest.Flags.BoolVar(&patchReplace, "replace", false, "Replace the input file")

	cmdPatchManifest.Flags.StringVar(&patchManifestFile, "manifest", "", "Replace image manifest with this file. Incompatible with other replace options.")
	cmdPatchManifest.Flags.StringVar(&patchName, "name", "", "Replace name")
	cmdPatchManifest.Flags.StringVar(&patchExec, "exec", "", "Replace the command line to launch the executable")
	cmdPatchManifest.Flags.StringVar(&patchUser, "user", "", "Replace user")
	cmdPatchManifest.Flags.StringVar(&patchGroup, "group", "", "Replace group")
	cmdPatchManifest.Flags.StringVar(&patchSupplementaryGIDs, "supplementary-groups", "", "Replace supplementary groups, expects a comma-separated list.")
	cmdPatchManifest.Flags.StringVar(&patchCaps, "capability", "", "Replace capability")
	cmdPatchManifest.Flags.StringVar(&patchMounts, "mounts", "", "Replace mount points")
	cmdPatchManifest.Flags.StringVar(&patchPorts, "ports", "", "Replace ports")
	cmdPatchManifest.Flags.StringVar(&patchIsolators, "isolators", "", "Replace isolators")
	cmdPatchManifest.Flags.StringVar(&patchDependencies, "dependencies", "", "Replace dependencies")

	cmdCatManifest.Flags.BoolVar(&catPrettyPrint, "pretty-print", false, "Print with better style")
}

func getIsolatorStr(name, value string) string {
	return fmt.Sprintf(`
                {
                    "name": "%s",
                    "value": { %s }
                }`, name, value)
}

func isolatorStrFromString(is string) (types.ACIdentifier, string, error) {
	is = "name=" + is
	v, err := url.ParseQuery(strings.Replace(is, ",", "&", -1))
	if err != nil {
		return "", "", err
	}

	var name string
	var values []string
	var acn *types.ACIdentifier

	for key, val := range v {
		if len(val) > 1 {
			return "", "", fmt.Errorf("label %s with multiple values %q", key, val)
		}

		switch key {
		case "name":
			acn, err = types.NewACIdentifier(val[0])
			if err != nil {
				return "", "", err
			}
			name = val[0]
		default:
			// (TODO)yifan: Not support the default boolean yet.
			values = append(values, fmt.Sprintf(`"%s": "%s"`, key, val[0]))
		}
	}
	return *acn, getIsolatorStr(name, strings.Join(values, ", ")), nil
}

func patchManifest(im *schema.ImageManifest) error {

	if patchName != "" {
		name, err := types.NewACIdentifier(patchName)
		if err != nil {
			return err
		}
		im.Name = *name
	}

	var app *types.App = im.App
	if patchExec != "" {
		if app == nil {
			// if the original manifest was missing an app and
			// patchExec is set let's assume the user is trying to
			// inject one...
			im.App = &types.App{}
			app = im.App
		}
		app.Exec = strings.Split(patchExec, " ")
	}

	if patchUser != "" || patchGroup != "" || patchSupplementaryGIDs != "" || patchCaps != "" || patchMounts != "" || patchPorts != "" || patchIsolators != "" || patchDependencies != "" {
		// ...but if we still don't have an app and the user is trying
		// to patch one of its other parameters, it's an error
		if app == nil {
			return fmt.Errorf("no app in the supplied manifest and no exec command provided")
		}
	}

	if patchUser != "" {
		app.User = patchUser
	}

	if patchGroup != "" {
		app.Group = patchGroup
	}

	if patchSupplementaryGIDs != "" {
		app.SupplementaryGIDs = []int{}
		gids := strings.Split(patchSupplementaryGIDs, ",")
		for _, g := range gids {
			gid, err := strconv.Atoi(g)
			if err != nil {
				return fmt.Errorf("invalid supplementary group %q: %v", g, err)
			}
			app.SupplementaryGIDs = append(app.SupplementaryGIDs, gid)
		}
	}

	if patchCaps != "" {
		isolator := app.Isolators.GetByName(types.LinuxCapabilitiesRetainSetName)
		if isolator != nil {
			return fmt.Errorf("isolator already exists")
		}

		// Instantiate a Isolator with the content specified by the --capability
		// parameter.
		caps, err := types.NewLinuxCapabilitiesRetainSet(strings.Split(patchCaps, ",")...)
		if err != nil {
			return fmt.Errorf("cannot parse capability %q: %v", patchCaps, err)
		}
		app.Isolators = append(app.Isolators, caps.AsIsolator())
	}

	if patchMounts != "" {
		mounts := strings.Split(patchMounts, ":")
		for _, m := range mounts {
			mountPoint, err := types.MountPointFromString(m)
			if err != nil {
				return fmt.Errorf("cannot parse mount point %q: %v", m, err)
			}
			app.MountPoints = append(app.MountPoints, *mountPoint)
		}
	}

	if patchPorts != "" {
		ports := strings.Split(patchPorts, ":")
		for _, p := range ports {
			port, err := types.PortFromString(p)
			if err != nil {
				return fmt.Errorf("cannot parse port %q: %v", p, err)
			}
			app.Ports = append(app.Ports, *port)
		}
	}

	if patchIsolators != "" {
		isolators := strings.Split(patchIsolators, ":")
		for _, is := range isolators {
			name, isolatorStr, err := isolatorStrFromString(is)
			if err != nil {
				return fmt.Errorf("cannot parse isolator %q: %v", is, err)
			}

			if _, ok := types.ResourceIsolatorNames[name]; !ok {
				return fmt.Errorf("isolator %s is not supported for patching", name)
			}

			isolator := &types.Isolator{}
			if err := isolator.UnmarshalJSON([]byte(isolatorStr)); err != nil {
				return fmt.Errorf("cannot unmarshal isolator %v: %v", isolatorStr, err)
			}
			app.Isolators = append(app.Isolators, *isolator)
		}
	}

	if patchDependencies != "" {
		dependencies := strings.Split(patchDependencies, ":")
		for _, depStr := range dependencies {
			depId, err := types.NewACIdentifier(depStr)
			if err != nil {
				return fmt.Errorf("cannot unmarshal dependency %v: %v", depStr, err)
			}
			dep := types.Dependency{
				ImageName: *depId,
			}
			im.Dependencies = append(im.Dependencies, dep)
		}
	}
	return nil
}

// extractManifest iterates over the tar reader and locate the manifest. Once
// located, the manifest can be printed, replaced or patched.
func extractManifest(tr *tar.Reader, tw *tar.Writer, printManifest bool, newManifest []byte) error {
Tar:
	for {
		hdr, err := tr.Next()
		switch err {
		case io.EOF:
			break Tar
		case nil:
			if filepath.Clean(hdr.Name) == aci.ManifestFile {
				var new_bytes []byte

				bytes, err := ioutil.ReadAll(tr)
				if err != nil {
					return err
				}

				if printManifest && !catPrettyPrint {
					fmt.Println(string(bytes))
				}

				im := &schema.ImageManifest{}
				err = im.UnmarshalJSON(bytes)
				if err != nil {
					return err
				}

				if printManifest && catPrettyPrint {
					output, err := json.MarshalIndent(im, "", "    ")
					if err != nil {
						return err
					}
					fmt.Println(string(output))
				}

				if tw == nil {
					return nil
				}

				if len(newManifest) == 0 {
					err = patchManifest(im)
					if err != nil {
						return err
					}

					new_bytes, err = im.MarshalJSON()
					if err != nil {
						return err
					}
				} else {
					new_bytes = newManifest
				}

				hdr.Size = int64(len(new_bytes))
				err = tw.WriteHeader(hdr)
				if err != nil {
					return err
				}

				_, err = tw.Write(new_bytes)
				if err != nil {
					return err
				}
			} else if tw != nil {
				err := tw.WriteHeader(hdr)
				if err != nil {
					return err
				}
				_, err = io.Copy(tw, tr)
				if err != nil {
					return err
				}
			}
		default:
			return fmt.Errorf("error reading tarball: %v", err)
		}
	}

	return nil
}

func runPatchManifest(args []string) (exit int) {
	var fh *os.File
	var err error

	if patchReplace && patchOverwrite {
		stderr("patch-manifest: Cannot use both --replace and --overwrite")
		return 1
	}
	if !patchReplace && len(args) != 2 {
		stderr("patch-manifest: Must provide input and output files (or use --replace)")
		return 1
	}
	if patchReplace && len(args) != 1 {
		stderr("patch-manifest: Must provide one file")
		return 1
	}
	if patchManifestFile != "" && (patchName != "" || patchExec != "" || patchUser != "" || patchGroup != "" || patchCaps != "" || patchMounts != "") {
		stderr("patch-manifest: --manifest is incompatible with other manifest editing options")
		return 1
	}

	inputFile = args[0]

	// Prepare output writer

	if patchReplace {
		fh, err = ioutil.TempFile(path.Dir(inputFile), ".actool-tmp."+path.Base(inputFile)+"-")
		if err != nil {
			stderr("patch-manifest: Cannot create temporary file: %v", err)
			return 1
		}
	} else {
		outputFile = args[1]

		ext := filepath.Ext(outputFile)
		if ext != schema.ACIExtension {
			stderr("patch-manifest: Extension must be %s (given %s)", schema.ACIExtension, ext)
			return 1
		}

		mode := os.O_CREATE | os.O_WRONLY
		if patchOverwrite {
			mode |= os.O_TRUNC
		} else {
			mode |= os.O_EXCL
		}

		fh, err = os.OpenFile(outputFile, mode, 0644)
		if err != nil {
			if os.IsExist(err) {
				stderr("patch-manifest: Output file exists (try --overwrite)")
			} else {
				stderr("patch-manifest: Unable to open output %s: %v", outputFile, err)
			}
			return 1
		}
	}

	var gw *gzip.Writer
	var w io.WriteCloser = fh
	if !patchNocompress {
		gw = gzip.NewWriter(fh)
		w = gw
	}
	tw := tar.NewWriter(w)

	defer func() {
		tw.Close()
		if !patchNocompress {
			gw.Close()
		}
		fh.Close()
		if exit != 0 && !patchOverwrite {
			os.Remove(fh.Name())
		}
	}()

	// Prepare input reader

	input, err := os.Open(inputFile)
	if err != nil {
		stderr("patch-manifest: Cannot open %s: %v", inputFile, err)
		return 1
	}
	defer input.Close()

	tr, err := aci.NewCompressedTarReader(input)
	if err != nil {
		stderr("patch-manifest: Cannot extract %s: %v", inputFile, err)
		return 1
	}
	defer tr.Close()

	var newManifest []byte

	if patchManifestFile != "" {
		mr, err := os.Open(patchManifestFile)
		if err != nil {
			stderr("patch-manifest: Cannot open %s: %v", patchManifestFile, err)
			return 1
		}
		defer input.Close()

		newManifest, err = ioutil.ReadAll(mr)
		if err != nil {
			stderr("patch-manifest: Cannot read %s: %v", patchManifestFile, err)
			return 1
		}
	}

	err = extractManifest(tr.Reader, tw, false, newManifest)
	if err != nil {
		stderr("patch-manifest: Unable to read %s: %v", inputFile, err)
		return 1
	}

	if patchReplace {
		err = os.Rename(fh.Name(), inputFile)
		if err != nil {
			stderr("patch-manifest: Cannot rename %q to %q: %v", fh.Name, inputFile, err)
			return 1
		}
	}

	return
}

func runCatManifest(args []string) (exit int) {
	if len(args) != 1 {
		stderr("cat-manifest: Must provide one file")
		return 1
	}

	inputFile = args[0]

	input, err := os.Open(inputFile)
	if err != nil {
		stderr("cat-manifest: Cannot open %s: %v", inputFile, err)
		return 1
	}
	defer input.Close()

	tr, err := aci.NewCompressedTarReader(input)
	if err != nil {
		stderr("cat-manifest: Cannot extract %s: %v", inputFile, err)
		return 1
	}
	defer tr.Close()

	err = extractManifest(tr.Reader, nil, true, nil)
	if err != nil {
		stderr("cat-manifest: Unable to read %s: %v", inputFile, err)
		return 1
	}

	return
}
