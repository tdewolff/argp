package argp

import (
	"fmt"
	"strconv"
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
	Map     map[string]string
	Struct  STypesStruct `short:"s"`
}

func (_ *STypes) Run() error {
	return nil
}

func TestArgpTypes(t *testing.T) {
	tests := []struct {
		args []string
		s    STypes
	}{
		{[]string{"--string", "val"}, STypes{String: "val"}},
		{[]string{"--string", ""}, STypes{String: ""}},
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
		{[]string{"--array", "[1", "2", "3]"}, STypes{Array: [3]int{1, 2, 3}}},
		{[]string{"--array[1]", "2"}, STypes{Array: [3]int{0, 2, 0}}},
		{[]string{"--slice", "[foo", "bar]"}, STypes{Slice: []string{"foo", "bar"}}},
		{[]string{"--slice", "[foo", "", "]"}, STypes{Slice: []string{"foo", ""}}},
		{[]string{"--slice", "[", "foo", "bar", "]"}, STypes{Slice: []string{"foo", "bar"}}},
		{[]string{"--slice", "[foo bar]"}, STypes{Slice: []string{"foo bar"}}},
		{[]string{"--slice", "[", "foo bar", "]"}, STypes{Slice: []string{"foo bar"}}},
		{[]string{"--slice", "foo,bar"}, STypes{Slice: []string{"foo", "bar"}}},
		{[]string{"--slice", "foo,,"}, STypes{Slice: []string{"foo", "", ""}}},
		{[]string{"--slice", "foo", ",", "bar"}, STypes{Slice: []string{"foo", "bar"}}},
		{[]string{"--slice", "foo,", "bar"}, STypes{Slice: []string{"foo", "bar"}}},
		{[]string{"--slice", "foo", ",bar"}, STypes{Slice: []string{"foo", "bar"}}},
		{[]string{"--slice", "foo bar,zim"}, STypes{Slice: []string{"foo bar", "zim"}}},
		{[]string{"--map", "{foo:2 bar:3}"}, STypes{Map: map[string]string{"foo": "2 bar:3"}}},
		{[]string{"--map", "{foo:2,bar:3}"}, STypes{Map: map[string]string{"foo": "2", "bar": "3"}}},
		{[]string{"--map", "{foo:2", "bar:3}"}, STypes{Map: map[string]string{"foo": "2", "bar": "3"}}},
		{[]string{"--map", "{", "foo", ":", "2", "", ":", "", "}"}, STypes{Map: map[string]string{"foo": "2", "": ""}}},
		{[]string{"--map[foo]=2", "--map[bar]=3"}, STypes{Map: map[string]string{"foo": "2", "bar": "3"}}},
		{[]string{"--struct", "{true", "{5.0}}"}, STypes{Struct: STypesStruct{true, struct{ Float64 float64 }{5.0}}}},
		{[]string{"-s", "{true", "{5.0}}"}, STypes{Struct: STypesStruct{true, struct{ Float64 float64 }{5.0}}}},
		{[]string{"--struct.bool", "--struct.struct.float64", "5.0"}, STypes{Struct: STypesStruct{true, struct{ Float64 float64 }{5.0}}}},
	}

	for _, tt := range tests {
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

type SSlice struct {
	Val [][]string
}

func (_ *SSlice) Run() error {
	return nil
}

func TestArgpSlice(t *testing.T) {
	tests := []struct {
		args []string
		s    SSlice
	}{
		{[]string{"--val", "foo,bar,zim"}, SSlice{[][]string{{"foo"}, {"bar"}, {"zim"}}}},
		{[]string{"--val", "[foo", "bar", "zim]"}, SSlice{[][]string{{"foo"}, {"bar"}, {"zim"}}}},
		{[]string{"--val", "[[foo", "bar", "zim]]"}, SSlice{[][]string{{"foo", "bar", "zim"}}}},
		{[]string{"--val", "[[foo]", "[bar zim]]"}, SSlice{[][]string{{"foo"}, {"bar zim"}}}},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%v", tt.args), func(t *testing.T) {
			s := SSlice{}
			argp := NewCmd(&s, "description")
			_, rest, err := argp.parse(tt.args)
			test.Error(t, err)
			test.T(t, s, tt.s)
			test.T(t, strings.Join(rest, " "), "")
		})
	}
}

type SStructVal struct {
	I []int
	M map[int]int
}

type SStruct struct {
	Val SStructVal
}

func (_ *SStruct) Run() error {
	return nil
}

func TestArgpStruct(t *testing.T) {
	tests := []struct {
		args []string
		s    SStruct
	}{
		{[]string{"--val", "{[5", "6]", "{7:8", "9:10}}"}, SStruct{SStructVal{[]int{5, 6}, map[int]int{7: 8, 9: 10}}}},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%v", tt.args), func(t *testing.T) {
			s := SStruct{}
			argp := NewCmd(&s, "description")
			_, rest, err := argp.parse(tt.args)
			test.Error(t, err)
			test.T(t, s, tt.s)
			test.T(t, strings.Join(rest, " "), "")
		})
	}
}

