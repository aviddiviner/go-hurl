package cli

import "path/filepath"

// RunContext is the context in which Hurl is executed: whether the standard
// streams are terminals, the config file path and whether we run in CI.
type RunContext struct {
	stdinTerm  bool
	stdoutTerm bool
	stderrTerm bool
	configFile string
	ci         bool
}

// NewRunContext creates a new context. The environment is captured and
// seen as immutable for the execution with this context.
func NewRunContext(env *EnvVars, stdinTerm, stdoutTerm, stderrTerm bool) *RunContext {
	return &RunContext{
		stdinTerm:  stdinTerm,
		stdoutTerm: stdoutTerm,
		stderrTerm: stderrTerm,
		configFile: configFilePath(env),
		ci:         env.IsCI(),
	}
}

// ConfigFilePath returns the config file path, or "" if there's none.
func (c *RunContext) ConfigFilePath() string { return c.configFile }

func (c *RunContext) IsStdinTerm() bool  { return c.stdinTerm }
func (c *RunContext) IsStdoutTerm() bool { return c.stdoutTerm }
func (c *RunContext) IsStderrTerm() bool { return c.stderrTerm }
func (c *RunContext) IsCI() bool         { return c.ci }

// configFilePath returns $XDG_CONFIG_HOME/hurl/config if XDG_CONFIG_HOME is
// set, else $HOME/.config/hurl/config if HOME is set.
func configFilePath(env *EnvVars) string {
	var dir string
	if xdg, ok := env.XdgConfigHome(); ok {
		dir = xdg
	} else if home, ok := env.Home(); ok {
		dir = filepath.Join(home, ".config")
	} else {
		return ""
	}
	return filepath.Join(dir, "hurl", "config")
}
