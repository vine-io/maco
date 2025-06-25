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

package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"go.uber.org/zap"
	"sigs.k8s.io/yaml"

	"github.com/vine-io/maco/pkg/logutil"
)

var (
	DefaultListenAddress = ":4500"
)

type Config struct {
	Listen   string `json:"listen" toml:"listen"`
	CertFile string `json:"cert-file" toml:"cert-file"`
	KeyFile  string `json:"key-file" toml:"key-file"`
	CaFile   string `json:"ca-file" toml:"ca-file"`

	EnableOpenAPI bool `json:"enable_openapi" toml:"enable_openapi"`

	DataRoot string `json:"data_root" toml:"data_root"`

	AutoAccept bool `json:"auto_accept" toml:"auto_accept"`

	Log *logutil.LogConfig `json:"log" toml:"log"`
}

func NewConfig() *Config {
	lc := logutil.NewLogConfig()
	cfg := &Config{
		Listen: DefaultListenAddress,
		Log:    &lc,
	}

	return cfg
}

func (cfg *Config) Init() error {
	if cfg.Log == nil {
		lc := logutil.NewLogConfig()
		cfg.Log = &lc
	}
	err := cfg.Log.SetupLogging()
	cfg.Log.SetupGlobalLoggers()
	if err != nil {
		return fmt.Errorf("init logger: %w", err)
	}

	if cfg.DataRoot == "" {
		home, _ := os.UserHomeDir()
		cfg.DataRoot = filepath.Join(home, ".maco")
		_ = os.MkdirAll(cfg.DataRoot, 0755)
	} else {
		_, err := os.Stat(cfg.DataRoot)
		if err != nil {
			if !os.IsNotExist(err) {
				return fmt.Errorf("read data root directory: %w", err)
			}
			_ = os.MkdirAll(cfg.DataRoot, 0755)
		}
		if strings.HasPrefix(cfg.DataRoot, "~") || strings.HasPrefix(cfg.DataRoot, "./") {
			abs, err := filepath.Abs(cfg.DataRoot)
			if err != nil {
				return fmt.Errorf("get data-root abs path: %w", err)
			}
			cfg.DataRoot = abs
		}
	}
	return nil
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

func (cfg *Config) Logger() *zap.Logger {
	return cfg.Log.GetLogger()
}
