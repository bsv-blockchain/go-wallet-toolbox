package constants

// DevKeys contains the development keys used for testing
type DevKeys struct {
	IdentityKey  string
	IdentityKey2 string
	PrivateKey   string
	PrivateKey2  string
}

// GetDevKeys returns the development keys
func GetDevKeys() DevKeys {
	return DevKeys{
		IdentityKey:  "020c0ca23c75f7312bad0c5d81bff858bdcf468d3ad69a60b46ae90cafef557b03",
		IdentityKey2: "03e14a6f57e27ed5399307641be23ec497f19df99ff1ce7ef04ec82200a6f90b2b",
		PrivateKey:   "5a39d6a914e96be64873f7b954efa926a7d79f648810fad2e2b3aa11d31f9f69",
		PrivateKey2:  "ca9e9dcb29fd7c7cf5ecebadd1a0dab029e571a570021e7ec699eb90acee333d",
	}
}
