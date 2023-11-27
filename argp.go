package argp

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ShowUsage can be returned from a command to show the help message.
var ShowUsage error = fmt.Errorf("bad command usage")

// Var is a command option or argument
type Var struct {
	Value       reflect.Value
	Name        string
	Long        string
	Short       rune // 0 if not used
	Index       int  // -1 if not used
	Rest        bool
	Default     interface{} // nil is not used
	Description string
	isSet       bool
}

// IsOption returns true for an option
func (v *Var) IsOption() bool {
	return !v.IsArgument()
}

// IsArgument returns true for an argument
func (v *Var) IsArgument() bool {
	return v.Index != -1 || v.Rest
}

// Set sets the variable's value
func (v *Var) Set(i interface{}) bool {
	val := reflect.ValueOf(i)
	if !val.CanConvert(v.Value.Type()) {
		return false
	}
	v.Value.Set(val.Convert(v.Value.Type()))
	return true
}

// Cmd is a command
type Cmd interface {
	Run() error
}

// Argp is a (sub) command parser
type Argp struct {
	Cmd
	Description string

	parent *Argp
	name   string
	vars   []*Var
	cmds   map[string]*Argp
	help   bool

	Error *log.Logger
}

// New returns a new command parser that can set options and returns the remaining arguments from `Argp.Parse`.
func New(description string) *Argp {
	return NewCmd(nil, description)
}

// NewCmd returns a new command parser that invokes the Run method of the passed command structure. The `Argp.Parse()` function will not return and call either os.Exit(0) or os.Exit(1).
func NewCmd(cmd Cmd, description string) *Argp {
	argp := &Argp{
		Cmd:         cmd,
		Description: description,
		name:        filepath.Base(os.Args[0]),
		cmds:        map[string]*Argp{},
	}
	if cmd != nil {
		v := reflect.ValueOf(cmd)
		if v.Type().Kind() != reflect.Ptr {
			panic("cmd: must pass a pointer to struct")
		}
		v = v.Elem()
		if v.Type().Kind() != reflect.Struct {
			panic("cmd: must pass a pointer to struct")
		}

		maxIndex := -1
		for j := 0; j < v.NumField(); j++ {
			tfield := v.Type().Field(j)
			vfield := v.Field(j)
			if vfield.IsValid() {
				variable := &Var{}
				variable.Value = vfield
				variable.Name = strings.ToLower(tfield.Name)
				variable.Index = -1

				if !isValidType(vfield.Type()) {
					panic(fmt.Sprintf("unsupported type %s", vfield.Type()))
				}

				name := tfield.Tag.Get("name")
				long, hasLong := tfield.Tag.Lookup("long")
				short := tfield.Tag.Get("short")
				index := tfield.Tag.Get("index")
				def, hasDef := tfield.Tag.Lookup("default")
				description := tfield.Tag.Get("desc")

				if name != "" {
					variable.Name = name
				}
				if !hasLong {
					variable.Long = variable.Name
				} else if long != "" {
					if !isValidName(long) {
						panic(fmt.Sprintf("invalid option name: --%v", long))
					} else if argp.findLong(long) != nil {
						panic(fmt.Sprintf("option name already exists: --%v", long))
					}
					variable.Long = strings.ToLower(long)
				}
				if short != "" {
					if !isValidName(short) {
						panic(fmt.Sprintf("invalid option name: --%v", short))
					}
					r, n := utf8.DecodeRuneInString(short)
					if len(short) != n || n == 0 {
						panic(fmt.Sprintf("option name must be one character: -%v", short))
					} else if argp.findShort(r) != nil {
						panic(fmt.Sprintf("option name already exists: -%v", string(r)))
					}
					variable.Short = r
				}
				if index != "" {
					if long != "" || short != "" {
						panic("can not set both an option name and index")
					}
					if index == "*" {
						if argp.findRest() != nil {
							panic("rest option already exists")
						} else if def != "" {
							panic("rest option can not have a default value")
						} else if variable.Value.Kind() != reflect.Slice || variable.Value.Type().Elem().Kind() != reflect.String {
							panic("rest option must be of type []string")
						}
						variable.Rest = true
					} else {
						i, err := strconv.Atoi(index)
						if err != nil || i < 0 {
							panic("index must be a non-negative integer or *")
						} else if argp.findIndex(i) != nil {
							panic(fmt.Sprintf("option index already exists: %v", i))
						}
						variable.Index = i
						if maxIndex < i {
							maxIndex = i
						}
					}
				}
				if hasDef {
					defVal := reflect.New(vfield.Type()).Elem()
					if _, err := ScanVar(defVal, []string{def}); err != nil {
						panic(fmt.Sprintf("default: %v", err))
					}
					variable.Default = defVal.Interface()
				}
				if description != "" {
					variable.Description = description
				}
				argp.vars = append(argp.vars, variable)
			}
		}
		optionalIndex := -1
		for i := 0; i <= maxIndex; i++ {
			if v := argp.findIndex(i); v == nil {
				panic(fmt.Sprintf("option indices must be continuous: index %v is missing", i))
			} else if v.Default != nil {
				if optionalIndex == -1 {
					optionalIndex = i
				}
			} else if optionalIndex != -1 {
				panic(fmt.Sprintf("required options cannot follow optional ones: index %v is required but %v is optional", i, optionalIndex))
			}
		}
	}
	if argp.findLong("help") == nil {
		if argp.findShort('h') == nil {
			argp.AddOpt(&argp.help, "h", "help", nil, "Help")
		} else {
			argp.AddOpt(&argp.help, "", "help", nil, "Help")
		}
	}
	return argp
}

