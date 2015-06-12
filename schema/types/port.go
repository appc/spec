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
)

type Port struct {
	Name            ACName `json:"name"`
	Protocol        string `json:"protocol"`
	Port            uint   `json:"port"`
	Count           uint   `json:"count"`
	SocketActivated bool   `json:"socketActivated"`
}

type ExposedPort struct {
	Name     ACName `json:"name"`
	HostPort uint   `json:"hostPort"`
}

type port Port

func (p *Port) UnmarshalJSON(data []byte) error {
	var pp port
	if err := json.Unmarshal(data, &pp); err != nil {
		return err
	}
	np := Port(pp)
	if err := np.assertValid(); err != nil {
		return err
	}
	if np.Count == 0 {
		np.Count = 1
	}
	*p = np
	return nil
}

func (p Port) MarshalJSON() ([]byte, error) {
	if err := p.assertValid(); err != nil {
		return nil, err
	}
	return json.Marshal(port(p))
}

func (p Port) assertValid() error {
	// Although there are no guarantees, most (if not all)
	// transport protocols use 16 bit ports
	if p.Port > 65535 || p.Port < 1 {
		return errors.New("port must be in 1-65535 range")
	}
	if p.Port+p.Count > 65536 {
		return errors.New("end of port range must be in 1-65535 range")
	}
	return nil
}
