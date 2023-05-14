package console

import (
	"strings"
	"sync"

	"github.com/reeflective/readline"
	"github.com/reeflective/readline/inputrc"
)

// Console is an integrated console application instance.
type Console struct {
	// Application ------------------------------------------------------------------

	// shell - The underlying shell provides the core readline functionality,
	// including but not limited to: inputs, completions, hints, history.
	shell *readline.Shell

	// Different menus with different command trees, prompt engines, etc.
	menus map[string]*Menu

	// Execution --------------------------------------------------------------------

	// LeaveNewline - If true, the console will leave an empty line before
	// executing the command.
	LeaveNewline bool

	// PreReadlineHooks - All the functions in this list will be executed,
	// in their respective orders, before the console starts reading
	// any user input (ie, before redrawing the prompt).
	PreReadlineHooks []func()

	// PreCmdRunLineHooks - Same as PreCmdRunHooks, but will have an effect on the
	// input line being ultimately provided to the command parser. This might
	// be used by people who want to apply supplemental, specific processing
	// on the command input line.
	PreCmdRunLineHooks []func(raw []string) (args []string, err error)

	// PreCmdRunHooks - Once the user has entered a command, but before executing
	// the target command, the console will execute every function in this list.
	// These hooks are distinct from the cobra.PreRun() or OnInitialize hooks,
	// and might be used in combination with them.
	PreCmdRunHooks []func()

	// PostCmdRunHooks are run after the target cobra command has been executed.
	// These hooks are distinct from the cobra.PreRun() or OnFinalize hooks,
	// and might be used in combination with them.
	PostCmdRunHooks []func()

	// True if the console is currently running a command. This is used by
	// the various asynchronous log/message functions, which need to adapt their
	// behavior (do we reprint the prompt, where, etc) based on this.
	isExecuting bool

	// concurrency management.
	mutex *sync.RWMutex

	// Other ------------------------------------------------------------------------

	printLogo func(c *Console)

	// A list of tags by which commands may have been registered, and which
	// can be set to true in order to hide all of the tagged commands.
	filters []string
}

// New - Instantiates a new console application, with sane but powerful defaults.
// This instance can then be passed around and used to bind commands, setup additional
// things, print asynchronous messages, or modify various operating parameters on the fly.
// The app parameter is an optional name of the application using this console.
func New(app string) *Console {
	console := &Console{
		shell: readline.NewShell(inputrc.WithApp(strings.ToLower(app))),
		menus: make(map[string]*Menu),
		mutex: &sync.RWMutex{},
	}

	// Quality of life improvements.
	console.setupShell()

	// Make a default menu and make it current.
	// Each menu is created with a default prompt engine.
	defaultMenu := console.NewMenu("")
	defaultMenu.active = true

	// Set the history for this menu
	for _, name := range defaultMenu.historyNames {
		console.shell.History.Add(name, defaultMenu.histories[name])
	}

	// Command completion, syntax highlighting, multiline callbacks, etc.
	console.shell.AcceptMultiline = console.acceptMultiline
	console.shell.Completer = console.complete
	console.shell.SyntaxHighlighter = console.highlightSyntax

	return console
}

// Shell returns the console readline shell instance, so that the user can
// further configure it or use some of its API for lower-level stuff.
func (c *Console) Shell() *readline.Shell {
	return c.shell
}

// SetPrintLogo - Sets the function that will be called to print the logo.
func (c *Console) SetPrintLogo(f func(c *Console)) {
	c.printLogo = f
}

// NewMenu - Create a new command menu, to which the user
// can attach any number of commands (with any nesting), as
// well as some specific items like history sources, prompt
// configurations, sets of expanded variables, and others.
func (c *Console) NewMenu(name string) *Menu {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	menu := newMenu(name, c)
	c.menus[name] = menu

	return menu
}

// CurrentMenu - Return the current console menu. Because the Context
// is just a reference, any modifications to this menu will persist.
func (c *Console) CurrentMenu() *Menu {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	return c.activeMenu()
}

// Menu returns one of the console menus by name, or nil if no menu is found.
func (c *Console) Menu(name string) *Menu {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	return c.menus[name]
}

// SwitchMenu - Given a name, the console switches its command menu:
// The next time the console rebinds all of its commands, it will only bind those
// that belong to this new menu. If the menu is invalid, i.e that no commands
// are bound to this menu name, the current menu is kept.
func (c *Console) SwitchMenu(menu string) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	// Only switch if the target menu was found.
	if target, found := c.menus[menu]; found && target != nil {
		current := c.activeMenu()
		if current != nil {
			current.active = false
		}

		target.active = true

		// Remove the currently bound history sources
		// (old menu) and bind the ones peculiar to this one.
		c.shell.History.Delete()

		for _, name := range target.historyNames {
			c.shell.History.Add(name, target.histories[name])
		}
	}
}

// SystemEditor - This function is a renamed-reexport of the underlying readline.StartEditorWithBuffer
// function, which enables you to conveniently edit files/buffers from within the console application.
// Naturally, the function will block until the editor is exited, and the updated buffer is returned.
// The filename parameter can be used to pass a specific filename.ext pattern, which might be useful
// if the editor has builtin filetype plugin functionality.
func (c *Console) SystemEditor(buffer []byte, filetype string) ([]byte, error) {
	emacs := c.shell.Config.GetString("editing-mode") == "emacs"

	edited, err := c.shell.Buffers.EditBuffer([]rune(string(buffer)), "", filetype, emacs)

	return []byte(string(edited)), err
}

func (c *Console) setupShell() {
	cfg := c.shell.Config

	// Some options should be set to on because they
	// are quite neceessary for efficient console use.
	cfg.Set("skip-completed-text", true)
	cfg.Set("menu-complete-display-prefix", true)
}

func (c *Console) reloadConfig() {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	menu := c.activeMenu()
	menu.prompt.bind(c.shell)
}

func (c *Console) activeMenu() *Menu {
	for _, menu := range c.menus {
		if menu.active {
			return menu
		}
	}

	// Else return the default menu.
	return c.menus[""]
}