// IsSet returns true if the option is set
func (argp *Argp) IsSet(name string) bool {
	if v := argp.findName(name); v != nil {
		return v.isSet
	}
	return false
}

// AddOpt adds an option
func (argp *Argp) AddOpt(dst interface{}, short, long string, def interface{}, description string) {
	v := reflect.ValueOf(dst)
	if _, ok := dst.(Scanner); !ok && v.Type().Kind() != reflect.Ptr {
		panic("dst: must pass pointer to variable or comply with argp.Scanner interface")
	} else if !ok {
		v = v.Elem()
	}

	variable := &Var{}
	variable.Value = v
	variable.Index = -1

	if !isValidType(v.Type()) {
		panic(fmt.Sprintf("unsupported type %s", v.Type()))
	} else if short == "" && long == "" {
		panic("must set short or long variable name")
	}

	if long != "" {
		if !isValidName(long) {
			panic(fmt.Sprintf("invalid option name: --%v", long))
		} else if argp.findLong(long) != nil {
			panic(fmt.Sprintf("option name already exists: --%v", long))
		}
		variable.Long = strings.ToLower(long)
	}
	if short != "" {
		if !isValidName(short) {
			panic(fmt.Sprintf("invalid option name: -%v", short))
		}
		r, n := utf8.DecodeRuneInString(short)
		if len(short) != n || n == 0 {
			panic(fmt.Sprintf("option name must be one character: -%v", short))
		} else if argp.findShort(r) != nil {
			panic(fmt.Sprintf("option name already exists: -%v", string(r)))
		}
		variable.Short = r
	}
	if def != nil {
		if _, ok := dst.(Setter); !ok && !reflect.ValueOf(def).CanConvert(v.Type()) {
			panic(fmt.Sprintf("default: expected type %v", v.Type()))
		}
		variable.Default = def
	}
	variable.Description = description
	argp.vars = append(argp.vars, variable)
}

// AddVal adds an indexed value
func (argp *Argp) AddVal(dst interface{}, def interface{}, description string) {
	v := reflect.ValueOf(dst)
	if _, ok := dst.(Scanner); !ok && v.Type().Kind() != reflect.Ptr {
		panic("dst: must pass pointer to variable or comply with argp.Scanner interface")
	} else if !ok {
		v = v.Elem()
	}

	variable := &Var{}
	variable.Value = v
	variable.Index = 0

	if !isValidType(v.Type()) {
		panic(fmt.Sprintf("unsupported type %s", v.Type()))
	}

	// find next free index
	for _, v := range argp.vars {
		if variable.Index <= v.Index {
			variable.Index = v.Index + 1
		}
	}

	if def != nil {
		if _, ok := dst.(Setter); !ok && !reflect.ValueOf(def).CanConvert(v.Type()) {
			panic(fmt.Sprintf("default: expected type %v", v.Type()))
		}
		variable.Default = def
	}
	variable.Description = description
	argp.vars = append(argp.vars, variable)
}

