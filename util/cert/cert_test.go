package cert

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/argoproj/argo-cd/v2/common"
)

const Test_Cert1CN = "CN=foo.example.com,OU=SpecOps,O=Capone\\, Inc,L=Chicago,ST=IL,C=US"
const Test_Cert2CN = "CN=bar.example.com,OU=Testsuite,O=Testing Corp,L=Hanover,ST=Lower Saxony,C=DE"
const Test_TLSValidSingleCert = `
-----BEGIN CERTIFICATE-----
MIIFvTCCA6WgAwIBAgIUGrTmW3qc39zqnE08e3qNDhUkeWswDQYJKoZIhvcNAQEL
BQAwbjELMAkGA1UEBhMCVVMxCzAJBgNVBAgMAklMMRAwDgYDVQQHDAdDaGljYWdv
MRQwEgYDVQQKDAtDYXBvbmUsIEluYzEQMA4GA1UECwwHU3BlY09wczEYMBYGA1UE
AwwPZm9vLmV4YW1wbGUuY29tMB4XDTE5MDcwODEzNTUwNVoXDTIwMDcwNzEzNTUw
NVowbjELMAkGA1UEBhMCVVMxCzAJBgNVBAgMAklMMRAwDgYDVQQHDAdDaGljYWdv
MRQwEgYDVQQKDAtDYXBvbmUsIEluYzEQMA4GA1UECwwHU3BlY09wczEYMBYGA1UE
AwwPZm9vLmV4YW1wbGUuY29tMIICIjANBgkqhkiG9w0BAQEFAAOCAg8AMIICCgKC
AgEA3csSO13w7qQXKeSLNcpeuAe6wAjXYbRkRl6ariqzTEDcFTKmy2QiXJTKoEGn
bvwxq0T91var7rxY88SGL/qi8Zmo0tVSR0XvKSKcghFIkQOTyDmVgMPZGCvixt4q
gQ7hUVSk4KkFmtcqBVuvnzI1d/DKfZAGKdmGcfRpuAsnVhac3swP0w4Tl1BFrK9U
vuIkz4KwXG77s5oB8rMUnyuLasLsGNpvpvXhkcQRhp6vpcCO2bS7kOTTelAPIucw
P37qkOEdZdiWCLrr57dmhg6tmcVlmBMg6JtmfLxn2HQd9ZrCKlkWxMk5NYs6CAW5
kgbDZUWQTAsnHeoJKbcgtPkIbxDRxNpPukFMtbA4VEWv1EkODXy9FyEKDOI/PV6K
/80oLkgCIhCkP2mvwSFheU0RHTuZ0o0vVolP5TEOq5iufnDN4wrxqb12o//XLRc0
RiLqGVVxhFdyKCjVxcLfII9AAp5Tse4PMh6bf6jDfB3OMvGkhMbJWhKXdR2NUTl0
esKawMPRXIn5g3oBdNm8kyRsTTnvB567pU8uNSmA8j3jxfGCPynI8JdiwKQuW/+P
WgLIflgxqAfG85dVVOsFmF9o5o24dDslvv9yHnHH102c6ijPCg1EobqlyFzqqxOD
Wf2OPjIkzoTH+O27VRugnY/maIU1nshNO7ViRX5zIxEUtNMCAwEAAaNTMFEwHQYD
VR0OBBYEFNY4gDLgPBidogkmpO8nq5yAq5g+MB8GA1UdIwQYMBaAFNY4gDLgPBid
ogkmpO8nq5yAq5g+MA8GA1UdEwEB/wQFMAMBAf8wDQYJKoZIhvcNAQELBQADggIB
AJ0WGioNtGNg3m6ywpmxNThorQD5ZvDMlmZlDVk78E2wfNyMhwbVhKhlAnONv0wv
kmsGjibY75nRZ+EK9PxSJ644841fryQXQ+bli5fhr7DW3uTKwaRsnzETJXRJuljq
6+c6Zyg1/mqwnyx7YvPgVh3w496DYx/jm6Fm1IEq3BzOmn6H/gGPq3gbURzEqI3h
P+kC2vJa8RZWrpa05Xk/Q1QUkErDX9vJghb9z3+GgirISZQzqWRghII/znv3NOE6
zoIgaaWNFn8KPeBVpUoboH+IhpgibsnbTbI0G7AMtFq6qm3kn/4DZ2N2tuh1G2tT
zR2Fh7hJbU7CrqxANrgnIoHG/nLSvzE24ckLb0Vj69uGQlwnZkn9fz6F7KytU+Az
NoB2rjufaB0GQi1azdboMvdGSOxhSCAR8otWT5yDrywCqVnEvjw0oxKmuRduNe2/
6AcG6TtK2/K+LHuhymiAwZM2qE6VD2odvb+tCzDkZOIeoIz/JcVlNpXE9FuVl250
9NWvugeghq7tUv81iJ8ninBefJ4lUfxAehTPQqX+zXcfxgjvMRCi/ig73nLyhmjx
r2AaraPFgrprnxUibP4L7jxdr+iiw5bWN9/B81PodrS7n5TNtnfnpZD6X6rThqOP
xO7Tr5lAo74vNUkF2EHNaI28/RGnJPm2TIxZqy4rNH6L
-----END CERTIFICATE-----
`

