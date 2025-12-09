package satoshi

import (
	"fmt"
	"iter"
	"reflect"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
	"github.com/go-softwarelab/common/pkg/types"
)

type Value int64

func (v Value) Int64() int64 {
	return int64(v)
}

func (v Value) UInt64() (uint64, error) {
	if v < 0 {
		return 0, fmt.Errorf("cannot convert negative satoshi to uint64")
	}
	return uint64(v), nil //nolint:gosec
}

func (v Value) MustUInt64() uint64 {
	u, err := v.UInt64()
	if err != nil {
		panic(err)
	}
	return u
}

func Zero() Value {
	return Value(0)
}

func From[T types.Integer](value T) (Value, error) {
	if err := Validate(value); err != nil {
		return 0, err
	}
	return Value(value), nil
}

func MustFrom[T types.Integer](value T) Value {
	v, err := From(value)
	if err != nil {
		panic(err)
	}
	return v
}

func Add[A types.Integer, B types.Integer](a A, b B) (Value, error) {
	satsA, err := From(a)
	if err != nil {
		return 0, err
	}
	satsB, err := From(b)
	if err != nil {
		return 0, err
	}
	c := satsA + satsB
	if err = Validate(c); err != nil {
		return 0, err
	}
	return c, nil
}

func MustAdd[A types.Integer, B types.Integer](a A, b B) Value {
	added, err := Add(a, b)
	if err != nil {
		panic(err)
	}
	return added
}

func Subtract[A types.Integer, B types.Integer](a A, b B) (Value, error) {
	satsA, err := From(a)
	if err != nil {
		return 0, err
	}
	satsB, err := From(b)
	if err != nil {
		return 0, err
	}
	c := satsA - satsB
	if err = validateInt(c); err != nil {
		return 0, err
	}
	return c, nil
}

func MustSubtract[A types.Integer, B types.Integer](a A, b B) Value {
	subtracted, err := Subtract(a, b)
	if err != nil {
		panic(err)
	}
	return subtracted
}

func Multiply[A types.Integer, B types.Integer](a A, b B) (Value, error) {
	satsA, err := From(a)
	if err != nil {
		return 0, err
	}
	satsB, err := From(b)
	if err != nil {
		return 0, err
	}
	c := satsA * satsB
	if err = validateInt(c); err != nil {
		return 0, err
	}
	return c, nil
}

func MustMultiply[A types.Integer, B types.Integer](a A, b B) Value {
	multiplied, err := Multiply(a, b)
	if err != nil {
		panic(err)
	}
	return multiplied
}

func Sum[T types.Integer](values iter.Seq[T]) (Value, error) {
	var err error
	var satsB Value
	val := Zero()

	for it := range values {
		satsB, err = From(it)
		if err != nil {
			return 0, err
		}

		val += satsB
		if err = validateInt(val); err != nil {
			return 0, err
		}
	}
	return val, nil
}

func MustSum[T types.Integer](values iter.Seq[T]) Value {
	sum, err := Sum(values)
	if err != nil {
		panic(err)
	}
	return sum
}

func Equal[A types.Integer, B types.Integer](a A, b B) (bool, error) {
	satsA, err := From(a)
	if err != nil {
		return false, err
	}
	satsB, err := From(b)
	if err != nil {
		return false, err
	}
	return satsA == satsB, nil
}

func MustEqual[A types.Integer, B types.Integer](a A, b B) bool {
	equal, err := Equal(a, b)
	if err != nil {
		panic(err)
	}
	return equal
}

func Validate[T types.Integer](value T) error {
	switch typed := (any)(value).(type) {
	case int:
		return validateInt(typed)
	case int64:
		return validateInt(typed)
	case uint:
		return validateUint(typed)
	case uint64:
		return validateUint(typed)
	case Value:
		return validateInt(typed)
	default:
		return validateGeneric(typed)
	}
}

func validateInt[T ~int | ~int64](value T) error {
	if value > primitives.MaxSatoshis {
		return fmt.Errorf("satoshi value %d exceeded max value %d", value, primitives.MaxSatoshis)
	}
	if value < -primitives.MaxSatoshis {
		return fmt.Errorf("satoshi value %d is less than minimum allowed value %d", value, -primitives.MaxSatoshis)
	}
	return nil
}

func validateUint[T ~uint | ~uint64](value T) error {
	if value > primitives.MaxSatoshis {
		return fmt.Errorf("satoshi value %d exceeded max value %d", value, primitives.MaxSatoshis)
	}
	return nil
}

func validateGeneric(value any) error {
	v := reflect.ValueOf(value)
	//nolint:exhaustive
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return validateInt(v.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return validateUint(v.Uint())
	default:
		return fmt.Errorf("unsupported type in validateGeneric")
	}
}