type SMapKey struct {
	F float64
	B bool
}

type SMap struct {
	Val map[SMapKey][]string
}

func (_ *SMap) Run() error {
	return nil
}

func TestArgpMap(t *testing.T) {
	tests := []struct {
		args []string
		s    SMap
	}{
		{[]string{"--val", "{{5.0", "true}:", "[foo", "bar]", "{6.0", "false}:", "[zim]}"}, SMap{map[SMapKey][]string{{5.0, true}: {"foo", "bar"}, {6.0, false}: {"zim"}}}},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%v", tt.args), func(t *testing.T) {
			s := SMap{}
			argp := NewCmd(&s, "description")
			_, rest, err := argp.parse(tt.args)
			test.Error(t, err)
			test.T(t, s, tt.s)
			test.T(t, strings.Join(rest, " "), "")
		})
	}
}

func TestArgpErrors(t *testing.T) {
	tests := []struct {
		args []string
		err  string
	}{
		{[]string{"--int"}, "option --int: missing value"},
		{[]string{"--int", "string"}, "option --int: invalid integer 'string'"},
		{[]string{"--uint", "-1"}, "option --uint: invalid positive integer '-1'"},
		{[]string{"--float64", "."}, "option --float64: invalid number '.'"},
		{[]string{"--array", "[1"}, "option --array: invalid array"},
		{[]string{"--array", "[1]"}, "option --array: expected 3 values for array"},
		{[]string{"--array", "[1", "2", "s]"}, "option --array: array index 2: invalid integer 's'"},
		{[]string{"--slice", "[s"}, "option --slice: invalid slice"},
		{[]string{"--map", "{foo:2"}, "option --map: invalid map"},
		{[]string{"--map", "{foo", "2}"}, "option --map: map key foo: missing semicolon"},
		{[]string{"--struct", "{true"}, "option --struct: invalid struct"},
		{[]string{"--struct", "{true}"}, "option --struct: missing struct fields"},
		{[]string{"--struct", "{5}"}, "option --struct: struct field Bool: invalid boolean '5'"},
		{[]string{"--struct", "{true", "{x}}"}, "option --struct: struct field Struct: struct field Float64: invalid number 'x'"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%v", tt.args), func(t *testing.T) {
			s := STypes{}
			argp := NewCmd(&s, "description")
			_, _, err := argp.parse(tt.args)
			serr := ""
			if err != nil {
				serr = err.Error()
			}
			test.T(t, serr, tt.err)
		})
	}

	var err error
	var cmd *Argp

	cmd = New("description")
	cmd.AddArg(&[]int{}, "", "")
	_, _, err = cmd.parse([]string{"5,s"})
	test.T(t, err, fmt.Errorf("argument 0: slice index 1: invalid integer 's'"))

	cmd = New("description")
	cmd.AddArg(&map[int]int{}, "", "")
	_, _, err = cmd.parse([]string{"{s:5}"})
	test.T(t, err, fmt.Errorf("argument 0: map key s: invalid integer 's'"))
	_, _, err = cmd.parse([]string{"{5:s}"})
	test.T(t, err, fmt.Errorf("argument 0: map key 5: invalid integer 's'"))

	cmd = New("description")
	cmd.AddOpt(&map[int]int{}, "", "val", "")
	_, _, err = cmd.parse([]string{"--val[s]=6"})
	test.T(t, err, fmt.Errorf("option --val[s]: map key s: invalid integer 's'"))
}

type SOptions struct {
	Foo  string `short:"f"`
	Bar  string `name:"barbar"`
	Baz  string `default:"default"`
	A    bool   `short:"a"`
	B    bool   `short:"b"`
	C    int    `short:"c"`
	Name string `name:"N-a_më"`
}

func (_ *SOptions) Run() error {
	return nil
}