const Test_TLSInvalidPEMData = `
MIIF1zCCA7+gAwIBAgIUQdTcSHY2Sxd3Tq/v1eIEZPCNbOowDQYJKoZIhvcNAQEL
BQAwezELMAkGA1UEBhMCREUxFTATBgNVBAgMDExvd2VyIFNheG9ueTEQMA4GA1UE
BwwHSGFub3ZlcjEVMBMGA1UECgwMVGVzdGluZyBDb3JwMRIwEAYDVQQLDAlUZXN0
c3VpdGUxGDAWBgNVBAMMD2Jhci5leGFtcGxlLmNvbTAeFw0xOTA3MDgxMzU2MTda
Fw0yMDA3MDcxMzU2MTdaMHsxCzAJBgNVBAYTAkRFMRUwEwYDVQQIDAxMb3dlciBT
YXhvbnkxEDAOBgNVBAcMB0hhbm92ZXIxFTATBgNVBAoMDFRlc3RpbmcgQ29ycDES
MBAGA1UECwwJVGVzdHN1aXRlMRgwFgYDVQQDDA9iYXIuZXhhbXBsZS5jb20wggIi
MA0GCSqGSIb3DQEBAQUAA4ICDwAwggIKAoICAQCv4mHMdVUcafmaSHVpUM0zZWp5
NFXfboxA4inuOkE8kZlbGSe7wiG9WqLirdr39Ts+WSAFA6oANvbzlu3JrEQ2CHPc
CNQm6diPREFwcDPFCe/eMawbwkQAPVSHPts0UoRxnpZox5pn69ghncBR+jtvx+/u
P6HdwW0qqTvfJnfAF1hBJ4oIk2AXiip5kkIznsAh9W6WRy6nTVCeetmIepDOGe0G
ZJIRn/OfSz7NzKylfDCat2z3EAutyeT/5oXZoWOmGg/8T7pn/pR588GoYYKRQnp+
YilqCPFX+az09EqqK/iHXnkdZ/Z2fCuU+9M/Zhrnlwlygl3RuVBI6xhm/ZsXtL2E
Gxa61lNy6pyx5+hSxHEFEJshXLtioRd702VdLKxEOuYSXKeJDs1x9o6cJ75S6hko
`

const Test_TLSInvalidSingleCert = `
-----BEGIN CERTIFICATE-----
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
XWyb96wrUlv+E8I=
-----END CERTIFICATE-----
`

