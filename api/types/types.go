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
	"net"
	"regexp"
	"strings"

	"github.com/vine-io/maco/pkg/iprange"
)

type MinionState string

const (
	Unaccepted MinionState = "unaccepted"
	Accepted   MinionState = "accepted"
	AutoSign   MinionState = "auto_sign"
	Denied     MinionState = "denied"
	Rejected   MinionState = "rejected"
)

// String 返回 MinionState 的字符串表示
//
// 此方法实现了 fmt.Stringer 接口，将 MinionState 类型转换为字符串。
//
// @returns:
//   - string: MinionState 的字符串值
//
// @example:
//
//	state := Accepted
//	fmt.Println(state.String()) // 输出: "accepted"
func (s MinionState) String() string {
	return string(s)
}

type SelectionTarget interface {
	Id() string
	IP() string
	Groups() []string
	Grains() map[string]string
	Pillars() map[string]string
}

type SelectionOption func(*SelectionOptions)

// WithHosts 创建一个基于主机名的选择选项
//
// 此函数返回一个 SelectionOption，用于匹配指定的单个主机名。
// 支持通配符 "*" 来匹配所有主机。
//
// @params:
//   - host: 要匹配的主机名，支持 "*" 通配符
//   - or: 可选的逻辑运算符标志
//   - true 或未提供: 使用 AND 逻辑连接（默认）
//   - false: 使用 OR 逻辑连接
//
// @returns:
//   - SelectionOption: 选择选项函数，用于配置 SelectionOptions
//
// @example:
//
//	// 匹配指定主机
//	opt1 := WithHosts("web01")
//
//	// 匹配所有主机
//	opt2 := WithHosts("*")
//
//	// 使用 OR 逻辑
//	opt3 := WithHosts("web01", false)
func WithHosts(host string, or ...bool) SelectionOption {
	s := &Selection{
		Hosts: []string{host},
	}
	f := true
	if len(or) > 0 && !or[0] {
		f = false
	}
	return func(o *SelectionOptions) { o.append(s, f) }
}

// WithList 创建一个基于主机名列表的选择选项
//
// 此函数返回一个 SelectionOption，用于匹配指定的多个主机名。
// 只要目标主机名在列表中，就会匹配成功。
//
// @params:
//   - hosts: 要匹配的主机名列表
//   - lg: 可选的逻辑运算符标志
//   - true 或未提供: 使用 AND 逻辑连接（默认）
//   - false: 使用 OR 逻辑连接
//
// @returns:
//   - SelectionOption: 选择选项函数，用于配置 SelectionOptions
//
// @example:
//
//	// 匹配多个指定主机
//	opt1 := WithList([]string{"web01", "web02", "web03"})
//
//	// 使用 OR 逻辑
//	opt2 := WithList([]string{"db01", "db02"}, false)
func WithList(hosts []string, lg ...bool) SelectionOption {
	s := &Selection{
		Hosts: hosts,
	}
	f := true
	if len(lg) > 0 && !lg[0] {
		f = false
	}
	return func(o *SelectionOptions) { o.append(s, f) }
}

// WithHostRegex 创建一个基于主机名正则表达式的选择选项
//
// 此函数返回一个 SelectionOption，用于匹配符合正则表达式模式的主机名。
// 使用 POSIX 正则表达式语法。
//
// @params:
//   - pattern: POSIX 正则表达式模式
//   - or: 可选的逻辑运算符标志
//   - true 或未提供: 使用 AND 逻辑连接（默认）
//   - false: 使用 OR 逻辑连接
//
// @returns:
//   - SelectionOption: 选择选项函数，用于配置 SelectionOptions
//
// @example:
//
//	// 匹配以 "web" 开头的主机
//	opt1 := WithHostRegex("^web.*")
//
//	// 匹配包含数字的主机名
//	opt2 := WithHostRegex(".*[0-9]+.*")
//
//	// 使用 OR 逻辑
//	opt3 := WithHostRegex("node[0-9]+", false)
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

