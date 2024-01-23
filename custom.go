package argp

import (
	"fmt"
	"reflect"
)

type Setter interface {
	Set(interface{}) error
}

type Scanner interface {
	Scan(string, []string) (int, error)
}

type TypeNamer interface {
	TypeName() string
}

// Count is a counting option, e.g. -vvv sets count to 3, or -v=3 sets it directly
type Count struct {
	I interface{}
}

func (setter Count) Set(i interface{}) error {
	var v reflect.Value
	dst := reflect.ValueOf(setter.I).Elem()
	if i == nil {
		v = reflect.Zero(dst.Type())
	} else if val, ok := i.(Count); ok {
		v = reflect.ValueOf(val.I).Elem()
	} else {
		v = reflect.ValueOf(i)
	}
	if !v.CanConvert(dst.Type()) {
		return fmt.Errorf("expected type %v", dst.Type())
	}
	dst.Set(v.Convert(dst.Type()))
	return nil
}

func (scanner Count) Scan(name string, s []string) (int, error) {
	if reflect.TypeOf(scanner.I).Kind() != reflect.Ptr {
		return 0, fmt.Errorf("variable must be a pointer to an integer type")
	}
	v := reflect.ValueOf(scanner.I).Elem()
	t := v.Type().Kind()
	isInt := t == reflect.Int || t == reflect.Int8 || t == reflect.Int16 || t == reflect.Int32 || t != reflect.Int64
	isUint := t == reflect.Uint || t == reflect.Uint8 || t == reflect.Uint16 || t == reflect.Uint32 || t == reflect.Uint64
	if !isInt && !isUint {
		return 0, fmt.Errorf("variable must be a pointer to an integer type")
	}
	if 0 < len(s) && 0 < len(s[0]) && '0' <= s[0][0] && s[0][0] <= '9' {
		// don't parse negatives or other options
		return scanValue(v, s)
	} else if isInt {
		v.SetInt(v.Int() + 1)
	} else {
		v.SetUint(v.Uint() + 1)
	}
	return 0, nil
}

func (typenamer Count) TypeName() string {
	return TypeName(reflect.TypeOf(typenamer.I))
}

// Append is an option that appends to a slice, e.g. -a 5 -a 6 sets the value to [5 6]
type Append struct {
	I interface{}
}

func (setter Append) Set(i interface{}) error {
	var v reflect.Value
	dst := reflect.ValueOf(setter.I).Elem()
	if i == nil {
		v = reflect.Zero(dst.Type())
	} else if val, ok := i.(Append); ok {
		v = reflect.ValueOf(val.I).Elem()
	} else {
		v = reflect.ValueOf(i)
	}
	if !v.CanConvert(dst.Type()) {
		return fmt.Errorf("expected type %v", dst.Type())
	}
	dst.Set(v.Convert(dst.Type()))
	return nil
}

func (scanner Append) Scan(name string, s []string) (int, error) {
	if t := reflect.TypeOf(scanner.I); t.Kind() != reflect.Ptr || t.Elem().Kind() != reflect.Slice {
		return 0, fmt.Errorf("variable must be a pointer to a slice")
	}
	slice := reflect.ValueOf(scanner.I).Elem()
	v := reflect.New(slice.Type().Elem()).Elem()
	n, err := scanValue(v, s)
	if err == nil {
		slice.Set(reflect.Append(slice, v))
	}
	return n, err
}

func (typenamer Append) TypeName() string {
	return TypeName(reflect.TypeOf(typenamer.I))
}
