package testdata

import _ "embed"

var (
	//go:embed bad_signature_bad.txt
	Bad_signature_bad_txt string

	//go:embed bad_signature_badkeyid.txt
	Bad_signature_badkeyid_txt string

	//go:embed bad_signature_malformed1.txt
	Bad_signature_malformed1_txt string

	//go:embed bad_signature_malformed2.txt
	Bad_signature_malformed2_txt string

	//go:embed bad_signature_malformed3.txt
	Bad_signature_malformed3_txt string

	//go:embed bad_signature_manipulated.txt
	Bad_signature_manipulated_txt string

	//go:embed bad_signature_nodata.txt
	Bad_signature_nodata_txt string

	//go:embed bad_signature_preeof1.txt
	Bad_signature_preeof1_txt string

	//go:embed bad_signature_preeof2.txt
	Bad_signature_preeof2_txt string

	//go:embed garbage.asc
	Garbage_asc string

	//go:embed github.asc
	Github_asc string

	//go:embed good_signature.txt
	Good_signature_txt string

	//go:embed janedoe.asc
	Janedoe_asc string

	//go:embed johndoe.asc
	Johndoe_asc string

	//go:embed multi.asc
	Multi_asc string

	//go:embed multi2.asc
	Multi2_asc string

	//go:embed unknown_signature1.txt
	Unknown_signature1_txt string

	//go:embed unknown_signature2.txt
	Unknown_signature2_txt string
)