func TestArgp(t *testing.T) {
	tests := []struct {
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
		{[]string{"-a"}, SOptions{Baz: "default", A: true}, ""},
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

	for _, tt := range tests {
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
	o := int64(4)
	var v bool
	argp := New("description")
	argp.AddOpt(&o, "", "name", "description")
	argp.AddArg(&v, "", "description")

	_, _, err := argp.parse([]string{"--name", "8", "true"})
	test.Error(t, err)
	test.T(t, o, int64(8))
	test.T(t, v, true)

	_, _, err = argp.parse([]string{})
	test.Error(t, err)
}

func TestArgpAddRest(t *testing.T) {
	var rest []string
	argp := New("description")
	argp.AddRest(&rest, "rest", "description")

	_, _, err := argp.parse([]string{"file1.txt", "file2.txt", "file3.txt"})
	test.Error(t, err)
	test.T(t, rest, []string{"file1.txt", "file2.txt", "file3.txt"})

	_, _, err = argp.parse([]string{})
	test.Error(t, err)
	test.T(t, rest, []string{})
}

func TestArgpUTF8(t *testing.T) {
	var v bool
	argp := New("description")
	argp.AddOpt(&v, "á", "", "description")

	_, _, err := argp.parse([]string{"-á"})
	test.Error(t, err)
	test.T(t, v, true)
}

func TestArgpCount(t *testing.T) {
	var i int
	argp := New("description")
	argp.AddOpt(Count{&i}, "i", "int", "description")

	_, _, err := argp.parse([]string{"-i", "-ii", "--int", "--int"})
	test.Error(t, err)
	test.T(t, i, 5)

	_, _, err = argp.parse([]string{"-i", "3"})
	test.Error(t, err)
	test.T(t, i, 3)

	_, _, err = argp.parse([]string{"--int", "3"})
	test.Error(t, err)
	test.T(t, i, 3)
}

func TestArgpAppend(t *testing.T) {
	var i []int
	var s []string
	argp := New("description")
	argp.AddOpt(Append{&i}, "i", "int", "description")
	argp.AddOpt(Append{&s}, "s", "string", "description")

	_, _, err := argp.parse([]string{"-i", "1", "--int", "2"})
	test.Error(t, err)
	test.T(t, i, []int{1, 2})

	_, _, err = argp.parse([]string{"-s", "foo", "--string", "bar"})
	test.Error(t, err)
	test.T(t, s, []string{"foo", "bar"})
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
	argp.AddArg(&v, "", "description")
	argp.AddOpt(&a, "a", "", "description")
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

func TestSplitArguments(t *testing.T) {
	tests := []struct {
		str  string
		args []string
	}{
		{"foobar", []string{"foobar"}},
		{"foo bar", []string{"foo", "bar"}},
		{"'foo bar'", []string{"foo bar"}},
		{"'foo'\"bar\"", []string{"foobar"}},
		{"'foo\\'bar'", []string{"foo'bar"}},
		{"foo ' bar '", []string{"foo", " bar "}},
	}

	for _, tt := range tests {
		t.Run(tt.str, func(t *testing.T) {
			args := splitArguments(tt.str)
			test.T(t, args, tt.args)
		})
	}
}

func TestCount(t *testing.T) {
	var count int
	argp := New("count variable")
	argp.AddOpt(Count{&count}, "c", "count", "")

	_, _, err := argp.parse([]string{"-ccc"})
	if err != nil {
		test.Error(t, err)
	}
	test.T(t, count, 3)
}

func ExampleCount() {
	var count int
	argp := New("count variable")
	argp.AddOpt(Count{&count}, "c", "count", "")

	_, _, err := argp.parse([]string{"-ccc"})
	if err != nil {
		panic(err)
	}
	fmt.Println(count)
	// Output: 3
}

type CustomVar struct {
	Num, Div float64
}

func (e *CustomVar) Help() (string, string) {
	return fmt.Sprintf("%v/%v", e.Num, e.Div), ""
}

func (e *CustomVar) Scan(name string, s []string) (int, error) {
	n := 0
	num := s[0]
	if idx := strings.IndexByte(s[0], '/'); idx != -1 {
		num = s[0][:idx]
		if idx+1 == len(s[0]) {
			s = s[1:]
			n++
		} else {
			s[0] = s[0][idx+1:]
		}
	} else if 1 < len(s) && 0 < len(s[1]) && s[1][0] == '/' {
		s = s[1:]
		n++
		if len(s[0]) == 1 {
			s = s[1:]
			n++
		} else {
			s[0] = s[0][1:]
		}
	} else {
		return 0, fmt.Errorf("missing fraction")
	}
	div := s[0]
	fnum, err := strconv.ParseFloat(num, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid number '%v'", num)
	}
	fdiv, err := strconv.ParseFloat(div, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid number '%v'", div)
	}
	e.Num = fnum
	e.Div = fdiv
	return n + 1, nil
}

func TestCustom(t *testing.T) {
	custom := CustomVar{}
	argp := New("custom variable")
	argp.AddOpt(&custom, "", "custom", "")

	_, _, err := argp.parse([]string{"--custom", "1", "/", "2"})
	if err != nil {
		test.Error(t, err)
	}
	test.T(t, custom, CustomVar{1.0, 2.0})
}

func ExampleCustom() {
	custom := CustomVar{}
	argp := New("custom variable")
	argp.AddOpt(&custom, "", "custom", "")

	_, _, err := argp.parse([]string{"--custom", "1", "/", "2"})
	if err != nil {
		panic(err)
	}
	fmt.Println(custom.Num, "/", custom.Div)
	// Output: 1 / 2
}
