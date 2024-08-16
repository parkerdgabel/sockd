package embedded

import (
	_ "embed"
)

//go:embed pypiPullerInstaller.py
var PyPiPullerInstaller_py string
