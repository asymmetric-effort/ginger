package config

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// Decode parses YAML input and decodes it into the given struct pointer.
func Decode(data []byte, v interface{}) error {
	expanded := interpolateEnvVars(string(data))
	node, err := Parse(expanded)
	if err != nil {
		return err
	}
	return decodeNode(node, reflect.ValueOf(v))
}

// DecodeStrict is like Decode but returns an error for unknown keys.
func DecodeStrict(data []byte, v interface{}) error {
	expanded := interpolateEnvVars(string(data))
	node, err := Parse(expanded)
	if err != nil {
		return err
	}
	return decodeNodeStrict(node, reflect.ValueOf(v), true)
}

func decodeNode(node *Node, v reflect.Value) error {
	return decodeNodeStrict(node, v, false)
}

func decodeNodeStrict(node *Node, v reflect.Value, strict bool) error {
	// Dereference pointer
	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		v = v.Elem()
	}

	switch node.Kind {
	case NodeScalar:
		return setScalar(v, node.Value)
	case NodeMapping:
		return setMapping(node, v, strict)
	case NodeSequence:
		return setSequence(node, v, strict)
	}
	return nil
}

func setScalar(v reflect.Value, s string) error {
	switch v.Kind() {
	case reflect.String:
		v.SetString(s)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if v.Type() == reflect.TypeOf(time.Duration(0)) {
			d, err := time.ParseDuration(s)
			if err != nil {
				return fmt.Errorf("invalid duration %q: %w", s, err)
			}
			v.SetInt(int64(d))
			return nil
		}
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid integer %q: %w", s, err)
		}
		v.SetInt(n)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid unsigned integer %q: %w", s, err)
		}
		v.SetUint(n)
	case reflect.Float32, reflect.Float64:
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return fmt.Errorf("invalid float %q: %w", s, err)
		}
		v.SetFloat(f)
	case reflect.Bool:
		b, err := parseBool(s)
		if err != nil {
			return err
		}
		v.SetBool(b)
	case reflect.Slice:
		if v.Type().Elem().Kind() == reflect.Uint8 {
			v.SetBytes([]byte(s))
		}
	case reflect.Interface:
		v.Set(reflect.ValueOf(autoType(s)))
	default:
		return fmt.Errorf("cannot set %s from scalar %q", v.Type(), s)
	}
	return nil
}

func setMapping(node *Node, v reflect.Value, strict bool) error {
	switch v.Kind() {
	case reflect.Struct:
		return setStruct(node, v, strict)
	case reflect.Map:
		return setMap(node, v, strict)
	case reflect.Interface:
		m := make(map[string]interface{})
		for _, key := range node.Keys {
			child := node.Map[key]
			val, err := nodeToInterface(child)
			if err != nil {
				return err
			}
			m[key] = val
		}
		v.Set(reflect.ValueOf(m))
	default:
		return fmt.Errorf("cannot decode mapping into %s", v.Type())
	}
	return nil
}

func setStruct(node *Node, v reflect.Value, strict bool) error {
	t := v.Type()
	fieldMap := make(map[string]int)
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		tag := field.Tag.Get("yaml")
		if tag == "-" {
			continue
		}
		name := tag
		if commaIdx := strings.Index(tag, ","); commaIdx >= 0 {
			name = tag[:commaIdx]
		}
		if name == "" {
			name = strings.ToLower(field.Name)
		}
		fieldMap[name] = i
	}

	for _, key := range node.Keys {
		idx, ok := fieldMap[key]
		if !ok {
			if strict {
				return &ParseError{
					Message: fmt.Sprintf("unknown key %q", key),
					Line:    node.Map[key].Line,
					Column:  node.Map[key].Column,
				}
			}
			continue
		}
		field := v.Field(idx)
		if err := decodeNodeStrict(node.Map[key], field, strict); err != nil {
			return fmt.Errorf("field %q: %w", key, err)
		}
	}
	return nil
}

