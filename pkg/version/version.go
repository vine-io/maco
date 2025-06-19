/*
Copyright 2025 The maco Authors

This library is free software; you can redistribute it and/or
modify it under the terms of the GNU Lesser General Public
License as published by the Free Software Foundation; either
version 2.1 of the License, or (at your option) any later version.

This library is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the GNU
Lesser General Public License for more details.

You should have received a copy of the GNU Lesser General Public
License along with this library;
*/

package bpmn

import (
	"fmt"
	"runtime"
	"strings"
)

var (
	GitCommit = ""
	GitTag    = ""
	BuildDate = ""
)

func Version() string {
	var version string

	if GitTag != "" {
		version = GitTag
	}

	if GitCommit != "" {
		version += fmt.Sprintf("-%s", GitCommit)
	}

	if BuildDate != "" {
		version += fmt.Sprintf("-%s", BuildDate)
	}

	if version == "" {
		version = "latest"
	}

	return version
}

func GoV() string {
	v := strings.TrimPrefix(runtime.Version(), "go")
	if strings.Count(v, ".") > 1 {
		v = v[:strings.LastIndex(v, ".")]
	}
	return v
}
