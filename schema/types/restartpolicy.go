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
	"fmt"
)

type RestartPolicy string

var validPolicies = map[RestartPolicy]struct{}{
	"always":    struct{}{},
	"onFailure": struct{}{},
	"never":     struct{}{},
}

type restartPolicy RestartPolicy

func (r *RestartPolicy) UnmarshalJSON(data []byte) error {
	var rp restartPolicy
	if err := json.Unmarshal(data, &rp); err != nil {
		return err
	}
	rr := RestartPolicy(rp)
	if err := rr.assertValid(); err != nil {
		return err
	}
	*r = rr
	return nil
}

func (r RestartPolicy) MarshalJSON() ([]byte, error) {
	if err := r.assertValid(); err != nil {
		return nil, err
	}
	return json.Marshal(restartPolicy(r))
}

func (r RestartPolicy) assertValid() error {
	if _, ok := validPolicies[r]; !ok {
		return fmt.Errorf("invalid restart policy %q", string(r))
	}
	return nil
}
