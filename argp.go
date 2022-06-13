package argp

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Count is a counting option, e.g. -vvv sets count to 3
type Count int

// Var is a command option or argument
type Var struct {
	Value       reflect.Value
	Name        string
	Long        string
	Short       rune
	Index       int
	Rest        bool
	Default     interface{}
	Description string
}

// SetString sets the variable's value from a string
func (v *Var) SetString(s ...string) (int, error) {
	i, n, err := parseVar(v.Value.Type(), s)
	if err != nil {
		return n, err
	}
	v.Set(i)
	return n, nil
}

// Set sets the variable's value
func (v *Var) Set(i interface{}) {
	v.Value.Set(reflect.ValueOf(i).Convert(v.Value.Type()))
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
			panic("must pass pointer to struct")
		}
		v = v.Elem()
		if v.Type().Kind() != reflect.Struct {
			panic("must pass pointer to struct")
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
				def := tfield.Tag.Get("default")
				description := tfield.Tag.Get("desc")

				if name != "" {
					variable.Name = name
				}
				if !hasLong {
					variable.Long = variable.Name
				} else if long != "" {
					if !isValidName(long) {
						panic("option names must be unicode letters or numbers")
					} else if argp.findLong(long) != nil {
						panic(fmt.Sprintf("long option name already exists: --%v", long))
					}
					variable.Long = strings.ToLower(long)
				}
				if short != "" {
					if !isValidName(short) {
						panic("option names must be unicode letters or numbers")
					}
					r, n := utf8.DecodeRuneInString(short)
					if len(short) != n || n == 0 {
						panic("short option names must be one character long")
					} else if argp.findShort(r) != nil {
						panic(fmt.Sprintf("short option name already exists: -%v", string(r)))
					}
					variable.Short = r
				}
				if index != "" {
					if long != "" || short != "" {
						panic("can not set both long/short option names and index")
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
				if def != "" {
					iDef, _, err := parseVar(variable.Value.Type(), []string{def})
					if err != nil {
						panic(fmt.Sprintf("bad option default: %v", err))
					}
					variable.Default = iDef
				}
				if description != "" {
					variable.Description = description
				}
				argp.vars = append(argp.vars, variable)
			}
		}
		for i := 0; i <= maxIndex; i++ {
			if argp.findIndex(i) == nil {
				panic(fmt.Sprintf("option indices must be continuous: index %v is missing", i))
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

// AddOpt adds an option
func (argp *Argp) AddOpt(i interface{}, short, long string, def interface{}, description string) {
	v := reflect.ValueOf(i)
	if v.Type().Kind() != reflect.Ptr {
		panic("must pass pointer")
	}
	v = v.Elem()

	variable := &Var{}
	variable.Value = v
	variable.Index = -1

	if !isValidType(v.Type()) {
		panic(fmt.Sprintf("unsupported type %s", v.Type()))
	}

	if long != "" {
		if !isValidName(long) {
			panic("option names must be unicode letters or numbers")
		} else if argp.findLong(long) != nil {
			panic(fmt.Sprintf("long option name already exists: --%v", long))
		}
		variable.Long = strings.ToLower(long)
	}
	if short != "" {
		if !isValidName(short) {
			panic("option names must be unicode letters or numbers")
		}
		r, n := utf8.DecodeRuneInString(short)
		if len(short) != n || n == 0 {
			panic("short option names must be one character long")
		} else if argp.findShort(r) != nil {
			panic(fmt.Sprintf("short option name already exists: -%v", string(r)))
		}
		variable.Short = r
	}
	if def != nil {
		variable.Default = def
	}
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

// PrintHelp prints the help overview. This is automatically called when unknown or bad options are passed, but you can call this explicitly in other cases.
func (argp *Argp) PrintHelp() {
	base := argp.name
	parent := argp.parent
	for parent != nil {
		base = parent.name + " " + base
		parent = parent.parent
	}

	options := []*Var{}
	arguments := []*Var{}
	for _, v := range argp.vars {
		if v.Index != -1 || v.Rest {
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
	} else {
		for _, v := range arguments {
			if v.Rest {
				args += " [" + v.Long + "...]"
			} else {
				args += " [" + v.Long + "]"
			}
		}
		fmt.Printf("Usage: %s%s\n", base, args)
	}

	if 0 < len(options) {
		fmt.Printf("\nOptions:\n")
		nMax := 0
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
			if nMax < n {
				nMax = n
			}
		}
		if 28 < nMax {
			nMax = 28
		} else if nMax < 10 {
			nMax = 10
		}
		for _, v := range options {
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
			if nMax < n {
				fmt.Printf("\n")
				n = 0
			}
			fmt.Printf("%s  %s\n", strings.Repeat(" ", nMax-n), v.Description)
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
		fmt.Printf("\n")
	} else if 0 < len(arguments) {
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
func (argp *Argp) Parse() []string {
	sub, rest, err := argp.parse(os.Args[1:])
	if err != nil {
		fmt.Printf("%v\n\n", err)
		sub.PrintHelp()
		os.Exit(1)
	} else if sub.help {
		sub.PrintHelp()
		os.Exit(0)
	} else if sub.Cmd != nil {
		if len(rest) != 0 {
			msg := "unknown arguments"
			if len(rest) == 1 {
				msg = "unknown argument"
			}
			fmt.Printf("%s: %v\n\n", msg, strings.Join(rest, " "))
			sub.PrintHelp()
			os.Exit(1)
		} else if err := sub.Cmd.Run(); err != nil {
			fmt.Printf("%v\n\n", err)
			os.Exit(1)
		} else {
			os.Exit(0)
		}
	}
	return rest
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

	// set default
	for _, v := range argp.vars {
		if v.Default != nil {
			v.Set(v.Default)
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
				name := arg[2:]
				if idx := strings.IndexByte(name, '='); idx != -1 {
					value := name[idx+1:]
					name = name[:idx]

					v := argp.findLong(name)
					if v == nil {
						return argp, nil, fmt.Errorf("unknown option --%s", name)
					}
					n, err := v.SetString(append([]string{value}, args[i+1:]...)...)
					i += n - 1
					if err != nil {
						return argp, nil, fmt.Errorf("option --%s: %v", name, err)
					}
				} else {
					v := argp.findLong(name)
					if v == nil {
						return argp, nil, fmt.Errorf("unknown option --%s", name)
					} else if v.Value.Kind() == reflect.Bool {
						if _, err := v.SetString("true"); err != nil {
							return argp, nil, fmt.Errorf("option --%s: %v", name, err)
						}
					} else if len(args) <= i+1 {
						return argp, nil, fmt.Errorf("option --%s: value is missing", name)
					} else {
						n, err := v.SetString(args[i+1:]...)
						i += n
						if err != nil {
							return argp, nil, fmt.Errorf("option --%s: %v", name, err)
						}
					}
				}
			} else {
				for j := 1; j < len(arg); {
					name, n := utf8.DecodeRuneInString(arg[j:])
					j += n

					v := argp.findShort(name)
					if v == nil {
						return argp, nil, fmt.Errorf("unknown option -%c", name)
					} else if v.Value.Kind() == reflect.Bool {
						if _, err := v.SetString("true"); err != nil {
							return argp, nil, fmt.Errorf("option -%c: %v", name, err)
						}
					} else if v.Value.Type() == reflect.TypeOf(Count(0)) && (j < len(arg) && arg[j] != '=' && (arg[j] < '0' || '9' < arg[j]) || j == len(arg) && (i+1 == len(args) || len(args[i+1]) == 0 || args[i+1][0] < '0' || '9' < args[i+1][0])) {
						v.Value.SetInt(v.Value.Int() + 1)
					} else {
						if j < len(arg) {
							if arg[j] == '=' {
								j++
							}
							value := arg[j:]
							n, err := v.SetString(append([]string{value}, args[i+1:]...)...)
							i += n - 1
							if err != nil {
								return argp, nil, fmt.Errorf("option -%c: %v", name, err)
							}
						} else if len(args) <= i+1 {
							return argp, nil, fmt.Errorf("option -%c: value is missing", name)
						} else {
							n, err := v.SetString(args[i+1:]...)
							i += n
							if err != nil {
								return argp, nil, fmt.Errorf("option -%c: %v", name, err)
							}
						}
						break
					}
				}
			}
		} else if 0 < len(arg) {
			rest = append(rest, arg)
		}
	}

	if 0 < len(rest) {
		// indexed arguments
		for index, arg := range rest {
			v := argp.findIndex(index)
			if v == nil {
				rest = rest[index:]
				break
			}
			if _, err := v.SetString(arg); err != nil {
				return argp, nil, fmt.Errorf("argument %d: %v", index, err)
			}
		}

		// rest arguments
		v := argp.findRest()
		if v != nil {
			v.Set(rest)
			rest = rest[:0]
		}
	}
	return argp, rest, nil
}

func parseVar(t reflect.Type, s []string) (interface{}, int, error) {
	switch t.Kind() {
	case reflect.String:
		return s[0], 1, nil
	case reflect.Bool:
		i, err := strconv.ParseBool(s[0])
		if err != nil {
			return nil, 0, fmt.Errorf("invalid boolean: %v", s[0])
		}
		return i, 1, nil
	case reflect.Int:
		i, err := strconv.ParseInt(s[0], 10, 0)
		if err != nil {
			return nil, 0, fmt.Errorf("invalid integer: %v", s[0])
		}
		return i, 1, nil
	case reflect.Int8:
		i, err := strconv.ParseInt(s[0], 10, 8)
		if err != nil {
			return nil, 0, fmt.Errorf("invalid integer: %v", s[0])
		}
		return i, 1, nil
	case reflect.Int16:
		i, err := strconv.ParseInt(s[0], 10, 16)
		if err != nil {
			return nil, 0, fmt.Errorf("invalid integer: %v", s[0])
		}
		return i, 1, nil
	case reflect.Int32:
		i, err := strconv.ParseInt(s[0], 10, 32)
		if err != nil {
			return nil, 0, fmt.Errorf("invalid integer: %v", s[0])
		}
		return i, 1, nil
	case reflect.Int64:
		i, err := strconv.ParseInt(s[0], 10, 64)
		if err != nil {
			return nil, 0, fmt.Errorf("invalid integer: %v", s[0])
		}
		return i, 1, nil
	case reflect.Uint:
		i, err := strconv.ParseUint(s[0], 10, 0)
		if err != nil {
			return nil, 0, fmt.Errorf("invalid positive integer: %v", s[0])
		}
		return i, 1, nil
	case reflect.Uint8:
		i, err := strconv.ParseUint(s[0], 10, 8)
		if err != nil {
			return nil, 0, fmt.Errorf("invalid positive integer: %v", s[0])
		}
		return i, 1, nil
	case reflect.Uint16:
		i, err := strconv.ParseUint(s[0], 10, 16)
		if err != nil {
			return nil, 0, fmt.Errorf("invalid positive integer: %v", s[0])
		}
		return i, 1, nil
	case reflect.Uint32:
		i, err := strconv.ParseUint(s[0], 10, 32)
		if err != nil {
			return nil, 0, fmt.Errorf("invalid positive integer: %v", s[0])
		}
		return i, 1, nil
	case reflect.Uint64:
		i, err := strconv.ParseUint(s[0], 10, 64)
		if err != nil {
			return nil, 0, fmt.Errorf("invalid positive integer: %v", s[0])
		}
		return i, 1, nil
	case reflect.Float32:
		i, err := strconv.ParseFloat(s[0], 32)
		if err != nil {
			return nil, 0, fmt.Errorf("invalid number: %v", s[0])
		}
		return i, 1, nil
	case reflect.Float64:
		i, err := strconv.ParseFloat(s[0], 64)
		if err != nil {
			return nil, 0, fmt.Errorf("invalid number: %v", s[0])
		}
		return i, 1, nil
	case reflect.Array:
		k := 0
		v := reflect.New(t).Elem()
		for j := 0; j < t.Len(); j++ {
			if len(s) <= k {
				return nil, 0, fmt.Errorf("missing values")
			}
			i, n, err := parseVar(t.Elem(), s[k:])
			if err != nil {
				return nil, 0, err
			}
			v.Index(j).Set(reflect.ValueOf(i).Convert(t.Elem()))
			k += n
		}
		return v.Interface(), k, nil
	case reflect.Slice:
		k := 0
		slice := []interface{}{}
		for k < len(s) {
			i, n, err := parseVar(t.Elem(), s[k:])
			if err != nil {
				if k == 0 {
					return nil, 0, err
				}
				break
			}
			slice = append(slice, i)
			k += n
		}
		if len(slice) == 0 {
			return nil, 0, fmt.Errorf("missing values")
		}
		v := reflect.MakeSlice(t, len(slice), len(slice))
		for j, i := range slice {
			v.Index(j).Set(reflect.ValueOf(i).Convert(t.Elem()))
		}
		return v.Interface(), k, nil
	case reflect.Struct:
		k := 0
		v := reflect.New(t).Elem()
		for j := 0; j < t.NumField(); j++ {
			if len(s) <= k {
				return nil, 0, fmt.Errorf("missing values")
			}
			i, n, err := parseVar(t.Field(j).Type, s[k:])
			if err != nil {
				return nil, 0, err
			}
			v.Field(j).Set(reflect.ValueOf(i).Convert(t.Field(j).Type))
			k += n
		}
		return v.Interface(), k, nil

	}
	panic(fmt.Sprintf("unsupported type %s", t)) // should never happen
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
	if t == reflect.TypeOf(Count(0)) {
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
