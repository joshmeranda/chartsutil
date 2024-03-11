package rebase

import (
	"fmt"
	"math/rand"
)

func randomSuffix() string {
	return fmt.Sprintf("%d", rand.Int())
}
