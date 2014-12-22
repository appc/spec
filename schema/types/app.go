package types

import (
	"encoding/json"
	"errors"
	"path/filepath"
)

type App struct {
	Exec          []string          `json:"exec"`
	EventHandlers []EventHandler    `json:"eventHandlers,omitempty"`
	User          string            `json:"user"`
	Group         string            `json:"group"`
	Environment   map[string]string `json:"environment,omitempty"`
	MountPoints   []MountPoint      `json:"mountPoints,omitempty"`
	Ports         []Port            `json:"ports,omitempty"`
	Isolators     []Isolator        `json:"isolators,omitempty"`
}

// app is a model to facilitate extra validation during the
// unmarshalling of the App
type app App

func (a *App) UnmarshalJSON(data []byte) error {
	ja := app{}
	err := json.Unmarshal(data, &ja)
	if err != nil {
		return err
	}
	na := App(ja)
	if err := na.assertValid(); err != nil {
		return err
	}
	if na.Environment == nil {
		na.Environment = make(map[string]string)
	}
	*a = na
	return nil
}

func (a App) MarshalJSON() ([]byte, error) {
	if err := a.assertValid(); err != nil {
		return nil, err
	}
	return json.Marshal(app(a))
}

func (a *App) assertValid() error {
	if len(a.Exec) < 1 {
		return errors.New(`Exec cannot be empty`)
	}
	if !filepath.IsAbs(a.Exec[0]) {
		return errors.New(`Exec[0] must be absolute path`)
	}
	if a.User == "" {
		return errors.New(`User is required`)
	}
	if a.Group == "" {
		return errors.New(`Group is required`)
	}
	return nil
}
