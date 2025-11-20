package testdata

import _ "embed"

var (
	//go:embed garbage.asc
	Garbage_asc string

	//go:embed github.asc
	Github_asc string

	//go:embed janedoe.asc
	Janedoe_asc string

	//go:embed johndoe.asc
	Johndoe_asc string

	//go:embed multi.asc
	Multi_asc string

	//go:embed multi2.asc
	Multi2_asc string
)