func (argp *Argp) AddRest(dst interface{}, name, description string) {
	v := reflect.ValueOf(dst)
	if _, ok := dst.(Scanner); !ok && v.Type().Kind() != reflect.Ptr {
		panic("dst: must pass pointer to variable or comply with argp.Scanner interface")
	} else if !ok {
		v = v.Elem()
	}

	variable := &Var{}
	variable.Value = v
	variable.Name = strings.ToLower(name)
	variable.Index = -1

	if argp.findRest() != nil {
		panic("rest option already exists")
	} else if v.Kind() != reflect.Slice || v.Type().Elem().Kind() != reflect.String {
		panic("rest option must be of type []string")
	}
	variable.Rest = true
	variable.Description = description
	argp.vars = append(argp.vars, variable)
}

// AddCmd adds a sub command
func (argp *Argp) AddCmd(cmd Cmd, name, description string) *Argp {
	if _, ok := argp.cmds[name]; ok {
		panic(fmt.Sprintf("command already exists: %v", name))
	} else if len(name) == 0 || name[0] == '-' {
		panic("invalid command name")
	}

	sub := NewCmd(cmd, description)
	sub.parent = argp
	sub.name = name
	argp.cmds[strings.ToLower(name)] = sub
	return sub
}

func wrapString(s string, cols int) (string, string) {
	if len(s) <= cols {
		return s, ""
	}
	minWidth := int(0.8*float64(cols) + 0.5)
	for i := cols; minWidth <= i; i-- {
		if s[i] == ' ' {
			return s[:i], s[i+1:]
		}
	}
	return s[:cols], s[cols:]
}

// PrintHelp prints the help overview. This is automatically called when unknown or bad options are passed, but you can call this explicitly in other cases.
func (argp *Argp) PrintHelp() {
	_, cols, _ := TerminalSize()

	base := argp.name
	parent := argp.parent
	for parent != nil {
		base = parent.name + " " + base
		parent = parent.parent
	}

	options := []*Var{}
	arguments := []*Var{}
	for _, v := range argp.vars {
		if v.IsArgument() {
			arguments = append(arguments, v)
		} else {
			options = append(options, v)
		}
	}
	sort.Slice(options, optionCmp(options))
	sort.Slice(arguments, argumentCmp(arguments))

	args := ""
	if 0 < len(options) {
		args += " [options]"
	}
	if 0 < len(argp.cmds) {
		fmt.Printf("Usage: %s%s [command] ...\n", base, args)
	}
	if 0 < len(arguments) {
		for _, v := range arguments {
			if !v.Rest {
				args += " [" + v.Long + "]"
			}
		}
		if rest := argp.findRest(); rest != nil {
			args += " " + rest.Name + "..."
		}
	}
	if 0 < len(arguments) || len(argp.cmds) == 0 {
		fmt.Printf("Usage: %s%s\n", base, args)
	}

	if 0 < len(options) {
		fmt.Printf("\nOptions:\n")
		nMax := 0
		types := []string{}
		for _, v := range options {
			n := 0
			if v.Short != 0 {
				n += 4
				if v.Long != "" {
					n += 4 + len(v.Long)
				}
			} else if v.Long != "" {
				n += 8 + len(v.Long)
			}

			var typename string
			if typenamer, ok := v.Value.Interface().(TypeNamer); ok {
				typename = typenamer.TypeName()
			} else {
				typename = TypeName(v.Value.Type())
			}
			types = append(types, typename)
			if typename != "" {
				n += 1 + len(typename)
			}

			if nMax < n {
				nMax = n
			}
		}
		if 28 < nMax {
			nMax = 28
		} else if nMax < 10 {
			nMax = 10
		}
		for i, v := range options {
			n := 0
			if v.Short != 0 {
				fmt.Printf("  -%s", string(v.Short))
				n += 4
				if v.Long != "" {
					fmt.Printf(", --%s", v.Long)
					n += 4 + len(v.Long)
				}
			} else if v.Long != "" {
				fmt.Printf("      --%s", v.Long)
				n += 8 + len(v.Long)
			}
			if types[i] != "" {
				fmt.Printf(" %s", types[i])
				n += 1 + len(types[i])
			}
			if nMax < n {
				fmt.Printf("\n")
				n = 0
			}
			fmt.Printf("%s  ", strings.Repeat(" ", nMax-n))

			desc := v.Description
			if v.Default != nil {
				desc += fmt.Sprintf(" (default: %v)", v.Default)
			}
			if cols < 60 {
				fmt.Printf("%s\n", desc)
			} else if 0 < len(desc) {
				n = nMax + 2
				for {
					var s string
					s, desc = wrapString(desc, cols-n)
					fmt.Printf("%s\n", s)
					if len(desc) == 0 {
						break
					}
					fmt.Printf(strings.Repeat(" ", n))
				}
			}
		}
	}

	if 0 < len(argp.cmds) {
		fmt.Printf("\nCommands:\n")
		nMax := 0
		cmds := []string{}
		for cmd, _ := range argp.cmds {
			if nMax < 2+len(cmd) {
				nMax = 2 + len(cmd)
			}
			cmds = append(cmds, cmd)
		}
		sort.Strings(cmds)

		if 28 < nMax {
			nMax = 28
		} else if nMax < 10 {
			nMax = 10
		}
		for _, cmd := range cmds {
			sub := argp.cmds[cmd]
			n := 2 + len(cmd)
			fmt.Printf("  %s", cmd)
			if nMax < n {
				fmt.Printf("\n")
				n = 0
			}
			fmt.Printf("%s  %s\n", strings.Repeat(" ", nMax-n), sub.Description)
		}
	}

	if 0 < len(arguments) {
		fmt.Printf("\nArguments:\n")
		nMax := 0
		for _, v := range options {
			n := 2 + len(v.Name)
			if nMax < n {
				nMax = n
			}
		}
		if 28 < nMax {
			nMax = 28
		} else if nMax < 10 {
			nMax = 10
		}
		for _, v := range arguments {
			n := 2 + len(v.Name)
			fmt.Printf("  %s", v.Name)
			if nMax < n {
				fmt.Printf("\n")
				n = 0
			}
			fmt.Printf("%s  %s\n", strings.Repeat(" ", nMax-n), v.Description)
		}
	}
}

