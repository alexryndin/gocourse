package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	//	"net/url"
	//	"reflect"
	"strconv"
)

func handleError(w http.ResponseWriter, err error) {
	resp := make(map[string]interface{})
	resp["error"] = ""
	status := 500

	switch errt := err.(type) {
	case ApiError:
		resp["error"] = errt.Error()
		status = errt.HTTPStatus
	default:
		resp["error"] = errt.Error()
	}
	marshalAndWrite(w, resp, status)
}

func marshalAndWrite(w http.ResponseWriter, resp map[string]interface{}, status int) {
	if enc, err := json.Marshal(resp); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "InternalServerError")
		return
	} else {
		w.WriteHeader(status)
		w.Write(enc)
		return
	}
}

func (h *MyApi) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/user/profile":
		h.profileWrapper(w, r)
	case "/user/create":
		h.createWrapper(w, r)
	default:
		err := ApiError{
			404,
			fmt.Errorf("unknown method"),
		}
		handleError(w, err)
		//	 h.wrapperDoSomeJob(w, r)
	}
}

func (h *OtherApi) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/user/create":
		h.createWrapper(w, r)
	default:
		err := ApiError{
			404,
			fmt.Errorf("unknown method"),
		}
		handleError(w, err)
		//	 h.wrapperDoSomeJob(w, r)
	}
}



func (h *MyApi) profileWrapper(w http.ResponseWriter, r *http.Request) {


	params := ProfileParams{}
	if err := params.decode(r); err != nil {
		handleError(w, err)
		return
	}
	if err := params.validate(); err != nil {
		handleError(w, err)
		return
	}
	profile, err := h.Profile(r.Context(), params)
	if err != nil {
		handleError(w, err)
		return
	}
	resp := make(map[string]interface{})
	resp["response"] = profile
	resp["error"] = ""
	marshalAndWrite(w, resp, 200)
}

func (h *MyApi) createWrapper(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodPost {
		err := ApiError{
			406,
			fmt.Errorf("bad method"),
		}
		handleError(w, err)
		return
	}


	if r.Header.Get("X-Auth") != "100500" {
		err := ApiError{
			403,
			fmt.Errorf("unauthorized"),
		}
		handleError(w, err)
		return
	}

	params := CreateParams{}
	if err := params.decode(r); err != nil {
		handleError(w, err)
		return
	}
	if err := params.validate(); err != nil {
		handleError(w, err)
		return
	}
	create, err := h.Create(r.Context(), params)
	if err != nil {
		handleError(w, err)
		return
	}
	resp := make(map[string]interface{})
	resp["response"] = create
	resp["error"] = ""
	marshalAndWrite(w, resp, 200)
}



func (h *OtherApi) createWrapper(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodPost {
		err := ApiError{
			406,
			fmt.Errorf("bad method"),
		}
		handleError(w, err)
		return
	}


	if r.Header.Get("X-Auth") != "100500" {
		err := ApiError{
			403,
			fmt.Errorf("unauthorized"),
		}
		handleError(w, err)
		return
	}

	params := OtherCreateParams{}
	if err := params.decode(r); err != nil {
		handleError(w, err)
		return
	}
	if err := params.validate(); err != nil {
		handleError(w, err)
		return
	}
	create, err := h.Create(r.Context(), params)
	if err != nil {
		handleError(w, err)
		return
	}
	resp := make(map[string]interface{})
	resp["response"] = create
	resp["error"] = ""
	marshalAndWrite(w, resp, 200)
}


	
func (dst *ProfileParams) decode(r *http.Request) error {
	dst.Login = r.FormValue("login")
	return nil
}
func (dst *ProfileParams) validate() error {
	if dst.Login == "" {
		return ApiError{
			http.StatusBadRequest,
			fmt.Errorf("login must me not empty"),
		}
	}
	

	return nil
}


func (dst *CreateParams) decode(r *http.Request) error {
	dst.Login = r.FormValue("login")
	dst.Name = r.FormValue("full_name")
	dst.Status = r.FormValue("status")
	i, err := strconv.Atoi(r.FormValue("age"))
	if err != nil {
		return ApiError{
			http.StatusBadRequest,
			fmt.Errorf("age must be int"),
		}
	}
	dst.Age = i
	
	return nil
}
func (dst *CreateParams) validate() error {
	if dst.Login == "" {
		return ApiError{
			http.StatusBadRequest,
			fmt.Errorf("login must me not empty"),
		}
	}
	if len(dst.Login) < 10 {
		return ApiError{
			http.StatusBadRequest,
			fmt.Errorf("login len must be >= 10"),
		}
	}
	if dst.Status == "" {
		dst.Status = "user"
	}
	status_map := map[string]bool{
		"user": true,
		"moderator": true,
		"admin": true,
	}
	if _, present := status_map[dst.Status]; !present {
		return ApiError{
			http.StatusBadRequest,
			fmt.Errorf("status must be one of [user, moderator, admin]"),
		}
	}
	if dst.Age < 0 {
		return ApiError{
			http.StatusBadRequest,
			fmt.Errorf("age must be >= 0"),
		}
	}
	if dst.Age > 128 {
		return ApiError{
			http.StatusBadRequest,
			fmt.Errorf("age must be <= 128"),
		}
	}
	

	return nil
}


func (dst *OtherCreateParams) decode(r *http.Request) error {
	dst.Username = r.FormValue("username")
	dst.Name = r.FormValue("account_name")
	dst.Class = r.FormValue("class")
	i, err := strconv.Atoi(r.FormValue("level"))
	if err != nil {
		return ApiError{
			http.StatusBadRequest,
			fmt.Errorf("age must be int"),
		}
	}
	dst.Level = i
	
	return nil
}
func (dst *OtherCreateParams) validate() error {
	if dst.Username == "" {
		return ApiError{
			http.StatusBadRequest,
			fmt.Errorf("username must me not empty"),
		}
	}
	if len(dst.Username) < 3 {
		return ApiError{
			http.StatusBadRequest,
			fmt.Errorf("username len must be >= 3"),
		}
	}
	if dst.Class == "" {
		dst.Class = "warrior"
	}
	class_map := map[string]bool{
		"warrior": true,
		"sorcerer": true,
		"rouge": true,
	}
	if _, present := class_map[dst.Class]; !present {
		return ApiError{
			http.StatusBadRequest,
			fmt.Errorf("class must be one of [warrior, sorcerer, rouge]"),
		}
	}
	if dst.Level < 1 {
		return ApiError{
			http.StatusBadRequest,
			fmt.Errorf("level must be >= 1"),
		}
	}
	if dst.Level > 50 {
		return ApiError{
			http.StatusBadRequest,
			fmt.Errorf("level must be <= 50"),
		}
	}
	

	return nil
}

