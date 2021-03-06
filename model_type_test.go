// Copyright 2015 Alex Browne.  All rights reserved.
// Use of this source code is governed by the MIT
// license, which can be found in the LICENSE file.

// File model_type_test.go contains unit tests for the
// code in model_type.go

package zoom

import (
	"reflect"
	"testing"
)

// regTestModel is a model type that is only used for testing
// the Register and RegisterName functions
type regTestModel struct {
	DefaultData
	Int    int
	Bool   bool
	String string
}

func TestRegister(t *testing.T) {
	regTestModels, err := Register(&regTestModel{})
	if err != nil {
		t.Fatalf("Unexpected error in Register: %s", err.Error())
	}
	expectedName := "regTestModel"
	expectedType := reflect.TypeOf(&regTestModel{})
	testRegisteredModelType(t, regTestModels, expectedName, expectedType)

	// Effectively unregister the type by removing it from the map
	delete(modelNameToSpec, regTestModels.Name())
	delete(modelTypeToSpec, regTestModels.spec.typ)
}

func TestRegisterName(t *testing.T) {
	expectedName := "customName"
	regTestModels, err := RegisterName(expectedName, &regTestModel{})
	if err != nil {
		t.Fatalf("Unexpected error in Register: %s", err.Error())
	}
	expectedType := reflect.TypeOf(&regTestModel{})
	testRegisteredModelType(t, regTestModels, expectedName, expectedType)

	// Effectively unregister the type by removing it from the map
	delete(modelNameToSpec, regTestModels.Name())
	delete(modelTypeToSpec, regTestModels.spec.typ)
}

func testRegisteredModelType(t *testing.T, modelType *ModelType, expectedName string, expectedType reflect.Type) {
	// Check that the name and type are correct
	if modelType.Name() != expectedName {
		t.Errorf("Registered name was incorrect. Expected %s but got %s", expectedName, modelType.Name())
	}
	if modelType.spec.typ == nil {
		t.Fatalf("Registered model spec had nil type")
	}
	if modelType.spec.typ != expectedType {
		t.Errorf("Registered type was incorrect. Expected %s but got %s", expectedType.String(), modelType.spec.typ.String())
	}

	// Check that the model type was added to the appropriate maps
	if !nameIsRegistered(expectedName) {
		t.Error("Registered spec was not added to the modelNameToSpec map")
	}
	if !typeIsRegistered(expectedType) {
		t.Error("Registered spec was not added to the modelTypeToSpec map")
	}

	// Check the underlying spec
	spec := modelType.spec
	if len(spec.fields) != 3 {
		t.Errorf("Expected spec to have 3 fields but got %d", len(spec.fields))
	}
	expectedFields := map[string]*fieldSpec{
		"Int": &fieldSpec{
			kind:      primativeField,
			name:      "Int",
			redisName: "Int",
			typ:       reflect.TypeOf(1),
			indexKind: noIndex,
		},
		"Bool": &fieldSpec{
			kind:      primativeField,
			name:      "Bool",
			redisName: "Bool",
			typ:       reflect.TypeOf(true),
			indexKind: noIndex,
		},
		"String": &fieldSpec{
			kind:      primativeField,
			name:      "String",
			redisName: "String",
			typ:       reflect.TypeOf(""),
			indexKind: noIndex,
		},
	}
	for _, expectedField := range expectedFields {
		gotField, found := spec.fieldsByName[expectedField.name]
		if !found {
			t.Errorf("Expected field with name %s but it was not in spec", expectedField.name)
		}
		if !reflect.DeepEqual(expectedField, gotField) {
			t.Errorf("Field with name %s was incorrect. Expected %+v but got %+v", expectedField.name, expectedField, gotField)
		}
	}
}

func TestSave(t *testing.T) {
	testingSetUp()
	defer testingTearDown()

	// Create and save a test model
	model := createTestModels(1)[0]
	if err := testModels.Save(model); err != nil {
		t.Errorf("Unexpected error in testModels.Save: %s", err.Error())
	}

	// Make sure the model was saved correctly
	expectModelExists(t, testModels, model)
	key, _ := testModels.ModelKey(model.Id())
	expectFieldEquals(t, key, "Int", model.Int)
	expectFieldEquals(t, key, "String", model.String)
	expectFieldEquals(t, key, "Bool", model.Bool)
}

