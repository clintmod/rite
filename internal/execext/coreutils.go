package execext

import (
	"runtime"
	"strconv"

	"github.com/clintmod/rite/internal/env"
)

var useGoCoreUtils bool

func init() {
	// If RITE_CORE_UTILS is set to either true or false, respect that.
	// By default, enable on Windows only.
	if v, err := strconv.ParseBool(env.GetRiteEnv("CORE_UTILS")); err == nil {
		useGoCoreUtils = v
	} else {
		useGoCoreUtils = runtime.GOOS == "windows"
	}
}