// WithRange 创建一个基于ID范围的选择选项
//
// 此函数返回一个 SelectionOption，用于匹配符合ID范围模式的主机。
// 支持前缀匹配（%prefix）和后缀匹配（suffix%）。
//
// @params:
//   - idt: ID范围模式
//   - "prefix%": 匹配以 "prefix" 开头的ID
//   - "%suffix": 匹配以 "suffix" 结尾的ID
//   - "%middle%": 匹配包含 "middle" 的ID
//   - or: 可选的逻辑运算符标志
//   - true 或未提供: 使用 AND 逻辑连接（默认）
//   - false: 使用 OR 逻辑连接
//
// @returns:
//   - SelectionOption: 选择选项函数，用于配置 SelectionOptions
//
// @example:
//
//	// 匹配以 "web" 开头的ID
//	opt1 := WithRange("web%")
//
//	// 匹配以 "-prod" 结尾的ID
//	opt2 := WithRange("%-prod")
//
//	// 使用 OR 逻辑
//	opt3 := WithRange("db%", false)
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

// WithHostGroup 创建一个基于主机组的选择选项
//
// 此函数返回一个 SelectionOption，用于匹配属于指定主机组的主机。
// 只要目标主机属于任一指定组，就会匹配成功。
//
// @params:
//   - groups: 要匹配的主机组列表
//   - or: 可选的逻辑运算符标志
//   - true 或未提供: 使用 AND 逻辑连接（默认）
//   - false: 使用 OR 逻辑连接
//
// @returns:
//   - SelectionOption: 选择选项函数，用于配置 SelectionOptions
//
// @example:
//
//	// 匹配 web 组的主机
//	opt1 := WithHostGroup([]string{"web"})
//
//	// 匹配多个组的主机
//	opt2 := WithHostGroup([]string{"web", "api", "frontend"})
//
//	// 使用 OR 逻辑
//	opt3 := WithHostGroup([]string{"database"}, false)
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

// WithIPCidr 创建一个基于IP CIDR的选择选项
//
// 此函数返回一个 SelectionOption，用于匹配指定CIDR网络范围内的主机。
// 支持IPv4和IPv6的CIDR表示法。
//
// @params:
//   - cidr: CIDR网络范围表示法（如 "192.168.1.0/24"）
//   - or: 可选的逻辑运算符标志
//   - true 或未提供: 使用 AND 逻辑连接（默认）
//   - false: 使用 OR 逻辑连接
//
// @returns:
//   - SelectionOption: 选择选项函数，用于配置 SelectionOptions
//
// @example:
//
//	// 匹配内网IP
//	opt1 := WithIPCidr("192.168.1.0/24")
//
//	// 匹配多个网段
//	opt2 := WithIPCidr("10.0.0.0/8")
//
//	// 使用 OR 逻辑
//	opt3 := WithIPCidr("172.16.0.0/12", false)
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

// WithGrains 创建一个基于Grains精确匹配的选择选项
//
// 此函数返回一个 SelectionOption，用于匹配具有指定Grains键值对的主机。
// Grains是主机的静态属性信息，如操作系统、架构等。
//
// @params:
//   - key: Grains的键名
//   - value: Grains的值（精确匹配）
//   - or: 可选的逻辑运算符标志
//   - true 或未提供: 使用 AND 逻辑连接（默认）
//   - false: 使用 OR 逻辑连接
//
// @returns:
//   - SelectionOption: 选择选项函数，用于配置 SelectionOptions
//
// @example:
//
//	// 匹配Linux系统
//	opt1 := WithGrains("os", "linux")
//
//	// 匹配x86_64架构
//	opt2 := WithGrains("arch", "x86_64")
//
//	// 使用 OR 逻辑
//	opt3 := WithGrains("environment", "production", false)
func WithGrains(key, value string, or ...bool) SelectionOption {
	s := &Selection{
		Grains: &SelectionKV{Key: key, Value: value},
	}
	f := true
	if len(or) > 0 && !or[0] {
		f = false
	}
	return func(o *SelectionOptions) { o.append(s, f) }
}

// WithGrainsRegex 创建一个基于Grains正则表达式匹配的选择选项
//
// 此函数返回一个 SelectionOption，用于匹配Grains值符合正则表达式的主机。
// 使用POSIX正则表达式语法进行模式匹配。
//
// @params:
//   - key: Grains的键名
//   - pattern: POSIX正则表达式模式
//   - or: 可选的逻辑运算符标志
//   - true 或未提供: 使用 AND 逻辑连接（默认）
//   - false: 使用 OR 逻辑连接
//
// @returns:
//   - SelectionOption: 选择选项函数，用于配置 SelectionOptions
//
// @example:
//
//	// 匹配Ubuntu系统（各版本）
//	opt1 := WithGrainsRegex("os", "ubuntu.*")
//
//	// 匹配内核版本
//	opt2 := WithGrainsRegex("kernel", "^5\\.[0-9]+")
//
//	// 使用 OR 逻辑
//	opt3 := WithGrainsRegex("hostname", "web[0-9]+", false)
func WithGrainsRegex(key, pattern string, or ...bool) SelectionOption {
	s := &Selection{
		GrainsPcre: &SelectionKV{Key: key, Value: pattern},
	}
	f := true
	if len(or) > 0 && !or[0] {
		f = false
	}
	return func(o *SelectionOptions) { o.append(s, f) }
}

