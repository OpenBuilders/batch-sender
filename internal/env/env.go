package env

import (
	"os"
	"strconv"
	"strings"
)

const NotExists = "~!-===X===-!~"

// GetString retrieves the value of the environment variable named by the key.
// It returns the value, or if the variable is not present, it returns the defaultValue.
func GetString(key, defaultValue string) string {
	value, exists := os.LookupEnv(key)
	if !exists {
		return defaultValue
	}
	return value
}

// GetBool returns true if the env variable with the key set and is truthy and
// defaultValue otherwise.
func GetBool(key string, defaultValue bool) bool {
	strValue := GetString(key, NotExists)
	if strValue == NotExists {
		return defaultValue
	}

	if strValue == "1" || strValue == "true" {
		return true
	}

	return false
}

// GetInt returns an integer if the env variable with the key set and contains
// an integer and defaultValue otherwise.
func GetInt(key string, defaultValue int) int {
	strValue := GetString(key, NotExists)
	if strValue == NotExists {
		return defaultValue
	}

	intValue, err := strconv.ParseInt(strValue, 10, 64)
	if err != nil {
		return defaultValue
	}

	return int(intValue)
}

func GetListOfIDs(key string, defaultValue []int64) []int64 {
	var ids []int64

	strOfIDs := GetString(key, "")
	if strOfIDs != "" {
		sliceOfStrIDs := strings.Split(strOfIDs, ",")
		for _, s := range sliceOfStrIDs {
			id, err := strconv.Atoi(s)
			if err != nil {
				continue
			}

			ids = append(ids, int64(id))
		}
	}

	return ids
}
