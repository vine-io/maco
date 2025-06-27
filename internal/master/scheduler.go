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

package master

import (
	"context"
	"sync"

	"github.com/alphadose/haxmap"
	iradix "github.com/hashicorp/go-immutable-radix/v2"

	"github.com/vine-io/maco/pkg/dsutil"
)

type pipe struct {
	ctx context.Context

	name string
	ip   string
}

type Scheduler struct {
	pmu   sync.RWMutex
	pipes *haxmap.Map[string, *pipe]

	gmu    sync.RWMutex
	groups *haxmap.Map[string, *dsutil.HashSet[string]]

	ipTree *iradix.Tree[string]

	storage *Storage
}

func NewScheduler(storage *Storage) (*Scheduler, error) {

	pipes := haxmap.New[string, *pipe]()
	groups := haxmap.New[string, *dsutil.HashSet[string]]()
	ipTree := iradix.New[string, *dsutil.HashSet[string]]()
	sch := &Scheduler{
		pipes:   pipes,
		groups:  groups,
		ipTree:  ipTree,
		storage: storage,
	}

	return sch, nil
}
