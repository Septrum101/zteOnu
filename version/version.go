package version

import (
	"fmt"
)

var (
	version = "dev"
	appName = "ZteONU"
	date    = "unknown"
	intro   = "https://github.com/thank243/zteOnu"
)

func Show() {
	fmt.Printf("%s %s, built at %s\nsource: %s\n", appName, version, date, intro)
}
