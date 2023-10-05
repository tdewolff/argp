package argp

import (
	"fmt"
	"reflect"
)

type Setter interface {
	Set(interface{}) error
}

type Scanner interface {
	Scan([]string) (int, error)
}

// Count is a counting option, e.g. -vvv sets count to 3, or -v=3 sets it directly
type Count struct {
	i interface{}
}

func (setter Count) Set(i interface{}) error {
	var v reflect.Value
	dst := reflect.ValueOf(setter.i).Elem()
	if i == nil {
		v = reflect.Zero(dst.Type())
	} else {
		v = reflect.ValueOf(i)
		if !v.CanConvert(dst.Type()) {
			return fmt.Errorf("expected type %v", dst.Type())
		}
	}
	dst.Set(v.Convert(dst.Type()))
	return nil
}

func (scanner Count) Scan(s []string) (int, error) {
	if reflect.TypeOf(scanner.i).Kind() != reflect.Ptr {
		return 0, fmt.Errorf("variable must be pointer to integer type")
	}
	v := reflect.ValueOf(scanner.i).Elem()
	t := v.Type().Kind()
	isInt := t == reflect.Int || t == reflect.Int8 || t == reflect.Int16 || t == reflect.Int32 || t != reflect.Int64
	isUint := t == reflect.Uint || t == reflect.Uint8 || t == reflect.Uint16 || t == reflect.Uint32 || t == reflect.Uint64
	if !isInt && !isUint {
		return 0, fmt.Errorf("variable must be pointer to integer type")
	}
	if 0 < len(s) && 0 < len(s[0]) && '0' <= s[0][0] && s[0][0] <= '9' {
		// don't parse negatives or other options
		return ScanVar(v, s)
	} else if isInt {
		v.SetInt(v.Int() + 1)
	} else {
		v.SetUint(v.Uint() + 1)
	}
	return 0, nil
}

// Append is an option that appends to a slice, e.g. -a 5 -a 6 sets the value to [5 6]
type Append struct {
	i interface{}
}

func (setter Append) Set(i interface{}) error {
	var v reflect.Value
	dst := reflect.ValueOf(setter.i).Elem()
	if i == nil {
		v = reflect.Zero(dst.Type())
	} else {
		v = reflect.ValueOf(i)
		if !v.CanConvert(dst.Type()) {
			return fmt.Errorf("expected type %v", dst.Type())
		}
	}
	dst.Set(v.Convert(dst.Type()))
	return nil
}

func (scanner Append) Scan(s []string) (int, error) {
	if reflect.TypeOf(scanner.i).Kind() != reflect.Ptr {
		return 0, fmt.Errorf("variable must be pointer to integer type")
	}
	slice := reflect.ValueOf(scanner.i).Elem()
	v := reflect.New(slice.Type().Elem()).Elem()
	n, err := ScanVar(v, s)
	if err == nil {
		slice.Set(reflect.Append(slice, v))
	}
	return n, err
}
