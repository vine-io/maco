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

package dbutil

import (
	"os"
	"testing"

	"github.com/dgraph-io/badger/v4"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestDB(t *testing.T) {
	dir, _ := os.MkdirTemp(os.TempDir(), "a")
	defer os.RemoveAll(dir)
	opt := &Options{
		Dir:    dir,
		Logger: zap.NewNop(),
	}
	db, err := OpenDB(opt)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	err = db.Set([]byte("a"), []byte("b"))
	if err != nil {
		t.Fatal(err)
	}

	val, err := db.Get([]byte("a"))
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, []byte("b"), val)

	err = db.Set([]byte("aa1"), []byte("bb"))
	if !assert.NoError(t, err) {
		t.Fatal(err)
	}
	err = db.Set([]byte("aa2"), []byte("bc"))
	if !assert.NoError(t, err) {
		t.Fatal(err)
	}
	err = db.Set([]byte("aa3"), []byte("bd"))
	if !assert.NoError(t, err) {
		t.Fatal(err)
	}
	vals := []string{}
	err = db.Range([]byte("aa"), func(k []byte, v []byte) error {
		vals = append(vals, string(v))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, []string{"bb", "bc", "bd"}, vals)

	if err = db.Delete([]byte("aaa")); err != nil {
		t.Fatal(err)
	}

	_, err = db.Get([]byte("aaa"))
	assert.Equal(t, err, badger.ErrKeyNotFound)
}
