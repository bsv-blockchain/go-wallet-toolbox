package testabilities

import (
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/testabilities/testservices"
)

type ServicesFixture interface {
	Bitails() testservices.BitailsFixture
	WhatsOnChain() testservices.WhatsOnChainFixture
	ARC() testservices.ARCFixture
	BHS() testservices.BHSFixture

	AllDown()
}

type servicesFixture struct {
	testabilities.ServicesFixture
}

func (s *servicesFixture) AllDown() {
	s.Transport().Reset()
}