func setMap(node *Node, v reflect.Value, strict bool) error {
	if v.IsNil() {
		v.Set(reflect.MakeMap(v.Type()))
	}
	valType := v.Type().Elem()
	for _, key := range node.Keys {
		child := node.Map[key]
		if valType.Kind() == reflect.Interface {
			val, err := nodeToInterface(child)
			if err != nil {
				return fmt.Errorf("map key %q: %w", key, err)
			}
			if val == nil {
				v.SetMapIndex(reflect.ValueOf(key), reflect.Zero(valType))
			} else {
				v.SetMapIndex(reflect.ValueOf(key), reflect.ValueOf(val))
			}
		} else {
			elem := reflect.New(valType).Elem()
			if err := decodeNodeStrict(child, elem, strict); err != nil {
				return fmt.Errorf("map key %q: %w", key, err)
			}
			v.SetMapIndex(reflect.ValueOf(key), elem)
		}
	}
	return nil
}

func setSequence(node *Node, v reflect.Value, strict bool) error {
	switch v.Kind() {
	case reflect.Slice:
		elemType := v.Type().Elem()
		slice := reflect.MakeSlice(v.Type(), len(node.Children), len(node.Children))
		for i, child := range node.Children {
			elem := reflect.New(elemType).Elem()
			if err := decodeNodeStrict(child, elem, strict); err != nil {
				return fmt.Errorf("index %d: %w", i, err)
			}
			slice.Index(i).Set(elem)
		}
		v.Set(slice)
	case reflect.Interface:
		items := make([]interface{}, len(node.Children))
		for i, child := range node.Children {
			val, err := nodeToInterface(child)
			if err != nil {
				return err
			}
			items[i] = val
		}
		v.Set(reflect.ValueOf(items))
	default:
		return fmt.Errorf("cannot decode sequence into %s", v.Type())
	}
	return nil
}

func nodeToInterface(node *Node) (interface{}, error) {
	switch node.Kind {
	case NodeScalar:
		return autoType(node.Value), nil
	case NodeMapping:
		m := make(map[string]interface{})
		for _, key := range node.Keys {
			val, err := nodeToInterface(node.Map[key])
			if err != nil {
				return nil, err
			}
			m[key] = val
		}
		return m, nil
	case NodeSequence:
		items := make([]interface{}, len(node.Children))
		for i, child := range node.Children {
			val, err := nodeToInterface(child)
			if err != nil {
				return nil, err
			}
			items[i] = val
		}
		return items, nil
	}
	return nil, nil
}

func parseBool(s string) (bool, error) {
	switch strings.ToLower(s) {
	case "true", "yes", "on":
		return true, nil
	case "false", "no", "off":
		return false, nil
	}
	return false, fmt.Errorf("invalid bool %q", s)
}

func autoType(s string) interface{} {
	if s == "null" || s == "~" || s == "" {
		return nil
	}
	if b, err := parseBool(s); err == nil {
		return b
	}
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		return n
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f
	}
	return s
}

// interpolateEnvVars replaces ${ENV_VAR} and ${ENV_VAR:-default} with values.
func interpolateEnvVars(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	i := 0
	for i < len(s) {
		if i+1 < len(s) && s[i] == '$' && s[i+1] == '{' {
			end := strings.Index(s[i:], "}")
			if end < 0 {
				b.WriteByte(s[i])
				i++
				continue
			}
			expr := s[i+2 : i+end]
			var name, defaultVal string
			var hasDefault bool
			if sepIdx := strings.Index(expr, ":-"); sepIdx >= 0 {
				name = expr[:sepIdx]
				defaultVal = expr[sepIdx+2:]
				hasDefault = true
			} else {
				name = expr
			}
			val, ok := os.LookupEnv(name)
			if !ok && hasDefault {
				val = defaultVal
			}
			b.WriteString(val)
			i += end + 1
		} else {
			b.WriteByte(s[i])
			i++
		}
	}
	return b.String()
}
