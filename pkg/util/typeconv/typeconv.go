package typeconv

import (
	"encoding/json"
	"fmt"
)

// AnyToFloat64 returns a float64 for given empty interface
func AnyToFloat64(v interface{}) (float64, error) {
	switch v.(type) {
	case float64:
		return v.(float64), nil
	default:
		return 0, fmt.Errorf("Type conversion failed")
	}
}

// AnyToString returns a string for given empty interface
func AnyToString(v interface{}) (string, error) {
	switch v.(type) {
	case string:
		return v.(string), nil
	default:
		return "", fmt.Errorf("Type conversion failed")
	}
}

// AnyToJSON returns a JSON byte array
func AnyToJSON(v interface{}) ([]byte, error) {
	js, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("Type conversion failed")
	}
	return js, nil
}
