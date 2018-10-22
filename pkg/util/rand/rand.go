package rand

import (
	"math/rand"
	"time"
)

// GenerateRandomInt32 returns a random generated int32 value
func GenerateRandomInt32() int32 {
	rand.Seed(time.Now().UnixNano())
	return 1 + rand.Int31()
}

// GenerateRandomString returns a random generated string with given lenght
func GenerateRandomString(l int) string {
	bytes := make([]byte, l)
	for i := 0; i < l; i++ {
		bytes[i] = byte(randInt(65, 90))
	}
	return string(bytes)
}

func randInt(min int, max int) int {
	return min + rand.Intn(max-min)
}
