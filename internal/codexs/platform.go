package codexs

type Platform interface {
	ID() string
	CheckSupported() error
	DefaultStoreRoot(appID string) (string, error)
	OpenTerminal(shellCommand string) error
	ExecProcess(path string, argv []string, env []string) error
}
