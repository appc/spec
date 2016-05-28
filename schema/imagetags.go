// Copyright 2016 The appc Authors
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

package schema

import (
	"encoding/json"
	"fmt"

	"github.com/appc/spec/schema/types"
)

type ImageTags struct {
	Aliases TagAliases `json:"aliases"`
	Labels  TagLabels  `json:"labels"`
}

type imageTags ImageTags

type TagAliases map[string]string

type TagLabels map[string]map[types.ACIdentifier]string

//TODO(sgotti) add validation, circular references checks etc...
func (t ImageTags) assertValid() error {
	return nil
}

func (t ImageTags) MarshalJSON() ([]byte, error) {
	if err := t.assertValid(); err != nil {
		return nil, err
	}
	return json.Marshal(imageTags(t))
}

func (t *ImageTags) UnmarshalJSON(data []byte) error {
	var jt imageTags
	if err := json.Unmarshal(data, &jt); err != nil {
		return err
	}
	nt := ImageTags(jt)
	if err := nt.assertValid(); err != nil {
		return err
	}
	*t = nt
	return nil
}

// Resolve will resolve tag Aliases until exausted (checking for circular
// dependencies) and then it will return the Labels referenced from the resolved
// tag if existing or nil if not.
func (t *ImageTags) Resolve(tag string) (map[types.ACIdentifier]string, error) {
	curtag := tag
	seen := map[string]struct{}{}
	seen[curtag] = struct{}{}
	for {
		end := true
		if alias, ok := t.Aliases[tag]; ok {
			if _, ok := seen[alias]; ok {
				return nil, fmt.Errorf("circular dependency between tag aliases")
			}
			curtag = alias
			seen[curtag] = struct{}{}
			end = false
			break
		}
		if end {
			break
		}
	}
	if labels, ok := t.Labels[curtag]; ok {
		return labels, nil
	}
	return nil, nil
}

// MergeTag will resolve image tags labels from tag and return the new merged labels
func (t *ImageTags) MergeTag(labels types.LabelsMap, tag string) (types.LabelsMap, error) {
	newlabels := labels.Copy()
	// if tag is empty stop here
	if tag == "" {
		return labels, nil
	}
	// Not tag data provided. Fallback setting version label value to tag value
	if t == nil {
		if _, ok := newlabels["version"]; !ok {
			newlabels["version"] = tag
			return newlabels, nil
		} else {
			return nil, fmt.Errorf("cannot set tag value to version label since version label is already defined")
		}
	}
	tagLabels, err := t.Resolve(tag)
	if err != nil {
		return nil, err
	}
	// No labels resolved from tag.
	if tagLabels == nil {
		return newlabels, nil
	}
	// Merge tagLabels with app labels. App specified labels have the precedence.
	for n, v := range tagLabels {
		if _, ok := newlabels[n]; !ok {
			newlabels[n] = v
		}
	}
	return newlabels, nil
}
