// Copyright 2013 Alex Browne.  All rights reserved.
// Use of this source code is governed by the MIT
// license, which can be found in the LICENSE file.

// File model.go contains code strictly related to DefaultData and Model.
// The Register() method and associated methods are also included here.

package zoom

import (
	"errors"
	"fmt"
	"github.com/stephenalexbrowne/zoom/util"
	"reflect"
)

// DefaultData should be embedded in any struct you wish to save.
// It includes all the required fields.
type DefaultData struct {
	Id string `redis:"-"`
	// TODO: add other default fields?
}

// Model is an interface encapsulating anything that can be saved.
// Any struct which includes an embedded DefaultData field satisfies
// the Model interface.
type Model interface {
	GetId() string
	SetId(string)
	// TODO: add getters and setters for other default fields?
}

type modelSpec struct {
	fieldNames []string
	sets       []*externalSet
	lists      []*externalList
	relations  map[string]relation
}

type externalSet struct {
	redisName string
	fieldName string
}

type externalList struct {
	redisName string
	fieldName string
}

type relation struct {
	redisName string
	fieldName string
	typ       relationType
}

type relationType int

const (
	ONE_TO_ONE = iota
	ONE_TO_MANY
)

// maps a type to a string identifier. The string is used
// as a key in the redis database.
var typeToName map[reflect.Type]string = make(map[reflect.Type]string)

// maps a string identifier to a type. This is so you can
// pass in a string for the *ById methods
var nameToType map[string]reflect.Type = make(map[string]reflect.Type)

// maps a string identifier to a modelSpec
var modelSpecs map[string]*modelSpec = make(map[string]*modelSpec)

// methods so that DefaultData (and any struct with DefaultData embedded)
// satisifies Model interface
func (d DefaultData) GetId() string {
	return d.Id
}

func (d *DefaultData) SetId(id string) {
	d.Id = id
}

// Register adds a type to the list of registered types. Any struct
// you wish to save must be registered first. Both name and type of in
// must be unique, i.e. not already registered.
func Register(in interface{}, name string) error {
	typ := reflect.TypeOf(in)

	// make sure the interface is the correct type
	if typ.Kind() != reflect.Ptr {
		return errors.New("zoom: schema must be a pointer to a struct")
	} else if typ.Elem().Kind() != reflect.Struct {
		return errors.New("zoom: schema must be a pointer to a struct")
	}

	// make sure the name and type have not been previously registered
	if alreadyRegisteredType(typ) {
		return NewTypeAlreadyRegisteredError(typ)
	}
	if alreadyRegisteredName(name) {
		return NewNameAlreadyRegisteredError(name)
	}

	// create a new model spec and register its lists and sets
	ms := &modelSpec{relations: make(map[string]relation)}
	if err := compileModelSpec(typ, ms); err != nil {
		return err
	}

	typeToName[typ] = name
	nameToType[name] = typ
	modelSpecs[name] = ms

	return nil
}

func compileModelSpec(typ reflect.Type, ms *modelSpec) error {
	// iterate through fields to find slices and arrays
	elem := typ.Elem()
	numFields := elem.NumField()
	for i := 0; i < numFields; i++ {
		field := elem.Field(i)
		if field.Name != "DefaultData" {
			ms.fieldNames = append(ms.fieldNames, field.Name)
		}
		if util.TypeIsPointerToStruct(field.Type) {
			// assume we're dealing with a one-to-one relation
			// get the redisName
			tag := field.Tag
			redisName := tag.Get("redis")
			if redisName == "-" {
				continue // skip field
			} else if redisName == "" {
				redisName = field.Name
			}
			ms.relations[field.Name] = relation{
				redisName: redisName,
				fieldName: field.Name,
				typ:       ONE_TO_ONE,
			}
		} else if util.TypeIsSliceOrArray(field.Type) {
			// we're dealing with a slice or an array, which should be converted to a list, set, or one-to-many relation
			tag := field.Tag
			redisName := tag.Get("redis")
			if redisName == "-" {
				continue // skip field
			} else if redisName == "" {
				redisName = field.Name
			}
			if util.TypeIsPointerToStruct(field.Type.Elem()) {
				// assume we're dealing with a one-to-many relation
				ms.relations[field.Name] = relation{
					redisName: redisName,
					fieldName: field.Name,
					typ:       ONE_TO_MANY,
				}
				continue
			}
			redisType := tag.Get("redisType")
			if redisType == "" || redisType == "list" {
				ms.lists = append(ms.lists, &externalList{redisName: redisName, fieldName: field.Name})
			} else if redisType == "set" {
				ms.sets = append(ms.sets, &externalSet{redisName: redisName, fieldName: field.Name})
			} else {
				msg := fmt.Sprintf("zoom: invalid struct tag for redisType: %s. must be either 'set' or 'list'\n", redisType)
				return errors.New(msg)
			}
		}
	}
	return nil
}

// UnregisterName removes a type (identified by name) from the list of
// registered types. You only need to call UnregisterName or UnregisterType,
// not both.
func UnregisterName(name string) error {
	typ, ok := nameToType[name]
	if !ok {
		return NewModelNameNotRegisteredError(name)
	}
	delete(nameToType, name)
	delete(typeToName, typ)
	return nil
}

// UnregisterName removes a type from the list of registered types.
// You only need to call UnregisterName or UnregisterType, not both.
func UnregisterType(typ reflect.Type) error {
	name, ok := typeToName[typ]
	if !ok {
		return NewModelTypeNotRegisteredError(typ)
	}
	delete(nameToType, name)
	delete(typeToName, typ)
	return nil
}

// alreadyRegisteredName returns true iff the model name has already been registered
func alreadyRegisteredName(n string) bool {
	_, ok := nameToType[n]
	return ok
}

// alreadyRegisteredType returns true iff the model type has already been registered
func alreadyRegisteredType(t reflect.Type) bool {
	_, ok := typeToName[t]
	return ok
}

// getRegisteredNameFromInterface gets the registered name of the model we're
// trying to save based on the interfaces type. If the interface's name/type
// has not been registered, returns a ModelTypeNotRegisteredError
func getRegisteredNameFromInterface(in interface{}) (string, error) {
	typ := reflect.TypeOf(in)
	name, ok := typeToName[typ]
	if !ok {
		return "", NewModelTypeNotRegisteredError(typ)
	}
	return name, nil
}

// getRegisteredTypeFromName gets the registered type of the model we're trying
// to save based on the model name. If the interface's name/type has not been registered,
// returns a ModelNameNotRegisteredError
func getRegisteredTypeFromName(name string) (reflect.Type, error) {
	typ, ok := nameToType[name]
	if !ok {
		return nil, NewModelNameNotRegisteredError(name)
	}
	return typ, nil
}
