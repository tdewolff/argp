package argp

import (
	"strings"
	"testing"

	"github.com/tdewolff/test"
)

type STypes struct {
	String  string
	Bool    bool
	Int     int
	Int8    int8
	Int16   int16
	Int32   int32
	Int64   int64
	Uint    uint
	Uint8   uint8
	Uint16  uint16
	Uint32  uint32
	Uint64  uint64
	Float32 float32
	Float64 float64
}

func TestArgpTypes(t *testing.T) {
	argpTests := []struct {
		args string
		s    STypes
		rest string
	}{
		{"--string val", STypes{String: "val"}, ""},
		{"--bool", STypes{Bool: true}, ""},
		{"--int 36", STypes{Int: 36}, ""},
		{"--int8 36", STypes{Int8: 36}, ""},
		{"--int16 36", STypes{Int16: 36}, ""},
		{"--int32 36", STypes{Int32: 36}, ""},
		{"--int64 36", STypes{Int64: 36}, ""},
		{"--uint 36", STypes{Uint: 36}, ""},
		{"--uint8 36", STypes{Uint8: 36}, ""},
		{"--uint16 36", STypes{Uint16: 36}, ""},
		{"--uint32 36", STypes{Uint32: 36}, ""},
		{"--uint64 36", STypes{Uint64: 36}, ""},
		{"--float32 36", STypes{Float32: 36}, ""},
		{"--float64 36", STypes{Float64: 36}, ""},
	}

	for _, tt := range argpTests {
		t.Run(tt.args, func(t *testing.T) {
			argp := NewArgp("description")

			s := &STypes{}
			err := argp.AddStruct(s)
			test.Error(t, err)

			rest, err := argp.parse(strings.Split(tt.args, " "))
			test.Error(t, err)
			test.T(t, *s, tt.s)
			test.T(t, strings.Join(rest, " "), tt.rest)
		})
	}
}

type SOptions struct {
	Foo  string `short:"f"`
	Bar  string `long:"barbar"`
	Baz  string `default:"default"`
	A    bool   `short:"a"`
	B    bool   `short:"b"`
	C    int    `short:"c"`
	Name string `long:"N-a_më"`
}

func TestArgp(t *testing.T) {
	argpTests := []struct {
		args string
		s    SOptions
		rest string
	}{
		{"--foo val", SOptions{Foo: "val", Baz: "default"}, ""},
		{"-f val", SOptions{Foo: "val", Baz: "default"}, ""},
		{"--barbar val", SOptions{Bar: "val", Baz: "default"}, ""},
		{"--baz val", SOptions{Baz: "val"}, ""},
		{"input1 input2", SOptions{Baz: "default"}, "input1 input2"},
		{"input1 --baz val input2", SOptions{Baz: "val"}, "input1 input2"},
		{"-a -b -c 5", SOptions{Baz: "default", A: true, B: true, C: 5}, ""},
		{"-a -b -c=5", SOptions{Baz: "default", A: true, B: true, C: 5}, ""},
		{"-a -b -c5", SOptions{Baz: "default", A: true, B: true, C: 5}, ""},
		{"-abc5", SOptions{Baz: "default", A: true, B: true, C: 5}, ""},
		{"-- -abc5", SOptions{Baz: "default"}, "-abc5"},
		{"--n-A_më val", SOptions{Baz: "default", Name: "val"}, ""},
		{"--Baz=-", SOptions{Baz: "-"}, ""},
		{"--Baz -", SOptions{Baz: "-"}, ""},
		{"-", SOptions{Baz: "default"}, "-"},
	}

	for _, tt := range argpTests {
		t.Run(tt.args, func(t *testing.T) {
			argp := NewArgp("description")

			s := &SOptions{}
			err := argp.AddStruct(s)
			test.Error(t, err)

			rest, err := argp.parse(strings.Split(tt.args, " "))
			test.Error(t, err)
			test.T(t, *s, tt.s)
			test.T(t, strings.Join(rest, " "), tt.rest)
		})
	}
}

func TestArgpAdd(t *testing.T) {
	argp := NewArgp("description")

	var v int64
	err := argp.Add(&v, "", "verb", 4, "")
	test.Error(t, err)

	_, err = argp.parse([]string{"--verb", "8"})
	test.Error(t, err)
	test.T(t, v, int64(8))

	_, err = argp.parse([]string{})
	test.Error(t, err)
	test.T(t, v, int64(4))
}

func TestArgpSub(t *testing.T) {
	argp := NewArgp("description")
	sub1 := NewArgp("first")
	sub2 := NewArgp("second")
	argp.AddCommand("first", sub1)
	argp.AddCommand("second", sub2)

	var a int
	err := argp.Add(&a, "a", "", nil, "")
	test.Error(t, err)

	var b int
	err = sub1.Add(&b, "b", "", nil, "")
	test.Error(t, err)

	var c int
	err = sub2.Add(&c, "c", "", nil, "")
	test.Error(t, err)

	_, err = argp.parse([]string{"-a", "1"})
	test.Error(t, err)
	test.T(t, a, 1)

	_, err = argp.parse([]string{"first", "-b", "2"})
	test.Error(t, err)
	test.T(t, b, 2)

	_, err = argp.parse([]string{"second", "-c", "3"})
	test.Error(t, err)
	test.T(t, c, 3)
}
