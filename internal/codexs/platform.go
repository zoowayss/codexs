package codexs

type Platform interface {
	ID() string
	CheckSupported() error
	DefaultStoreRoot(appID string) (string, error)
	ParseTerminalKind(value string) (TerminalKind, error)
	OpenTerminal(kind TerminalKind, shellCommand string) error
	ExecProcess(path string, argv []string, env []string) error
}
