package db

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/settings"
)

const (
	Test_Cert1CN = "CN=foo.example.com,OU=SpecOps,O=Capone\\, Inc,L=Chicago,ST=IL,C=US"
	Test_Cert2CN = "CN=bar.example.com,OU=Testsuite,O=Testing Corp,L=Hanover,ST=Lower Saxony,C=DE"
)

var Test_TLS_Subjects []string = []string{
	"CN=foo.example.com,OU=SpecOps,O=Capone\\, Inc,L=Chicago,ST=IL,C=US",
	"CN=bar.example.com,OU=Testsuite,O=Testing Corp,L=Hanover,ST=Lower Saxony,C=DE",
}

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
bitbucket.org ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQDQeJzhupRu0u0cdegZIa8e86EG2qOCsIsD1Xw0xSeiPDlCr7kq97NLmMbpKTX6Esc30NuoqEEHCuc7yWtwp8dI76EEEB1VqY9QJq6vk+aySyboD5QF61I/1WeTwu+deCbgKMGbUijeXhtfbxSxm6JwGrXrhBdofTsbKRUsrN1WoNgUa8uqN1Vx6WAJw1JHPhglEGGHea6QICwJOAr/6mrui/oB7pkaWKHj3z7d1IC4KWLtY47elvjbaTlkN04Kc/5LFEirorGYVbt15kAUlqGM65pk6ZBxtaO3+30LVlORZkxOh+LKL/BvbZ/iRNhItLqNyieoQj/uh/7Iv4uyH/cV/0b4WDSd3DptigWq84lJubb9t/DnZlrJazxyDCulTmKdOR7vs9gMTo+uoIrPSb8ScTtvw65+odKAlBj59dhnVp9zd7QUojOpXlL62Aw56U4oO+FALuevvMjiWeavKhJqlR7i5n9srYcrNV7ttmDw7kf/97P5zauIhxcjX+xHv4M=
# GitHub
github.com ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQCj7ndNxQowgcQnjshcLrqPEiiphnt+VTTvDP6mHBL9j1aNUkY4Ue1gvwnGLVlOhGeYrnZaMgRK6+PKCUXaDbC7qtbW8gIkhL7aGCsOr/C56SJMy/BCZfxd1nWzAOxSDPgVsmerOBYfNqltV9/hWCqBywINIR+5dIg6JTJ72pcEpEjcYgXkE2YEFXV1JHnsKgbLWNlhScqb2UmyRkQyytRLtL+38TGxkxCflmO+5Z8CSSNY7GidjMIZ7Q4zMjA2n1nGrlTDkzwDCsw+wqFPGQA179cnfGWOWRVruj16z6XyvxvjJwbz0wQZ75XK5tKSb7FNyeIEs4TT4jk+S4dhPeAUC5y+bDYirYgM4GC7uEnztnZyaVWQ7B381AK4Qdrwt51ZqExKbQpTUNn+EjqoTwvqNj4kqx5QUCI0ThS/YkOxJCXmPUWZbhjpCg56i+2aB6CmK2JGhn57K5mj0MNdBXA4/WnwH6XoPWJzK5Nyu2zB3nAZp+S5hpQs+p1vN1/wsjk=
# GitLab
gitlab.com ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdHAyNTYAAAAIbmlzdHAyNTYAAABBBFSMqzJeV9rUzU4kWitGjeR4PWSa29SPqJ1fVkhtj3Hw9xjLVXVYrU9QlYWrOLXBpQ6KWjbjTDTdDkoohFzgbEY=
gitlab.com ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIAfuCHKVTjquxvt6CM6tdG4SLp1Btn/nOeHHE5UOzRdf
gitlab.com ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQCsj2bNKTBSpIYDEGk9KxsGh3mySTRgMtXL583qmBpzeQ+jqCMRgBqB98u3z++J1sKlXHWfM9dyhSevkMwSbhoR8XIq/U0tCNyokEi/ueaBMCvbcTHhO7FcwzY92WK4Yt0aGROY5qX2UKSeOvuP4D6TPqKF1onrSzH9bx9XUf2lEdWT/ia1NEKjunUqu1xOB/StKDHMoX4/OKyIzuS0q/T1zOATthvasJFoPrAjkohTyaDUz2LN5JoH839hViyEG82yB+MjcFV5MU3N1l1QL3cVUCh93xSaua1N85qivl+siMkPGbO5xR/En4iEY6K2XPASUEMaieWVNTRCtJ4S8H+9
# Azure
ssh.dev.azure.com ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC7Hr1oTWqNqOlzGJOfGJ4NakVyIzf1rXYd4d7wo6jBlkLvCA4odBlL0mDUyZ0/QUfTTqeu+tm22gOsv+VrVTMk6vwRU75gY/y9ut5Mb3bR5BV58dKXyq9A9UeB5Cakehn5Zgm6x1mKoVyf+FFn26iYqXJRgzIZZcZ5V6hrE0Qg39kZm4az48o0AUbf6Sp4SLdvnuMa2sVNwHBboS7EJkm57XQPVU3/QpyNLHbWDdzwtrlS+ez30S3AdYhLKEOxAG8weOnyrtLJAUen9mTkol8oII1edf7mWWbWVf0nBmly21+nZcmCTISQBtdcyPaEno7fFQMDD26/s0lfKob4Kw8H
vs-ssh.visualstudio.com ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC7Hr1oTWqNqOlzGJOfGJ4NakVyIzf1rXYd4d7wo6jBlkLvCA4odBlL0mDUyZ0/QUfTTqeu+tm22gOsv+VrVTMk6vwRU75gY/y9ut5Mb3bR5BV58dKXyq9A9UeB5Cakehn5Zgm6x1mKoVyf+FFn26iYqXJRgzIZZcZ5V6hrE0Qg39kZm4az48o0AUbf6Sp4SLdvnuMa2sVNwHBboS7EJkm57XQPVU3/QpyNLHbWDdzwtrlS+ez30S3AdYhLKEOxAG8weOnyrtLJAUen9mTkol8oII1edf7mWWbWVf0nBmly21+nZcmCTISQBtdcyPaEno7fFQMDD26/s0lfKob4Kw8H
`

const Test_InvalidSSHKnownHostsData = `
bitbucket.org AAAAB3NzaC1yc2EAAAADAQABAAABgQDQeJzhupRu0u0cdegZIa8e86EG2qOCsIsD1Xw0xSeiPDlCr7kq97NLmMbpKTX6Esc30NuoqEEHCuc7yWtwp8dI76EEEB1VqY9QJq6vk+aySyboD5QF61I/1WeTwu+deCbgKMGbUijeXhtfbxSxm6JwGrXrhBdofTsbKRUsrN1WoNgUa8uqN1Vx6WAJw1JHPhglEGGHea6QICwJOAr/6mrui/oB7pkaWKHj3z7d1IC4KWLtY47elvjbaTlkN04Kc/5LFEirorGYVbt15kAUlqGM65pk6ZBxtaO3+30LVlORZkxOh+LKL/BvbZ/iRNhItLqNyieoQj/uh/7Iv4uyH/cV/0b4WDSd3DptigWq84lJubb9t/DnZlrJazxyDCulTmKdOR7vs9gMTo+uoIrPSb8ScTtvw65+odKAlBj59dhnVp9zd7QUojOpXlL62Aw56U4oO+FALuevvMjiWeavKhJqlR7i5n9srYcrNV7ttmDw7kf/97P5zauIhxcjX+xHv4M=
# GitHub
github.com ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQCj7ndNxQowgcQnjshcLrqPEiiphnt+VTTvDP6mHBL9j1aNUkY4Ue1gvwnGLVlOhGeYrnZaMgRK6+PKCUXaDbC7qtbW8gIkhL7aGCsOr/C56SJMy/BCZfxd1nWzAOxSDPgVsmerOBYfNqltV9/hWCqBywINIR+5dIg6JTJ72pcEpEjcYgXkE2YEFXV1JHnsKgbLWNlhScqb2UmyRkQyytRLtL+38TGxkxCflmO+5Z8CSSNY7GidjMIZ7Q4zMjA2n1nGrlTDkzwDCsw+wqFPGQA179cnfGWOWRVruj16z6XyvxvjJwbz0wQZ75XK5tKSb7FNyeIEs4TT4jk+S4dhPeAUC5y+bDYirYgM4GC7uEnztnZyaVWQ7B381AK4Qdrwt51ZqExKbQpTUNn+EjqoTwvqNj4kqx5QUCI0ThS/YkOxJCXmPUWZbhjpCg56i+2aB6CmK2JGhn57K5mj0MNdBXA4/WnwH6XoPWJzK5Nyu2zB3nAZp+S5hpQs+p1vN1/wsjk=
# GitLab
gitlab.com ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdHAyNTYAAAAIbmlzdHAyNTYAAABBBFSMqzJeV9rUzU4kWitGjeR4PWSa29SPqJ1fVkhtj3Hw9xjLVXVYrU9QlYWrOLXBpQ6KWjbjTDTdDkoohFzgbEY=
gitlab.com ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIAfuCHKVTjquxvt6CM6tdG4SLp1Btn/nOeHHE5UOzRdf
gitlab.com ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQCsj2bNKTBSpIYDEGk9KxsGh3mySTRgMtXL583qmBpzeQ+jqCMRgBqB98u3z++J1sKlXHWfM9dyhSevkMwSbhoR8XIq/U0tCNyokEi/ueaBMCvbcTHhO7FcwzY92WK4Yt0aGROY5qX2UKSeOvuP4D6TPqKF1onrSzH9bx9XUf2lEdWT/ia1NEKjunUqu1xOB/StKDHMoX4/OKyIzuS0q/T1zOATthvasJFoPrAjkohTyaDUz2LN5JoH839hViyEG82yB+MjcFV5MU3N1l1QL3cVUCh93xSaua1N85qivl+siMkPGbO5xR/En4iEY6K2XPASUEMaieWVNTRCtJ4S8H+9
# Azure
ssh.dev.azure.com ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC7Hr1oTWqNqOlzGJOfGJ4NakVyIzf1rXYd4d7wo6jBlkLvCA4odBlL0mDUyZ0/QUfTTqeu+tm22gOsv+VrVTMk6vwRU75gY/y9ut5Mb3bR5BV58dKXyq9A9UeB5Cakehn5Zgm6x1mKoVyf+FFn26iYqXJRgzIZZcZ5V6hrE0Qg39kZm4az48o0AUbf6Sp4SLdvnuMa2sVNwHBboS7EJkm57XQPVU3/QpyNLHbWDdzwtrlS+ez30S3AdYhLKEOxAG8weOnyrtLJAUen9mTkol8oII1edf7mWWbWVf0nBmly21+nZcmCTISQBtdcyPaEno7fFQMDD26/s0lfKob4Kw8H
vs-ssh.visualstudio.com ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC7Hr1oTWqNqOlzGJOfGJ4NakVyIzf1rXYd4d7wo6jBlkLvCA4odBlL0mDUyZ0/QUfTTqeu+tm22gOsv+VrVTMk6vwRU75gY/y9ut5Mb3bR5BV58dKXyq9A9UeB5Cakehn5Zgm6x1mKoVyf+FFn26iYqXJRgzIZZcZ5V6hrE0Qg39kZm4az48o0AUbf6Sp4SLdvnuMa2sVNwHBboS7EJkm57XQPVU3/QpyNLHbWDdzwtrlS+ez30S3AdYhLKEOxAG8weOnyrtLJAUen9mTkol8oII1edf7mWWbWVf0nBmly21+nZcmCTISQBtdcyPaEno7fFQMDD26/s0lfKob4Kw8H
`

var Test_SSH_Hostname_Entries []string = []string{
	"bitbucket.org",
	"github.com",
	"gitlab.com",
	"gitlab.com",
	"gitlab.com",
	"ssh.dev.azure.com",
	"vs-ssh.visualstudio.com",
}

var Test_SSH_Subtypes []string = []string{
	"ssh-rsa",
	"ssh-rsa",
	"ecdsa-sha2-nistp256",
	"ssh-ed25519",
	"ssh-rsa",
	"ssh-rsa",
	"ssh-rsa",
}

var Test_TLS_Hostnames []string = []string{
	"test.example.com",
	"test.example.com",
	"github.com",
}

const (
	Test_NumSSHKnownHostsExpected   = 7
	Test_NumTLSCertificatesExpected = 3
)

func getCertClientset() *fake.Clientset {
	cm := v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "argocd-cm",
			Namespace: testNamespace,
			Labels: map[string]string{
				"app.kubernetes.io/part-of": "argocd",
			},
		},
		Data: nil,
	}

	sshCM := v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "argocd-ssh-known-hosts-cm",
			Namespace: testNamespace,
			Labels: map[string]string{
				"app.kubernetes.io/part-of": "argocd",
			},
		},
		Data: map[string]string{
			"ssh_known_hosts": Test_ValidSSHKnownHostsData,
		},
	}

	tlsCM := v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "argocd-tls-certs-cm",
			Namespace: testNamespace,
			Labels: map[string]string{
				"app.kubernetes.io/part-of": "argocd",
			},
		},
		Data: map[string]string{
			"test.example.com": Test_TLSValidMultiCert,
			"gitlab.com":       Test_TLSValidSingleCert,
		},
	}

	return fake.NewSimpleClientset([]runtime.Object{&cm, &sshCM, &tlsCM}...)
}

func Test_ListCertificate(t *testing.T) {
	clientset := getCertClientset()
	db := NewDB(testNamespace, settings.NewSettingsManager(context.Background(), clientset, testNamespace), clientset)
	assert.NotNil(t, db)

	// List all SSH known host entries from configuration.
	// Expected: List of 7 entries
	certList, err := db.ListRepoCertificates(context.Background(), &CertificateListSelector{
		HostNamePattern: "*",
		CertType:        "ssh",
	})
	require.NoError(t, err)
	assert.NotNil(t, certList)
	assert.Len(t, certList.Items, Test_NumSSHKnownHostsExpected)
	for idx, entry := range certList.Items {
		assert.Equal(t, entry.ServerName, Test_SSH_Hostname_Entries[idx])
		assert.Equal(t, entry.CertSubType, Test_SSH_Subtypes[idx])
	}

	// List all TLS certificates from configuration.
	// Expected: List of 3 entries
	certList, err = db.ListRepoCertificates(context.Background(), &CertificateListSelector{
		HostNamePattern: "*",
		CertType:        "https",
	})
	require.NoError(t, err)
	assert.NotNil(t, certList)
	assert.Len(t, certList.Items, Test_NumTLSCertificatesExpected)

	// List all certificates using selector
	// Expected: List of 10 entries
	certList, err = db.ListRepoCertificates(context.Background(), &CertificateListSelector{
		HostNamePattern: "*",
		CertType:        "*",
	})
	require.NoError(t, err)
	assert.NotNil(t, certList)
	assert.Len(t, certList.Items, Test_NumTLSCertificatesExpected+Test_NumSSHKnownHostsExpected)

	// List all certificates using nil selector
	// Expected: List of 10 entries
	certList, err = db.ListRepoCertificates(context.Background(), nil)
	require.NoError(t, err)
	assert.NotNil(t, certList)
	assert.Len(t, certList.Items, Test_NumTLSCertificatesExpected+Test_NumSSHKnownHostsExpected)

	// List all certificates matching a host name pattern
	// Expected: List of 4 entries, all with servername gitlab.com
	certList, err = db.ListRepoCertificates(context.Background(), &CertificateListSelector{
		HostNamePattern: "gitlab.com",
		CertType:        "*",
	})
	require.NoError(t, err)
	assert.NotNil(t, certList)
	assert.Len(t, certList.Items, 4)
	for _, entry := range certList.Items {
		assert.Equal(t, "gitlab.com", entry.ServerName)
	}

	// List all TLS certificates matching a host name pattern
	certList, err = db.ListRepoCertificates(context.Background(), &CertificateListSelector{
		HostNamePattern: "gitlab.com",
		CertType:        "https",
	})
	require.NoError(t, err)
	assert.NotNil(t, certList)
	assert.Len(t, certList.Items, 1)
	assert.Equal(t, "gitlab.com", certList.Items[0].ServerName)
	assert.Equal(t, "https", certList.Items[0].CertType)
}

func Test_CreateSSHKnownHostEntries(t *testing.T) {
	clientset := getCertClientset()
	db := NewDB(testNamespace, settings.NewSettingsManager(context.Background(), clientset, testNamespace), clientset)
	assert.NotNil(t, db)

	// Valid known hosts entry
	certList, err := db.CreateRepoCertificate(context.Background(), &v1alpha1.RepositoryCertificateList{
		Items: []v1alpha1.RepositoryCertificate{
			{
				ServerName: "foo.example.com",
				CertType:   "ssh",
				CertData:   []byte("ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQDioSMcGxdVkHaQzRjP71nY4mgVHXjuZiYN9NBiUxNZ0DYGjTIENI3uV45XxrS6PQfoyekUlVlHK2jwpcPrqAg6rlAdMD5WIxzvCnFjCuPA6Ljk8p0ZmYbvriDcgtj+UfGEdyUTgxH2gch6KwTY0eAbLue15IuXtoNzpLxk29iGRi5ZXNAbSBjeB3hm2PKLa6LnDqdkvc+nqoYqn1Fvx7ZJIh0apBCJpOtHPON4rnl7QQvNg9pWulZ5GKcpYMRfTpvHyFTEyrsVT5GH38l9s355GqU7GxQ/i6Tj1D0MKrIB2WmdjOnujM/ELLsrkYspMhn8ZRpCphN/LTcrOWsb0AM69drvYlhc6cnNAtC4UXp0GUy1HsBiJCsUm9/1Gz23VLDRvWop8yE8+PE3Ho5eL7ad9wmOG0mSOYEqVvAstmd8vzbD6oRuY8qV8X3tt9ph2tMAve0Qbo0NN3c51c9OfdXtJaSyckjEjaK7zjnArnYfladZZVlf2Tv8FsV0sJmfSAE="),
			},
		},
	}, false)
	require.NoError(t, err)
	assert.NotNil(t, certList)
	assert.Len(t, certList.Items, 1)

	// Valid known hosts entry
	certList, err = db.CreateRepoCertificate(context.Background(), &v1alpha1.RepositoryCertificateList{
		Items: []v1alpha1.RepositoryCertificate{
			{
				ServerName: "[foo.example.com]:2222",
				CertType:   "ssh",
				CertData:   []byte("ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQDioSMcGxdVkHaQzRjP71nY4mgVHXjuZiYN9NBiUxNZ0DYGjTIENI3uV45XxrS6PQfoyekUlVlHK2jwpcPrqAg6rlAdMD5WIxzvCnFjCuPA6Ljk8p0ZmYbvriDcgtj+UfGEdyUTgxH2gch6KwTY0eAbLue15IuXtoNzpLxk29iGRi5ZXNAbSBjeB3hm2PKLa6LnDqdkvc+nqoYqn1Fvx7ZJIh0apBCJpOtHPON4rnl7QQvNg9pWulZ5GKcpYMRfTpvHyFTEyrsVT5GH38l9s355GqU7GxQ/i6Tj1D0MKrIB2WmdjOnujM/ELLsrkYspMhn8ZRpCphN/LTcrOWsb0AM69drvYlhc6cnNAtC4UXp0GUy1HsBiJCsUm9/1Gz23VLDRvWop8yE8+PE3Ho5eL7ad9wmOG0mSOYEqVvAstmd8vzbD6oRuY8qV8X3tt9ph2tMAve0Qbo0NN3c51c9OfdXtJaSyckjEjaK7zjnArnYfladZZVlf2Tv8FsV0sJmfSAE="),
			},
		},
	}, false)
	require.NoError(t, err)
	assert.NotNil(t, certList)
	assert.Len(t, certList.Items, 1)

	// Invalid hostname
	// Result: Error
	certList, err = db.CreateRepoCertificate(context.Background(), &v1alpha1.RepositoryCertificateList{
		Items: []v1alpha1.RepositoryCertificate{
			{
				ServerName: "foo..example.com",
				CertType:   "ssh",
				CertData:   []byte("ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQDioSMcGxdVkHaQzRjP71nY4mgVHXjuZiYN9NBiUxNZ0DYGjTIENI3uV45XxrS6PQfoyekUlVlHK2jwpcPrqAg6rlAdMD5WIxzvCnFjCuPA6Ljk8p0ZmYbvriDcgtj+UfGEdyUTgxH2gch6KwTY0eAbLue15IuXtoNzpLxk29iGRi5ZXNAbSBjeB3hm2PKLa6LnDqdkvc+nqoYqn1Fvx7ZJIh0apBCJpOtHPON4rnl7QQvNg9pWulZ5GKcpYMRfTpvHyFTEyrsVT5GH38l9s355GqU7GxQ/i6Tj1D0MKrIB2WmdjOnujM/ELLsrkYspMhn8ZRpCphN/LTcrOWsb0AM69drvYlhc6cnNAtC4UXp0GUy1HsBiJCsUm9/1Gz23VLDRvWop8yE8+PE3Ho5eL7ad9wmOG0mSOYEqVvAstmd8vzbD6oRuY8qV8X3tt9ph2tMAve0Qbo0NN3c51c9OfdXtJaSyckjEjaK7zjnArnYfladZZVlf2Tv8FsV0sJmfSAE="),
			},
		},
	}, false)
	require.Error(t, err)
	assert.Nil(t, certList)

	// Check if it really was added
	// Result: List of 1 entry
	certList, err = db.ListRepoCertificates(context.Background(), &CertificateListSelector{
		HostNamePattern: "foo.example.com",
		CertType:        "ssh",
	})
	require.NoError(t, err)
	assert.NotNil(t, certList)
	assert.Len(t, certList.Items, 1)

	// Existing cert, same data, no upsert
	// Result: no error, should return 0 added certificates
	certList, err = db.CreateRepoCertificate(context.Background(), &v1alpha1.RepositoryCertificateList{
		Items: []v1alpha1.RepositoryCertificate{
			{
				ServerName: "foo.example.com",
				CertType:   "ssh",
				CertData:   []byte("ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQDioSMcGxdVkHaQzRjP71nY4mgVHXjuZiYN9NBiUxNZ0DYGjTIENI3uV45XxrS6PQfoyekUlVlHK2jwpcPrqAg6rlAdMD5WIxzvCnFjCuPA6Ljk8p0ZmYbvriDcgtj+UfGEdyUTgxH2gch6KwTY0eAbLue15IuXtoNzpLxk29iGRi5ZXNAbSBjeB3hm2PKLa6LnDqdkvc+nqoYqn1Fvx7ZJIh0apBCJpOtHPON4rnl7QQvNg9pWulZ5GKcpYMRfTpvHyFTEyrsVT5GH38l9s355GqU7GxQ/i6Tj1D0MKrIB2WmdjOnujM/ELLsrkYspMhn8ZRpCphN/LTcrOWsb0AM69drvYlhc6cnNAtC4UXp0GUy1HsBiJCsUm9/1Gz23VLDRvWop8yE8+PE3Ho5eL7ad9wmOG0mSOYEqVvAstmd8vzbD6oRuY8qV8X3tt9ph2tMAve0Qbo0NN3c51c9OfdXtJaSyckjEjaK7zjnArnYfladZZVlf2Tv8FsV0sJmfSAE="),
			},
		},
	}, false)
	require.NoError(t, err)
	assert.NotNil(t, certList)
	assert.Empty(t, certList.Items)

	// Existing cert, different data, no upsert
	// Result: Error
	certList, err = db.CreateRepoCertificate(context.Background(), &v1alpha1.RepositoryCertificateList{
		Items: []v1alpha1.RepositoryCertificate{
			{
				ServerName: "foo.example.com",
				CertType:   "ssh",
				CertData:   []byte("ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQCj7ndNxQowgcQnjshcLrqPEiiphnt+VTTvDP6mHBL9j1aNUkY4Ue1gvwnGLVlOhGeYrnZaMgRK6+PKCUXaDbC7qtbW8gIkhL7aGCsOr/C56SJMy/BCZfxd1nWzAOxSDPgVsmerOBYfNqltV9/hWCqBywINIR+5dIg6JTJ72pcEpEjcYgXkE2YEFXV1JHnsKgbLWNlhScqb2UmyRkQyytRLtL+38TGxkxCflmO+5Z8CSSNY7GidjMIZ7Q4zMjA2n1nGrlTDkzwDCsw+wqFPGQA179cnfGWOWRVruj16z6XyvxvjJwbz0wQZ75XK5tKSb7FNyeIEs4TT4jk+S4dhPeAUC5y+bDYirYgM4GC7uEnztnZyaVWQ7B381AK4Qdrwt51ZqExKbQpTUNn+EjqoTwvqNj4kqx5QUCI0ThS/YkOxJCXmPUWZbhjpCg56i+2aB6CmK2JGhn57K5mj0MNdBXA4/WnwH6XoPWJzK5Nyu2zB3nAZp+S5hpQs+p1vN1/wsjk="),
			},
		},
	}, false)
	require.Error(t, err)
	assert.Nil(t, certList)

	// Existing cert, different data, upsert
	certList, err = db.CreateRepoCertificate(context.Background(), &v1alpha1.RepositoryCertificateList{
		Items: []v1alpha1.RepositoryCertificate{
			{
				ServerName: "foo.example.com",
				CertType:   "ssh",
				CertData:   []byte("ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQCj7ndNxQowgcQnjshcLrqPEiiphnt+VTTvDP6mHBL9j1aNUkY4Ue1gvwnGLVlOhGeYrnZaMgRK6+PKCUXaDbC7qtbW8gIkhL7aGCsOr/C56SJMy/BCZfxd1nWzAOxSDPgVsmerOBYfNqltV9/hWCqBywINIR+5dIg6JTJ72pcEpEjcYgXkE2YEFXV1JHnsKgbLWNlhScqb2UmyRkQyytRLtL+38TGxkxCflmO+5Z8CSSNY7GidjMIZ7Q4zMjA2n1nGrlTDkzwDCsw+wqFPGQA179cnfGWOWRVruj16z6XyvxvjJwbz0wQZ75XK5tKSb7FNyeIEs4TT4jk+S4dhPeAUC5y+bDYirYgM4GC7uEnztnZyaVWQ7B381AK4Qdrwt51ZqExKbQpTUNn+EjqoTwvqNj4kqx5QUCI0ThS/YkOxJCXmPUWZbhjpCg56i+2aB6CmK2JGhn57K5mj0MNdBXA4/WnwH6XoPWJzK5Nyu2zB3nAZp+S5hpQs+p1vN1/wsjk="),
			},
		},
	}, true)
	require.NoError(t, err)
	assert.NotNil(t, certList)
	assert.Len(t, certList.Items, 1)

	// Invalid known hosts entry, case 1: key sub type missing
	// Result: Error
	certList, err = db.CreateRepoCertificate(context.Background(), &v1alpha1.RepositoryCertificateList{
		Items: []v1alpha1.RepositoryCertificate{
			{
				ServerName: "bar.example.com",
				CertType:   "ssh",
				CertData:   []byte("AAAAB3NzaC1yc2EAAAADAQABAAABgQDioSMcGxdVkHaQzRjP71nY4mgVHXjuZiYN9NBiUxNZ0DYGjTIENI3uV45XxrS6PQfoyekUlVlHK2jwpcPrqAg6rlAdMD5WIxzvCnFjCuPA6Ljk8p0ZmYbvriDcgtj+UfGEdyUTgxH2gch6KwTY0eAbLue15IuXtoNzpLxk29iGRi5ZXNAbSBjeB3hm2PKLa6LnDqdkvc+nqoYqn1Fvx7ZJIh0apBCJpOtHPON4rnl7QQvNg9pWulZ5GKcpYMRfTpvHyFTEyrsVT5GH38l9s355GqU7GxQ/i6Tj1D0MKrIB2WmdjOnujM/ELLsrkYspMhn8ZRpCphN/LTcrOWsb0AM69drvYlhc6cnNAtC4UXp0GUy1HsBiJCsUm9/1Gz23VLDRvWop8yE8+PE3Ho5eL7ad9wmOG0mSOYEqVvAstmd8vzbD6oRuY8qV8X3tt9ph2tMAve0Qbo0NN3c51c9OfdXtJaSyckjEjaK7zjnArnYfladZZVlf2Tv8FsV0sJmfSAE="),
			},
		},
	}, false)
	require.Error(t, err)
	assert.Nil(t, certList)

	// Invalid known hosts entry, case 2: invalid base64 data
	// Result: Error
	certList, err = db.CreateRepoCertificate(context.Background(), &v1alpha1.RepositoryCertificateList{
		Items: []v1alpha1.RepositoryCertificate{
			{
				ServerName: "bar.example.com",
				CertType:   "ssh",
				CertData:   []byte("ssh-rsa AAAAB3Nza1yc2EAAAADAQABAAABgQDioSMcGxdVkHaQzRjP71nY4mgVHXjuZiYN9NBiUxNZ0DYGjTIENI3uV45XxrS6PQfoyekUlVlHK2jwpcPrqAg6rlAdMD5WIxzvCnFjCuPA6Ljk8p0ZmYbvriDcgtj+UfGEdyUTgxH2gch6KwTY0eAbLue15IuXtoNzpLxk29iGRi5ZXNAbSBjeB3hm2PKLa6LnDqdkvc+nqoYqn1Fvx7ZJIh0apBCJpOtHPON4rnl7QQvNg9pWulZ5GKcpYMRfTpvHyFTEyrsVT5GH38l9s355GqU7GxQ/i6Tj1D0MKrIB2WmdjOnujM/ELLsrkYspMhn8ZRpCphN/LTcrOWsb0AM69drvYlhc6cnNAtC4UXp0GUy1HsBiJCsUm9/1Gz23VLDRvWop8yE8+PE3Ho5eL7ad9wmOG0mSOYEqVvAstmd8vzbD6oRuY8qV8X3tt9ph2tMAve0Qbo0NN3c51c9OfdXtJaSyckjEjaK7zjnArnYfladZZVlf2Tv8FsV0sJmfSAE="),
			},
		},
	}, false)
	require.Error(t, err)
	assert.Nil(t, certList)
}

func Test_CreateTLSCertificates(t *testing.T) {
	clientset := getCertClientset()
	db := NewDB(testNamespace, settings.NewSettingsManager(context.Background(), clientset, testNamespace), clientset)
	assert.NotNil(t, db)

	// Valid TLS certificate
	// Expected: List of 1 entry
	certList, err := db.CreateRepoCertificate(context.Background(), &v1alpha1.RepositoryCertificateList{
		Items: []v1alpha1.RepositoryCertificate{
			{
				ServerName: "foo.example.com",
				CertType:   "https",
				CertData:   []byte(Test_TLSValidSingleCert),
			},
		},
	}, false)
	require.NoError(t, err)
	assert.NotNil(t, certList)
	assert.Len(t, certList.Items, 1)

	// Invalid hostname
	// Result: Error
	certList, err = db.CreateRepoCertificate(context.Background(), &v1alpha1.RepositoryCertificateList{
		Items: []v1alpha1.RepositoryCertificate{
			{
				ServerName: "foo..example",
				CertType:   "https",
				CertData:   []byte(Test_TLSValidSingleCert),
			},
		},
	}, false)
	require.Error(t, err)
	assert.Nil(t, certList)

	// Check if it really was added
	// Result: Return new certificate
	certList, err = db.ListRepoCertificates(context.Background(), &CertificateListSelector{
		HostNamePattern: "foo.example.com",
		CertType:        "https",
	})
	require.NoError(t, err)
	assert.NotNil(t, certList)
	assert.Len(t, certList.Items, 1)

	// Valid TLS certificates, multiple PEMs in data
	// Expected: List of 2 entry
	certList, err = db.CreateRepoCertificate(context.Background(), &v1alpha1.RepositoryCertificateList{
		Items: []v1alpha1.RepositoryCertificate{
			{
				ServerName: "bar.example.com",
				CertType:   "https",
				CertData:   []byte(Test_TLSValidMultiCert),
			},
		},
	}, false)
	require.NoError(t, err)
	assert.NotNil(t, certList)
	assert.Len(t, certList.Items, 2)

	// Check if it really was added
	// Result: Return new certificate
	certList, err = db.ListRepoCertificates(context.Background(), &CertificateListSelector{
		HostNamePattern: "bar.example.com",
		CertType:        "https",
	})
	require.NoError(t, err)
	assert.NotNil(t, certList)
	assert.Len(t, certList.Items, 2)

	// Valid TLS certificate, existing cert, same data, no upsert
	// Expected: List of 0 entry
	certList, err = db.CreateRepoCertificate(context.Background(), &v1alpha1.RepositoryCertificateList{
		Items: []v1alpha1.RepositoryCertificate{
			{
				ServerName: "foo.example.com",
				CertType:   "https",
				CertData:   []byte(Test_TLSValidSingleCert),
			},
		},
	}, false)
	require.NoError(t, err)
	assert.NotNil(t, certList)
	assert.Empty(t, certList.Items)

	// Valid TLS certificate, existing cert, different data, no upsert
	// Expected: Error
	certList, err = db.CreateRepoCertificate(context.Background(), &v1alpha1.RepositoryCertificateList{
		Items: []v1alpha1.RepositoryCertificate{
			{
				ServerName: "foo.example.com",
				CertType:   "https",
				CertData:   []byte(Test_TLSValidMultiCert),
			},
		},
	}, false)
	require.Error(t, err)
	assert.Nil(t, certList)

	// Valid TLS certificate, existing cert, different data, upsert
	// Expected: List of 2 entries
	certList, err = db.CreateRepoCertificate(context.Background(), &v1alpha1.RepositoryCertificateList{
		Items: []v1alpha1.RepositoryCertificate{
			{
				ServerName: "foo.example.com",
				CertType:   "https",
				CertData:   []byte(Test_TLSValidMultiCert),
			},
		},
	}, true)
	require.NoError(t, err)
	assert.NotNil(t, certList)
	assert.Len(t, certList.Items, 2)

	// Check if upsert was successful
	// Expected: List of 2 entries, matching hostnames & cert types
	certList, err = db.ListRepoCertificates(context.Background(), &CertificateListSelector{
		HostNamePattern: "foo.example.com",
		CertType:        "https",
	})
	require.NoError(t, err)
	assert.NotNil(t, certList)
	assert.Len(t, certList.Items, 2)
	for _, entry := range certList.Items {
		assert.Equal(t, "foo.example.com", entry.ServerName)
		assert.Equal(t, "https", entry.CertType)
	}

	// Invalid PEM data, new cert
	// Expected: Error
	certList, err = db.CreateRepoCertificate(context.Background(), &v1alpha1.RepositoryCertificateList{
		Items: []v1alpha1.RepositoryCertificate{
			{
				ServerName: "baz.example.com",
				CertType:   "https",
				CertData:   []byte(Test_TLSInvalidPEMData),
			},
		},
	}, false)
	require.Error(t, err)
	assert.Nil(t, certList)

	// Valid PEM data, new cert, but invalid certificate
	// Expected: Error
	certList, err = db.CreateRepoCertificate(context.Background(), &v1alpha1.RepositoryCertificateList{
		Items: []v1alpha1.RepositoryCertificate{
			{
				ServerName: "baz.example.com",
				CertType:   "https",
				CertData:   []byte(Test_TLSInvalidSingleCert),
			},
		},
	}, false)
	require.Error(t, err)
	assert.Nil(t, certList)

	// Invalid PEM data, existing cert, upsert
	// Expected: Error
	certList, err = db.CreateRepoCertificate(context.Background(), &v1alpha1.RepositoryCertificateList{
		Items: []v1alpha1.RepositoryCertificate{
			{
				ServerName: "baz.example.com",
				CertType:   "https",
				CertData:   []byte(Test_TLSInvalidPEMData),
			},
		},
	}, true)
	require.Error(t, err)
	assert.Nil(t, certList)

	// Valid PEM data, existing cert, but invalid certificate, upsert
	// Expected: Error
	certList, err = db.CreateRepoCertificate(context.Background(), &v1alpha1.RepositoryCertificateList{
		Items: []v1alpha1.RepositoryCertificate{
			{
				ServerName: "baz.example.com",
				CertType:   "https",
				CertData:   []byte(Test_TLSInvalidSingleCert),
			},
		},
	}, true)
	require.Error(t, err)
	assert.Nil(t, certList)
}

func Test_RemoveSSHKnownHosts(t *testing.T) {
	clientset := getCertClientset()
	db := NewDB(testNamespace, settings.NewSettingsManager(context.Background(), clientset, testNamespace), clientset)
	assert.NotNil(t, db)

	// Remove single SSH known hosts entry by hostname
	// Expected: List of 1 entry
	certList, err := db.RemoveRepoCertificates(context.Background(), &CertificateListSelector{
		HostNamePattern: "github.com",
		CertType:        "ssh",
	})
	require.NoError(t, err)
	assert.NotNil(t, certList)
	assert.Len(t, certList.Items, 1)

	// Check whether entry was really removed
	// Expected: List of 0 entries
	certList, err = db.ListRepoCertificates(context.Background(), &CertificateListSelector{
		HostNamePattern: "github.com",
		CertType:        "ssh",
	})
	require.NoError(t, err)
	assert.NotNil(t, certList)
	assert.Empty(t, certList.Items)

	// Remove single SSH known hosts entry by sub type
	// Expected: List of 1 entry
	certList, err = db.RemoveRepoCertificates(context.Background(), &CertificateListSelector{
		CertType:    "ssh",
		CertSubType: "ssh-ed25519",
	})
	require.NoError(t, err)
	assert.NotNil(t, certList)
	assert.Len(t, certList.Items, 1)

	// Check whether entry was really removed
	// Expected: List of 0 entries
	certList, err = db.ListRepoCertificates(context.Background(), &CertificateListSelector{
		CertType:    "ssh",
		CertSubType: "ssh-ed25519",
	})
	require.NoError(t, err)
	assert.NotNil(t, certList)
	assert.Empty(t, certList.Items)

	// Remove all remaining SSH known hosts entries
	// Expected: List of 5 entry
	certList, err = db.RemoveRepoCertificates(context.Background(), &CertificateListSelector{
		CertType: "ssh",
	})
	require.NoError(t, err)
	assert.NotNil(t, certList)
	assert.Len(t, certList.Items, 5)

	// Check whether the entries were really removed
	// Expected: List of 0 entries
	certList, err = db.ListRepoCertificates(context.Background(), &CertificateListSelector{
		CertType: "ssh",
	})
	require.NoError(t, err)
	assert.NotNil(t, certList)
	assert.Empty(t, certList.Items)
}

func Test_RemoveTLSCertificates(t *testing.T) {
	clientset := getCertClientset()
	db := NewDB(testNamespace, settings.NewSettingsManager(context.Background(), clientset, testNamespace), clientset)
	assert.NotNil(t, db)

	// Remove single TLS certificate entry by hostname
	// Expected: List of 1 entry
	certList, err := db.RemoveRepoCertificates(context.Background(), &CertificateListSelector{
		HostNamePattern: "gitlab.com",
		CertType:        "https",
	})
	require.NoError(t, err)
	assert.NotNil(t, certList)
	assert.Len(t, certList.Items, 1)

	// Check whether entry was really removed
	// Expected: List of 0 entries
	certList, err = db.ListRepoCertificates(context.Background(), &CertificateListSelector{
		HostNamePattern: "gitlab.com",
		CertType:        "https",
	})
	require.NoError(t, err)
	assert.NotNil(t, certList)
	assert.Empty(t, certList.Items)

	// Remove all TLS certificate entry for hostname
	// Expected: List of 2 entry
	certList, err = db.RemoveRepoCertificates(context.Background(), &CertificateListSelector{
		HostNamePattern: "test.example.com",
		CertType:        "https",
	})
	require.NoError(t, err)
	assert.NotNil(t, certList)
	assert.Len(t, certList.Items, 2)

	// Check whether entries were really removed
	// Expected: List of 0 entries
	certList, err = db.ListRepoCertificates(context.Background(), &CertificateListSelector{
		HostNamePattern: "test.example.com",
		CertType:        "https",
	})
	require.NoError(t, err)
	assert.NotNil(t, certList)
	assert.Empty(t, certList.Items)
}