func TestFind(t *testing.T) {
	testingSetUp()
	defer testingTearDown()

	// Create and save some test models
	models, err := createAndSaveTestModels(1)
	if err != nil {
		t.Errorf("Unexpected error saving test models: %s", err.Error())
	}
	model := models[0]

	// Find the model in the database and store it in modelCopy
	modelCopy := &testModel{}
	if err := testModels.Find(model.Id(), modelCopy); err != nil {
		t.Errorf("Unexpected error in testModels.Find: %s", err.Error())
	}
	if !reflect.DeepEqual(model, modelCopy) {
		t.Errorf("Found model was incorrect.\n\tExpected: %+v\n\tBut got:  %+v", model, modelCopy)
	}
}

func TestFindAll(t *testing.T) {
	testingSetUp()
	defer testingTearDown()

	// Create and save some test models
	models, err := createAndSaveTestModels(5)
	if err != nil {
		t.Errorf("Unexpected error saving test models: %s", err.Error())
	}

	// Use MFind to find four of the models in the database and store them in
	// modelsCopy
	modelsCopy := []*testModel{}
	ids := []string{}
	for _, model := range models[1:] {
		ids = append(ids, model.Id())
	}
	if err := testModels.FindAll(&modelsCopy); err != nil {
		t.Errorf("Unexpected error in testModels.FindAll: %s", err.Error())
	}

	// Check the models in modelsCopy
	if len(modelsCopy) != len(models) {
		t.Errorf("modelsCopy was the wrong length. Expected %d but got %d", len(models), len(modelsCopy))
	}
	modelsById := map[string]*testModel{}
	for _, model := range models {
		modelsById[model.Id()] = model
	}
	for i, modelCopy := range modelsCopy {
		if modelCopy.Id() == "" {
			t.Errorf("modelsCopy[%d].Id() is empty.", i)
			continue
		}
		model, found := modelsById[modelCopy.Id()]
		if !found {
			t.Errorf("modelsCopy[%d].Id() was invalid. Got %s but expected one of %v", i, modelCopy.Id(), ids)
			continue
		}
		if !reflect.DeepEqual(model, modelCopy) {
			t.Errorf("Found model was incorrect.\n\tExpected: %+v\n\tBut got:  %+v", model, modelCopy)
		}
	}
}

func TestCount(t *testing.T) {
	testingSetUp()
	defer testingTearDown()

	// Expect count to be zero if we haven't saved any models
	got, err := testModels.Count()
	if err != nil {
		t.Errorf("Unexpected error in testModels.Count: %s", err.Error())
	}
	if got != 0 {
		t.Errorf("Expected Count to be 0 when no models existed but got %d", got)
	}

	// Create and save some test models
	expected := 5
	_, err = createAndSaveTestModels(expected)
	if err != nil {
		t.Errorf("Unexpected error saving test models: %s", err.Error())
	}

	// Expect count to be 5
	got, err = testModels.Count()
	if err != nil {
		t.Errorf("Unexpected error in testModels.Count: %s", err.Error())
	}
	if got != expected {
		t.Errorf("Expected Count to be %d but got %d", expected, got)
	}

}

func TestDelete(t *testing.T) {
	testingSetUp()
	defer testingTearDown()

	// Create and save a test model
	models, err := createAndSaveTestModels(1)
	if err != nil {
		t.Errorf("Unexpected error saving test models: %s", err.Error())
	}
	model := models[0]

	// Delete the model we just saved
	deleted, err := testModels.Delete(model.Id())
	if err != nil {
		t.Errorf("Unexpected error in testModels.Delete: %s", err.Error())
	}
	if !deleted {
		t.Errorf("Expected deleted to be true but got false")
	}

	// Make sure the model was deleted
	expectModelDoesNotExist(t, testModels, model)

	// A second call to Delete should return false
	deleted, err = testModels.Delete(model.Id())
	if err != nil {
		t.Errorf("Unexpected error in testModels.Delete: %s", err.Error())
	}
	if deleted {
		t.Errorf("Expected deleted to be false but got true")
	}
}

func TestDeleteAll(t *testing.T) {
	testingSetUp()
	defer testingTearDown()

	// The first time we call DeleteAll we expect count to be 0 because
	// there are no models
	count, err := testModels.DeleteAll()
	if err != nil {
		t.Errorf("Unexpected error in testModels.Delete: %s", err.Error())
	}
	if count != 0 {
		t.Errorf("Expected count to be 0 but got %d", count)
	}

	// Create and save some test models
	models, err := createAndSaveTestModels(5)
	if err != nil {
		t.Errorf("Unexpected error saving test models: %s", err.Error())
	}

	// Call DeleteAll again
	count, err = testModels.DeleteAll()
	if err != nil {
		t.Errorf("Unexpected error in testModels.Delete: %s", err.Error())
	}
	if count != 5 {
		t.Errorf("Expected count to be 5 but got %d", count)
	}

	// Make sure the models were deleted
	expectModelsDoNotExist(t, testModels, Models(models))
}