const Test_TLSValidMultiCert = `
-----BEGIN CERTIFICATE-----
MIIFvTCCA6WgAwIBAgIUGrTmW3qc39zqnE08e3qNDhUkeWswDQYJKoZIhvcNAQEL
BQAwbjELMAkGA1UEBhMCVVMxCzAJBgNVBAgMAklMMRAwDgYDVQQHDAdDaGljYWdv
MRQwEgYDVQQKDAtDYXBvbmUsIEluYzEQMA4GA1UECwwHU3BlY09wczEYMBYGA1UE
AwwPZm9vLmV4YW1wbGUuY29tMB4XDTE5MDcwODEzNTUwNVoXDTIwMDcwNzEzNTUw
NVowbjELMAkGA1UEBhMCVVMxCzAJBgNVBAgMAklMMRAwDgYDVQQHDAdDaGljYWdv
MRQwEgYDVQQKDAtDYXBvbmUsIEluYzEQMA4GA1UECwwHU3BlY09wczEYMBYGA1UE
AwwPZm9vLmV4YW1wbGUuY29tMIICIjANBgkqhkiG9w0BAQEFAAOCAg8AMIICCgKC
AgEA3csSO13w7qQXKeSLNcpeuAe6wAjXYbRkRl6ariqzTEDcFTKmy2QiXJTKoEGn
bvwxq0T91var7rxY88SGL/qi8Zmo0tVSR0XvKSKcghFIkQOTyDmVgMPZGCvixt4q
gQ7hUVSk4KkFmtcqBVuvnzI1d/DKfZAGKdmGcfRpuAsnVhac3swP0w4Tl1BFrK9U
vuIkz4KwXG77s5oB8rMUnyuLasLsGNpvpvXhkcQRhp6vpcCO2bS7kOTTelAPIucw
P37qkOEdZdiWCLrr57dmhg6tmcVlmBMg6JtmfLxn2HQd9ZrCKlkWxMk5NYs6CAW5
kgbDZUWQTAsnHeoJKbcgtPkIbxDRxNpPukFMtbA4VEWv1EkODXy9FyEKDOI/PV6K
/80oLkgCIhCkP2mvwSFheU0RHTuZ0o0vVolP5TEOq5iufnDN4wrxqb12o//XLRc0
RiLqGVVxhFdyKCjVxcLfII9AAp5Tse4PMh6bf6jDfB3OMvGkhMbJWhKXdR2NUTl0
esKawMPRXIn5g3oBdNm8kyRsTTnvB567pU8uNSmA8j3jxfGCPynI8JdiwKQuW/+P
WgLIflgxqAfG85dVVOsFmF9o5o24dDslvv9yHnHH102c6ijPCg1EobqlyFzqqxOD
Wf2OPjIkzoTH+O27VRugnY/maIU1nshNO7ViRX5zIxEUtNMCAwEAAaNTMFEwHQYD
VR0OBBYEFNY4gDLgPBidogkmpO8nq5yAq5g+MB8GA1UdIwQYMBaAFNY4gDLgPBid
ogkmpO8nq5yAq5g+MA8GA1UdEwEB/wQFMAMBAf8wDQYJKoZIhvcNAQELBQADggIB
AJ0WGioNtGNg3m6ywpmxNThorQD5ZvDMlmZlDVk78E2wfNyMhwbVhKhlAnONv0wv
kmsGjibY75nRZ+EK9PxSJ644841fryQXQ+bli5fhr7DW3uTKwaRsnzETJXRJuljq
6+c6Zyg1/mqwnyx7YvPgVh3w496DYx/jm6Fm1IEq3BzOmn6H/gGPq3gbURzEqI3h
P+kC2vJa8RZWrpa05Xk/Q1QUkErDX9vJghb9z3+GgirISZQzqWRghII/znv3NOE6
zoIgaaWNFn8KPeBVpUoboH+IhpgibsnbTbI0G7AMtFq6qm3kn/4DZ2N2tuh1G2tT
zR2Fh7hJbU7CrqxANrgnIoHG/nLSvzE24ckLb0Vj69uGQlwnZkn9fz6F7KytU+Az
NoB2rjufaB0GQi1azdboMvdGSOxhSCAR8otWT5yDrywCqVnEvjw0oxKmuRduNe2/
6AcG6TtK2/K+LHuhymiAwZM2qE6VD2odvb+tCzDkZOIeoIz/JcVlNpXE9FuVl250
9NWvugeghq7tUv81iJ8ninBefJ4lUfxAehTPQqX+zXcfxgjvMRCi/ig73nLyhmjx
r2AaraPFgrprnxUibP4L7jxdr+iiw5bWN9/B81PodrS7n5TNtnfnpZD6X6rThqOP
xO7Tr5lAo74vNUkF2EHNaI28/RGnJPm2TIxZqy4rNH6L
-----END CERTIFICATE-----
-----BEGIN CERTIFICATE-----
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
XWyb96wrUlv+E8I=
-----END CERTIFICATE-----
`

