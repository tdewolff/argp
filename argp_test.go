package argp

import (
	"strings"
	"testing"

	"github.com/tdewolff/test"
)

type STypesStruct struct {
	Bool   bool
	Struct struct {
		Float64 float64
	}
}

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
	Array   [3]int
	Slice   []string
	Struct  STypesStruct
}

func (_ *STypes) Run() error {
	return nil
}

func TestArgpTypes(t *testing.T) {
	argpTests := []struct {
		args []string
		s    STypes
	}{
		{[]string{"--string", "val"}, STypes{String: "val"}},
		{[]string{"--bool"}, STypes{Bool: true}},
		{[]string{"--int", "36"}, STypes{Int: 36}},
		{[]string{"--int8", "36"}, STypes{Int8: 36}},
		{[]string{"--int16", "36"}, STypes{Int16: 36}},
		{[]string{"--int32", "36"}, STypes{Int32: 36}},
		{[]string{"--int64", "36"}, STypes{Int64: 36}},
		{[]string{"--uint", "36"}, STypes{Uint: 36}},
		{[]string{"--uint8", "36"}, STypes{Uint8: 36}},
		{[]string{"--uint16", "36"}, STypes{Uint16: 36}},
		{[]string{"--uint32", "36"}, STypes{Uint32: 36}},
		{[]string{"--uint64", "36"}, STypes{Uint64: 36}},
		{[]string{"--float32", "36"}, STypes{Float32: 36}},
		{[]string{"--float64", "36"}, STypes{Float64: 36}},
		{[]string{"--array", "1", "2", "3"}, STypes{Array: [3]int{1, 2, 3}}},
		{[]string{"--slice", "foo", "bar"}, STypes{Slice: []string{"foo", "bar"}}},
		//{[]string{"--slice", "[foo", "bar]"}, STypes{Slice: []string{"foo", "bar"}}},
		//{[]string{"--slice", "[foo bar]"}, STypes{Slice: []string{"foo", "bar"}}},
		// TODO: think of how to parse arrays/slices/structs, with comma, brackets, or both?
		//{"--slice foo,bar", STypes{Slice: []string{"foo", "bar"}}},
		//{"--slice [foo bar]", STypes{Slice: []string{"foo", "bar"}}},
		{[]string{"--struct", "true", "5.0"}, STypes{Struct: STypesStruct{true, struct{ Float64 float64 }{5.0}}}},
		//{"--struct [true 5.0]", STypes{Struct: STypesStruct{true, struct{ Float64 float64 }{5.0}}}},
		//{"--struct.bool true --struct.struct.float64 5.0", STypes{Struct: STypesStruct{true, struct{ Float64 float64 }{5.0}}}},
	}

	for _, tt := range argpTests {
		t.Run(fmt.Sprintf("%v", tt.args), func(t *testing.T) {
			s := STypes{}
			argp := NewCmd(&s, "description")
			_, rest, err := argp.parse(tt.args)
			test.Error(t, err)
			test.T(t, s, tt.s)
			test.T(t, strings.Join(rest, " "), "")
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

func (_ *SOptions) Run() error {
	return nil
}

func TestArgp(t *testing.T) {
	argpTests := []struct {
		args []string
		s    SOptions
		rest string
	}{
		{[]string{"--foo", "val"}, SOptions{Foo: "val", Baz: "default"}, ""},
		{[]string{"-f", "val"}, SOptions{Foo: "val", Baz: "default"}, ""},
		{[]string{"--barbar", "val"}, SOptions{Bar: "val", Baz: "default"}, ""},
		{[]string{"--baz", "val"}, SOptions{Baz: "val"}, ""},
		{[]string{"input1", "input2"}, SOptions{Baz: "default"}, "input1 input2"},
		{[]string{"input1", "--baz", "val", "input2"}, SOptions{Baz: "val"}, "input1 input2"},
		{[]string{"-a", "-b", "-c", "5"}, SOptions{Baz: "default", A: true, B: true, C: 5}, ""},
		{[]string{"-a", "-b", "-c=5"}, SOptions{Baz: "default", A: true, B: true, C: 5}, ""},
		{[]string{"-a", "-b", "-c5"}, SOptions{Baz: "default", A: true, B: true, C: 5}, ""},
		{[]string{"-abc5"}, SOptions{Baz: "default", A: true, B: true, C: 5}, ""},
		{[]string{"--", "-abc5"}, SOptions{Baz: "default"}, "-abc5"},
		{[]string{"--n-A_më", "val"}, SOptions{Baz: "default", Name: "val"}, ""},
		{[]string{"--Baz=-"}, SOptions{Baz: "-"}, ""},
		{[]string{"--Baz", "-"}, SOptions{Baz: "-"}, ""},
		{[]string{"-"}, SOptions{Baz: "default"}, "-"},
	}

	for _, tt := range argpTests {
		t.Run(fmt.Sprintf("%v", tt.args), func(t *testing.T) {
			s := SOptions{}
			argp := NewCmd(&s, "description")

			_, rest, err := argp.parse(tt.args)
			test.Error(t, err)
			test.T(t, s, tt.s)
			test.T(t, strings.Join(rest, " "), tt.rest)
		})
	}
}

func TestArgpAdd(t *testing.T) {
	var o int64
	var v bool
	argp := New("description")
	argp.AddOpt(&o, "", "long", 4, "description")
	argp.AddVal(&v, false, "description")

	_, _, err := argp.parse([]string{"--long", "8", "true"})
	test.Error(t, err)
	test.T(t, o, int64(8))
	test.T(t, v, true)

	_, _, err = argp.parse([]string{})
	test.Error(t, err)
	test.T(t, o, int64(4))
	test.T(t, v, false)
}

func TestArgpUTF8(t *testing.T) {
	var v bool
	argp := New("description")
	argp.AddOpt(&v, "á", "", false, "description")

	_, _, err := argp.parse([]string{"-á"})
	test.Error(t, err)
	test.T(t, v, true)
}

func TestArgpCount(t *testing.T) {
	var i Count
	argp := New("description")
	argp.AddOpt(&i, "i", "int", 0, "description")

	_, _, err := argp.parse([]string{"-i", "-ii", "--int", "--int"})
	test.Error(t, err)
	test.T(t, i, Count(5))

	_, _, err = argp.parse([]string{"-i", "3"})
	test.Error(t, err)
	test.T(t, i, Count(3))

	_, _, err = argp.parse([]string{"--int", "3"})
	test.Error(t, err)
	test.T(t, i, Count(3))
}

type SSub1 struct {
	B int `short:"b"`
}

func (_ *SSub1) Run() error {
	return nil
}

type SSub2 struct {
	C int `short:"c"`
}

func (_ *SSub2) Run() error {
	return nil
}

func TestArgpSubCommand(t *testing.T) {
	var v string
	var a int
	sub1 := SSub1{}
	sub2 := SSub2{}
	argp := New("description")
	argp.AddVal(&v, "", "description")
	argp.AddOpt(&a, "a", "", 0, "description")
	argp.AddCmd(&sub1, "one", "description")
	argp.AddCmd(&sub2, "two", "description")

	_, _, err := argp.parse([]string{"val", "-a", "1"})
	test.Error(t, err)
	test.T(t, v, "val")
	test.T(t, a, 1)

	_, _, err = argp.parse([]string{"one", "-b", "2"})
	test.Error(t, err)
	test.T(t, sub1.B, 2)

	_, _, err = argp.parse([]string{"two", "-c", "3"})
	test.Error(t, err)
	test.T(t, sub2.C, 3)
}
