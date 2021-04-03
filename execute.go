package gonsole

import (
	"fmt"

	"github.com/jessevdk/go-flags"
)

// execute - The user has entered a command input line, the arguments
// have been processed: we synchronize a few elements of the console,
// then pass these arguments to the command parser for execution and error handling.
func (c *Console) execute(args []string) {

	// Asynchronous messages do not mess with the prompt from now on,
	// until end of execution. Once we are done executing the command,
	// they can again.
	c.isExecuting = true
	defer func() {
		c.isExecuting = false
	}()

	// Execute the command line.
	result, err := c.parser.ParseArgs(args)

	// Process the errors raised by the parser.
	// A few of them are not really errors, and trigger some stuff.
	if err != nil {
		if err == nil {
			return
		}
		parserErr, ok := err.(*flags.Error)
		if !ok {
			return
		}

		// If the error type is a detected -h, --help flag, print custom help.
		if parserErr.Type == flags.ErrHelp {
			c.handleHelpFlag(result)
			return
		}

		// Else, we print the raw parser error
		fmt.Println(parserError + parserErr.Error())
	}

	return
}
