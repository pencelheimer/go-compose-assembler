package assemble

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"

	"github.com/pencelheimer/go-compose-assembler/internal/errs"
)

// normalizeScalar renders a decoded scalar as the string stored in .env.
// Whole numbers become integer digits, booleans "true"/"false"; numbers with
// a fractional part are rejected so the user quotes them (e.g. "1.27").
func normalizeScalar(component, param string, v any) (string, error) {
	mismatch := func(actual string) error {
		return errs.ParamTypeMismatchError{Component: component, Param: param, ExpectedType: "scalar", ActualType: actual}
	}
	switch x := v.(type) {
	case string:
		return x, nil
	case bool:
		return strconv.FormatBool(x), nil
	case int:
		return strconv.Itoa(x), nil
	case int8, int16, int32, int64:
		return fmt.Sprint(x), nil
	case uint, uint8, uint16, uint32, uint64:
		return fmt.Sprint(x), nil
	case float32:
		return normalizeFloat(float64(x), mismatch)
	case float64:
		return normalizeFloat(x, mismatch)
	case json.Number:
		if i, err := x.Int64(); err == nil {
			return strconv.FormatInt(i, 10), nil
		}
		f, err := x.Float64()
		if err != nil {
			return "", mismatch("number")
		}
		return normalizeFloat(f, mismatch)
	case []any, []string:
		return "", mismatch("list")
	case map[string]any:
		return "", mismatch("map")
	default:
		return "", mismatch(fmt.Sprintf("%T", v))
	}
}

func normalizeFloat(f float64, mismatch func(string) error) (string, error) {
	if math.IsNaN(f) || math.IsInf(f, 0) || f != math.Trunc(f) || math.Abs(f) >= 1<<53 {
		return "", mismatch("fractional number (quote the value)")
	}
	return strconv.FormatInt(int64(f), 10), nil
}

// checkList verifies a list parameter value is a slice.
func checkList(component, param string, v any) error {
	switch v.(type) {
	case []any, []string:
		return nil
	}
	actual := fmt.Sprintf("%T", v)
	switch v.(type) {
	case string, bool, int, int64, float64, json.Number:
		actual = "scalar"
	case map[string]any:
		actual = "map"
	}
	return errs.ParamTypeMismatchError{Component: component, Param: param, ExpectedType: "list", ActualType: actual}
}
