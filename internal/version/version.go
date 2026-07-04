package version

import "fmt"

const (
	Major = 0
	Minor = 0
	Patch = 4
	Pre   = ""
)

func String() string {
	v := fmt.Sprintf("ginger v%d.%d.%d", Major, Minor, Patch)
	if Pre != "" {
		v += "-" + Pre
	}
	return v
}