// Taken from hack/ssh_known_hosts
const Test_ValidSSHKnownHostsData = `
# BitBucket
bitbucket.org ssh-rsa AAAAB3NzaC1yc2EAAAABIwAAAQEAubiN81eDcafrgMeLzaFPsw2kNvEcqTKl/VqLat/MaB33pZy0y3rJZtnqwR2qOOvbwKZYKiEO1O6VqNEBxKvJJelCq0dTXWT5pbO2gDXC6h6QDXCaHo6pOHGPUy+YBaGQRGuSusMEASYiWunYN0vCAI8QaXnWMXNMdFP3jHAJH0eDsoiGnLPBlBp4TNm6rYI74nMzgz3B9IikW4WVK+dc8KZJZWYjAuORU3jc1c/NPskD2ASinf8v3xnfXeukU0sJ5N6m5E8VLjObPEO+mN2t/FZTMZLiFqPWc/ALSqnMnnhwrNi2rbfg/rd/IpL8Le3pSBne8+seeFVBoGqzHM9yXw==
# GitHub
github.com ssh-rsa AAAAB3NzaC1yc2EAAAABIwAAAQEAq2A7hRGmdnm9tUDbO9IDSwBK6TbQa+PXYPCPy6rbTrTtw7PHkccKrpp0yVhp5HdEIcKr6pLlVDBfOLX9QUsyCOV0wzfjIJNlGEYsdlLJizHhbn2mUjvSAHQqZETYP81eFzLQNnPHt4EVVUh7VfDESU84KezmD5QlWpXLmvU31/yMf+Se8xhHTvKSCZIFImWwoG6mbUoWf9nzpIoaSjB+weqqUUmpaaasXVal72J+UX2B+2RPW3RcT0eOzQgqlJL3RKrTJvdsjE3JEAvGq3lGHSZXy28G3skua2SmVi/w4yCE6gbODqnTWlg7+wC604ydGXA8VJiS5ap43JXiUFFAaQ==
# GitLab
gitlab.com ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdHAyNTYAAAAIbmlzdHAyNTYAAABBBFSMqzJeV9rUzU4kWitGjeR4PWSa29SPqJ1fVkhtj3Hw9xjLVXVYrU9QlYWrOLXBpQ6KWjbjTDTdDkoohFzgbEY=
gitlab.com ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIAfuCHKVTjquxvt6CM6tdG4SLp1Btn/nOeHHE5UOzRdf
gitlab.com ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQCsj2bNKTBSpIYDEGk9KxsGh3mySTRgMtXL583qmBpzeQ+jqCMRgBqB98u3z++J1sKlXHWfM9dyhSevkMwSbhoR8XIq/U0tCNyokEi/ueaBMCvbcTHhO7FcwzY92WK4Yt0aGROY5qX2UKSeOvuP4D6TPqKF1onrSzH9bx9XUf2lEdWT/ia1NEKjunUqu1xOB/StKDHMoX4/OKyIzuS0q/T1zOATthvasJFoPrAjkohTyaDUz2LN5JoH839hViyEG82yB+MjcFV5MU3N1l1QL3cVUCh93xSaua1N85qivl+siMkPGbO5xR/En4iEY6K2XPASUEMaieWVNTRCtJ4S8H+9
# Azure
ssh.dev.azure.com ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC7Hr1oTWqNqOlzGJOfGJ4NakVyIzf1rXYd4d7wo6jBlkLvCA4odBlL0mDUyZ0/QUfTTqeu+tm22gOsv+VrVTMk6vwRU75gY/y9ut5Mb3bR5BV58dKXyq9A9UeB5Cakehn5Zgm6x1mKoVyf+FFn26iYqXJRgzIZZcZ5V6hrE0Qg39kZm4az48o0AUbf6Sp4SLdvnuMa2sVNwHBboS7EJkm57XQPVU3/QpyNLHbWDdzwtrlS+ez30S3AdYhLKEOxAG8weOnyrtLJAUen9mTkol8oII1edf7mWWbWVf0nBmly21+nZcmCTISQBtdcyPaEno7fFQMDD26/s0lfKob4Kw8H
vs-ssh.visualstudio.com ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC7Hr1oTWqNqOlzGJOfGJ4NakVyIzf1rXYd4d7wo6jBlkLvCA4odBlL0mDUyZ0/QUfTTqeu+tm22gOsv+VrVTMk6vwRU75gY/y9ut5Mb3bR5BV58dKXyq9A9UeB5Cakehn5Zgm6x1mKoVyf+FFn26iYqXJRgzIZZcZ5V6hrE0Qg39kZm4az48o0AUbf6Sp4SLdvnuMa2sVNwHBboS7EJkm57XQPVU3/QpyNLHbWDdzwtrlS+ez30S3AdYhLKEOxAG8weOnyrtLJAUen9mTkol8oII1edf7mWWbWVf0nBmly21+nZcmCTISQBtdcyPaEno7fFQMDD26/s0lfKob4Kw8H
`

