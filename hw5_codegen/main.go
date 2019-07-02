package main

// это программа для которой ваш кодогенератор будет писать код
// запускать через go test -v, как обычно

// этот код закомментирован чтобы он не светился в тестовом покрытии

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	//	"reflect"
	"strconv"
)

func marshalAndWrite(w http.ResponseWriter, resp map[string]interface{}) {
	if enc, err := json.Marshal(resp); err != nil {
		fmt.Fprint(w, "InternalServerError")
		return
	} else {
		fmt.Fprint(w, enc)
		return
	}
}

func (h *MyApi) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "URL:", r.URL.String())
	fmt.Fprintln(w, "URL Path :", r.URL.Path)
	switch r.URL.Path {
	case "/user/Profile":
		var resp map[string]interface{}
		pp := ProfileParams{}
		if err := pp.decode(r.URL.Query()); err != nil {
			resp["error"] = err.Error()
			marshalAndWrite(w, resp)
			return
		}
		if err := pp.validate(); err != nil {
			resp["error"] = err.Error()
			marshalAndWrite(w, resp)
		}
		if profile, err := h.Profile(r.Context(), pp); err != nil {
			resp["error"] = err.Error()
			if enc, err := json.Marshal(resp); err != nil {
				fmt.Fprint(w, "InternalServerError")
				return
			} else {
				fmt.Fprint(w, enc)
				return
			}

		}
	case "/user/Create":
		cp := CreateParams{}
		if err := cp.decode(r.URL.Query()); err != nil {
			fmt.Fprint(w, err.Error())
			return
		}

		if err := cp.validate(); err != nil {
			fmt.Fprint(w, err.Error())
			return
		}
		h.Create(r.Context(), cp)
	default:
		fmt.Fprint(w, "404")
		//	 h.wrapperDoSomeJob(w, r)
	}
}

func (dst *ProfileParams) decode(src url.Values) error {
	dst.Login = src.Get("login")
	return nil
}

func (dst *ProfileParams) validate() error {
	if dst.Login == "" {
		return fmt.Errorf("error: Login required")
	}
	return nil
}

func (dst *CreateParams) decode(src url.Values) error {
	dst.Login = src.Get("login")
	dst.Name = src.Get("full_name")
	dst.Status = src.Get("status")
	i, err := strconv.Atoi(src.Get("age"))
	if err != nil {
		return fmt.Errorf("Couldn't parse str to int")
	}
	dst.Age = i
	return nil
}

func (dst *CreateParams) validate() error {

	if dst.Login == "" {
		return fmt.Errorf("error: Login required")
	}
	if len(dst.Login) < 10 {
		return fmt.Errorf("error:  Login must be > 10")
	}
	status_map := map[string]bool{
		"user":      true,
		"moderator": true,
		"admin":     true,
	}
	if dst.Status == "" {
		dst.Status = "user"
	}
	if _, present := status_map[dst.Status]; !present {
		return fmt.Errorf("status must be one of [user, moderator, admin]")
	}
	if dst.Age < 0 {
		return fmt.Errorf("error:  age must be >= 0")
	}
	if dst.Age > 128 {
		return fmt.Errorf("error:  age must be < 128")
	}
	return nil
}

func main() {
	// будет вызван метод ServeHTTP у структуры MyApi
	http.Handle("/user/", NewMyApi())

	fmt.Println("starting server at :8080")
	http.ListenAndServe(":8080", nil)
}
