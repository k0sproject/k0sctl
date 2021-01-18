package config

import (
	"fmt"
)

// Mapping is an alias of map[string]interface{} - which YAML terminology refers to as "mapping"
type Mapping map[string]interface{}

// UnmarshalYAML overridden since by default nested maps would be parsed
// into map[interface]interface{} which in turn is not able to be marshaled into json
func (m *Mapping) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var result map[interface{}]interface{}
	if err := unmarshal(&result); err != nil {
		return err
	}
	*m = cleanUpInterfaceMap(result)
	return nil
}

// Cleans up a slice of interfaces into slice of actual values
func cleanUpInterfaceArray(in []interface{}) []interface{} {
	result := make([]interface{}, len(in))
	for i, v := range in {
		result[i] = cleanUpMapValue(v)
	}
	return result
}

// Cleans up the map keys to be strings
func cleanUpInterfaceMap(in map[interface{}]interface{}) Mapping {
	result := make(Mapping)
	for k, v := range in {
		result[fmt.Sprintf("%v", k)] = cleanUpMapValue(v)
	}
	return result
}

// Cleans up the value in the map, recurses in case of arrays and maps
func cleanUpMapValue(v interface{}) interface{} {
	switch v := v.(type) {
	case []interface{}:
		return cleanUpInterfaceArray(v)
	case map[interface{}]interface{}:
		return cleanUpInterfaceMap(v)
	case string:
		return v
	case int:
		return v
	case bool:
		return v
	default:
		return fmt.Sprintf("%v", v)
	}
}
