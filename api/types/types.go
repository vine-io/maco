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

package types

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
)

type MinionState string

const (
	Unaccepted MinionState = "unaccepted"
	Accepted   MinionState = "accepted"
	AutoSign   MinionState = "auto_sign"
	Denied     MinionState = "denied"
	Rejected   MinionState = "rejected"
)

func (s MinionState) String() string {
	return string(s)
}

type SelectionOption func(*SelectionOptions)

func WithHosts(host string, or ...bool) SelectionOption {
	s := &Selection{
		Host: []string{host},
	}
	f := true
	if len(or) > 0 && !or[0] {
		f = false
	}
	return func(o *SelectionOptions) { o.append(s, f) }
}

func WithList(hosts []string, lg ...bool) SelectionOption {
	s := &Selection{
		Host: hosts,
	}
	f := true
	if len(lg) > 0 && !lg[0] {
		f = false
	}
	return func(o *SelectionOptions) { o.append(s, f) }
}

func WithHostRegex(pattern string, or ...bool) SelectionOption {
	s := &Selection{
		HostPcre: pattern,
	}
	f := true
	if len(or) > 0 && !or[0] {
		f = false
	}
	return func(o *SelectionOptions) { o.append(s, f) }
}

func WithRange(idt string, or ...bool) SelectionOption {
	s := &Selection{
		IdRange: idt,
	}
	f := true
	if len(or) > 0 && !or[0] {
		f = false
	}
	return func(o *SelectionOptions) { o.append(s, f) }
}

func WithHostGroup(groups []string, or ...bool) SelectionOption {
	s := &Selection{
		HostGroups: groups,
	}
	f := true
	if len(or) > 0 && !or[0] {
		f = false
	}
	return func(o *SelectionOptions) { o.append(s, f) }
}

func WithIPCidr(cidr string, or ...bool) SelectionOption {
	s := &Selection{
		IpCidr: cidr,
	}
	f := true
	if len(or) > 0 && !or[0] {
		f = false
	}
	return func(o *SelectionOptions) { o.append(s, f) }
}

func WithGrains(key, value string, or ...bool) SelectionOption {
	s := &Selection{
		Grains: &SelectionPair{Key: key, Value: value},
	}
	f := true
	if len(or) > 0 && !or[0] {
		f = false
	}
	return func(o *SelectionOptions) { o.append(s, f) }
}

func WithGrainsRegex(key, pattern string, or ...bool) SelectionOption {
	s := &Selection{
		GrainsPcre: &SelectionPair{Key: key, Value: pattern},
	}
	f := true
	if len(or) > 0 && !or[0] {
		f = false
	}
	return func(o *SelectionOptions) { o.append(s, f) }
}

func WithPillar(key string, value string, lg ...bool) SelectionOption {
	s := &Selection{
		Pillar: &SelectionPair{Key: key, Value: value},
	}
	f := true
	if len(lg) > 0 && !lg[0] {
		f = false
	}
	return func(o *SelectionOptions) { o.append(s, f) }
}

func WithPillarRegex(key, pattern string, or ...bool) SelectionOption {
	s := &Selection{
		PillarPcre: &SelectionPair{Key: key, Value: pattern},
	}
	f := true
	if len(or) > 0 && !or[0] {
		f = false
	}
	return func(o *SelectionOptions) { o.append(s, f) }
}

func (m *SelectionOptions) and(s *Selection) {
	m.Selections = append(m.Selections, &Selection{And: &LogicAnd{}}, s)
}

func (m *SelectionOptions) or(s *Selection) {
	m.Selections = append(m.Selections, &Selection{Or: &LogicOr{}}, s)
}

func (m *SelectionOptions) append(s *Selection, and bool) {
	if m.Selections == nil {
		m.Selections = []*Selection{s}
	} else {
		if and {
			m.and(s)
		} else {
			m.or(s)
		}
	}
}

func (m *SelectionOptions) Validate() error {
	hasSelection := false

	lastIsLogic := false
	for i, s := range m.Selections {
		text := s.ToText()
		if text != "" && text != "and" && text != "or" {
			hasSelection = true
		}
		if text == "" {
			return fmt.Errorf("empty selection at selection[%d]", i)
		}
		if i == 0 && s.isLogic() {
			return fmt.Errorf("invalid selection[0]: %s", s.String())
		}
		if lastIsLogic && s.isLogic() {
			return fmt.Errorf("continuous logic selection at selection[%d]", i)
		}

		pattern := ""
		if s.HostPcre != "" {
			pattern = s.HostPcre
		}
		if idx := strings.Index(text, "@"); idx > 0 {
			tag := text[:idx]
			if tag == "E" || tag == "P" || tag == "J" {
				_, before, ok := strings.Cut(text, ":")
				if ok {
					pattern = before
				} else {
					pattern = text
				}
			}
		}
		if pattern == "" {
			_, err := regexp.CompilePOSIX(pattern)
			if err != nil {
				return fmt.Errorf("invalid regexp '%s' at selection[%d]", pattern, i)
			}
		}

		lastIsLogic = s.isLogic()
	}

	if !hasSelection {
		return fmt.Errorf("no selection options found")
	}

	return nil
}

