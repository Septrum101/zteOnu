package version

import (
	"fmt"
)

var (
	version = "0.0.7"
	appName = "ZteONU"
	date    = "09/10/2024"
	intro   = "https://github.com/stich86/zteOnu"
)

func Show() {
	fmt.Printf("%s %s, built at %s\nsource: %s\n", appName, version, date, intro)
}
