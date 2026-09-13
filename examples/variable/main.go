// Command variable is the "Store a JSON Value in a Variable" walkthrough from
// docs/getting-started.md, kept runnable so an API change that breaks it is caught by
// CI's example-build step rather than silently going stale in the doc.
//
// Run from the qortoo-go repository root:
//
//	go run ./examples/variable
package main

import (
	"fmt"
	"log"

	"github.com/qortoo/qortoo-go"
)

type Profile struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

func main() {
	client, err := qortoo.NewClient("my-collection", "my-client")
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	variable, err := client.SubscribeOrCreateVariable("profile", nil)
	if err != nil {
		log.Fatal(err)
	}
	defer variable.Close()

	// Set returns the value held just before the call — nil before the first Set.
	previous, err := variable.Set(Profile{Name: "ada", Age: 36})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(previous)

	var profile Profile
	if err := variable.Get(&profile); err != nil {
		log.Fatal(err)
	}
	fmt.Println(profile)
}
