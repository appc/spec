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

package types

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strconv"

	"github.com/appc/spec/schema/common"
)

// Volume encapsulates a volume which should be mounted into the filesystem
// of all apps in a PodManifest
type Volume struct {
	Name ACName `json:"name"`
	Kind string `json:"kind"`

	// currently used only by "host"
	// TODO(jonboulle): factor out?
	Source   string `json:"source,omitempty"`
	ReadOnly *bool  `json:"readOnly,omitempty"`

	// currently used only by "empty"
	Mode string `json:"mode,omitempty"`
	UID  int    `json:"uid,omitempty"`
	GID  int    `json:"gid,omitempty"`
}

type volume Volume

func (v Volume) assertValid() error {
	if v.Name.Empty() {
		return errors.New("name must be set")
	}

	switch v.Kind {
	case "empty":
		if v.Source != "" {
			return errors.New("source for empty volume must be empty")
		}
		if v.Mode == "" {
			return errors.New("mode for empty volume must be set")
		}
		if v.UID == -1 {
			return errors.New("uid for empty volume must be set")
		}
		if v.GID == -1 {
			return errors.New("gid for empty volume must be set")
		}
		return nil
	case "host":
		if v.Source == "" {
			return errors.New("source for host volume cannot be empty")
		}
		if !filepath.IsAbs(v.Source) {
			return errors.New("source for host volume must be absolute path")
		}
		return nil
	default:
		return errors.New(`unrecognized volume kind: should be one of "empty", "host"`)
	}
}

func (v *Volume) UnmarshalJSON(data []byte) error {
	var vv volume
	vv.Mode = "0755"
	vv.UID = 0
	vv.GID = 0
	if err := json.Unmarshal(data, &vv); err != nil {
		return err
	}
	nv := Volume(vv)
	if err := nv.assertValid(); err != nil {
		return err
	}
	if nv.Kind != "empty" {
		nv.Mode = ""
		nv.UID = -1
		nv.GID = -1
	}
	*v = nv
	return nil
}

func (v Volume) MarshalJSON() ([]byte, error) {
	if err := v.assertValid(); err != nil {
		return nil, err
	}
	return json.Marshal(volume(v))
}

func (v Volume) String() string {
	s := fmt.Sprintf("%s,kind=%s,readOnly=%t", v.Name, v.Kind, *v.ReadOnly)
	if v.Source != "" {
		s = s + fmt.Sprintf(",source=%s", v.Source)
	}
	if v.Mode != "" && v.UID != -1 && v.GID != -1 {
		s = s + fmt.Sprintf(",mode=%s,uid=%d,gid=%d", v.Mode, v.UID, v.GID)
	}
	return s
}

// VolumeFromString takes a command line volume parameter and returns a volume
//
// Example volume parameters:
// 	database,kind=host,source=/tmp,readOnly=true
func VolumeFromString(vp string) (*Volume, error) {
	vol := Volume{
		Mode: "0755",
		UID:  0,
		GID:  0,
	}

	vp = "name=" + vp
	vpQuery, err := common.MakeQueryString(vp)
	if err != nil {
		return nil, err
	}

	v, err := url.ParseQuery(vpQuery)
	if err != nil {
		return nil, err
	}
	for key, val := range v {
		if len(val) > 1 {
			return nil, fmt.Errorf("label %s with multiple values %q", key, val)
		}

		switch key {
		case "name":
			acn, err := NewACName(val[0])
			if err != nil {
				return nil, err
			}
			vol.Name = *acn
		case "kind":
			vol.Kind = val[0]
		case "source":
			vol.Source = val[0]
		case "readOnly":
			ro, err := strconv.ParseBool(val[0])
			if err != nil {
				return nil, err
			}
			vol.ReadOnly = &ro
		case "mode":
			vol.Mode = val[0]
		case "uid":
			u, err := strconv.Atoi(val[0])
			if err != nil {
				return nil, err
			}
			vol.UID = u
		case "gid":
			g, err := strconv.Atoi(val[0])
			if err != nil {
				return nil, err
			}
			vol.GID = g
		default:
			return nil, fmt.Errorf("unknown volume parameter %q", key)
		}
	}
	err = vol.assertValid()
	if err != nil {
		return nil, err
	}
	if vol.Kind != "empty" {
		vol.Mode = ""
		vol.UID = -1
		vol.GID = -1
	}

	return &vol, nil
}
