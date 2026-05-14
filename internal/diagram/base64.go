package diagram

import "encoding/base64"

func base64Decode(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}
