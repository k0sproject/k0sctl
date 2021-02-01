package cluster

import "fmt"

// Mapping is a YAML "mapping" ([key]value struct) hosting an embedded raw k0s config
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

// Dig is very naive implementation for ruby-like Hash.dig functionality
// Returns nested value which is specified by the sequence of the key object by calling dig at each step. otherwise nil
func (m *Mapping) Dig(keys ...string) interface{} {
	v, ok := (*m)[keys[0]]
	if !ok {
		return nil
	}
	switch v := v.(type) {
	case Mapping:
		if len(keys) == 1 {
			return v
		}
		return v.Dig(keys[1:]...)
	default:
		if len(keys) > 1 {
			return nil
		}
		return v
	}
}

// DigString is like Dig but returns the value as string type
func (m *Mapping) DigString(keys ...string) string {
	v := m.Dig(keys...)
	val, ok := v.(string)
	if !ok {
		return ""
	}
	return val
}

// DigMapping always returns a mapping, creating missing or overwriting non-mapping branches in between
func (m *Mapping) DigMapping(keys ...string) Mapping {
	k := keys[0]
	cur := (*m)[k]
	switch v := cur.(type) {
	case Mapping:
		if len(keys) > 1 {
			return v.DigMapping(keys[1:]...)
		}
		return v
	default:
		n := Mapping{}
		(*m)[k] = n
		if len(keys) > 1 {
			return n.DigMapping(keys[1:]...)
		}
		return n
	}
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
