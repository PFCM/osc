package osc

import (
	"bytes"
	"encoding/binary"
	"math"
	"math/rand"
	"reflect"
	"strings"
	"testing"
)

func TestMessageRoundtrip(t *testing.T) {
	const (
		maxAddr   = 30
		maxString = 25
		maxArgs   = 50
	)
	str := func() string {
		const chars = "abcdefghijklmnopqrstuvwzyz"
		b := make([]byte, rand.Intn(maxString))
		for i := range b {
			b[i] = chars[rand.Intn(len(chars))]
		}
		return string(b)
	}
	args := []func() Argument{
		func() Argument {
			i := Int32(rand.Int31())
			return &i
		},
		func() Argument {
			u := rand.Uint32()
			f := Float32(math.Float32frombits(u))
			return &f
		},
		func() Argument {
			s := String(str())
			return &s
		},
		func() Argument {
			return True{}
		},
		func() Argument {
			return False{}
		},
	}
	arguments := func() []Argument {
		as := make([]Argument, rand.Intn(maxArgs))
		for i := range as {
			as[i] = args[rand.Intn(len(args))]()
		}
		return as
	}
	pattern := func() string {
		path := make([]string, rand.Intn(maxAddr)+1)
		for i := range path {
			if i == 0 {
				// should start with /
				continue
			}
			path[i] = str()
		}
		return strings.Join(path, "/")
	}

	msgs := []*Message{
		{},
		{Pattern: ""},
		{Pattern: "hi"},
		{Pattern: "/hi"},
		{Pattern: "/hi", Arguments: []Argument{}},
	}
	for i := 0; i < 1000; i++ {
		msgs = append(msgs, &Message{
			Pattern:   pattern(),
			Arguments: arguments(),
		})
	}

	for _, msg := range msgs {
		enc := msg.Append(nil)
		got, err := ParseMessage(enc)
		if err != nil {
			t.Errorf("ParseMessage: %v\n(%v)", err, msg)
			continue
		}
		gotEnc := got.Append(nil)
		if msg.Arguments == nil {
			msg.Arguments = []Argument{}
		}
		if got.Arguments == nil {
			got.Arguments = []Argument{}
		}
		// replace NaNs so we can compare with DeepEqual.
		for i, a := range msg.Arguments {
			if f, ok := a.(*Float32); ok {
				if f != nil && math.IsNaN(float64(*f)) {
					g := Float32(0)
					msg.Arguments[i] = &g
				}
			}
		}
		for i, a := range got.Arguments {
			if f, ok := a.(*Float32); ok {
				if f != nil && math.IsNaN(float64(*f)) {
					g := Float32(0)
					got.Arguments[i] = &g
				}
			}
		}
		if !reflect.DeepEqual(msg, got) {
			t.Errorf("Message did not survive round trip:\nwant: %v\n got: %v\n%q", msg, got, enc)
		}
		if !bytes.Equal(enc, gotEnc) {
			t.Errorf("Unstable encoding:\n first: %q\nsecond: %q", enc, gotEnc)
		}
	}
}

func TestInt32(t *testing.T) {
	// Testing every int32 is probably overkill, but we can test
	// quite a few.
	cases := []int32{math.MaxInt32, math.MinInt32, -1, 0, 1}
	for i := 0; i < 10000; i++ {
		cases = append(cases, rand.Int31())
	}
	b1, b2 := make([]byte, 4), make([]byte, 4)
	for _, i := range cases {
		j := Int32(i)
		b1 = j.Append(b1[:0])
		binary.BigEndian.PutUint32(b2, uint32(i))
		if !bytes.Equal(b1, b2) {
			t.Errorf("Int32(%d).Append = %x, want: %x", i, b1, b2)
			continue
		}
		if _, err := j.Consume(b1); err != nil {
			t.Errorf("Int32.UnmarshalBinary(%x): unexpected error", b1)
			continue
		}
		if int32(j) != i {
			t.Errorf("Int32.UnmarshalBinary(%x) = %d, want: %d", b1, j, i)
		}
	}
}

func TestFloat32(t *testing.T) {
	cases := []float32{
		math.MaxFloat32,
		-math.MaxFloat32,
		0, -0,
		float32(math.NaN()),
		math.SmallestNonzeroFloat32,
		math.Float32frombits(0x00800000), // smallest normal float32
	}
	for i := 0; i < 10000; i++ {
		cases = append(cases, (rand.Float32()*2-1)*math.MaxFloat32)
	}

	b1, b2 := make([]byte, 4), make([]byte, 4)
	for _, f := range cases {
		g := Float32(f)
		b1 = g.Append(b1[:0])
		binary.BigEndian.PutUint32(b2, math.Float32bits(f))
		if !bytes.Equal(b1, b2) {
			t.Errorf("Float32(%f).Append = %x, want: %x", f, b1, b2)
			continue
		}
		if _, err := g.Consume(b1); err != nil {
			t.Errorf("Float32.UnmarshalBinary(%x): unexpected error", b1)
			continue
		}
		// To avoid any float weirdness, compare the bits; we expect
		// bitwise equality here anyway.
		got := math.Float32bits(float32(g))
		want := math.Float32bits(f)
		if got != want {
			t.Errorf("Float32.UnmarshalBinary(%x) = %f, want: %f", b1, g, f)
		}
	}
}

