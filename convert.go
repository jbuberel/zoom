// Copyright 2014 Alex Browne.  All rights reserved.
// Use of this source code is governed by the MIT
// license, which can be found in the LICENSE file.

// File scan.go contains code that converts go data structures
// to and from a format that redis can understand

package zoom

import (
	"fmt"
	"github.com/garyburd/redigo/redis"
	"reflect"
	"strconv"
)

func scanModel(replies []interface{}, mr *modelRef, includes []string) error {
	fieldNames := []string{}
	if len(includes) == 0 {
		fieldNames = mr.modelSpec.fieldNames()
	} else {
		fieldNames = includes
	}
	ms := mr.modelSpec
	includedFields := []*fieldSpec{}
	for _, name := range fieldNames {
		includedFields = append(includedFields, ms.fields[name])
	}
	for i, reply := range replies {
		replyBytes, err := redis.Bytes(reply, nil)
		if err != nil {
			return err
		} else if string(replyBytes) == "NULL" {
			// skip null fields
			continue
		}
		fs := includedFields[i]
		fieldVal := mr.fieldValue(fs.name)
		switch fs.kind {
		case primativeField:
			if err := scanPrimativeVal(replyBytes, fieldVal); err != nil {
				return err
			}
		case pointerField:
			if err := scanPointerVal(replyBytes, fieldVal); err != nil {
				return err
			}
		default:
			if err := scanInconvertibleVal(replyBytes, fieldVal); err != nil {
				return err
			}
		}
	}
	return nil
}

func scanPrimativeVal(src []byte, dest reflect.Value) error {
	if len(src) == 0 {
		return nil // skip blanks
	}
	switch dest.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		srcInt, err := strconv.ParseInt(string(src), 10, 0)
		if err != nil {
			return fmt.Errorf("zoom: could not convert %s to int.", string(src))
		}
		dest.SetInt(srcInt)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		srcUint, err := strconv.ParseUint(string(src), 10, 0)
		if err != nil {
			return fmt.Errorf("zoom: could not convert %s to uint.", string(src))
		}
		dest.SetUint(srcUint)

	case reflect.Float32, reflect.Float64:
		srcFloat, err := strconv.ParseFloat(string(src), 64)
		if err != nil {
			return fmt.Errorf("zoom: could not convert %s to float.", string(src))
		}
		dest.SetFloat(srcFloat)
	case reflect.Bool:
		srcBool, err := strconv.ParseBool(string(src))
		if err != nil {
			return fmt.Errorf("zoom: could not convert %s to bool.", string(src))
		}
		dest.SetBool(srcBool)
	case reflect.String:
		dest.SetString(string(src))
	case reflect.Slice, reflect.Array:
		// Slice or array of bytes
		dest.SetBytes(src)
	default:
		return fmt.Errorf("zoom: don't know how to scan primative type: %T.\n", src)
	}
	return nil
}

func scanPointerVal(src []byte, dest reflect.Value) error {
	dest.Set(reflect.New(dest.Type().Elem()))
	return scanPrimativeVal(src, dest.Elem())
}

func scanInconvertibleVal(src []byte, dest reflect.Value) error {
	if len(src) == 0 {
		return nil // skip blanks
	}
	// TODO: account for json, msgpack or other custom fallbacks
	if err := defaultMarshalerUnmarshaler.Unmarshal(src, dest.Addr().Interface()); err != nil {
		return err
	}
	return nil
}