// WithPillar 创建一个基于Pillar精确匹配的选择选项
//
// 此函数返回一个 SelectionOption，用于匹配具有指定Pillar键值对的主机。
// Pillar是主机的配置数据，通常包含敏感信息和配置参数。
//
// @params:
//   - key: Pillar的键名
//   - value: Pillar的值（精确匹配）
//   - lg: 可选的逻辑运算符标志
//   - true 或未提供: 使用 AND 逻辑连接（默认）
//   - false: 使用 OR 逻辑连接
//
// @returns:
//   - SelectionOption: 选择选项函数，用于配置 SelectionOptions
//
// @example:
//
//	// 匹配特定角色
//	opt1 := WithPillar("role", "webserver")
//
//	// 匹配环境
//	opt2 := WithPillar("environment", "production")
//
//	// 使用 OR 逻辑
//	opt3 := WithPillar("cluster", "east", false)
func WithPillar(key string, value string, lg ...bool) SelectionOption {
	s := &Selection{
		Pillar: &SelectionKV{Key: key, Value: value},
	}
	f := true
	if len(lg) > 0 && !lg[0] {
		f = false
	}
	return func(o *SelectionOptions) { o.append(s, f) }
}

// WithPillarRegex 创建一个基于Pillar正则表达式匹配的选择选项
//
// 此函数返回一个 SelectionOption，用于匹配Pillar值符合正则表达式的主机。
// 使用POSIX正则表达式语法进行模式匹配。
//
// @params:
//   - key: Pillar的键名
//   - pattern: POSIX正则表达式模式
//   - or: 可选的逻辑运算符标志
//   - true 或未提供: 使用 AND 逻辑连接（默认）
//   - false: 使用 OR 逻辑连接
//
// @returns:
//   - SelectionOption: 选择选项函数，用于配置 SelectionOptions
//
// @example:
//
//	// 匹配所有web相关角色
//	opt1 := WithPillarRegex("role", "web.*")
//
//	// 匹配版本号
//	opt2 := WithPillarRegex("version", "^1\\.[0-9]+")
//
//	// 使用 OR 逻辑
//	opt3 := WithPillarRegex("service", ".*-api$", false)
func WithPillarRegex(key, pattern string, or ...bool) SelectionOption {
	s := &Selection{
		PillarPcre: &SelectionKV{Key: key, Value: pattern},
	}
	f := true
	if len(or) > 0 && !or[0] {
		f = false
	}
	return func(o *SelectionOptions) { o.append(s, f) }
}

// and 添加一个AND逻辑选择条件
//
// 此方法是内部方法，用于向SelectionOptions中添加使用AND逻辑连接的选择条件。
// 会自动插入LogicAnd操作符，然后添加新的选择条件。
//
// @params:
//   - s: 要添加的Selection选择条件
//
// @note:
//   - 这是一个私有方法，仅供内部使用
//   - 会在现有选择条件后添加AND操作符和新条件
func (m *SelectionOptions) and(s *Selection) {
	m.Selections = append(m.Selections, &Selection{And: &LogicAnd{}}, s)
}

// or 添加一个OR逻辑选择条件
//
// 此方法是内部方法，用于向SelectionOptions中添加使用OR逻辑连接的选择条件。
// 会自动插入LogicOr操作符，然后添加新的选择条件。
//
// @params:
//   - s: 要添加的Selection选择条件
//
// @note:
//   - 这是一个私有方法，仅供内部使用
//   - 会在现有选择条件后添加OR操作符和新条件
func (m *SelectionOptions) or(s *Selection) {
	m.Selections = append(m.Selections, &Selection{Or: &LogicOr{}}, s)
}

// append 根据逻辑标志添加选择条件
//
// 此方法是内部方法，用于向SelectionOptions中添加选择条件。
// 如果是第一个条件则直接添加，否则根据逻辑标志选择AND或OR连接。
//
// @params:
//   - s: 要添加的Selection选择条件
//   - and: 逻辑连接标志
//   - true: 使用AND逻辑连接
//   - false: 使用OR逻辑连接
//
// @note:
//   - 这是一个私有方法，仅供内部使用
//   - 第一个条件直接添加，后续条件根据逻辑标志连接
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

