package qortoo

/*
#include <qortoo.h>
*/
import "C"

import "fmt"

// expectedABIMajor is the qortoo-ffi ABI major version this package is written
// against. A mismatch with the linked native library means struct layouts, calling
// conventions, or exported symbols may not agree, so the binding cannot safely
// proceed — checked once at package init, before any other native call.
const expectedABIMajor = 0

func init() {
	if got := uint32(C.qortoo_abi_version_major()); got != expectedABIMajor {
		panic(fmt.Sprintf(
			"qortoo: linked qortoo-ffi ABI major version %d does not match %d, the version this package is built against; use a matching native SDK",
			got, expectedABIMajor,
		))
	}
}