// Parse parses the command line arguments and returns the remaining unparsed arguments. When the main command was instantiated with `NewCmd` instead, this command will not return and you need to catch the remaining arguments with `index="*"` in the struct tag.
func (argp *Argp) Parse() {
	sub, rest, err := argp.parse(os.Args[1:])
	if err != nil {
		fmt.Printf("%v\n\n", err)
		sub.PrintHelp()
		os.Exit(1)
	} else if sub.help || sub.Cmd == nil {
		sub.PrintHelp()
		os.Exit(0)
	} else {
		if len(rest) != 0 {
			msg := "unknown arguments"
			if len(rest) == 1 {
				msg = "unknown argument"
			}
			fmt.Printf("%s: %v\n\n", msg, strings.Join(rest, " "))
			sub.PrintHelp()
			os.Exit(1)
		} else if err := sub.Cmd.Run(); err != nil {
			if err == ShowUsage {
				sub.PrintHelp()
			} else if argp.Error != nil {
				argp.Error.Println(err)
			} else {
				fmt.Printf("ERROR: %v\n", err)
			}
			os.Exit(1)
		} else {
			os.Exit(0)
		}
	}
}

func (argp *Argp) findName(name string) *Var {
	if name == "" {
		return nil
	}
	for _, v := range argp.vars {
		if v.Name == name || v.Long == name {
			return v
		} else if v.Short != 0 && utf8.RuneCountInString(name) == 1 {
			r, _ := utf8.DecodeRuneInString(name)
			if r == v.Short {
				return v
			}
		}
	}
	return nil
}

func (argp *Argp) findShort(short rune) *Var {
	for _, v := range argp.vars {
		if v.Short != 0 && v.Short == short {
			return v
		}
	}
	return nil
}

func (argp *Argp) findLong(long string) *Var {
	long = strings.ToLower(long)
	for _, v := range argp.vars {
		if v.Long != "" && v.Long == long {
			return v
		}
	}
	return nil
}

func (argp *Argp) findIndex(index int) *Var {
	for _, v := range argp.vars {
		if v.Index == index {
			return v
		}
	}
	return nil
}

func (argp *Argp) findRest() *Var {
	for _, v := range argp.vars {
		if v.Rest {
			return v
		}
	}
	return nil
}