// Validate 验证选择选项的有效性
//
// 此方法检查SelectionOptions中的所有选择条件是否有效和合理。
// 验证包括：选择条件非空、逻辑连接符位置正确、正则表达式语法正确等。
//
// @returns:
//   - error: 验证错误信息，如果验证通过则返回nil
//
// @note:
//   - 验证第一个条件不能是逻辑连接符
//   - 验证不能有连续的逻辑连接符
//   - 验证正则表达式语法正确性
//   - 验证IP CIDR表示法正确性
//   - 至少必须有一个有效的选择条件
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
		if pattern != "" {
			_, err := regexp.CompilePOSIX(pattern)
			if err != nil {
				return fmt.Errorf("invalid regexp '%s' at selection[%d]", pattern, i)
			}
		}
		if len(s.IpCidr) != 0 {
			_, err := iprange.ParseRanges(s.IpCidr)
			if err != nil {
				return fmt.Errorf("invalid ip range at selection[%d]", i)
			}
		}

		lastIsLogic = s.isLogic()
	}

	if !hasSelection {
		return fmt.Errorf("no selection options found")
	}

	return nil
}

// MatchId 返回匹配 id 的结果
//
// @params:
//   - id: minion id
//
// @returns:
//   - matched: 匹配结果
//   - hit: id 匹配是否命中
func (m *Selection) MatchId(id string) (bool, bool) {
	hit := false
	if len(m.Hosts) != 0 {
		hit = true
		if m.Hosts[0] == "*" {
			return true, hit
		}
		for _, value := range m.Hosts {
			if value == "*" || value == id {
				return true, hit
			}
		}
		return false, hit
	}
	if len(m.HostPcre) != 0 {
		hit = true
		re, err := regexp.CompilePOSIX(m.HostPcre)
		if err != nil {
			return false, hit
		}
		return re.MatchString(id), hit
	}
	if len(m.IdRange) != 0 {
		ok := true
		hit = true
		if m.IdRange[0] == '%' {
			ok = strings.HasSuffix(id, m.IdRange[1:])
		}
		if m.IdRange[len(m.IdRange)-1] == '%' {
			ok = ok && strings.HasPrefix(id, m.IdRange[:len(m.IdRange)-1])
		}
		return ok, hit
	}
	return false, hit
}

// MatchIP 返回匹配 ip 的结果
// @params:
//   - ip: minion ip
//
// @returns:
//   - matched: 匹配结果
//   - hit: ip 匹配是否命中
func (m *Selection) MatchIP(ip string) (bool, bool) {
	hit := false
	if len(m.IpCidr) != 0 {
		hit = true
		ranges, err := iprange.ParseRanges(m.IpCidr)
		if err != nil {
			return false, hit
		}
		for _, rng := range ranges {
			matched := rng.Contains(net.ParseIP(ip))
			if matched {
				return true, matched
			}
		}
		return false, hit
	}
	return false, hit
}

