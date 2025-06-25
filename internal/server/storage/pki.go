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

package storage

import (
	"fmt"
	"os"
	"path/filepath"
)

func (s *Storage) walkPkiNodes(state string) ([]string, error) {
	switch state {
	case nodePath, nodePrePath, nodeAutoPath, nodeDeniedPath, nodeRejectPath:
	default:
		return nil, fmt.Errorf("invalid state: %s", state)
	}
	dir := filepath.Join(s.dir, "pki", state)
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	nodes := make([]string, 0, len(files))
	for i, entry := range files {
		nodes[i] = entry.Name()
	}
	return nodes, nil
}

func (s *Storage) addNode(id string, pubKey []byte) error {
	nodeId := filepath.Join(s.dir, "pki", nodePrePath, id)
	return os.WriteFile(nodeId, pubKey, 0600)
}

func (s *Storage) acceptNode(id string) error {
	preId := filepath.Join(s.dir, "pki", nodePrePath, id)
	nodeId := filepath.Join(s.dir, "pki", nodePath, id)

	stat, _ := os.Stat(preId)
	if stat != nil {
		return os.Rename(preId, nodeId)
	}

	rejectId := filepath.Join(s.dir, "pki", nodeRejectPath, id)
	stat, _ = os.Stat(rejectId)
	if stat != nil {
		return os.Rename(rejectId, nodeId)
	}
	return ErrNotFound
}

func (s *Storage) rejectNode(id string) error {
	preId := filepath.Join(s.dir, "pki", nodePrePath, id)
	nodeId := filepath.Join(s.dir, "pki", nodeRejectPath, id)

	stat, _ := os.Stat(preId)
	if stat != nil {
		return os.Rename(preId, nodeId)
	}

	acceptId := filepath.Join(s.dir, "pki", nodePath, id)
	stat, _ = os.Stat(acceptId)
	if stat != nil {
		return os.Rename(acceptId, nodeId)
	}

	return ErrNotFound
}

func (s *Storage) deleteNode(id string) error {
	preId := filepath.Join(s.dir, "pki", nodePrePath, id)
	exists, err := removeFile(preId)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	nodeId := filepath.Join(s.dir, "pki", nodePath, id)
	exists, err = removeFile(nodeId)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	rejectId := filepath.Join(s.dir, "pki", nodeRejectPath, id)
	exists, err = removeFile(rejectId)
	if err != nil {
		return err
	}

	if !exists {
		return ErrNotFound
	}

	return nil
}

func removeFile(path string) (bool, error) {
	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	err = os.Remove(path)
	if err != nil {
		return true, err
	}
	return true, nil
}