func (argp *Argp) parse(args []string) (*Argp, []string, error) {
	// sub commands
	if 0 < len(args) {
		for cmd, sub := range argp.cmds {
			if cmd == strings.ToLower(args[0]) {
				return sub.parse(args[1:])
			}
		}
	}

	// set defaults
	for _, v := range argp.vars {
		if v.Default != nil {
			if setter, ok := v.Value.Interface().(Setter); ok {
				if err := setter.Set(v.Default); err != nil {
					return argp, nil, fmt.Errorf("default: %v", err)
				}
			} else if ok := v.Set(v.Default); !ok {
				return argp, nil, fmt.Errorf("default: expected type %v", v.Value.Type())
			}
		}
	}

	rest := []string{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			rest = append(rest, args[i+1:]...)
			break
		}
		if 1 < len(arg) && arg[0] == '-' {
			if 1 < len(arg) && arg[1] == '-' {
				split := false
				s := args[i+1:]
				name := arg[2:]
				if idx := strings.IndexByte(arg, '='); idx != -1 {
					name = arg[2:idx]
					if idx+1 < len(arg) {
						s = append([]string{arg[idx+1:]}, args[i+1:]...)
						split = true
					}
				}

				indices := strings.Split(name, ".")
				v := argp.findLong(indices[0])
				if v == nil {
					return argp, nil, fmt.Errorf("unknown option --%s", name)
				}
				n, err := scanIndexedVar(v.Value, indices[1:], s)
				if err != nil {
					return argp, nil, fmt.Errorf("option --%s: %v", name, err)
				} else {
					i += n
					if split {
						i--
					}
				}
				v.isSet = true
			} else {
				for j := 1; j < len(arg); {
					name, n := utf8.DecodeRuneInString(arg[j:])
					j += n

					v := argp.findShort(name)
					if v == nil {
						return argp, nil, fmt.Errorf("unknown option -%c", name)
					} else if v.Value.Kind() == reflect.Bool {
						v.Set(true)
					} else {
						if j < len(arg) {
							hasEquals := arg[j] == '='
							if hasEquals {
								j++
							}
							value := arg[j:]
							n, err := ScanVar(v.Value, append([]string{value}, args[i+1:]...))
							if n == 0 {
								if hasEquals {
									return argp, nil, fmt.Errorf("option -%c: must not have value", name)
								}
								continue // can be of form: -abc
							}
							i += n - 1
							if err != nil {
								return argp, nil, fmt.Errorf("option -%c: %v", name, err)
							}
						} else {
							n, err := ScanVar(v.Value, args[i+1:])
							if n == 0 {
								continue // can be of form: -abc
							}
							i += n
							if err != nil {
								return argp, nil, fmt.Errorf("option -%c: %v", name, err)
							}
						}
						break
					}
					v.isSet = true
				}
			}
		} else if 0 < len(arg) {
			rest = append(rest, arg)
		}
	}

	// indexed arguments
	index := 0
	for _, arg := range rest {
		v := argp.findIndex(index)
		if v == nil {
			break
		}
		if _, err := ScanVar(v.Value, []string{arg}); err != nil {
			return argp, nil, fmt.Errorf("argument %d: %v", index, err)
		}
		v.isSet = true
		index++
	}
	for _, v := range argp.vars {
		if v.Index != -1 && index <= v.Index && v.Default == nil {
			return argp, nil, fmt.Errorf("argument %v is missing", v.Name)
		}
	}

	// rest arguments
	v := argp.findRest()
	rest = rest[index:]
	if v != nil {
		v.Set(rest)
		rest = rest[:0]
		v.isSet = true
	}
	return argp, rest, nil
}

var zeroValue = reflect.Value{}

func truncEnd(s []string) ([]string, []string, bool) {
	if len(s) == 0 {
		return []string{}, s, false
	}
	levels := []byte{}
	for n, item := range s {
		for i := 0; i < len(item); i++ {
			switch item[i] {
			case '{', '[':
				levels = append(levels, item[i]+2)
			case '}', ']':
				if len(levels) == 0 || levels[len(levels)-1] != item[i] {
					return nil, s, false // opening/closing brackets don't match, or too many closing
				} else if len(levels) == 1 {
					if i+1 == len(item) {
						return s[:n+1], s[n+1:], false
					}
					// split
					k := s[n:]
					s = append(s[:n:n], s[n][:i+1])
					k[0] = k[0][i+1:]
					return s, k, true
				}
				levels = levels[:len(levels)-1]
			}
		}
	}
	return nil, s, false // no closing bracket found
}

