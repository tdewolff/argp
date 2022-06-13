# GNU command line argument parser
Command line argument parser following the GNU standard.

    ./test -vo out.png --size 256 input.txt

## Examples
### Default usage
A regular command with short and long options.

```go
import "github.com/tdewolff/argp"

func main() {
    var verbose bool
    var output string
    var size int

    argp := argp.New("CLI tool description")
    argp.AddOpt(&verbose, "v", "verbose", nil, "")
    argp.AddOpt(&output, "o", "output", nil, "Output file name")
    argp.AddOpt(&size, "", "size", 512, "Image size")
    inputs := argp.Parse()

    // ...
}
```

with help output

```
Usage: test [options]

Options:
  -h, --help     Help
  -o, --output   Output file name
      --size     Image size
  -v, --verbose
```

### Sub commands
Example with sub commands using a main command for when no sub command is used, and a sub command named "cmd". For the main command we can also use `New` and `AddOpt` instead and process the command after `argp.Parse()`.

```go
import "github.com/tdewolff/argp"

func main() {
    argp := argp.NewCmd(&Main{}, "CLI tool description")
    argp.AddCmd(&Command{}, "cmd", "Sub command")
    argp.Parse()
}

type Main struct {
    Version bool `short:"v"`
}

func (cmd *Main) Run() error {
    // ...
}

type Command struct {
    Verbose bool `short:"v" long:""`
    Output string `short:"o" desc:"Output file name"`
    Size int `default:"512" desc:"Image size"`
}

func (cmd *Command) Run() error {
    // ...
}
```

### Options
Basic types
```go
var v string
argp.AddOpt(&v, "v", "var", "default", "description")

var v bool
argp.AddOpt(&v, "v", "var", "true", "description")

var v int // also: int8, int16, int32, int64
argp.AddOpt(&v, "v", "var", "42", "description")

var v uint // also: uint8, uint16, uint32, uint64
argp.AddOpt(&v, "v", "var", "42", "description")

var v float64 // also: float32
argp.AddOpt(&v, "v", "var", "4.2", "description")
```

Composite types
```go
var v [2]int // element can be any valid basic or composite type
argp.AddOpt(&v, "v", "var", "4 2", "description")
// value: [2]int{4, 2}

var v []int // element can be any valid basic or composite type
argp.AddOpt(&v, "v", "var", "4 2 1", "description")
// value: []int{4, 2, 1}

var v struct {
    S string
    I int
    B [2]bool
} // fields can be any valid basic or composite type
argp.AddOpt(&v, "v", "var", "string 42 0 1", "description")
// value: struct{S string, I int, B [2]bool}{"string", 42, false, true}
```

### Option tags
The following struct will accept the following options and arguments:
- `-v` or `--var` with a default value of 42
- The first argument called `first` with a default value of 4.2
- The other arguments called `rest`

```go
type Command struct {
    Var1 int `short:"v" long:"var" default:"42" desc:"Description"`
    Var2 float64 `name:"first" index:"0" default:"4.2"`
    Var3 []string `name:"rest" index:"*"`
}
```
