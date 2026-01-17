package grit

import "reflect"

// registered models
var models = map[string]interface{}{}

// RegisterModel binds collection name to model
func RegisterModel(name string, model interface{}) {
	models[name] = model
}

// clone creates a new instance of a model
func clone(model interface{}) interface{} {
	t := reflect.TypeOf(model).Elem()
	return reflect.New(t).Interface()
}

// makeSlice creates a slice of model type
func makeSlice(model interface{}) interface{} {
	t := reflect.TypeOf(model).Elem()
	sliceType := reflect.SliceOf(t)
	return reflect.New(sliceType).Interface()
}