func scanIndexedVar(v reflect.Value, indices []string, s []string) (int, error) {
	if _, ok := v.Interface().(Scanner); ok {
		// implements Scanner
		return ScanVar(v, s)
	}

	if 0 < len(indices) {
		switch v.Kind() {
		case reflect.Array, reflect.Slice:
			index := 0
			if t := v.Type().Elem().Kind(); t == reflect.Int || t == reflect.Int8 || t == reflect.Int16 || t == reflect.Int32 || t == reflect.Int64 {
				i, err := strconv.ParseInt(indices[0], 10, 64)
				if err != nil {
					return 0, fmt.Errorf("index '%v': invalid integer", indices[0])
				}
				index = int(i)
			} else if t == reflect.Uint || t == reflect.Uint8 || t == reflect.Uint16 || t == reflect.Uint32 || t == reflect.Uint64 {
				i, err := strconv.ParseUint(indices[0], 10, 64)
				if err != nil {
					return 0, fmt.Errorf("index '%v': invalid positive integer", indices[0])
				}
				index = int(i)
			} else {
				return 0, fmt.Errorf("index '%v': unsupported type %v", indices[0], v.Type().Elem())
			}
			if v.IsNil() || index < 0 || v.Len() <= index {
				return 0, fmt.Errorf("index '%v': out of range", indices[0])
			}
			return scanIndexedVar(v.Field(index), indices[1:], s)
		case reflect.Map:
			key := reflect.ValueOf(indices[0])
			if t := v.Type().Key().Kind(); t == reflect.Int || t == reflect.Int8 || t == reflect.Int16 || t == reflect.Int32 || t == reflect.Int64 {
				i, err := strconv.ParseInt(indices[0], 10, 64)
				if err != nil {
					return 0, fmt.Errorf("index '%v': invalid integer", indices[0])
				}
				key = reflect.ValueOf(i).Convert(v.Type().Key())
			} else if t == reflect.Uint || t == reflect.Uint8 || t == reflect.Uint16 || t == reflect.Uint32 || t == reflect.Uint64 {
				i, err := strconv.ParseUint(indices[0], 10, 64)
				if err != nil {
					return 0, fmt.Errorf("index '%v': invalid positive integer", indices[0])
				}
				key = reflect.ValueOf(i).Convert(v.Type().Key())
			} else if t != reflect.String {
				return 0, fmt.Errorf("index '%v': unsupported type %v", indices[0], v.Type().Key())
			} else if v.IsNil() {
				v.Set(reflect.MakeMap(v.Type()))
			}
			field := reflect.New(v.Type().Elem()).Elem()
			n, err := scanIndexedVar(field, indices[1:], s)
			if err == nil {
				v.SetMapIndex(key, field)
			}
			return n, err
		case reflect.Struct:
			indices[0] = strings.Title(indices[0]) // TODO; deprecated
			field := v.FieldByName(indices[0])
			if field == zeroValue {
				return 0, fmt.Errorf("index '%v': missing field in struct", indices[0])
			}
			return scanIndexedVar(field, indices[1:], s)
		default:
			panic(fmt.Sprintf("index '%v': unsupported type %v", indices[0], v.Type())) // should never happen
		}
	}
	n, err := ScanVar(v, s)
	if err != nil && v.Kind() == reflect.Bool {
		v.SetBool(true)
		return 0, nil
	}
	return n, err
}

