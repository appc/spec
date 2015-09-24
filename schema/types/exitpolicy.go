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

type ExitPolicy string

var validPolicies = map[ExitPolicy]struct{}{
	"untilAll":     struct{}{},
	"onAny":        struct{}{},
	"onAnyFailure": struct{}{},
}

type exitPolicy ExitPolicy

func (e *ExitPolicy) UnmarshalJSON(data []byte) error {
	var ep exitPolicy
	if err := json.Unmarshal(data, &ep); err != nil {
		return err
	}
	ee := ExitPolicy(ep)
	if err := ee.assertValid(); err != nil {
		return err
	}
	*e = ee
	return nil
}

func (e ExitPolicy) MarshalJSON() ([]byte, error) {
	if err := e.assertValid(); err != nil {
		return nil, err
	}
	return json.Marshal(exitPolicy(e))
}

func (e ExitPolicy) assertValid() error {
	if _, ok := validPolicies[e]; !ok {
		return fmt.Errorf("invalid exit policy %q", string(e))
	}
	return nil
}
