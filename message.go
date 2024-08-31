package osc

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"time"
)

// Message represents an OSC message.
type Message struct {
	// Pattern is the address pattern, a string beginning with a "/".
	Pattern string
	// Arguments is the values.
	Arguments []Argument
}

// ParseMessage parses a message.
func ParseMessage(buf []byte) (*Message, error) {
	// A message begins with the address, which is a string.
	var addr String
	buf, err := addr.Consume(buf)
	if err != nil {
		return nil, fmt.Errorf("reading address pattern: %w", err)
	}
	// Next is the type tag string.
	var tt String
	buf, err = tt.Consume(buf)
	if err != nil {
		return nil, fmt.Errorf("reading type tag: %w", err)
	}
	if len(tt) == 0 || tt[0] != ',' {
		// TODO: the spec talks about handling this case, but it is
		// unclear how.
		return nil, fmt.Errorf("invalid type tag string: %q", tt)
	}
	args := make([]Argument, len(tt)-1)
	for i, t := range tt[1:] {
		c, ok := newByTypeTag[t]
		if !ok {
			return nil, fmt.Errorf("unknown type tag %c", t)
		}
		a := c()
		buf, err = a.Consume(buf)
		if err != nil {
			return nil, fmt.Errorf("reading argument %d (%c): %w", i, t, err)
		}
		args[i] = a
	}

	return &Message{
		Pattern:   string(addr),
		Arguments: args,
	}, nil
}

// Append encodes the message and appends it to the provided slice.
func (m Message) Append(b []byte) []byte {
	addr := String(m.Pattern)
	b = addr.Append(b)

	typeTag := make([]rune, 0, len(m.Arguments)+1)
	typeTag = append(typeTag, ',')
	for _, a := range m.Arguments {
		typeTag = append(typeTag, a.TypeTag())
	}
	tt := String(typeTag)
	b = tt.Append(b)

	for _, a := range m.Arguments {
		b = a.Append(b)
	}
	return b
}

// newByTypeTag holds functions to construct new functions fomr a given typetag.
// TODO: there's probably others.
var newByTypeTag = map[rune]func() Argument{
	Int32(0).TypeTag():   func() Argument { return new(Int32) },
	Float32(0).TypeTag(): func() Argument { return new(Float32) },
	String("").TypeTag(): func() Argument { return new(String) },
	TimeTag{}.TypeTag():  func() Argument { return new(TimeTag) },
	True{}.TypeTag():     func() Argument { return True{} },
	False{}.TypeTag():    func() Argument { return False{} },
	Null{}.TypeTag():     func() Argument { return Null{} },
	Impulse{}.TypeTag():  func() Argument { return Impulse{} },
}

// Argument represents an OSC value.
type Argument interface {
	// TypeTag must return the type tag of the argument, a single character.
	TypeTag() rune
	// Append appends the binary representation of the argument to the
	// provided byte slice.
	Append([]byte) []byte
	// Consume fills in the argument from the provided bytes, returning any
	// remainder.
	Consume([]byte) ([]byte, error)
}

// Int32 is the OSC int32: a "32-bit big-endian two’s complement integer"
type Int32 int32

func (Int32) TypeTag() rune { return 'i' }

func (i Int32) Append(b []byte) []byte {
	return binary.BigEndian.AppendUint32(b, uint32(i))
}

func (i *Int32) Consume(b []byte) ([]byte, error) {
	if l := len(b); l < 4 {
		return nil, fmt.Errorf("expect int32, only %d bytes", l)
	}
	u := binary.BigEndian.Uint32(b)
	*i = Int32(u)
	return b[4:], nil
}

func (i Int32) String() string {
	return fmt.Sprintf("Int32(%d)", i)
}

// Float32 is a normal float32: "32-bit big-endian IEEE 754 floating point
// number"
type Float32 float32

func (Float32) TypeTag() rune { return 'f' }

func (f Float32) Append(b []byte) []byte {
	return binary.BigEndian.AppendUint32(b, math.Float32bits(float32(f)))
}

func (f *Float32) Consume(b []byte) ([]byte, error) {
	if l := len(b); l < 4 {
		return nil, fmt.Errorf("expect float32, only %d bytes", l)
	}
	u := binary.BigEndian.Uint32(b)
	*f = Float32(math.Float32frombits(u))
	return b[4:], nil
}

func (f Float32) String() string {
	return fmt.Sprintf("Float32(%f)", f)
}

