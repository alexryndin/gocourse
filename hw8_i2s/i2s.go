package main

import (
	"fmt"
	"reflect"
)

func i2s(data interface{}, out interface{}) error {
	if reflect.ValueOf(out).Kind() != reflect.Ptr {
		return fmt.Errorf("Expected ptr")
	}
	dest := reflect.ValueOf(out).Elem()
	val := reflect.ValueOf(data)
	//	switch dest.Kind() {
	//	case reflect.Slice:
	//	case reflect.Struct:
	//		set_field(dest, val)
	//	}
	if err := set_field(dest, val); err != nil {
		return err
	}
	return nil

}

func set_struct(strct reflect.Value, data reflect.Value) error {
	if data.Kind() != reflect.Map {
		return fmt.Errorf("Tried to set struct fields not of map values")
	}

	for i := 0; i < strct.NumField(); i++ {
		destf := strct.Field(i)
		fieldname := strct.Type().Field(i).Name
		// fieldname := destf.Elem().Name()
		new_val := reflect.ValueOf(data.MapIndex(reflect.ValueOf(fieldname)).Interface())
		if !new_val.IsZero() {

			//set_field(destf, new_val)
			if err := set_field(destf, new_val); err != nil {
				return err
			}
		}
	}

	return nil

}

func set_field(field reflect.Value, data reflect.Value) error {
	if field.Kind() == data.Kind() && data.Kind() != reflect.Slice && data.Kind() != reflect.Struct {
		field.Set(data)
		return nil
	}

	switch field.Kind() {
	case reflect.Int:
		if data.Kind() == reflect.Float64 {
			field.Set(data.Convert(field.Type()))
		} else {
			return fmt.Errorf("cannot set int")
		}
	case reflect.Slice:
		if data.Kind() == reflect.Slice {
			new_slice := reflect.MakeSlice(field.Type(), 0, 0)
			for i := 0; i < data.Len(); i++ {
				new_elem := reflect.New(field.Type().Elem()).Elem()
				if err := set_struct(new_elem, reflect.ValueOf(data.Index(i).Interface())); err != nil {
					return err
				}
				// set_struct(new_elem, reflect.ValueOf(data.Index(i).Interface()))
				new_slice = reflect.Append(new_slice, new_elem)
			}
			field.Set(new_slice)
		} else {
			return fmt.Errorf("Incopat type")
		}
	case reflect.Struct:
		if err := set_struct(field, data); err != nil {
			return err
		}
		//set_struct(field, data)

	default:
		return fmt.Errorf("Error")
	}

	return nil

}
