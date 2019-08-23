// The package provides a way to create compact JWTs, "cJWT".
//
// cJWTs for JWT with a large payload can be 1/3 the size of the original.
// This is achieved by compressing the payload before base 64 encoding it.
//
// cJWTs are only compact if they are smaller than the original JWTs.
// as long as they are smaller than the cJWT representation.
//
// To help differentiate,
// * JWT have 3 dots. cJWTs have 4 dots.
// * cJWTs start with "cJWT/v1:"
//
package cjwt

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"strings"
)

var encoding = base64.RawStdEncoding
var magicNumber = "cJWT/v1:"

// CompactJWT JWT to either a cJWT or return the original JWT, whichever is smaller.
func CompactJWT(text string) (string, error) {
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
	cJWT := fmt.Sprintf("%s.%s.%s.%s", magicNumber, header, compressedPayload, signature)
	if len(cJWT) < len(text) {
		return cJWT, nil
	} else {
		return text, nil
	}
}

// JWT expands either a cJWT or a JWT to a JWT.
func JWT(text string) (string, error) {
	parts := strings.SplitN(text, ".", 4)
	if len(parts) == 3 {
		return text, nil
	}
	if len(parts) != 4 {
		return "", fmt.Errorf("cJWT '%s' should have 4 dot-delimited parts", text)
	}
	magic := parts[0]
	if magic != magicNumber {
		return "", fmt.Errorf("cJWT magic '%s' does not equal '%s'", magic, magicNumber)
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