const Test_InvalidSSHKnownHostsData = `
bitbucket.org AAAAB3NzaC1yc2EAAAABIwAAAQEAubiN81eDcafrgMeLzaFPsw2kNvEcqTKl/VqLat/MaB33pZy0y3rJZtnqwR2qOOvbwKZYKiEO1O6VqNEBxKvJJelCq0dTXWT5pbO2gDXC6h6QDXCaHo6pOHGPUy+YBaGQRGuSusMEASYiWunYN0vCAI8QaXnWMXNMdFP3jHAJH0eDsoiGnLPBlBp4TNm6rYI74nMzgz3B9IikW4WVK+dc8KZJZWYjAuORU3jc1c/NPskD2ASinf8v3xnfXeukU0sJ5N6m5E8VLjObPEO+mN2t/FZTMZLiFqPWc/ALSqnMnnhwrNi2rbfg/rd/IpL8Le3pSBne8+seeFVBoGqzHM9yXw==
# GitHub
github.com ssh-rsa AAAAB3NzaC1yc2EAAAABIwAAAQEAq2A7hRGmdnm9tUDbO9IDSwBK6TbQa+PXYPCPy6rbTrTtw7PHkccKrpp0yVhp5HdEIcKr6pLlVDBfOLX9QUsyCOV0wzfjIJNlGEYsdlLJizHhbn2mUjvSAHQqZETYP81eFzLQNnPHt4EVVUh7VfDESU84KezmD5QlWpXLmvU31/yMf+Se8xhHTvKSCZIFImWwoG6mbUoWf9nzpIoaSjB+weqqUUmpaaasXVal72J+UX2B+2RPW3RcT0eOzQgqlJL3RKrTJvdsjE3JEAvGq3lGHSZXy28G3skua2SmVi/w4yCE6gbODqnTWlg7+wC604ydGXA8VJiS5ap43JXiUFFAaQ==
# GitLab
gitlab.com ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdHAyNTYAAAAIbmlzdHAyNTYAAABBBFSMqzJeV9rUzU4kWitGjeR4PWSa29SPqJ1fVkhtj3Hw9xjLVXVYrU9QlYWrOLXBpQ6KWjbjTDTdDkoohFzgbEY=
gitlab.com ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIAfuCHKVTjquxvt6CM6tdG4SLp1Btn/nOeHHE5UOzRdf
gitlab.com ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQCsj2bNKTBSpIYDEGk9KxsGh3mySTRgMtXL583qmBpzeQ+jqCMRgBqB98u3z++J1sKlXHWfM9dyhSevkMwSbhoR8XIq/U0tCNyokEi/ueaBMCvbcTHhO7FcwzY92WK4Yt0aGROY5qX2UKSeOvuP4D6TPqKF1onrSzH9bx9XUf2lEdWT/ia1NEKjunUqu1xOB/StKDHMoX4/OKyIzuS0q/T1zOATthvasJFoPrAjkohTyaDUz2LN5JoH839hViyEG82yB+MjcFV5MU3N1l1QL3cVUCh93xSaua1N85qivl+siMkPGbO5xR/En4iEY6K2XPASUEMaieWVNTRCtJ4S8H+9
# Azure
ssh.dev.azure.com ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC7Hr1oTWqNqOlzGJOfGJ4NakVyIzf1rXYd4d7wo6jBlkLvCA4odBlL0mDUyZ0/QUfTTqeu+tm22gOsv+VrVTMk6vwRU75gY/y9ut5Mb3bR5BV58dKXyq9A9UeB5Cakehn5Zgm6x1mKoVyf+FFn26iYqXJRgzIZZcZ5V6hrE0Qg39kZm4az48o0AUbf6Sp4SLdvnuMa2sVNwHBboS7EJkm57XQPVU3/QpyNLHbWDdzwtrlS+ez30S3AdYhLKEOxAG8weOnyrtLJAUen9mTkol8oII1edf7mWWbWVf0nBmly21+nZcmCTISQBtdcyPaEno7fFQMDD26/s0lfKob4Kw8H
vs-ssh.visualstudio.com ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC7Hr1oTWqNqOlzGJOfGJ4NakVyIzf1rXYd4d7wo6jBlkLvCA4odBlL0mDUyZ0/QUfTTqeu+tm22gOsv+VrVTMk6vwRU75gY/y9ut5Mb3bR5BV58dKXyq9A9UeB5Cakehn5Zgm6x1mKoVyf+FFn26iYqXJRgzIZZcZ5V6hrE0Qg39kZm4az48o0AUbf6Sp4SLdvnuMa2sVNwHBboS7EJkm57XQPVU3/QpyNLHbWDdzwtrlS+ez30S3AdYhLKEOxAG8weOnyrtLJAUen9mTkol8oII1edf7mWWbWVf0nBmly21+nZcmCTISQBtdcyPaEno7fFQMDD26/s0lfKob4Kw8H
`

func Test_TLSCertificate_ValidPEM_ValidCert(t *testing.T) {
	// Valid PEM data, single certificate, expect array of length 1
	certificates, err := ParseTLSCertificatesFromData(Test_TLSValidSingleCert)
	assert.Nil(t, err)
	assert.Equal(t, len(certificates), 1)
	// Expect good decode
	x509Cert, err := DecodePEMCertificateToX509(certificates[0])
	assert.Nil(t, err)
	assert.Equal(t, x509Cert.Subject.String(), Test_Cert1CN)
}

func Test_TLSCertificate_ValidPEM_InvalidCert(t *testing.T) {
	// Valid PEM data, but invalid certificate
	certificates, err := ParseTLSCertificatesFromData(Test_TLSInvalidSingleCert)
	assert.Nil(t, err)
	assert.Equal(t, len(certificates), 1)
	// Expect bad decode
	_, err = DecodePEMCertificateToX509(certificates[0])
	assert.NotNil(t, err)
}

func Test_TLSCertificate_InvalidPEM(t *testing.T) {
	// Invalid PEM data, expect array of length 0
	certificates, err := ParseTLSCertificatesFromData(Test_TLSInvalidPEMData)
	assert.Nil(t, err)
	assert.Equal(t, len(certificates), 0)
}

