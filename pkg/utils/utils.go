package utils

import "encoding/json"

func Dumps(object any) {
	// print as JSON for debugging purposes
	data, err := json.MarshalIndent(object, "", "  ")
	if err != nil {
		panic(err)
	}
	println(string(data))
}