func ScanVar(v reflect.Value, s []string) (int, error) {
	if scanner, ok := v.Interface().(Scanner); ok {
		// implements Scanner
		return scanner.Scan(s)
	} else if len(s) == 0 {
		return 0, fmt.Errorf("missing value")
	}

	n := 0
	switch v.Kind() {
	case reflect.String:
		v.SetString(s[0])
		n++
	case reflect.Bool:
		i, err := strconv.ParseBool(s[0])
		if err != nil {
			return 0, fmt.Errorf("invalid boolean '%v'", s[0])
		}
		v.SetBool(i)
		n++
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		i, err := strconv.ParseInt(s[0], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid integer '%v'", s[0])
		}
		v.SetInt(i)
		n++
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		i, err := strconv.ParseUint(s[0], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid positive integer '%v'", s[0])
		}
		v.SetUint(i)
		n++
	case reflect.Float32, reflect.Float64:
		i, err := strconv.ParseFloat(s[0], 64)
		if err != nil {
			return 0, fmt.Errorf("invalid number '%v'", s[0])
		}
		v.SetFloat(i)
		n++
	case reflect.Array, reflect.Slice:
		var split, comma bool
		if len(s[0]) == 0 {
			return 0, fmt.Errorf("missing value")
		} else if s[0][0] != '[' {
			comma = true
		} else if s, _, split = truncEnd(s); s == nil || split {
			if v.Kind() == reflect.Slice {
				return 0, fmt.Errorf("invalid slice")
			}
			return 0, fmt.Errorf("invalid array")
		}
		n = len(s)
		if !comma {
			if len(s[0]) == 1 {
				s = s[1:]
			} else {
				s[0] = s[0][1:]
			}
			if len(s[len(s)-1]) == 1 {
				s = s[:len(s)-1]
			} else {
				s[len(s)-1] = s[len(s)-1][:len(s[len(s)-1])-1]
			}
		}

		j := 0
		slice := reflect.Zero(reflect.SliceOf(v.Type().Elem()))
		for {
			if j != 0 && comma {
				// consume comma
				for 0 < len(s) && len(s[0]) == 0 {
					s = s[1:]
				}
				if len(s) == 0 || s[0][0] != ',' {
					break
				} else if len(s[0]) == 1 {
					s = s[1:]
				} else {
					s[0] = s[0][1:]
				}
			}

			// consume value
			var sVal []string
			if len(s) == 0 {
				if !comma || j == 0 {
					break
				}
				// empty value after final comma
				sVal = []string{""}
			} else if 0 < len(s[0]) && (s[0][0] == '{' || s[0][0] == '[') {
				if comma {
					return 0, fmt.Errorf("index %v: invalid value", j)
				}
				sVal, s, split = truncEnd(s)
				if split {
					return 0, fmt.Errorf("index %v: invalid value", j)
				}
			} else if idx := strings.IndexByte(s[0], ','); idx != -1 && comma {
				sVal = []string{s[0][:idx]}
				s[0] = s[0][idx:]
			} else {
				sVal = []string{s[0]}
				s = s[1:]
			}
			val := reflect.New(v.Type().Elem()).Elem()
			if _, err := ScanVar(val, sVal); err != nil {
				return 0, fmt.Errorf("index %v: %v", j, err)
			}
			slice = reflect.Append(slice, val)
			j++
		}
		if v.Kind() == reflect.Array {
			if j != v.Len() {
				return 0, fmt.Errorf("expected %v values", v.Len())
			}
			v.Set(slice.Convert(v.Type()))
		} else {
			v.Set(slice)
		}
	case reflect.Map:
		var split bool
		if len(s[0]) == 0 || s[0][0] != '{' {
			return 0, fmt.Errorf("missing value")
		} else if s, _, split = truncEnd(s); s == nil || split {
			return 0, fmt.Errorf("invalid map")
		}
		n = len(s)
		if len(s[0]) == 1 {
			s = s[1:]
		} else {
			s[0] = s[0][1:]
		}
		if len(s[len(s)-1]) == 1 {
			s = s[:len(s)-1]
		} else {
			s[len(s)-1] = s[len(s)-1][:len(s[len(s)-1])-1]
		}

		for 0 < len(s) {
			// consume key
			var sKey []string
			if 0 < len(s[0]) && (s[0][0] == '{' || s[0][0] == '[') {
				sKey, s, _ = truncEnd(s)
				if len(s) == 0 || len(s[0]) == 0 || s[0][0] != ':' {
					return 0, fmt.Errorf("key '%v': missing semicolon", strings.Join(sKey, " "))
				}
				if len(s[0]) == 1 {
					s = s[1:]
				} else {
					s[0] = s[0][1:]
				}
			} else if idx := strings.IndexByte(s[0], ':'); idx == -1 {
				sKey = []string{s[0]}
				s = s[1:]
				for 0 < len(s) && len(s[0]) == 0 {
					s = s[1:]
				}
				if len(s) == 0 {
					break
				} else if s[0][0] != ':' {
					return 0, fmt.Errorf("key '%v': missing semicolon", strings.Join(sKey, " "))
				} else if len(s[0]) == 1 {
					s = s[1:]
				} else {
					s[0] = s[0][1:]
				}
			} else {
				sKey = []string{s[0][:idx]}
				s[0] = s[0][idx+1:]
			}
			key := reflect.New(v.Type().Key()).Elem()
			if _, err := ScanVar(key, sKey); err != nil {
				return 0, fmt.Errorf("key: %v", err)
			}

			// consume value
			index := strings.Join(sKey, " ")
			var sVal []string
			if len(s) == 0 {
				// empty value after semicolon
				sVal = []string{""}
			} else if 0 < len(s[0]) && (s[0][0] == '{' || s[0][0] == '[') {
				sVal, s, split = truncEnd(s)
				if split {
					return 0, fmt.Errorf("key '%v': invalid value", index)
				}
			} else {
				sVal = []string{s[0]}
				s = s[1:]
			}
			val := reflect.New(v.Type().Elem()).Elem()
			if _, err := ScanVar(val, sVal); err != nil {
				return 0, fmt.Errorf("key '%v': %v", index, err)
			}

			if v.IsNil() {
				v.Set(reflect.MakeMap(v.Type()))
			}
			v.SetMapIndex(key, val)
		}
	case reflect.Struct:
		var split bool
		if len(s[0]) == 0 || s[0][0] != '{' {
			return 0, fmt.Errorf("missing value")
		} else if s, _, split = truncEnd(s); s == nil || split {
			return 0, fmt.Errorf("invalid struct")
		}
		n = len(s)
		if len(s[0]) == 1 {
			s = s[1:]
		} else {
			s[0] = s[0][1:]
		}
		if len(s[len(s)-1]) == 1 {
			s = s[:len(s)-1]
		} else {
			s[len(s)-1] = s[len(s)-1][:len(s[len(s)-1])-1]
		}

		j := 0
		for j < v.NumField() {
			// consume value
			field := v.Type().Field(j).Name
			for 0 < len(s) && len(s[0]) == 0 {
				s = s[1:]
			}
			if len(s) == 0 {
				break
			}
			var sVal []string
			if s[0][0] == '{' || s[0][0] == '[' {
				sVal, s, split = truncEnd(s)
				if split {
					return 0, fmt.Errorf("field %v: invalid value", field)
				}
			} else {
				sVal = []string{s[0]}
				s = s[1:]
			}
			if _, err := ScanVar(v.Field(j), sVal); err != nil {
				return 0, fmt.Errorf("field %v: %v", field, err)
			}
			j++
		}
		if j != v.NumField() {
			return 0, fmt.Errorf("missing values")
		} else if len(s) != 0 {
			return 0, fmt.Errorf("too many values")
		}
	default:
		panic(fmt.Sprintf("unsupported type %v", v.Type())) // should never happen
	}
	return n, nil
}