func TestStringConsume(t *testing.T) {
	nt := func(s string) []byte {
		b := append([]byte(s), 0)
		for len(b)%4 > 0 {
			b = append(b, 0)
		}
		return b
	}
	type testCase struct {
		in      []byte
		out     string
		tail    []byte
		wantErr bool
	}
	cases := []testCase{{
		in:  []byte{'a', 'B', 'c', 0},
		out: "aBc",
	}, {
		in:   []byte{'a', 0, 0, 0, 0},
		out:  "a",
		tail: []byte{0},
	}, {
		in:      []byte("not terminated"),
		wantErr: true,
	}, {
		in:      []byte{}, // empty string, not terminated.
		wantErr: true,
	}, {
		in:  []byte{0}, // empty string, terminated.
		out: "",
	}, {
		in:  []byte{0, 0}, // empty string, excess termination
		out: "",
	}, {
		in:  []byte{0, 0, 0},
		out: "",
	}, {
		in:  []byte{0, 0, 0, 0},
		out: "",
	}}

	const in = "on the longer side"
	for i := 0; i < len(in); i++ {
		cases = append(cases, testCase{
			in:   append(nt(in[:i]), in[i:]...),
			out:  in[:i],
			tail: []byte(in[i:]),
		})
	}

	for _, c := range cases {
		var got String
		gotTail, err := got.Consume(c.in)
		if err != nil {
			if !c.wantErr {
				t.Errorf("String.Consume(%q) = %v", c.in, err)
			}
			continue
		}
		if string(got) != c.out {
			t.Errorf("String.Consume(%q) = %q, want %q", c.in, got, c.out)
		}
		if !bytes.Equal(gotTail, c.tail) {
			t.Errorf("String.Consume(%q): tail = %q, want %q", c.in, gotTail, c.tail)
		}
	}
}

func TestArgRoundTrip(t *testing.T) {
	t.Run("Int32", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			j := Int32(rand.Int31())
			testArgRoundTrip(t, &j, func() *Int32 {
				return new(Int32)
			})
		}
	})
	t.Run("Float32", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			f := Float32(rand.Float32())
			testArgRoundTrip(t, &f, func() *Float32 {
				return new(Float32)
			})
		}
	})
	t.Run("String", func(t *testing.T) {
		const chars = "1234567890abcdefghijklmnop"
		inputs := make([]String, 100)
		for i := range inputs {
			n := rand.Intn(25)
			b := make([]byte, n)
			for j := range b {
				b[j] = chars[rand.Intn(len(chars))]
			}
			inputs[i] = String(b)
		}
		inputs[0] = String("")
		for _, s := range inputs {
			testArgRoundTrip(t, &s, func() *String {
				return new(String)
			})
		}
	})
	t.Run("TimeTag", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			b := make([]byte, 8)
			rand.Read(b)
			tt := new(TimeTag)
			if _, err := tt.Consume(b); err != nil {
				t.Errorf("TimeTag.Consume: %v", err)
			}
			testArgRoundTrip(t, tt, func() *TimeTag {
				return new(TimeTag)
			})
		}
	})
	t.Run("True", func(t *testing.T) {
		// There's only one value to test.
		testArgRoundTrip(t, True{}, func() True { return True{} })
	})
	t.Run("False", func(t *testing.T) {
		// There's only one value to test.
		testArgRoundTrip(t, False{}, func() False { return False{} })
	})
	t.Run("Null", func(t *testing.T) {
		// There's only one value to test.
		testArgRoundTrip(t, Null{}, func() Null { return Null{} })
	})
	t.Run("Impulse", func(t *testing.T) {
		// There's only one value to test.
		testArgRoundTrip(t, Impulse{}, func() Impulse { return Impulse{} })
	})
}

func testArgRoundTrip[T Argument](t *testing.T, a T, mk func() T) {
	t.Helper()
	enc := a.Append(nil)
	// Add some random bytes to the end, to make sure Consume doesn't touch
	// them.
	var tail [11]byte
	rand.Read(tail[:])
	enc = append(enc, tail[:]...)

	got := mk()
	gotTail, err := got.Consume(enc)
	if err != nil {
		t.Fatalf("Round trip (%c: %v) failed: %v", a.TypeTag(), a, err)
	}
	if !reflect.DeepEqual(a, got) {
		t.Errorf("Round trip (%c) failed:\n got: %v\nwant: %v", a.TypeTag(), got, a)
	}
	if !bytes.Equal(tail[:], gotTail) {
		t.Errorf("Round trip (%c) filed: wrong leftovers after Consume:\n got: %x\nwant: %x", a.TypeTag(), gotTail, tail)
	}
}
