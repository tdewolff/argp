# GNU command line argument parser
Command line argument parser following the GNU standard.

    ./test -vo out.png --size 256 input.txt

## Example

    import "github.com/tdewolff/argp"

    func main() {
        var verbose bool
        var output string
        var size int

        argp := argp.New()
        argp.Add(&verbose, "v", "verbose", nil, "")
        argp.Add(&output, "o", "output", nil, "Output file name")
        argp.Add(&size, "", "size", 512, "Image size")
        inputs := argp.Parse()

        // ...
    }

Use structure:

    import "github.com/tdewolff/argp"

    type Options struct {
        Verbose bool `short:"v"`
        Output string `short:"o" desc:"Output file name"`
        Size int `default:"512" desc:"Image size"`
    }

    func main() {
        options := Options{}

        argp := argp.New()
        argp.AddStruct(&options)
        inputs := argp.Parse()

        // ...
    }
