package version

import (
	"fmt"
)

var (
	Version = "0.0.1"
	AppName = "ZteONU"
	Intro   = "github.com/thank243/zteOnu"
)

func Show() {
	fmt.Printf("%s %s (%s) \n", AppName, Version, Intro)
}
