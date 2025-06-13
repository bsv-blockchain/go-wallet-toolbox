package main

import (
	"context"
	"fmt"
	"log"

	"github.com/4chain-ag/go-wallet-toolbox/examples/src/core"
	"github.com/4chain-ag/go-wallet-toolbox/pkg/defs"
)

func main() {
	ctx := context.Background()
	network := defs.NetworkTestnet
	identityKey := "020c0ca23c75f7312bad0c5d81bff858bdcf468d3ad69a60b46ae90cafef557b03"
	
	res, err := core.MatchUser(ctx, network, identityKey)

	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(res)
}
