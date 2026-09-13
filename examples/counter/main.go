// Command counter is the "Create and Update a Counter" walkthrough from
// docs/getting-started.md, kept runnable so an API change that breaks it is caught by
// CI's example-build step rather than silently going stale in the doc.
//
// Run from the qortoo-go repository root:
//
//	go run ./examples/counter
package main

import (
	"fmt"
	"log"

	"github.com/qortoo/qortoo-go"
)

func main() {
	client, err := qortoo.NewClient("my-collection", "my-client")
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	counter, err := client.SubscribeOrCreateCounter("visits", nil)
	if err != nil {
		log.Fatal(err)
	}
	defer counter.Close()

	value, err := counter.IncreaseBy(1)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(value)
}
