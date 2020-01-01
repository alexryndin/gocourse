package main

import (
	"fmt"
	"reflect"
)

func i2s(data interface{}, out interface{}) error {
	dest := reflect.ValueOf(out).Elem()
	fmt.Println(dest.Kind())
	val := reflect.ValueOf(data)
	fmt.Println(val.Kind())
	for i := 0; i < dest.NumField(); i++ {
		destf := dest.Field(i)
		fieldname := dest.Type().Field(i).Name
		// fieldname := destf.Elem().Name()
		fmt.Println("lil", destf.Kind(), fieldname)
		new_val := reflect.ValueOf(val.MapIndex(reflect.ValueOf(fieldname)).Interface())
		switch destf.Kind() {
		case reflect.Int:
			if new_val.Kind() == reflect.Float64 {
				destf.Set(new_val.Convert(destf.Type()))

			}
		}
		destf.Set(new_val.Convert(destf.Type()))
		fmt.Println("new_val", new_val.Kind())
		fmt.Println("destf", destf.Kind())

	}
	fmt.Println(data)
	fmt.Println(out)
	return nil

}

func set_field(field reflect.Value, data reflect.Value) error {

}
