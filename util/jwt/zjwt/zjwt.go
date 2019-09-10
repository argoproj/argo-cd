// The package provides a way to create compact JWTs, "zJWT".
//
// zJWTs for JWT with a large payload can be 1/3 the size of the original.
// This is achieved by gzipping the payload before base 64 encoding it.
//
// zJWTs are only compact if they are smaller than the original JWTs.
// as long as they are smaller than the zJWT representation.
//
// To help differentiate, zJWTs start with "zJWT/v1:"
package zjwt

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
)

var encoding = base64.RawStdEncoding

// some magic text that is easy to search the Internet for and find your way to docs
var magicNumber = "zJWT/v1"

// when to use ZJWT
// - "never" - never use it - good for it goes wrong
// - "" - when better
// - "always" - always use it - good for forcing this on for testing
var featureFlag = os.Getenv("ARGOCD_ZJWT_FEATURE_FLAG")

// the smallest size JWT we'll compress, 3k chosen as cookies max out at 4k
var minSize = 3000

// ZJWT turns a JWT into either a zJWT or return the original JWT, whichever is smaller.
func ZJWT(text string) (string, error) {
	if featureFlag == "never" || featureFlag != "always" && len(text) < minSize {
		return text, nil
	}
	parts := strings.SplitN(text, ".", 3)
	if len(parts) != 3 {
		return "", fmt.Errorf("JWT '%s' should have 3 dot-delimited parts", text)
	}
	header := parts[0]
	payload := parts[1]
	signature := parts[2]
	decodedPayload, err := encoding.DecodeString(payload)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	_, err = w.Write(decodedPayload)
	if err != nil {
		return "", err
	}
	err = w.Flush()
	if err != nil {
		return "", err
	}
	err = w.Close()
	if err != nil {
		return "", err
	}
	compressedPayload := encoding.EncodeToString(buf.Bytes())
	zJWT := fmt.Sprintf("%s.%s.%s.%s", magicNumber, header, compressedPayload, signature)
	if featureFlag == "always" || len(zJWT) < len(text) {
		return zJWT, nil
	} else {
		return text, nil
	}
}

// JWT expands either a zJWT or a JWT to a JWT.
func JWT(text string) (string, error) {
	parts := strings.SplitN(text, ".", 4)
	if len(parts) == 3 {
		return text, nil
	}
	if len(parts) != 4 {
		return "", fmt.Errorf("zJWT '%s' should have 4 dot-delimited parts", text)
	}
	part0 := parts[0]
	if part0 != magicNumber {
		return "", fmt.Errorf("the first part of the zJWT '%s' does not equal '%s'", part0, magicNumber)
	}
	header := parts[1]
	payload := parts[2]
	signature := parts[3]
	decodedPayload, err := encoding.DecodeString(payload)
	if err != nil {
		return "", err
	}
	r, err := gzip.NewReader(bytes.NewReader(decodedPayload))
	if err != nil {
		return "", err
	}
	uncompressedPayload, err := ioutil.ReadAll(r)
	if err != nil {
		return "", err
	}
	err = r.Close()
	if err != nil {
		return "", err
	}
	encodedPayload := encoding.EncodeToString(uncompressedPayload)
	jwt := fmt.Sprintf("%s.%s.%s", header, encodedPayload, signature)
	return jwt, nil
}
