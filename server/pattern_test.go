package server

import (
	"fmt"
	"math/rand"
	"testing"
)

const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func TestPatternMatch(t *testing.T) {
	p := func(m ...matcher) Pattern {
		return Pattern{matchers: m}
	}
	c := func(b byte) charMatcher {
		return charMatcher{b}
	}
	star := wildcard{false}
	ques := wildcard{true}
	cc := func(s string) charClass {
		c := charClass{}
		for i := range s {
			c.chars[s[i]] = true
		}
		return c
	}
	rands := func() string {
		b := make([]byte, rand.Intn(10)+1)
		for i := range b {
			b[i] = chars[rand.Intn(len(chars))]
		}
		return string(b)
	}
	randos := func(f func(string) string) (s []string) {
		for i := 0; i < 10; i++ {
			s = append(s, f(rands()))
		}
		return s
	}
	allChars := func(f func(rune) string) (s []string) {
		for _, r := range chars {
			s = append(s, f(r))
		}
		return s
	}
	allSingles := allChars(func(r rune) string { return string(r) })
	slice := func(ss ...string) []string { return ss }
	for _, c := range []struct {
		pattern Pattern
		in      []string
		want    bool
	}{{
		pattern: p(),
		in:      slice(""),
		want:    true,
	}, {
		pattern: p(),
		in:      randos(func(s string) string { return s }),
		want:    false,
	}, {
		pattern: p(c('a')),
		in:      slice("a"),
		want:    true,
	}, {
		pattern: p(c('a')),
		in:      slice("aa", "ab", "b"),
		want:    false,
	}, {
		pattern: p(c('a'), c('b')),
		in:      slice("ab"),
		want:    true,
	}, {
		pattern: p(c('a'), c('b')),
		in:      slice("aa", "abc"),
		want:    false,
	}, {
		pattern: p(ques),
		in:      allSingles,
		want:    true,
	}, {
		pattern: p(ques),
		in:      append(randos(func(s string) string { return s + "end" }), ""),
		want:    false,
	}, {
		pattern: p(ques, c('a')),
		in:      allChars(func(r rune) string { return fmt.Sprintf("%ca", r) }),
		want:    true,
	}, {
		pattern: p(ques, c('a')),
		in: randos(func(s string) string {
			if len(s) == 1 {
				// definitely shouldn't match
				return s
			}
			s = s[:1] + "b" + s[1:] // second character not 'a'
			return s
		}),
		want: false,
	}, {
		pattern: p(c('a'), ques),
		in:      allChars(func(r rune) string { return fmt.Sprintf("a%c", r) }),
		want:    true,
	}, {
		pattern: p(c('a'), ques),
		in: append(allChars(func(r rune) string { return fmt.Sprintf("b%c", r) }),
			"a", ""),
		want: false,
	}, {
		pattern: p(star),
		in:      append(randos(func(s string) string { return s }), ""),
		want:    true,
	}, {
		pattern: p(c('a'), star),
		in:      append(randos(func(s string) string { return "a" + s }), "a"),
		want:    true,
	}, {
		pattern: p(c('a'), star),
		in:      randos(func(s string) string { return "b" + s }),
		want:    false,
	}, {
		pattern: p(star, c('a')),
		in:      append(randos(func(s string) string { return s + "a" }), "a"),
		want:    true,
	}, {
		pattern: p(star, c('a')),
		in:      randos(func(s string) string { return s + "b" }),
		want:    false,
	}, {
		pattern: p(cc("a")),
		in:      slice("a"),
		want:    true,
	}, {
		pattern: p(cc("a")),
		in: func() (s []string) {
			for _, c := range chars {
				if c == 'a' {
					continue
				}
				s = append(s, string(c))
			}
			return s
		}(),
		want: false,
	}, {
		pattern: p(cc("abc")),
		in:      slice("a", "b", "c"),
		want:    true,
	}, {
		pattern: p(cc("abc")),
		in: append(func() (s []string) {
			for _, c := range chars {
				if c == 'a' || c == 'b' || c == 'c' {
					continue
				}
				s = append(s, string(c))
			}
			return s
		}(), randos(func(s string) string {
			switch s[0] {
			case 'a', 'b', 'c':
				s = "d" + s
			}
			return s
		})...),
		want: false,
	}, {
		pattern: p(func(cc charClass) charClass {
			cc.invert = true
			return cc
		}(cc("abc"))),
		in: func() (s []string) {
			for _, c := range chars {
				if c == 'a' || c == 'b' || c == 'c' {
					continue
				}
				s = append(s, string(c))
			}
			return s
		}(),
		want: true,
	}, {
		pattern: p(func(cc charClass) charClass {
			cc.invert = true
			return cc
		}(cc("abc"))),
		in: append(randos(func(s string) string {
			return "a" + s
		}), "", "a", "b", "c"),
		want: false,
	}} {
		t.Run(fmt.Sprintf("%s/%v", c.pattern, c.want), func(t *testing.T) {
			for _, in := range c.in {
				t.Run(fmt.Sprintf("%q", in), func(t *testing.T) {
					got := c.pattern.Match(in)
					if got != c.want {
						t.Errorf("Mismatch:\nPattern: %v\ninput: %q\nwant: %v, got: %v",
							c.pattern, in, c.want, got)
					}
				})
			}
		})
	}
}

func TestParseCharClass(t *testing.T) {
	cc := func(s string) (cc charClass) {
		for i := range s {
			cc.chars[s[i]] = true
		}
		return cc
	}
	not := func(cc charClass) charClass {
		cc.invert = !cc.invert
		return cc
	}
	for _, c := range []struct {
		in   string
		want charClass
		rem  string
		err  bool
	}{{
		in:   "[a]",
		want: cc("a"),
	}, {
		in:   "[!a]",
		want: not(cc("a")),
	}, {
		in:   "[abc]",
		want: cc("abc"),
	}, {
		in:   "[!abc]",
		want: not(cc("abc")),
	}, {
		in:   "[a-e]",
		want: cc("abcde"),
	}, {
		in:   "[!a-e]",
		want: not(cc("abcde")),
	}, {
		in:   "[a!]",
		want: cc("a!"),
	}, {
		in:   "[-a]",
		want: cc("-a"),
	}, {
		in:   "[a-]",
		want: cc("-a"),
	}, {
		in:   "[!-]",
		want: not(cc("-")),
	}, {
		in:   "[!!--]",
		want: not(cc("!\"#$%&'()*+,-")),
	}, {
		in:  "[b-a]",
		err: true,
	}, {
		in:  "abc",
		err: true,
	}, {
		in:  "[abc",
		err: true,
	}, {
		in:  "abc]",
		err: true,
	}, {
		in:   "[a]bc",
		want: cc("a"),
		rem:  "bc",
	}, {
		in:   "[!a][b]",
		want: not(cc("a")),
		rem:  "[b]",
	}} {
		t.Run(c.in, func(t *testing.T) {
			got, rem, err := parseCharClass(c.in)
			if err != nil {
				if !c.err {
					t.Fatalf("parseCharClass(%q): %v", c.in, err)
				}
				return
			}
			if c.err {
				t.Fatalf("parseCharClass(%q): want err, got: %v", c.in, got)
			}
			if got != c.want {
				t.Errorf("parseCharClass(%q) = %v, want: %v", c.in, got, c.want)
			}
			if rem != c.rem {
				t.Errorf("parseCharClass(%q): remainder %q, want %q", c.in, rem, c.rem)
			}
		})
	}
}
