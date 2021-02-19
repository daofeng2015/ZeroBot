package zero

import (
	"runtime"
	"sort"
	"sync"
)

type (
	Response uint8
	// Rule filter the event
	Rule    func(event *Event, state State) bool
	Handler func(matcher *Matcher, event Event, state State) Response
)

const (
	SuccessResponse Response = iota
	RejectResponse
	FinishResponse
)

// Matcher 是 ZeroBot 匹配和处理事件的最小单元
type Matcher struct {
	// Temp 是否为临时Matcher，临时 Matcher 匹配一次后就会删除当前 Matcher
	Temp bool
	// Block 是否阻断后续 Matcher，为 true 时当前Matcher匹配成功后，后续Matcher不参与匹配
	Block bool
	// Priority 优先级，越小优先级越高
	Priority int
	// State 上下文
	State State
	// Event 当前匹配到的事件
	Event *Event
	// Type 匹配的事件类型
	Type Rule
	// Rules 匹配规则
	Rules []Rule
	// Handler 处理事件的函数
	Handler Handler
}

var (
	// 所有主匹配器列表
	matcherList = make([]*Matcher, 0)
	// Matcher 修改读写锁
	matcherLock = sync.RWMutex{}
)

// State store the context of a matcher.
type State map[string]interface{}

func sortMatcher() {
	sort.Slice(matcherList, func(i, j int) bool { // 按优先级排序
		return matcherList[i].Priority < matcherList[j].Priority
	})
}

// SetBlock 设置是否阻断后面的 Matcher 触发
func (m *Matcher) SetBlock(block bool) *Matcher {
	m.Block = block
	return m
}

// SetPriority 设置当前 Matcher 优先级
func (m *Matcher) SetPriority(priority int) *Matcher {
	matcherLock.Lock()
	defer matcherLock.Unlock()
	m.Priority = priority
	sortMatcher()
	return m
}

// On 添加新的主匹配器
func On(type_ string, rules ...Rule) *Matcher {
	var i = 1
	var hooked Rule
	for {
		_, file, _, ok := runtime.Caller(i)
		if !ok {
			break
		}
		if hooker, ok := hooks[file]; ok { // find hook -> add hook
			hooked = hooker.Hook()
			break
		}
		i++
	}
	var matcher = &Matcher{
		State: map[string]interface{}{},
		Type:  Type(type_),
		Rules: func() []Rule {
			if hooked != nil {
				return append(rules, hooked)
			}
			return rules
		}(),
	}
	return StoreMatcher(matcher)
}

// StoreMatcher store a matcher to matcher list.
func StoreMatcher(m *Matcher) *Matcher {
	matcherLock.Lock()
	defer matcherLock.Unlock()
	matcherList = append(matcherList, m)
	sortMatcher()
	return m
}

// StoreTempMatcher store a matcher only triggered once.
func StoreTempMatcher(m *Matcher) *Matcher {
	m.Temp = true
	StoreMatcher(m)
	return m
}

// Delete remove the matcher from list
func (m *Matcher) Delete() {
	matcherLock.Lock()
	defer matcherLock.Unlock()
	for i, matcher := range matcherList {
		if m == matcher {
			matcherList = append(matcherList[:i], matcherList[i+1:]...)
		}
	}
}

func (m *Matcher) run(event Event) {
	m.Event = &event
	if m.Handler == nil {
		return
	}
	switch m.Handler(m, event, m.State) {
	case RejectResponse:
		StoreTempMatcher(&Matcher{
			Type:     Type("message"),
			Block:    m.Block,
			Priority: m.Priority,
			State:    m.State,
			Rules: []Rule{
				CheckUser(event.UserID),
			},
			Handler: m.Handler,
		})
		return
	}
}

// Get ..
func (m *Matcher) Get(prompt string) string {
	ch := make(chan string)
	event := m.Event
	Send(*event, prompt)
	StoreTempMatcher(&Matcher{
		Priority: m.Priority,
		Block:    m.Block,
		Type:     Type("message"),
		State:    map[string]interface{}{},
		Rules: []Rule{
			CheckUser(event.UserID),
		},
		Handler: func(_ *Matcher, ev Event, _ State) Response {
			ch <- ev.RawMessage
			return SuccessResponse
		},
	})
	return <-ch
}

func (m *Matcher) copy() *Matcher {
	return &Matcher{
		State:    copyState(m.State),
		Type:     m.Type,
		Rules:    m.Rules,
		Block:    m.Block,
		Priority: m.Priority,
		Handler:  m.Handler,
		Temp:     m.Temp,
	}
}

// 拷贝字典
func copyState(src State) State {
	dst := make(State)
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// Handle 直接处理事件
func (m *Matcher) Handle(handler Handler) *Matcher {
	m.Handler = handler
	return m
}

// OnMessage 消息触发器
func OnMessage(rules ...Rule) *Matcher {
	return On("message", rules...)
}

// OnNotice 系统提示触发器
func OnNotice(rules ...Rule) *Matcher {
	return On("notice", rules...)
}

// OnRequest 请求消息触发器
func OnRequest(rules ...Rule) *Matcher {
	return On("request", rules...)
}

// OnMetaEvent 元事件触发器
func OnMetaEvent(rules ...Rule) *Matcher {
	return On("meta_event", rules...)
}

// OnPrefix 前缀触发器
func OnPrefix(prefix string, rules ...Rule) *Matcher {
	return OnMessage(append([]Rule{PrefixRule(prefix)}, rules...)...)
}

// OnSuffix 后缀触发器
func OnSuffix(suffix string, rules ...Rule) *Matcher {
	return OnMessage(append([]Rule{SuffixRule(suffix)}, rules...)...)
}

// OnCommand 命令触发器
func OnCommand(commands string, rules ...Rule) *Matcher {
	return OnMessage(append([]Rule{CommandRule(commands)}, rules...)...)
}

// OnRegex 正则触发器
func OnRegex(regexPattern string, rules ...Rule) *Matcher {
	return OnMessage(append([]Rule{RegexRule(regexPattern)}, rules...)...)
}

// OnKeyword 关键词触发器
func OnKeyword(keyword string, rules ...Rule) *Matcher {
	return OnMessage(append([]Rule{KeywordRule(keyword)}, rules...)...)
}

// OnFullMatch 完全匹配触发器
func OnFullMatch(src string, rules ...Rule) *Matcher {
	return OnMessage(append([]Rule{FullMatchRule(src)}, rules...)...)
}

// OnFullMatchGroup 完全匹配触发器组
func OnFullMatchGroup(src []string, rules ...Rule) *Matcher {
	return OnMessage(append([]Rule{FullMatchRule(src...)}, rules...)...)
}

// OnKeywordGroup 关键词触发器组
func OnKeywordGroup(keywords []string, rules ...Rule) *Matcher {
	return OnMessage(append([]Rule{KeywordRule(keywords...)}, rules...)...)
}

// OnCommandGroup 命令触发器组
func OnCommandGroup(commands []string, rules ...Rule) *Matcher {
	return OnMessage(append([]Rule{CommandRule(commands...)}, rules...)...)
}

// OnPrefixGroup 前缀触发器组
func OnPrefixGroup(prefix []string, rules ...Rule) *Matcher {
	return OnMessage(append([]Rule{PrefixRule(prefix...)}, rules...)...)
}

// OnSuffixGroup 后缀触发器组
func OnSuffixGroup(suffix []string, rules ...Rule) *Matcher {
	return OnMessage(append([]Rule{SuffixRule(suffix...)}, rules...)...)
}
