// proxy_env.go — shared env helper, no build tag
package main

import "os"

func getenv(key string) string { return os.Getenv(key) }