func Test_TLSCertificate_ValidPEM_ValidCert_Multi(t *testing.T) {
	// Valid PEM data, two certificates, expect array of length 2
	certificates, err := ParseTLSCertificatesFromData(Test_TLSValidMultiCert)
	assert.Nil(t, err)
	assert.Equal(t, len(certificates), 2)
	// Expect good decode
	x509Cert, err := DecodePEMCertificateToX509(certificates[0])
	assert.Nil(t, err)
	assert.Equal(t, x509Cert.Subject.String(), Test_Cert1CN)
	x509Cert, err = DecodePEMCertificateToX509(certificates[1])
	assert.Nil(t, err)
	assert.Equal(t, x509Cert.Subject.String(), Test_Cert2CN)
}

func Test_TLSCertificate_ValidPEM_ValidCert_FromFile(t *testing.T) {
	// Valid PEM data, single certificate from file, expect array of length 1
	certificates, err := ParseTLSCertificatesFromPath("../../test/certificates/cert1.pem")
	assert.Nil(t, err)
	assert.Equal(t, len(certificates), 1)
	// Expect good decode
	x509Cert, err := DecodePEMCertificateToX509(certificates[0])
	assert.Nil(t, err)
	assert.Equal(t, x509Cert.Subject.String(), Test_Cert1CN)
}

func Test_TLSCertPool(t *testing.T) {
	certificates, err := ParseTLSCertificatesFromData(Test_TLSValidMultiCert)
	assert.Nil(t, err)
	assert.Equal(t, len(certificates), 2)
	certPool := GetCertPoolFromPEMData(certificates)
	assert.NotNil(t, certPool)
}

func Test_TLSCertificate_CertFromNonExistingFile(t *testing.T) {
	// Non-existing file, expect err
	_, err := ParseTLSCertificatesFromPath("../../test/certificates/cert_nonexisting.pem")
	assert.NotNil(t, err)
}

func Test_SSHKnownHostsData_ParseData(t *testing.T) {
	// Expect valid data with 7 known host entries
	entries, err := ParseSSHKnownHostsFromData(Test_ValidSSHKnownHostsData)
	assert.Nil(t, err)
	assert.Equal(t, len(entries), 7)
}

func Test_SSHKnownHostsData_ParseFile(t *testing.T) {
	// Expect valid data with 7 known host entries
	entries, err := ParseSSHKnownHostsFromPath("../../test/certificates/ssh_known_hosts")
	assert.Nil(t, err)
	assert.Equal(t, len(entries), 7)
}

func Test_SSHKnownHostsData_ParseNonExistingFile(t *testing.T) {
	// Expect valid data with 7 known host entries
	entries, err := ParseSSHKnownHostsFromPath("../../test/certificates/ssh_known_hosts_invalid")
	assert.NotNil(t, err)
	assert.Nil(t, entries)
}

func Test_SSHKnownHostsData_Tokenize(t *testing.T) {
	// All entries should parse to valid SSH public keys
	// All entries should be tokenizable, and tokens should be feedable to decoder
	entries, err := ParseSSHKnownHostsFromData(Test_ValidSSHKnownHostsData)
	assert.Nil(t, err)
	for _, entry := range entries {
		hosts, _, err := KnownHostsLineToPublicKey(entry)
		assert.Nil(t, err)
		assert.Equal(t, len(hosts), 1)
		hoststring, subtype, certdata, err := TokenizeSSHKnownHostsEntry(entry)
		assert.Nil(t, err)
		hosts, _, err = TokenizedDataToPublicKey(hoststring, subtype, string(certdata))
		assert.Nil(t, err)
		assert.Equal(t, len(hosts), 1)
	}
}

func Test_MatchHostName(t *testing.T) {
	matchHostName := "foo.example.com"
	assert.Equal(t, MatchHostName(matchHostName, "*"), true)
	assert.Equal(t, MatchHostName(matchHostName, "*.example.com"), true)
	assert.Equal(t, MatchHostName(matchHostName, "foo.*"), true)
	assert.Equal(t, MatchHostName(matchHostName, "foo.*.com"), true)
	assert.Equal(t, MatchHostName(matchHostName, "fo?.example.com"), true)
	assert.Equal(t, MatchHostName(matchHostName, "foo?.example.com"), false)
	assert.Equal(t, MatchHostName(matchHostName, "bar.example.com"), false)
	assert.Equal(t, MatchHostName(matchHostName, "*.otherexample.com"), false)
	assert.Equal(t, MatchHostName(matchHostName, "foo.otherexample.*"), false)
}