// ToText 将Selection转换为文本表示
//
// 此方法将Selection的内容转换为可读的文本格式。
// 使用特定的标记符号来表示不同类型的选择条件。
//
// @returns:
//   - string: 文本表示的选择条件
//
// @note:
//
//	文本格式规则：
//	- Host: 直接显示主机名列表（逗号分隔）
//	- HostPcre: "E@pattern" 格式
//	- IdRange: "R@range" 格式
//	- HostGroups: "N@groups" 格式
//	- IpCidr: "S@cidr" 格式
//	- Grains: "G@key:value" 格式
//	- GrainsPcre: "P@key:pattern" 格式
//	- Pillar: "I@key:value" 格式
//	- PillarPcre: "J@key:pattern" 格式
//	- And: "and"
//	- Or: "or"
func (m *Selection) ToText() string {
	if len(m.Hosts) != 0 {
		return strings.Join(m.Hosts, ",")
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

	if kv := m.Grains; kv != nil {
		return fmt.Sprintf("G@%s:%s", kv.Key, kv.Value)
	}
	if kv := m.GrainsPcre; kv != nil {
		return fmt.Sprintf("P@%s:%s", kv.Key, kv.Value)
	}
	if kv := m.Pillar; kv != nil {
		return fmt.Sprintf("I@%s:%s", kv.Key, kv.Value)
	}
	if kv := m.PillarPcre; kv != nil {
		return fmt.Sprintf("J@%s:%s", kv.Key, kv.Value)
	}
	if m.And != nil {
		return "and"
	}
	if m.Or != nil {
		return "or"
	}
	return ""
}

// isLogic 检查Selection是否为逻辑连接符
//
// 此方法检查当前Selection是否为逻辑连接符（AND或OR）。
//
// @returns:
//   - bool: 如果是逻辑连接符则返回true，否则返回false
//
// @note:
//   - 用于在验证和处理过程中判断选择条件类型
//   - 逻辑连接符不能出现在首位或连续出现
func (m *Selection) isLogic() bool {
	if m.And != nil {
		return true
	}
	if m.Or != nil {
		return true
	}
	return false
}

// NewSelectionOptions 创建一个新的SelectionOptions实例
//
// 此函数使用提供的选项函数创建一个新的SelectionOptions实例。
// 所有选项都会被应用，然后执行验证。
//
// @params:
//   - opts: 可变参数的SelectionOption函数列表
//
// @returns:
//   - *SelectionOptions: 创建的SelectionOptions实例
//   - error: 如果验证失败则返回错误信息
//
// @example:
//
//	// 创建简单的选择选项
//	options, err := NewSelectionOptions(
//	    WithHosts("web01"),
//	    WithHostGroup([]string{"web"}, false),
//	    WithIPCidr("192.168.1.0/24"),
//	)
//	if err != nil {
//	    // 处理错误
//	}
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

// ParseSelection 解析文本格式的选择条件并创建SelectionOptions实例
//
// 此函数将文本格式的选择条件解析为SelectionOptions结构。
// 支持多种选择条件类型的文本表示和逻辑运算符的组合。
//
// @params:
//   - text: 文本格式的选择条件字符串
//
// @returns:
//   - *SelectionOptions: 解析后的SelectionOptions实例
//   - error: 解析过程中的错误信息
//
// @note:
//
//	支持的文本格式：
//	- Host: 直接主机名或主机名列表（逗号分隔）
//	- HostPcre: "E@pattern" 格式的正则表达式
//	- IdRange: "R@range" 格式的ID范围匹配
//	- HostGroups: "N@groups" 格式的主机组
//	- IpCidr: "S@cidr" 格式的IP CIDR
//	- Grains: "G@key:value" 格式的精确匹配
//	- GrainsPcre: "P@key:pattern" 格式的正则匹配
//	- Pillar: "I@key:value" 格式的精确匹配
//	- PillarPcre: "J@key:pattern" 格式的正则匹配
//	- 逻辑运算符: "and", "or"
//
// @example:
//
//	// 解析简单主机选择
//	options, err := ParseSelection("node1,node2")
//
//	// 解析复杂条件组合
//	options, err := ParseSelection("E@web[0-9]+ and G@os:linux or N@database")
//
//	// 解析所有主机
//	options, err := ParseSelection("*")
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
				if _, err := iprange.ParseRanges(value); err != nil {
					return nil, fmt.Errorf("invalid ip range regexp 'S@%s'", value)
				}
				selection = &Selection{IpCidr: value}
			case "G":
				selection = &Selection{Grains: &SelectionKV{Key: key, Value: value}}
			case "P":
				_, err := regexp.CompilePOSIX(value)
				if err != nil {
					return nil, fmt.Errorf("invalid grains regexp 'P@%s:%s'", key, value)
				}

				selection = &Selection{GrainsPcre: &SelectionKV{Key: key, Value: value}}
			case "I":
				selection = &Selection{Pillar: &SelectionKV{Key: key, Value: value}}
			case "J":
				_, err := regexp.CompilePOSIX(value)
				if err != nil {
					return nil, fmt.Errorf("invalid pillar regexp 'J@%s:%s'", key, value)
				}

				selection = &Selection{PillarPcre: &SelectionKV{Key: key, Value: value}}
			case "and":
				selection = &Selection{And: &LogicAnd{}}
			case "or":
				selection = &Selection{Or: &LogicOr{}}
			case "":
				if value == "*" {
					selection = &Selection{Hosts: []string{"*"}}
				} else {
					selection = &Selection{Hosts: strings.Split(value, ",")}
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

// MatchTarget 检查目标是否匹配选择条件
//
// 此方法遍历所有选择条件，并使用逻辑运算符（AND/OR）组合匹配结果。
// 支持多种匹配类型：主机ID、IP地址、主机组、Grains、Pillar等。
//
// @params:
//   - target: 要匹配的目标对象，必须实现 SelectionTarget 接口
//   - simple: 简化模式标志
//     true: 仅匹配基本条件（ID、IP、主机组），跳过 Grains 和 Pillar 匹配
//     false: 完整匹配所有条件类型
//
// @returns:
//   - match: 匹配结果
//     true: 目标匹配选择条件
//     false: 目标不匹配选择条件
//   - hit: 匹配是否命中
//
// @note:
//   - 逻辑运算符按顺序处理：先处理的条件结果会影响后续的逻辑运算
//   - 每个选择条件会尝试匹配目标的不同属性，命中任一属性即视为该条件匹配
//   - 在简化模式下，Grains 和 Pillar 匹配会被跳过，提高性能
//
// @example:
//
//	options := &SelectionOptions{...}
//	target := &myTarget{id: "web01", ip: "192.168.1.10", groups: []string{"web"}}
//
//	// 完整匹配
//	matched := options.MatchTarget(target, false)
//
//	// 简化匹配（仅基本条件）
//	matched := options.MatchTarget(target, true)
func (m *SelectionOptions) MatchTarget(target SelectionTarget, simple bool) (bool, bool) {
	result := true
	resultHit := false
	var lastMatch bool
	for _, s := range m.Selections {
		if s.And != nil {
			result = result && lastMatch
			continue

		}
		if s.Or != nil {
			result = result || lastMatch
			continue
		}

		id := target.Id()
		if len(id) != 0 {
			matched, hit := s.MatchId(id)
			if hit {
				lastMatch = matched
				resultHit = true
				continue
			}
		}

		ip := target.IP()
		if len(ip) != 0 {
			matched, hit := s.MatchIP(ip)
			if hit {
				lastMatch = matched
				resultHit = true
				continue
			}
		}

		groups := target.Groups()
		if len(groups) != 0 {
			for _, g := range groups {
				for _, h := range s.HostGroups {
					resultHit = true
					if g == h {
						lastMatch = true
						goto GROUPEXIT
					}
				}
			}
		GROUPEXIT:
		}

		if simple {
			continue
		}

		grains := target.Grains()
		if len(grains) != 0 {
			if kv := s.Grains; kv != nil {
				resultHit = true
				value, ok := grains[kv.Key]
				if ok {
					resultHit = true
					lastMatch = value == kv.Value
				}
				continue
			}
			if kv := s.GrainsPcre; kv != nil {
				resultHit = true
				value, ok := grains[kv.Key]
				if ok {
					re, err := regexp.CompilePOSIX(kv.Value)
					if err == nil {
						lastMatch = re.MatchString(value)
					}
				}
				continue
			}
		}

		pillars := target.Pillars()
		if len(pillars) != 0 {
			if kv := s.Pillar; kv != nil {
				value, ok := pillars[kv.Key]
				if ok {
					resultHit = true
					lastMatch = value == kv.Value
				}
				continue
			}
			if kv := s.PillarPcre; kv != nil {
				value, ok := pillars[kv.Key]
				if ok {
					resultHit = true
					re, err := regexp.CompilePOSIX(kv.Value)
					if err == nil {
						lastMatch = re.MatchString(value)
					}
				}
				continue
			}
		}
	}

	return result && lastMatch, resultHit
}

// ToText 将SelectionOptions转换为文本表示
//
// 此方法将SelectionOptions中的所有选择条件转换为可读的文本格式。
// 各个选择条件之间用空格分隔，保持原有的逻辑运算符顺序。
//
// @returns:
//   - string: 文本表示的选择条件
//
// @note:
//
//	输出格式与ParseSelection接受的格式兼容，可以进行双向转换：
//	- Host: 主机名列表（逗号分隔）
//	- HostPcre: "E@pattern" 格式
//	- IdRange: "R@range" 格式
//	- HostGroups: "N@groups" 格式
//	- IpCidr: "S@cidr" 格式
//	- Grains: "G@key:value" 格式
//	- GrainsPcre: "P@key:pattern" 格式
//	- Pillar: "I@key:value" 格式
//	- PillarPcre: "J@key:pattern" 格式
//	- 逻辑运算符: "and", "or"
//
// @example:
//
//	options := &SelectionOptions{...}
//	text := options.ToText()
//	// 可能输出: "node1,node2 and E@web[0-9]+ or G@os:linux"
//
//	// 可以与ParseSelection双向转换
//	parsed, _ := ParseSelection(text)
//	assert.Equal(t, text, parsed.ToText())
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
