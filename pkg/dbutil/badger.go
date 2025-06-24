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
	"github.com/dgraph-io/badger/v4"
	"go.uber.org/zap"
)

type logger struct {
	lg *zap.Logger
}

func (lg *logger) Errorf(format string, args ...interface{}) {
	lg.lg.Sugar().Errorf(format, args...)
}

func (lg *logger) Warningf(format string, args ...interface{}) {
	lg.lg.Sugar().Warnf(format, args...)
}

func (lg *logger) Infof(format string, args ...interface{}) {
	lg.lg.Sugar().Infof(format, args...)
}

func (lg *logger) Debugf(format string, args ...interface{}) {
	lg.lg.Sugar().Debugf(format, args...)
}

type Options struct {
	Dir    string
	Logger *zap.Logger
}

type DB struct {
	db *badger.DB
}

func OpenDB(opt *Options) (*DB, error) {

	lg := &logger{
		lg: opt.Logger,
	}
	dbOpts := badger.DefaultOptions(opt.Dir).
		WithLogger(lg)

	badgerDB, err := badger.Open(dbOpts)
	if err != nil {
		return nil, err
	}
	db := &DB{
		db: badgerDB,
	}
	return db, nil
}

func (db *DB) Get(key []byte) ([]byte, error) {
	txn := db.db.NewTransaction(false)
	defer txn.Discard()

	item, err := txn.Get(key)
	if err != nil {
		return nil, err
	}

	var value []byte
	err = item.Value(func(val []byte) error {
		value = val
		return nil
	})
	if err != nil {
		return nil, err
	}

	return value, nil
}

func (db *DB) Range(prefix []byte, fn func(key []byte, value []byte) error) error {
	txn := db.db.NewTransaction(false)
	defer txn.Discard()

	opts := badger.DefaultIteratorOptions
	opts.PrefetchValues = false
	it := txn.NewIterator(opts)
	defer it.Close()
	for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
		item := it.Item()
		err := item.Value(func(value []byte) error {
			return fn(item.Key(), value)
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func (db *DB) Set(key, value []byte) error {
	txn := db.db.NewTransaction(true)
	defer txn.Discard()

	var err error
	err = txn.Set(key, value)
	if err != nil {
		return err
	}

	err = txn.Commit()
	if err != nil {
		return err
	}

	return nil
}

func (db *DB) Delete(key []byte) error {
	txn := db.db.NewTransaction(true)
	defer txn.Discard()

	err := txn.Delete(key)
	if err != nil {
		return err
	}

	err = txn.Commit()
	if err != nil {
		return err
	}

	return nil
}

func (db *DB) Sync() error {
	return db.db.Sync()
}

func (db *DB) Close() error {
	return db.db.Close()
}