func isValidName(s string) bool {
	for i, r := range s {
		if !(unicode.IsLetter(r) || unicode.IsNumber(r) || r == '-' && 0 < i || r == '_') {
			return false
		}
	}
	return true
}

func isValidType(t reflect.Type) bool {
	if t.Implements(reflect.TypeOf((*Scanner)(nil)).Elem()) {
		// implements Scanner
		return true
	}
	return isValidSubType(t)
}

func isValidSubType(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.String, reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Float32, reflect.Float64:
		return true
	case reflect.Array, reflect.Slice:
		return isValidSubType(t.Elem())
	case reflect.Map:
		return isValidSubType(t.Key()) && isValidSubType(t.Elem())
	case reflect.Struct:
		for i := 0; i < t.NumField(); i++ {
			if !isValidSubType(t.Field(i).Type) {
				return false
			}
		}
		return true
	}
	return false
}

func TypeName(t reflect.Type) string {
	k := t.Kind()
	if k == reflect.Int || k == reflect.Int8 || k == reflect.Int16 || k == reflect.Int32 || k == reflect.Int64 {
		return "int"
	} else if k == reflect.Uint || k == reflect.Uint8 || k == reflect.Uint16 || k == reflect.Uint32 || k == reflect.Uint64 {
		return "uint"
	} else if k == reflect.Float32 || k == reflect.Float64 {
		return "float"
	} else if k == reflect.Array || k == reflect.Slice {
		return "[]" + TypeName(t.Elem())
	} else if k == reflect.Map {
		return "map[" + TypeName(t.Key()) + "]" + TypeName(t.Elem())
	} else if k == reflect.String {
		return "string"
	} else if k == reflect.Struct {
		return "struct"
	}
	return ""
}

func optionCmp(vars []*Var) func(int, int) bool {
	return func(i, j int) bool {
		if vars[i].Short != 0 {
			if vars[j].Short != 0 {
				return vars[i].Short < vars[j].Short
			} else {
				return string(vars[i].Short) < vars[j].Long
			}
		} else if vars[j].Short != 0 {
			return vars[i].Long < string(vars[j].Short)
		}
		return vars[i].Long < vars[j].Long
	}
}

func argumentCmp(vars []*Var) func(int, int) bool {
	return func(i, j int) bool {
		if vars[i].Rest {
			return false
		} else if vars[j].Rest {
			return true
		}
		return vars[i].Index < vars[j].Index
	}
}