func Test_SSHFingerprintSHA256(t *testing.T) {
	// actual SHA256 fingerprints for keys defined above
	fingerprints := [...]string{
		"zzXQOXSRBEiUtuE8AikJYKwbHaxvSc0ojez9YXaGp1A",
		"nThbg6kXUpJWGl7E1IGOCspRomTxdCARLviKw6E5SY8",
		"HbW3g8zUjNSksFbqTiUWPWg2Bq1x8xdGUrliXFzSnUw",
		"eUXGGm1YGsMAS7vkcx6JOJdOGHPem5gQp4taiCfCLB8",
		"ROQFvPThGrW4RuWLoL9tq9I9zJ42fK4XywyRtbOz/EQ",
		"ohD8VZEXGWo6Ez8GSEJQ9WpafgLFsOfLOtGGQCQo6Og",
		"ohD8VZEXGWo6Ez8GSEJQ9WpafgLFsOfLOtGGQCQo6Og",
	}
	entries, err := ParseSSHKnownHostsFromData(Test_ValidSSHKnownHostsData)
	assert.Nil(t, err)
	assert.Equal(t, len(entries), 7)
	for idx, entry := range entries {
		_, pubKey, err := KnownHostsLineToPublicKey(entry)
		assert.Nil(t, err)
		fp := SSHFingerprintSHA256(pubKey)
		assert.Equal(t, fp, fingerprints[idx])
	}
}

func Test_SSHFingerPrintSHA256FromString(t *testing.T) {
	// actual SHA256 fingerprints for keys defined above
	fingerprints := [...]string{
		"zzXQOXSRBEiUtuE8AikJYKwbHaxvSc0ojez9YXaGp1A",
		"nThbg6kXUpJWGl7E1IGOCspRomTxdCARLviKw6E5SY8",
		"HbW3g8zUjNSksFbqTiUWPWg2Bq1x8xdGUrliXFzSnUw",
		"eUXGGm1YGsMAS7vkcx6JOJdOGHPem5gQp4taiCfCLB8",
		"ROQFvPThGrW4RuWLoL9tq9I9zJ42fK4XywyRtbOz/EQ",
		"ohD8VZEXGWo6Ez8GSEJQ9WpafgLFsOfLOtGGQCQo6Og",
		"ohD8VZEXGWo6Ez8GSEJQ9WpafgLFsOfLOtGGQCQo6Og",
	}
	entries, err := ParseSSHKnownHostsFromData(Test_ValidSSHKnownHostsData)
	assert.Nil(t, err)
	assert.Equal(t, len(entries), 7)
	for idx, entry := range entries {
		fp := SSHFingerprintSHA256FromString(entry)
		assert.Equal(t, fp, fingerprints[idx])
	}
}

func Test_ServerNameWithoutPort(t *testing.T) {
	hostNames := map[string]string{
		"localhost":            "localhost",
		"localhost:9443":       "localhost",
		"localhost:":           "localhost",
		"localhost:abc":        "localhost",
		"localhost.:22":        "localhost.",
		"foo.example.com:443":  "foo.example.com",
		"foo.example.com.:443": "foo.example.com.",
	}
	for inp, res := range hostNames {
		assert.Equal(t, res, ServerNameWithoutPort(inp))
	}
}

func Test_ValidHostnames(t *testing.T) {
	hostNames := map[string]bool{
		"localhost":                          true,
		"localhost.localdomain":              true,
		"foo.example.com":                    true,
		"argocd-server.svc.kubernetes.local": true,
		"localhost.":                         true,
		"github.com.":                        true,
		"foo_bar.example.com":                true,
		"_svc.example.com":                   true,
		"_svc.example_.com":                  false,
		"_.example.com":                      false,
		"localhost..":                        false,
		"localhost..localdomain":             false,
		".localhost":                         false,
		"local_host":                         true,
		"localhost.local_domain":             true,
	}

	for hostName, valid := range hostNames {
		t.Run(fmt.Sprintf("Test validity for hostname %s", hostName), func(t *testing.T) {
			assert.Equal(t, valid, IsValidHostname(hostName, false))
		})
	}
}

func Test_ValidFQDNs(t *testing.T) {
	hostNames := map[string]bool{
		"localhost":                          false,
		"localhost.localdomain":              false,
		"foo.example.com.":                   true,
		"argocd-server.svc.kubernetes.local": false,
		"localhost.":                         true,
		"github.com.":                        true,
		"localhost..":                        false,
		"localhost..localdomain":             false,
		"localhost..localdomain.":            false,
		".localhost":                         false,
		"local_host":                         false,
		"localhost.local_domain":             false,
		"localhost.local_domain.":            false,
	}

	for hostName, valid := range hostNames {
		assert.Equal(t, valid, IsValidHostname(hostName, true))
	}
}

func Test_EscapeBracketPattern(t *testing.T) {
	// input: expected output
	patternList := map[string]string{
		"foo.bar":         "foo.bar",
		"[foo.bar]":       `\[foo.bar\]`,
		"foo[bar]baz":     `foo\[bar\]baz`,
		`foo\[bar\]baz`:   `foo\\[bar\\]baz`,
		"foo[[[bar]]]baz": `foo\[\[\[bar\]\]\]baz`,
	}

	for original, expected := range patternList {
		assert.Equal(t, nonBracketedPattern(original), expected)
	}
}