func (m *Selection) ToText() string {
	if len(m.Host) != 0 {
		return strings.Join(m.Host, ",")
	}
	if len(m.HostPcre) != 0 {
		return fmt.Sprintf("E@%s", m.HostPcre)
	}
	if len(m.IdRange) != 0 {
		return fmt.Sprintf("R@%s", m.IdRange)
	}
	if len(m.HostGroups) != 0 {
		return fmt.Sprintf("N@%s", strings.Join(m.HostGroups, ","))
	}
	if len(m.IpCidr) != 0 {
		return fmt.Sprintf("S@%s", m.IpCidr)
	}

	if pair := m.Grains; pair != nil {
		return fmt.Sprintf("G@%s:%s", pair.Key, pair.Value)
	}
	if pair := m.GrainsPcre; pair != nil {
		return fmt.Sprintf("P@%s:%s", pair.Key, pair.Value)
	}
	if pair := m.Pillar; pair != nil {
		return fmt.Sprintf("I@%s:%s", pair.Key, pair.Value)
	}
	if pair := m.PillarPcre; pair != nil {
		return fmt.Sprintf("J@%s:%s", pair.Key, pair.Value)
	}
	if m.And != nil {
		return "and"
	}
	if m.Or != nil {
		return "or"
	}
	return ""
}

func (m *Selection) isLogic() bool {
	if m.And != nil {
		return true
	}
	if m.Or != nil {
		return true
	}
	return false
}

func (m *SelectionOptions) ToText() string {
	buf := bytes.NewBufferString("")

	length := len(m.Selections)
	for i, selection := range m.Selections {
		buf.WriteString(selection.ToText())
		if i < length-1 {
			buf.WriteString(" ")
		}
	}

	return buf.String()
}

func NewSelectionOptions(opts ...SelectionOption) (*SelectionOptions, error) {
	options := &SelectionOptions{}

	for _, opt := range opts {
		opt(options)
	}

	if err := options.Validate(); err != nil {
		return nil, err
	}

	return options, nil
}

func ParseSelection(text string) (*SelectionOptions, error) {
	selections := make([]*Selection, 0)

	tag := ""
	key := ""
	value := ""

	text = strings.TrimSpace(text)

	length := len(text)
	i, j := 0, 0
	for {
		if (j < length && text[j] == ' ') || j == length {

			for k := i; k < j; k++ {
				if text[k] == '@' {
					tag = strings.TrimSpace(text[i:k])
					i = k + 1
				}
				if text[k] == ':' {
					key = strings.TrimSpace(text[i:k])
					i = k + 1
				}
			}
			value = strings.TrimSpace(text[i:j])

			var selection *Selection
			switch tag {
			case "E":
				_, err := regexp.CompilePOSIX(value)
				if err != nil {
					return nil, fmt.Errorf("invalid pillar regexp 'E@%s'", value)
				}

				selection = &Selection{HostPcre: value}
			case "R":
				selection = &Selection{IdRange: value}
			case "N":
				selection = &Selection{HostGroups: strings.Split(value, ",")}
			case "S":
				selection = &Selection{IpCidr: value}
			case "G":
				selection = &Selection{Grains: &SelectionPair{Key: key, Value: value}}
			case "P":
				_, err := regexp.CompilePOSIX(value)
				if err != nil {
					return nil, fmt.Errorf("invalid grains regexp 'P@%s:%s'", key, value)
				}

				selection = &Selection{GrainsPcre: &SelectionPair{Key: key, Value: value}}
			case "I":
				selection = &Selection{Pillar: &SelectionPair{Key: key, Value: value}}
			case "J":
				_, err := regexp.CompilePOSIX(value)
				if err != nil {
					return nil, fmt.Errorf("invalid pillar regexp 'J@%s:%s'", key, value)
				}

				selection = &Selection{PillarPcre: &SelectionPair{Key: key, Value: value}}
			case "and":
				selection = &Selection{And: &LogicAnd{}}
			case "or":
				selection = &Selection{Or: &LogicOr{}}
			case "":
				if value == "*" {
					selection = &Selection{Host: []string{"*"}}
				} else {
					selection = &Selection{Host: strings.Split(value, ",")}
				}
			}

			if selection != nil {
				selections = append(selections, selection)
			}

			tag = ""
			key = ""
			value = ""
		}

		if j >= length {
			break
		}

		if j < length && text[j] == ' ' {
			i = j
		}
		j += 1
	}

	options := &SelectionOptions{
		Selections: selections,
	}

	return options, nil
}
