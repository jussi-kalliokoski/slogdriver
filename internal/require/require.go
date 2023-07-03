package require

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
)

func Equal[T any](tb testing.TB, expected, received T, message ...string) {
	tb.Helper()

	if err := equal(tb, expected, received); !isEqual(err) {
		if len(message) != 0 {
			tb.Fatal(message)
		} else {
			tb.Fatal(err)
		}
	}
}

func NoError(tb testing.TB, err error) {
	tb.Helper()
	if err != nil {
		tb.Fatalf("expected no error, got %##v", err)
	}
}

func Error(tb testing.TB, err error) {
	tb.Helper()
	if err == nil {
		tb.Fatalf("expected error, got <nil>")
	}
}

func equal[T any](tb testing.TB, expected, received T) error {
	if err := equalEqualer(tb, expected, received); err != nil {
		return err
	}

	return equalReflect(tb, expected, received)
}

func equalEqualer[T any](tb testing.TB, expected, received T) error {
	type Equaler interface {
		Equal(T) bool
	}

	if eq, ok := any(expected).(Equaler); ok {
		if !eq.Equal(received) {
			return fmt.Errorf("expected %##v, got %##v", expected, received)
		}
		return errEqual
	}

	return nil
}

func equalReflect(tb testing.TB, expected, received any) error {
	t := reflect.TypeOf(expected)
	if t != reflect.TypeOf(received) {
		return fmt.Errorf("expected %##v, got %##v: mismatched types", expected, received)
	}

	switch t.Kind() {
	case
		reflect.Bool,
		reflect.Int,
		reflect.Int8,
		reflect.Int16,
		reflect.Int32,
		reflect.Int64,
		reflect.Uint,
		reflect.Uint8,
		reflect.Uint16,
		reflect.Uint32,
		reflect.Uint64,
		reflect.Uintptr,
		reflect.Float32,
		reflect.Float64,
		reflect.Complex64,
		reflect.Complex128,
		reflect.String,
		reflect.UnsafePointer:
		if expected != received {
			return fmt.Errorf("expected %##v, got %##v", expected, received)
		}
		return errEqual
	case reflect.Array, reflect.Slice:
		return equalIndexed(tb, expected, received)
	case reflect.Map:
		return equalMaps(tb, expected, received)
	case reflect.Pointer:
		return equalPointers(tb, expected, received)
	case reflect.Struct:
		return equalStructs(tb, expected, received)
	case reflect.Chan, reflect.Func:
		ep := reflect.ValueOf(expected)
		rp := reflect.ValueOf(received)
		if ep.IsNil() == rp.IsNil() {
			return errEqual
		}
	}

	return fmt.Errorf("expected %##v, got %##v: values are not comparable", expected, received)
}

func equalIndexed(tb testing.TB, expected, received any) error {
	es := reflect.ValueOf(expected)
	rs := reflect.ValueOf(received)
	if es.IsNil() != rs.IsNil() {
		return fmt.Errorf("expected %##v, got %##v", expected, received)
	}
	var errs error
	elen := es.Len()
	rlen := rs.Len()
	for i := 0; i < elen; i++ {
		ev := es.Index(i).Interface()
		if i >= rlen {
			errs = errors.Join(errs, fmt.Errorf("missing value %##v at index %d", ev, i))
		} else {
			rv := rs.Index(i).Interface()
			if err := equalReflect(tb, ev, rv); !isEqual(err) {
				errs = errors.Join(errs, fmt.Errorf("values at index %##v differ: %w", i, err))
			}
		}
	}
	for i := elen; i < rlen; i++ {
		rv := rs.Index(i).Interface()
		if i >= elen {
			errs = errors.Join(errs, fmt.Errorf("extra value %##v at index %d", rv, i))
		} else {
			ev := es.Index(i).Interface()
			if err := equalReflect(tb, ev, rv); !isEqual(err) {
				errs = errors.Join(errs, fmt.Errorf("values at index %d differ: %w", i, err))
			}
		}
	}

	if errs != nil {
		return fmt.Errorf("expected %##v, got %##v: %w", expected, received, errs)
	}

	return errEqual
}

func equalMaps(tb testing.TB, expected, received any) error {
	em := reflect.ValueOf(expected)
	rm := reflect.ValueOf(received)
	if em.IsNil() != rm.IsNil() {
		return fmt.Errorf("expected %##v, got %##v", expected, received)
	}
	var errs error
	ekeys := map[interface{}]struct{}{}
	eiter := em.MapRange()
	var zeroValue reflect.Value
	for eiter.Next() {
		i := eiter.Key()
		k := i.Interface()
		ekeys[k] = struct{}{}
		ev := eiter.Value().Interface()
		rvi := rm.MapIndex(i)
		if rvi == zeroValue {
			errs = errors.Join(errs, fmt.Errorf("missing value %##v at index %##v", ev, i))
		} else {
			rv := rvi.Interface()
			if err := equalReflect(tb, ev, rv); !isEqual(err) {
				errs = errors.Join(errs, fmt.Errorf("values at index %##v differ: %w", k, err))
			}
		}
	}
	riter := em.MapRange()
	for riter.Next() {
		i := riter.Key()
		k := i.Interface()
		if _, ok := ekeys[k]; ok {
			continue
		}
		rv := riter.Value().Interface()
		evi := em.MapIndex(i)
		if evi == zeroValue {
			errs = errors.Join(errs, fmt.Errorf("extra value %##v at index %##v", rv, i))
		} else {
			ev := evi.Interface()
			if err := equalReflect(tb, ev, rv); !isEqual(err) {
				errs = errors.Join(errs, fmt.Errorf("values at index %##v differ: %w", k, err))
			}
		}
	}

	if errs != nil {
		return fmt.Errorf("expected %##v, got %##v: %w", expected, received, errs)
	}

	return errEqual
}

func equalPointers(tb testing.TB, expected, received any) error {
	ep := reflect.ValueOf(expected)
	rp := reflect.ValueOf(received)
	if ep.IsNil() != rp.IsNil() {
		return fmt.Errorf("expected %##v, got %##v", expected, received)
	}
	if expected == received {
		return errEqual
	}
	ev := ep.Elem().Interface()
	rv := rp.Elem().Interface()
	return equalReflect(tb, ev, rv)
}

func equalStructs(tb testing.TB, expected, received any) error {
	es := reflect.ValueOf(expected)
	rs := reflect.ValueOf(received)
	t := es.Type()
	n := t.NumField()
	var errs error
	for i := 0; i < n; i++ {
		ev := es.Field(i).Interface()
		rv := rs.Field(i).Interface()
		if err := equalReflect(tb, ev, rv); !isEqual(err) {
			errs = errors.Join(errs, fmt.Errorf("values in field %q differ: %w", t.Field(i).Name, err))
		}
	}

	if errs != nil {
		return fmt.Errorf("expected %##v, got %##v: %w", expected, received, errs)
	}

	return errEqual
}

var errEqual = errors.New("equal")

func isEqual(err error) bool {
	return err == errEqual || err == nil
}
