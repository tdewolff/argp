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

// NewCmd returns a new command parser that invokes the Run method of the passed command structure. The `Argp.Parse()` function will not return and will call os.Exit() with 0, 1 or 2 as the argument.
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
				variable.Name = fromFieldname(tfield.Name)
				variable.Index = -1
				option := reflect.TypeOf(cmd).String() + "." + tfield.Name

				if !isValidType(vfield.Type()) {
					panic(fmt.Sprintf("unsupported type %s", vfield.Type()))
				}

				name, hasName := tfield.Tag.Lookup("name")
				short := tfield.Tag.Get("short")
				index := tfield.Tag.Get("index")
				def, hasDef := tfield.Tag.Lookup("default")
				description := tfield.Tag.Get("desc")

				if hasName {
					variable.Name = strings.ToLower(name)
				}
				if variable.Name == "" {
					variable.Name = short
				}
				if !isValidName(variable.Name) {
					panic(fmt.Sprintf("%v: invalid option name: --%v", option, variable.Name))
				} else if argp.findName(variable.Name) != nil {
					panic(fmt.Sprintf("%v: option name already exists: --%v", option, variable.Name))
				}
				if short != "" {
					if !isValidName(short) {
						panic(fmt.Sprintf("%v: invalid short option name: --%v", option, short))
					}
					r, n := utf8.DecodeRuneInString(short)
					if len(short) != n || n == 0 {
						panic(fmt.Sprintf("%v: short option name must be one character: -%v", option, short))
					} else if argp.findShort(r) != nil {
						panic(fmt.Sprintf("%v: short option name already exists: -%v", option, string(r)))
					}
					variable.Short = r
				}
				if index != "" {
					if short != "" {
						panic(fmt.Sprintf("%v: can not set both an option short name and index", option))
					}
					if index == "*" {
						if argp.findRest() != nil {
							panic(fmt.Sprintf("%v: rest option already exists", option))
						} else if def != "" {
							panic(fmt.Sprintf("%v: rest option can not have a default value", option))
						} else if variable.Value.Kind() != reflect.Slice || variable.Value.Type().Elem().Kind() != reflect.String {
							panic(fmt.Sprintf("%v: rest option must be of type []string", option))
						}
						variable.Rest = true
					} else {
						i, err := strconv.Atoi(index)
						if err != nil || i < 0 {
							panic(fmt.Sprintf("%v: index must be a non-negative integer or *", option))
						} else if argp.findIndex(i) != nil {
							panic(fmt.Sprintf("%v: option index already exists: %v", option, i))
						}
						variable.Index = i
						if maxIndex < i {
							maxIndex = i
						}
					}
				}
				if hasDef {
					defVal := reflect.New(vfield.Type()).Elem()
					if _, err := scanVar(defVal, "", splitArguments(def)); err != nil {
						panic(fmt.Sprintf("%v: bad default value: %v", option, err))
					}
					variable.Default = defVal.Interface()
				} else if variable.Index != -1 {
					variable.Default = vfield.Interface()
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
	if argp.findName("help") == nil {
		if argp.findShort('h') == nil {
			argp.AddOpt(&argp.help, "h", "help", "Help")
		} else {
			argp.AddOpt(&argp.help, "", "help", "Help")
		}
	}
	return argp
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

// IsSet returns true if the option is set
func (argp *Argp) IsSet(name string) bool {
	if v := argp.findName(name); v != nil {
		return v.isSet
	}
	return false
}

// AddOpt adds an option
func (argp *Argp) AddOpt(dst interface{}, short, name string, description string) {
	v := reflect.ValueOf(dst)
	_, isCustom := dst.(Custom)
	if !isCustom && v.Type().Kind() != reflect.Ptr {
		panic("dst: must pass pointer to variable or comply with argp.Custom interface")
	} else if !isCustom {
		v = v.Elem()
	}

	variable := &Var{}
	variable.Value = v
	variable.Index = -1

	if !isValidType(v.Type()) {
		panic(fmt.Sprintf("unsupported type %s", v.Type()))
	} else if name == "" {
		name = short
		if name == "" {
			panic("must set option name")
		}
	}

	if !isValidName(name) {
		panic(fmt.Sprintf("invalid option name: --%v", name))
	} else if argp.findName(name) != nil {
		panic(fmt.Sprintf("option name already exists: --%v", name))
	}
	variable.Name = strings.ToLower(name)
	if short != "" {
		if !isValidName(short) {
			panic(fmt.Sprintf("invalid short option name: -%v", short))
		}
		r, n := utf8.DecodeRuneInString(short)
		if len(short) != n || n == 0 {
			panic(fmt.Sprintf("short option name must be one character: -%v", short))
		} else if argp.findShort(r) != nil {
			panic(fmt.Sprintf("short option name already exists: -%v", string(r)))
		}
		variable.Short = r
	}
	if !isCustom {
		variable.Default = v.Interface()
	}
	variable.Description = description
	argp.vars = append(argp.vars, variable)
}

// AddArg adds an indexed value
func (argp *Argp) AddArg(dst interface{}, name, description string) {
	v := reflect.ValueOf(dst)
	_, isCustom := dst.(Custom)
	if !isCustom && v.Type().Kind() != reflect.Ptr {
		panic("dst: must pass pointer to variable or comply with argp.Custom interface")
	} else if !isCustom {
		v = v.Elem()
	}

	variable := &Var{}
	variable.Value = v
	variable.Name = strings.ToLower(name)
	variable.Index = 0
	if !isValidType(v.Type()) {
		panic(fmt.Sprintf("unsupported type %s", v.Type()))
	}
	for _, v := range argp.vars {
		// find next free index
		if variable.Index <= v.Index {
			variable.Index = v.Index + 1
		}
	}
	if !isCustom {
		variable.Default = v.Interface()
	}
	variable.Description = description
	argp.vars = append(argp.vars, variable)
}

func (argp *Argp) AddRest(dst interface{}, name, description string) {
	v := reflect.ValueOf(dst)
	_, isCustom := dst.(Custom)
	if !isCustom && v.Type().Kind() != reflect.Ptr {
		panic("dst: must pass pointer to variable or comply with argp.Custom interface")
	} else if !isCustom {
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
	if !isCustom {
		variable.Default = v.Interface()
	}
	variable.Description = description
	argp.vars = append(argp.vars, variable)
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

type optionHelp struct {
	short, name, typ, desc string
}

func appendStructHelps(helps []optionHelp, root string, v reflect.Value) []optionHelp {
	for i := 0; i < v.NumField(); i++ {
		field := v.Type().Field(i)
		name := root + "."
		if tagName := field.Tag.Get("name"); tagName != "" {
			name += tagName
		} else if tagShort := field.Tag.Get("short"); tagShort != "" {
			name += tagShort
		} else {
			name += fromFieldname(field.Name)
		}
		if field.Type.Kind() == reflect.Struct {
			helps = appendStructHelps(helps, name, v.Field(i))
		} else {
			if deflt := v.Field(i); !deflt.IsZero() {
				val := fmt.Sprintf("%v", deflt)
				if space := strings.IndexByte(val, ' '); space != -1 {
					val = "'" + val + "'"
				}
				name += "=" + val
			}
			typ := TypeName(field.Type)
			desc := field.Tag.Get("desc")
			helps = append(helps, optionHelp{
				short: "",
				name:  name,
				typ:   typ,
				desc:  desc,
			})
		}
	}
	return helps
}

func getOptionHelps(vs []*Var) []optionHelp {
	helps := []optionHelp{}
	for _, v := range vs {
		var val, typ string
		if custom, ok := v.Value.Interface().(Custom); ok {
			val, typ = custom.Help()
		} else if v.Value.Kind() == reflect.Struct {
			helps = appendStructHelps(helps, v.Name, v.Value)
			continue
		} else {
			if v.Default != nil && !reflect.ValueOf(v.Default).IsZero() {
				val = fmt.Sprint(v.Default)
			}
			typ = TypeName(v.Value.Type())
		}

		var short, name string
		if v.Short != 0 {
			short = string(v.Short)
		}
		name = v.Name
		if val != "" {
			if space := strings.IndexByte(val, ' '); space != -1 {
				val = "'" + val + "'"
			}
			if name != "" {
				name += "=" + val
			} else {
				short += "=" + val
			}
		}
		helps = append(helps, optionHelp{
			short: short,
			name:  name,
			typ:   typ,
			desc:  v.Description,
		})

	}
	return helps
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
	sort.Slice(options, sortOption(options))
	sort.Slice(arguments, sortArgument(arguments))

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
				args += " " + v.Name
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
		optionHelps := getOptionHelps(options)

		fmt.Printf("\nOptions:\n")
		nMax := 0
		for _, o := range optionHelps {
			n := 0
			if o.short != "" {
				n += 4
				if o.name != "" {
					n += 4 + len(o.name)
				}
			} else if o.name != "" {
				n += 8 + len(o.name)
			}
			if o.typ != "" {
				n += 1 + len(o.typ)
			}
			n++ // whitespace before description
			if nMax < n {
				nMax = n
			}
		}
		if 30 < nMax {
			nMax = 30
		} else if nMax < 10 {
			nMax = 10
		}
		for _, o := range optionHelps {
			n := 0
			if o.short != "" {
				fmt.Printf("  -%s, --%s", o.short, o.name)
				n += 8 + len(o.name)
			} else if o.name != "" {
				fmt.Printf("      --%s", o.name)
				n += 8 + len(o.name)
			}
			if o.typ != "" {
				fmt.Printf(" %s", o.typ)
				n += 1 + len(o.typ)
			}
			if nMax <= n {
				fmt.Printf("\n")
				n = 0
			}
			fmt.Printf("%s", strings.Repeat(" ", nMax-n))
			if cols < 60 {
				fmt.Printf("%s\n", o.desc)
			} else if 0 < len(o.desc) {
				n = nMax
				for {
					var s string
					s, o.desc = wrapString(o.desc, cols-n)
					fmt.Printf("%s\n", s)
					if len(o.desc) == 0 {
						break
					}
					fmt.Print(strings.Repeat(" ", n))
				}
			} else {
				fmt.Printf("\n")
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
		for _, v := range arguments {
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

// Parse parses the command line arguments. When the main command was instantiated with `NewCmd`, this command will exit.
func (argp *Argp) Parse() {
	cmd, rest, err := argp.parse(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n\n", err)
		cmd.PrintHelp()
		os.Exit(2)
	} else if cmd.help || cmd != argp && cmd.Cmd == nil {
		cmd.PrintHelp()
		os.Exit(0)
	} else if cmd.Cmd != nil {
		if len(rest) != 0 {
			msg := "unknown arguments"
			if len(rest) == 1 {
				msg = "unknown argument"
			}
			fmt.Fprintf(os.Stderr, "%s: %v\n\n", msg, strings.Join(rest, " "))
			cmd.PrintHelp()
			os.Exit(2)
		} else if err := cmd.Cmd.Run(); err != nil {
			// Exit with status 2 on bad usage and with status 1 when we don't know the nature of the error.
			if err == ShowUsage {
				cmd.PrintHelp()
			} else if argp.Error != nil {
				argp.Error.Println(err)
			} else {
				fmt.Fprintf(os.Stderr,"ERROR: %v\n", err)
				os.Exit(1)
			}
			os.Exit(2)
		} else {
			os.Exit(0)
		}
	}
}

func (argp *Argp) findShort(short rune) *Var {
	for _, v := range argp.vars {
		if v.Short != 0 && v.Short == short {
			return v
		}
	}
	return nil
}

func (argp *Argp) findName(name string) *Var {
	if name == "" {
		return nil
	}
	name = strings.ToLower(name)
	if i := strings.IndexAny(name, ".["); i != -1 {
		name = name[:i]
	}
	for _, v := range argp.vars {
		if v.Name == name || v.Name == "" && string(v.Short) == name {
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
			if ok := v.Set(v.Default); !ok {
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

				v := argp.findName(name)
				if v == nil {
					return argp, nil, fmt.Errorf("unknown option --%s", name)
				}
				n, err := scanVar(v.Value, name, s)
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
					} else {
						s := append([]string{arg[j:]}, args[i+1:]...)
						hasEquals := j < len(arg) && arg[j] == '='
						if hasEquals {
							s[0] = s[0][1:]
						}
						valueGlued := 0 < len(s[0])
						if !valueGlued {
							s = s[1:]
						}
						n, err := scanVar(v.Value, string(name), s)
						if err != nil {
							return argp, nil, fmt.Errorf("option -%c: %v", name, err)
						} else if n == 0 {
							continue // can be of the form -abc
						}
						if valueGlued {
							n--
						}
						i += n
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
		if _, err := scanVar(v.Value, "", []string{arg}); err != nil {
			return argp, nil, fmt.Errorf("argument %d: %v", index, err)
		}
		v.isSet = true
		index++
	}
	for _, v := range argp.vars {
		if v.Index != -1 && index <= v.Index && v.Default == nil {
			name := v.Name
			if name == "" {
				name = strconv.Itoa(index)
			}
			return argp, nil, fmt.Errorf("argument %v is missing", name)
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

// scanVar parses a slice of strings into the given value.
func scanVar(v reflect.Value, name string, s []string) (int, error) {
	if scanner, ok := v.Interface().(Custom); ok {
		// implements Custom
		return scanner.Scan(name, s)
	}

	if i := strings.IndexAny(name, ".["); i != -1 {
		j := -1
		origName := name
		if name[i] == '.' {
			j = strings.IndexAny(name[i+1:], ".[")
			if j == -1 {
				j = len(name)
			} else {
				j += i + 1
			}
			name = name[i+1 : j]
		} else {
			j = strings.IndexByte(name[i+1:], ']')
			if j == -1 {
				return 0, fmt.Errorf("expected terminating ] in variable index")
			}
			j += i + 2
			name = name[i+1 : j-1]
		}
		rest := origName[j:]

		switch v.Kind() {
		case reflect.Array, reflect.Slice:
			typ := "array"
			if v.Kind() == reflect.Slice {
				typ = "slice"
			}

			var index int
			if _, err := scanValue(reflect.ValueOf(&index).Elem(), []string{name}); err != nil {
				return 0, fmt.Errorf("%v index %v: %v", typ, name, err)
			} else if v.Kind() == reflect.Slice && v.IsNil() || index < 0 || v.Len() <= index {
				// TODO: slice expand range?
				return 0, fmt.Errorf("%v index %v: out of range", typ, index)
			}
			return scanVar(v.Index(index), rest, s)
		case reflect.Map:
			key := reflect.New(v.Type().Key()).Elem()
			if _, err := scanValue(key, []string{name}); err != nil {
				return 0, fmt.Errorf("map key %v: %v", name, err)
			} else if v.IsNil() {
				v.Set(reflect.MakeMap(v.Type()))
			}
			field := reflect.New(v.Type().Elem()).Elem()
			n, err := scanVar(field, rest, s)
			if err == nil {
				v.SetMapIndex(key, field)
			}
			return n, err
		case reflect.Struct:
			t := v.Type()
			found := false
			for i := 0; i < t.NumField(); i++ {
				if t.Field(i).Tag.Get("name") == name || t.Field(i).Tag.Get("short") == name || fromFieldname(t.Field(i).Name) == name {
					name = t.Field(i).Name
					found = true
					break
				}
			}
			if !found {
				name = toFieldname(name)
			}
			field := v.FieldByName(name)
			zero := reflect.Value{}
			if field == zero {
				return 0, fmt.Errorf("struct field: missing field %v in struct", name)
			}
			return scanVar(field, rest, s)
		default:
			return 0, fmt.Errorf("unexpected %v in name %v", string(origName[i]), origName)
		}
	}

	n, err := scanValue(v, s)
	if err != nil && v.Kind() == reflect.Bool {
		v.SetBool(true)
		return 0, nil
	}
	return n, err
}

// truncEnd splits the arguments and returns values for an array/slice/map/struct and remaining
// the first value is nil when either brackets don't match or closing brackets are missing
// the last return value indicates if the closing bracket was in the middle of an item (bad syntax)
func truncEnd(s []string) ([]string, []string, bool) {
	if len(s) == 0 {
		return []string{}, s, false
	}
	levels := []byte{}
	for n, item := range s {
		for i := 0; i < len(item); i++ {
			switch item[i] {
			case '{', '[':
				levels = append(levels, item[i]+2) // + 2 to get the closing bracket
			case '}', ']':
				if len(levels) == 0 || levels[len(levels)-1] != item[i] {
					return nil, s, false // open/close brackets don't match, or too many closes
				} else if len(levels) == 1 {
					if i+1 == len(item) {
						return s[:n+1], s[n+1:], false
					}
					// split last item
					k := s[n:]
					s = append(s[:n:n], s[n][:i+1])
					k[0] = k[0][i+1:]
					return s, k, true
				}
				levels = levels[:len(levels)-1]
			}
		}
	}
	if len(levels) == 0 {
		// there were no brackets to begin with
		return s[:1], s[1:], false
	}
	return nil, s, false // no closing bracket found
}

func scanValue(v reflect.Value, s []string) (int, error) {
	if len(s) == 0 {
		if v.Kind() == reflect.String {
			v.SetString("")
			return 0, nil
		}
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
		typ := "array"
		if v.Kind() == reflect.Slice {
			typ = "slice"
		}

		var split, comma bool
		if len(s[0]) == 0 {
			return 1, nil
		} else if s[0][0] != '[' {
			comma = true
		} else if s, _, split = truncEnd(s); s == nil || split {
			return 0, fmt.Errorf("invalid %v", typ)
		} else {
			// !comma
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
		}

		j := 0
		slice := reflect.Zero(reflect.SliceOf(v.Type().Elem()))
		if v.Kind() == reflect.Slice {
			slice = reflect.Value(v)
		}
		for {
			if j != 0 && comma {
				// consume comma
				for 0 < len(s) && len(s[0]) == 0 {
					s = s[1:]
					n++
				}
				if len(s) == 0 || s[0][0] != ',' {
					break
				} else if len(s[0]) == 1 {
					s = s[1:]
					n++
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
					return 0, fmt.Errorf("%v index %v: invalid value", typ, j)
				}
				sVal, s, split = truncEnd(s)
				if sVal == nil || split {
					return 0, fmt.Errorf("%v index %v: invalid value", typ, j)
				}
			} else if idx := strings.IndexByte(s[0], ','); idx != -1 && comma {
				sVal = []string{s[0][:idx]}
				s[0] = s[0][idx:]
			} else {
				sVal = []string{s[0]}
				s = s[1:]
				if comma {
					n++
				}
			}
			val := reflect.New(v.Type().Elem()).Elem()
			if _, err := scanValue(val, sVal); err != nil {
				return 0, fmt.Errorf("%v index %v: %v", typ, j, err)
			}
			slice = reflect.Append(slice, val)
			j++
		}
		if v.Kind() == reflect.Array {
			if j != v.Len() {
				return 0, fmt.Errorf("expected %v values for %v", v.Len(), typ)
			}
			v.Set(slice.Convert(v.Type()))
		} else {
			v.Set(slice)
		}
	case reflect.Map:
		var split bool
		if len(s[0]) == 0 {
			return 1, nil
		} else if s[0][0] != '{' {
			return 0, fmt.Errorf("invalid map")
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
				sKey, s, split = truncEnd(s)
				if sKey == nil {
					return 0, fmt.Errorf("invalid map key")
				} else if len(s) == 0 || len(s[0]) == 0 || s[0][0] != ':' {
					return 0, fmt.Errorf("map key %v: missing semicolon", strings.Join(sKey, " "))
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
					return 0, fmt.Errorf("map key %v: missing semicolon", strings.Join(sKey, " "))
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
			if _, err := scanValue(key, sKey); err != nil {
				return 0, fmt.Errorf("map key %v: %v", strings.Join(sKey, " "), err)
			}

			// consume value
			index := strings.Join(sKey, " ")
			var sVal []string
			if len(s) == 0 {
				// empty value after semicolon
				sVal = []string{""}
			} else if 0 < len(s[0]) && (s[0][0] == '{' || s[0][0] == '[') {
				sVal, s, split = truncEnd(s)
				if sVal == nil || split {
					return 0, fmt.Errorf("map key %v: invalid value", index)
				}
			} else if idx := strings.IndexByte(s[0], ','); idx != -1 {
				sVal = []string{s[0][:idx]}
				s[0] = s[0][idx+1:]
			} else {
				sVal = []string{s[0]}
				s = s[1:]
			}
			val := reflect.New(v.Type().Elem()).Elem()
			if _, err := scanValue(val, sVal); err != nil {
				return 0, fmt.Errorf("map key %v: %v", index, err)
			}

			if v.IsNil() {
				v.Set(reflect.MakeMap(v.Type()))
			}
			v.SetMapIndex(key, val)
		}
	case reflect.Struct:
		var split bool
		if len(s[0]) == 0 {
			return 1, nil
		} else if s[0][0] != '{' {
			return 0, fmt.Errorf("invalid struct")
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
				if sVal == nil || split {
					return 0, fmt.Errorf("struct field %v: invalid value", field)
				}
			} else {
				sVal = []string{s[0]}
				s = s[1:]
			}
			if _, err := scanValue(v.Field(j), sVal); err != nil {
				return 0, fmt.Errorf("struct field %v: %v", field, err)
			}
			j++
		}
		if j != v.NumField() {
			return 0, fmt.Errorf("missing struct fields")
		} else if len(s) != 0 {
			return 0, fmt.Errorf("too many struct fields")
		}
	default:
		panic(fmt.Sprintf("unsupported type %v", v.Type())) // should never happen
	}
	return n, nil
}

// isValidName returns true if the short or long option name is valid.
func isValidName(s string) bool {
	for i, r := range s {
		if !unicode.IsLetter(r) && !unicode.IsNumber(r) && r != '_' && (r != '-' || i == 0) {
			return false
		}
	}
	return true
}

// isValidType returns true if the destination variable type is supported. Either it implements the Custom interface, or is a valid base type.
func isValidType(t reflect.Type) bool {
	if t.Implements(reflect.TypeOf((*Custom)(nil)).Elem()) {
		// implements Custom
		return true
	}
	return isValidBaseType(t)
}

func isValidBaseType(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.String, reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Float32, reflect.Float64:
		return true
	case reflect.Array, reflect.Slice:
		return isValidBaseType(t.Elem())
	case reflect.Map:
		return isValidBaseType(t.Key()) && isValidBaseType(t.Elem())
	case reflect.Struct:
		for i := 0; i < t.NumField(); i++ {
			if !isValidBaseType(t.Field(i).Type) {
				return false
			}
		}
		return true
	}
	return false
}

// TypeName returns the type's name.
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

// sortOption sorts options by short and then name.
func sortOption(vars []*Var) func(int, int) bool {
	return func(i, j int) bool {
		if vars[i].Short != 0 {
			if vars[j].Short != 0 {
				return vars[i].Short < vars[j].Short
			} else {
				return string(vars[i].Short) < vars[j].Name
			}
		} else if vars[j].Short != 0 {
			return vars[i].Name < string(vars[j].Short)
		}
		return vars[i].Name < vars[j].Name
	}
}

// sortArgument sorts arguments by index and then rest.
func sortArgument(vars []*Var) func(int, int) bool {
	return func(i, j int) bool {
		if vars[i].Rest {
			return false
		} else if vars[j].Rest {
			return true
		}
		return vars[i].Index < vars[j].Index
	}
}

func fromFieldname(field string) string {
	name := make([]byte, 0, len(field))
	prevUpper := false
	for i, r := range field {
		if unicode.IsTitle(r) || unicode.IsUpper(r) {
			rNext, n := utf8.DecodeRuneInString(field[i+utf8.RuneLen(r):])
			if i != 0 && n != 0 && (!prevUpper || !unicode.IsTitle(rNext) && !unicode.IsUpper(rNext)) {
				name = append(name, '-')
			}
			name = utf8.AppendRune(name, unicode.ToLower(r))
			prevUpper = true
		} else {
			name = utf8.AppendRune(name, r)
			prevUpper = false
		}
	}
	return string(name)
}

func toFieldname(name string) string {
	field := make([]byte, 0, len(name))
	capitalize := true
	for _, r := range name {
		if capitalize {
			field = utf8.AppendRune(field, unicode.ToTitle(r))
			capitalize = false
		} else if r == '-' {
			capitalize = true
		} else {
			field = utf8.AppendRune(field, r)
		}
	}
	return string(field)
}

func splitArguments(s string) []string {
	i := 0
	var esc bool
	var quote rune
	arg := ""
	args := []string{}
	for j, r := range s {
		if r == '\\' {
			if i < j {
				arg += s[i:j]
			}
			i = j + 1
			esc = true
		} else if esc {
			esc = false
		} else if (quote == 0 || quote == r) && r == '\'' || r == '"' {
			if quote == 0 {
				quote = r
			} else {
				quote = 0
			}
			if i < j {
				arg += s[i:j]
			}
			i = j + 1
		} else if quote == 0 && unicode.IsSpace(r) {
			if i < j {
				args = append(args, arg+s[i:j])
				arg = ""
			}
			i = j + utf8.RuneLen(r)
		}
	}
	if i < len(s) {
		args = append(args, arg+s[i:])
	} else {
		args = append(args, arg)
	}
	return args
}
