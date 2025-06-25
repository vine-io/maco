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

func (s *Storage) walkPkiMinions(state string) ([]string, error) {
	switch state {
	case minionPath, minionPrePath, minionAutoPath, minionDeniedPath, minionRejectPath:
	default:
		return nil, fmt.Errorf("invalid state: %s", state)
	}
	dir := filepath.Join(s.dir, "pki", state)
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	minions := make([]string, 0, len(files))
	for i, entry := range files {
		minions[i] = entry.Name()
	}
	return minions, nil
}

func (s *Storage) addMinion(id string, pubKey []byte) error {
	minionId := filepath.Join(s.dir, "pki", minionPrePath, id)
	return os.WriteFile(minionId, pubKey, 0600)
}

func (s *Storage) acceptMinion(id string) error {
	preId := filepath.Join(s.dir, "pki", minionPrePath, id)
	minionId := filepath.Join(s.dir, "pki", minionPath, id)

	stat, _ := os.Stat(preId)
	if stat != nil {
		return os.Rename(preId, minionId)
	}

	rejectId := filepath.Join(s.dir, "pki", minionRejectPath, id)
	stat, _ = os.Stat(rejectId)
	if stat != nil {
		return os.Rename(rejectId, minionId)
	}
	return ErrNotFound
}

func (s *Storage) rejectMinion(id string) error {
	preId := filepath.Join(s.dir, "pki", minionPrePath, id)
	minionId := filepath.Join(s.dir, "pki", minionRejectPath, id)

	stat, _ := os.Stat(preId)
	if stat != nil {
		return os.Rename(preId, minionId)
	}

	acceptId := filepath.Join(s.dir, "pki", minionPath, id)
	stat, _ = os.Stat(acceptId)
	if stat != nil {
		return os.Rename(acceptId, minionId)
	}

	return ErrNotFound
}

func (s *Storage) deleteMinion(id string) error {
	preId := filepath.Join(s.dir, "pki", minionPrePath, id)
	exists, err := removeFile(preId)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	minionId := filepath.Join(s.dir, "pki", minionPath, id)
	exists, err = removeFile(minionId)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	rejectId := filepath.Join(s.dir, "pki", minionRejectPath, id)
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
