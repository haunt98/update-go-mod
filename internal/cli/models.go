package cli

// See https://pkg.go.dev/cmd/go#hdr-List_packages_or_modules
type Module struct {
	Update   *Module
	Replace  *Module
	Path     string
	Version  string
	Main     bool
	Indirect bool
}

// https://pkg.go.dev/cmd/go#hdr-Edit_go_mod_from_tools_or_scripts
type GoMod struct {
	Module    ModPath
	Go        string
	Toolchain string
}

type ModPath struct {
	Path       string
	Deprecated string
}
