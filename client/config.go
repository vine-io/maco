/*
Copyright 2025 The maco Authors

This program is offered under a commercial and under the AGPL license.
For AGPL licensing, see below.

AGPL licensing:
This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/BurntSushi/toml"
	"sigs.k8s.io/yaml"
)

const (
	DefaultTimeout = time.Second * 10
)

type Config struct {
	once sync.Once

	Target         string        `json:"target" toml:"target"`
	DialTimeout    time.Duration `json:"dial-timeout" toml:"dial-timeout"`
	RequestTimeout time.Duration `json:"request-timeout" toml:"request-timeout"`

	CertFile string `json:"cert-file" toml:"cert-file"`
	KeyFile  string `json:"key-file" toml:"key-file"`
	CaFile   string `json:"ca-file" toml:"ca-file"`
}

func NewConfig(target string) *Config {
	opts := &Config{
		Target:         target,
		DialTimeout:    DefaultTimeout,
		RequestTimeout: DefaultTimeout,
	}
	return opts
}

func FromPath(filename string) (*Config, error) {
	var cfg Config
	var err error
	ext := filepath.Ext(filename)
	switch ext {
	case ".toml":
		_, err = toml.DecodeFile(filename, &cfg)
	case ".yaml", ".yml":
		err = yaml.Unmarshal([]byte(filename), &cfg)
	case ".json":
		err = json.Unmarshal([]byte(filename), &cfg)
	default:
		return nil, fmt.Errorf("invalid config format: %s", ext)
	}
	if err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (cfg *Config) Init() error {
	var err error
	cfg.once.Do(func() {
		err = cfg.init()
	})
	return err
}

func (cfg *Config) init() error {
	if cfg.Target == "" {
		return fmt.Errorf("missing target")
	}
	if cfg.DialTimeout <= 0 {
		cfg.DialTimeout = DefaultTimeout
	}
	if cfg.RequestTimeout <= 0 {
		cfg.RequestTimeout = DefaultTimeout
	}
	return nil
}

// Save saves config text to specific file path
func (cfg *Config) Save(filename string) error {
	var err error
	var data []byte
	ext := filepath.Ext(filename)
	switch ext {
	case ".toml":
		buf := bytes.NewBufferString("")
		err = toml.NewEncoder(buf).Encode(cfg)
		if err == nil {
			data = buf.Bytes()
		}
	case ".yaml", ".yml":
		data, err = yaml.Marshal(cfg)
	case ".json":
		data, err = json.Marshal(cfg)
	default:
		return fmt.Errorf("invalid config format: %s", ext)
	}
	if err != nil {
		return err
	}
	return os.WriteFile(filename, data, 0755)
}