// String is an ASCII string, on the wire it's null-terminated and padded for
// alignment.
type String string

func (String) TypeTag() rune { return 's' }

func (s String) Append(b []byte) []byte {
	// Avoid a temporary conversion.
	for i := range s {
		b = append(b, s[i])
	}
	// 0 pad at least once, at most 3 times until the total length is a
	// multiple of 4 bytes.
	b = append(b, 0)
	for len(b)%4 > 0 {
		b = append(b, 0)
	}
	return b
}

func (s *String) Consume(b []byte) ([]byte, error) {
	end := bytes.IndexByte(b, 0)
	if end < 0 {
		return nil, fmt.Errorf("no termination in string %q", b)
	}
	*s = String(b[:end])
	// Total number of bytes must be a multiple of 4, so we can just
	// figure out how much padding there is from the length. Because
	// the spec requires the padding, don't worry about whether the bytes
	// are actually 0 or not.
	// TODO: is this an ok assumption?
	// TODO: maybe we should actually check the padding is correct?
	end = min(end+4-end%4, len(b))
	return b[end:], nil
}

func (s String) String() string {
	return fmt.Sprintf("String(%q)", string(s))
}

// TimeTag is an OSC timetag. On the wire it's a "64-bit big-endian fixed-point
// time tag" with the same encoding used by NTP. It's one of non-standard types
// in the original spec, but it is mandatory in 1.1. We just wrap a time.Time so
// it's easy to use, and assume everything is in UTC.
type TimeTag struct {
	time.Time
}

func (TimeTag) TypeTag() rune { return 't' }

// epoch is the starting point for TimeTags.
var epoch = time.Date(1900, time.January, 1, 0, 0, 0, 0, time.UTC)

func (t TimeTag) Append(b []byte) []byte {
	seconds := t.Sub(epoch).Seconds()
	if seconds <= 0 {
		// A go time could be well before epoch, cut off anything there.
		return append(b, 0, 0, 0, 0, 0, 0, 0, 0)
	}
	// The highest 4 bytes are the integer number of seconds and
	// the lowest four bytes are however much of the fractional part
	// fits in.
	const stepsPerSecond = float64(int64(1) << 32)
	base, frac := math.Modf(seconds)
	out := (uint64(base) << 32) + uint64(frac*stepsPerSecond)
	return binary.BigEndian.AppendUint64(b, out)
}

func (t *TimeTag) Consume(b []byte) ([]byte, error) {
	if l := len(b); l < 8 {
		return nil, fmt.Errorf("expected timetag (8 bytes), only %d bytes", l)
	}
	raw := binary.BigEndian.Uint64(b)
	seconds := float64(raw >> 32)
	seconds += float64(raw&0xffffffff) / float64(1<<32)
	*t = TimeTag{epoch.Add(time.Duration(seconds * float64(time.Second)))}
	return b[8:], nil
}

func (t TimeTag) String() string {
	return fmt.Sprintf("TimeTag(%v)", t.Time)
}

/*
   Additional mandatory types from the OSC 1.1 NIME paper
   (https://ccrma.stanford.edu/groups/osc/files/2009-NIME-OSC-1.1.pdf)
*/

// True is a boolean true, it contains no data.
type True struct{}

func (True) TypeTag() rune                    { return 'T' }
func (True) Append(b []byte) []byte           { return b }
func (True) Consume(b []byte) ([]byte, error) { return b, nil }
func (True) String() string                   { return "True" }

// False is a boolean false value, it contains no data.
type False struct{}

func (False) TypeTag() rune                    { return 'F' }
func (False) Append(b []byte) []byte           { return b }
func (False) Consume(b []byte) ([]byte, error) { return b, nil }
func (False) String() string                   { return "False" }

// Null is just an empty value.
type Null struct{}

func (Null) TypeTag() rune                    { return 'N' }
func (Null) Append(b []byte) []byte           { return b }
func (Null) Consume(b []byte) ([]byte, error) { return b, nil }
func (Null) String() string                   { return "Null" }

// Impulse (aka "bang", or "Infinitum" in OSC 1.0 is another empty type.
type Impulse struct{}

func (Impulse) TypeTag() rune                    { return 'I' }
func (Impulse) Append(b []byte) []byte           { return b }
func (Impulse) Consume(b []byte) ([]byte, error) { return b, nil }
func (Impulse) String() string                   { return "Impulse" }
