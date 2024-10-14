package util

import (
	"os"

	"go.uber.org/zap"
)

var (
	log zap.SugaredLogger
)

/*
*
Looks up environment variable using the provided key,
returns the fallback if the environment variable is not found
*
*/
func LookupEnv(key string, fallback string, sensitive bool) string {

	sourcedVal, exists := os.LookupEnv(key)

	// sensitive fields
	envKeyZapField := zap.String("env_key", key)
	fallbackZapField := zap.String("fallback", fallback)
	envVarValueZapField := zap.String("value", sourcedVal)

	// insensitive field(s)
	sensitiveZapField := zap.Bool("sensitive", sensitive)

	log.Debugln("looking up environment variable", sensitiveZapField)

	if exists && !sensitive {
		log.Debugln("found environment variable", envKeyZapField, sensitiveZapField, envVarValueZapField)
		return sourcedVal
	} else if exists && sensitive {
		log.Debugln("found environment variable", sensitiveZapField)
		return sourcedVal
	} else if !exists && !sensitive {
		log.Debugln("did not find environment variable, using fallback", envKeyZapField, sensitiveZapField, fallbackZapField)
		return fallback
	} else {
		log.Debugln("did not find environment variable, using fallback", sensitiveZapField)
		return fallback
	}
}

func AnyStringsEmpty(strings ...string) bool {
	for _, str := range strings {
		if str == "" {
			return true
		}
	}
	return false
}
