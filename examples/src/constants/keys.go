package constants

// DevKeys contains the development keys used for testing
type DevKeys struct {
	AliceIdentityKey  string
	AlicePrivateKey   string
	BobIdentityKey    string
	BobPrivateKey     string
}

// GetDevKeys returns the development keys
func GetDevKeys() DevKeys {
	return DevKeys{
		AliceIdentityKey:  "020c0ca23c75f7312bad0c5d81bff858bdcf468d3ad69a60b46ae90cafef557b03",
		AlicePrivateKey:   "5a39d6a914e96be64873f7b954efa926a7d79f648810fad2e2b3aa11d31f9f69",
		BobIdentityKey:    "03e14a6f57e27ed5399307641be23ec497f19df99ff1ce7ef04ec82200a6f90b2b",
		BobPrivateKey:     "ca9e9dcb29fd7c7cf5ecebadd1a0dab029e571a570021e7ec699eb90acee333d",
	}
}
