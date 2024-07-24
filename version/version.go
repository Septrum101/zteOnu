package version

import (
	"fmt"
)

var (
	Version = "0.0.6"
	AppName = "ZteONU"
	Intro   = "github.com/thank243/zteOnu"
)

func Show() {
	fmt.Printf("%s %s (%s) \n", AppName, Version, Intro)
}
