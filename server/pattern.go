package server

import (
	"errors"
	"fmt"
	"strings"
)

// Pattern represents a parsed OSC Address pattern, usually received
// with an OSC message.
type Pattern struct {
	matchers []matcher
}

func (p Pattern) Match(s string) bool {
	states := []*matchState{{p.matchers, s}}
	for len(states) > 0 {
		var s *matchState
		l := len(states) - 1
		s, states = states[l], states[:l]
		next, accept := s.match()
		if accept {
			return true
		}
		states = append(states, next...)
	}
	return false
}

func (p Pattern) String() string {
	var sb strings.Builder
	for _, m := range p.matchers {
		sb.WriteString(m.String())
	}
	return sb.String()
}

type matchState struct {
	matchers []matcher
	s        string
}

func (m *matchState) match() (next []*matchState, accept bool) {
	if len(m.s) == 0 {
		// We're done, success if all the remaining matchers
		// could match nothing.
		// TODO: having to special case this is definitely weird
		for _, m := range m.matchers {
			w, ok := m.(wildcard)
			if !ok {
				return nil, false
			}
			if w.single {
				return nil, false
			}
		}
		return nil, true
	}
	if len(m.matchers) == 0 {
		// no matchers, but there must be some input.
		return nil, false
	}
	// Still matchers, still input.
	results := m.matchers[0].match(m.s[0])
	if results == noMatch {
		return nil, false
	}
	if (results & matchAdvanceBoth) != 0 {
		next = append(next, &matchState{
			matchers: m.matchers[1:],
			s:        m.s[1:],
		})
	}
	if (results & matchAdvanceMatcher) != 0 {
		next = append(next, &matchState{
			matchers: m.matchers[1:],
			s:        m.s,
		})
	}
	if (results & matchAdvanceInput) != 0 {
		next = append(next, &matchState{
			matchers: m.matchers,
			s:        m.s[1:],
		})
	}
	return next, false
}

type matcher interface {
	match(byte) matchResult
	String() string
}

type matchResult byte

const (
	noMatch                         = 0
	matchAdvanceBoth    matchResult = 1 << iota // try the next matcher with the next character
	matchAdvanceMatcher                         // success, but don't move the input
	matchAdvanceInput                           // success, and current matcher could match more
)

// charMatcher is a matcher that matches an exact byte.
type charMatcher struct {
	c byte
}

func (c charMatcher) String() string {
	return fmt.Sprintf("%c", c.c)
}

func (c charMatcher) match(b byte) matchResult {
	if c.c == b {
		return matchAdvanceBoth
	}
	return noMatch
}

type wildcard struct {
	single bool // true if ?, false if *
}

func (w wildcard) match(byte) matchResult {
	if w.single {
		return matchAdvanceBoth
	}
	return matchAdvanceBoth | matchAdvanceMatcher | matchAdvanceInput
}

func (w wildcard) String() string {
	if w.single {
		return "?"
	}
	return "*"
}

// TODO: range helpers
type charClass struct {
	chars  [256]bool
	invert bool
}

func (cc charClass) match(b byte) matchResult {
	if cc.chars[b] != cc.invert {
		return matchAdvanceBoth
	}
	return noMatch
}

func (cc charClass) String() string {
	var sb strings.Builder
	sb.WriteString("[")
	if cc.invert {
		sb.WriteString("!")
	}
	for i, ok := range cc.chars {
		if ok {
			fmt.Fprintf(&sb, "%c", i)
		}
	}
	sb.WriteString("]")
	return sb.String()
}

func parseMatcher(s string) (matcher, string, error) {
	if len(s) != 0 {
		return nil, "", errors.New("unexpected end of input")
	}
	switch s[0] {
	case '[':
		return parseCharClass(s)
	case '*':
		return wildcard{}, s[1:], nil
	case '?':
		return wildcard{single: true}, s[1:], nil
	}
	return charMatcher{s[0]}, s[1:], nil
}

func parseCharClass(s string) (charClass, string, error) {
	var cc charClass
	s, ok := strings.CutPrefix(s, "[")
	if !ok {
		return cc, "", fmt.Errorf("expect %q, got: %q", "[", s)
	}
	if len(s) == 0 {
		return cc, "", fmt.Errorf("expect character class, got EOF")
	}
	if s[0] == '!' {
		s = s[1:]
		cc.invert = true
	}
	end := strings.IndexByte(s, ']')
	if end < 0 {
		return cc, "", fmt.Errorf("expect %q somewhere, got: %q", "]", s)
	}
	for i := 0; i < end; i++ {
		c := s[i]
		if c == '-' {
			if i > 0 && (i+1) < end {
				next := s[i+1]
				if next < s[i-1] {
					return cc, "", fmt.Errorf("invalid range %c-%c, %c<%c",
						s[i-1], next, next, s[i-1])
				}
				for d := s[i-1]; d < next; d++ {
					cc.chars[d] = true
				}
				continue
			}
		}
		cc.chars[c] = true
	}
	return cc, s[end+1:], nil
}