func TestGetTLSCertificateDataPath(t *testing.T) {
	t.Run("Get default path", func(t *testing.T) {
		os.Setenv(common.EnvVarTLSDataPath, "")
		path := GetTLSCertificateDataPath()
		assert.Equal(t, common.DefaultPathTLSConfig, path)
	})

	t.Run("Get custom path", func(t *testing.T) {
		os.Setenv(common.EnvVarTLSDataPath, "/some/where")
		path := GetTLSCertificateDataPath()
		assert.Equal(t, "/some/where", path)
		os.Setenv(common.EnvVarTLSDataPath, "")
	})
}

func TestGetSSHKnownHostsDataPath(t *testing.T) {
	t.Run("Get default path", func(t *testing.T) {
		os.Setenv(common.EnvVarSSHDataPath, "")
		p := GetSSHKnownHostsDataPath()
		assert.Equal(t, path.Join(common.DefaultPathSSHConfig, "ssh_known_hosts"), p)
	})

	t.Run("Get custom path", func(t *testing.T) {
		os.Setenv(common.EnvVarSSHDataPath, "/some/where")
		path := GetSSHKnownHostsDataPath()
		assert.Equal(t, "/some/where/ssh_known_hosts", path)
		os.Setenv(common.EnvVarSSHDataPath, "")
	})
}

func TestGetCertificateForConnect(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		temppath, err := ioutil.TempDir("", "argocd-cert-test")
		if err != nil {
			panic(err)
		}
		cert, err := ioutil.ReadFile("../../test/fixture/certs/argocd-test-server.crt")
		if err != nil {
			panic(err)
		}
		err = ioutil.WriteFile(path.Join(temppath, "127.0.0.1"), cert, 0666)
		if err != nil {
			panic(err)
		}
		defer os.RemoveAll(temppath)
		os.Setenv(common.EnvVarTLSDataPath, temppath)
		certs, err := GetCertificateForConnect("127.0.0.1")
		assert.NoError(t, err)
		assert.Len(t, certs, 1)
	})

	t.Run("No cert found", func(t *testing.T) {
		temppath, err := ioutil.TempDir("", "argocd-cert-test")
		if err != nil {
			panic(err)
		}
		defer os.RemoveAll(temppath)
		os.Setenv(common.EnvVarTLSDataPath, temppath)
		certs, err := GetCertificateForConnect("127.0.0.1")
		assert.NoError(t, err)
		assert.Len(t, certs, 0)
	})

	t.Run("No valid cert in file", func(t *testing.T) {
		temppath, err := ioutil.TempDir("", "argocd-cert-test")
		if err != nil {
			panic(err)
		}
		err = ioutil.WriteFile(path.Join(temppath, "127.0.0.1"), []byte("foobar"), 0666)
		if err != nil {
			panic(err)
		}
		defer os.RemoveAll(temppath)
		os.Setenv(common.EnvVarTLSDataPath, temppath)
		certs, err := GetCertificateForConnect("127.0.0.1")
		assert.Error(t, err)
		assert.Len(t, certs, 0)
		assert.Contains(t, err.Error(), "No certificates found")
	})

}

func TestGetCertBundlePathForRepository(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		temppath, err := ioutil.TempDir("", "argocd-cert-test")
		if err != nil {
			panic(err)
		}
		cert, err := ioutil.ReadFile("../../test/fixture/certs/argocd-test-server.crt")
		if err != nil {
			panic(err)
		}
		err = ioutil.WriteFile(path.Join(temppath, "127.0.0.1"), cert, 0666)
		if err != nil {
			panic(err)
		}
		defer os.RemoveAll(temppath)
		os.Setenv(common.EnvVarTLSDataPath, temppath)
		certpath, err := GetCertBundlePathForRepository("127.0.0.1")
		assert.NoError(t, err)
		assert.Equal(t, certpath, path.Join(temppath, "127.0.0.1"))
	})

	t.Run("No cert found", func(t *testing.T) {
		temppath, err := ioutil.TempDir("", "argocd-cert-test")
		if err != nil {
			panic(err)
		}
		defer os.RemoveAll(temppath)
		os.Setenv(common.EnvVarTLSDataPath, temppath)
		certpath, err := GetCertBundlePathForRepository("127.0.0.1")
		assert.NoError(t, err)
		assert.Empty(t, certpath)
	})

	t.Run("No valid cert in file", func(t *testing.T) {
		temppath, err := ioutil.TempDir("", "argocd-cert-test")
		if err != nil {
			panic(err)
		}
		err = ioutil.WriteFile(path.Join(temppath, "127.0.0.1"), []byte("foobar"), 0666)
		if err != nil {
			panic(err)
		}
		defer os.RemoveAll(temppath)
		os.Setenv(common.EnvVarTLSDataPath, temppath)
		certpath, err := GetCertBundlePathForRepository("127.0.0.1")
		assert.NoError(t, err)
		assert.Empty(t, certpath)
	})

}
