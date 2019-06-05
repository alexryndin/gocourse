package main

import (
	"bufio"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strings"
	"strconv"
	"testing"
	"time"
)

func min(a int, b int) int {
	if a < b {
		return a
	} else {
		return b
	}

}

type TestCase struct {
	Req     *SearchRequest
	IsError bool
}

type Users struct {
	Version string    `xml:"version,attr"`
	List    []XmlUser `xml:"row"`
}

type XmlUser struct {
	Id       int    `xml:"id"`
	isActibe bool   `xml:"isActive"`
	Name     string `xml:"first_name"`
	LastName string `xml:"last_name"`
	Age      int    `xml:"age"`
	About    string `xml:"about"`
	Gender   string `xml:"gender"`
}

type ById []XmlUser

func (a ById) Len() int           { return len(a) }
func (a ById) Less(i, j int) bool { return a[i].Id < a[j].Id }
func (a ById) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }

type ByAbout []XmlUser

func (a ByAbout) Len() int           { return len(a) }
func (a ByAbout) Less(i, j int) bool { return a[i].About < a[j].About }
func (a ByAbout) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }

type ByName []XmlUser

func (a ByName) Len() int           { return len(a) }
func (a ByName) Less(i, j int) bool { return a[i].Name < a[j].Name }
func (a ByName) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func nameOrAbout(User XmlUser, query string) bool {
	return strings.Contains(User.About, query) ||
		strings.Contains(User.Name, query) ||
		strings.Contains(User.LastName, query)
}

func filterXmlUsers(users []XmlUser, query string) []XmlUser {
	vsf := make([]XmlUser, 0, 10)
	for _, v := range users {
		if nameOrAbout(v, query) {
			vsf = append(vsf, v)
		}
	}
	return vsf
}

func timeOutHandler(w http.ResponseWriter, r *http.Request) {
	time.Sleep(2999 * time.Millisecond)
	w.WriteHeader(http.StatusGatewayTimeout)
	return
}

func unknHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Print(w.Header().Get("Location"))
	w.Header().Del("Location")
	w.Header().Set("Location", "")
	w.WriteHeader(http.StatusOK)
	w.Header().Write(w)
	return
}

func handler(w http.ResponseWriter, r *http.Request) {
	users, err := getXmlStruct()
	check(err)
	// fmt.Fprintln(w, "URL:", r.URL.String())
	if token := r.Header.Get("AccessToken"); token == "" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	query := r.URL.Query().Get("query")
	if query != "" {
		// fmt.Fprintln(w, "‘query‘ is", query)
		users = filterXmlUsers(users, query)
	}

	limit := r.URL.Query().Get("limit")
	ilimit, err := strconv.Atoi(limit)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Println("500, Couldn't get limit")
		return
	}
	if limit != "" {
		// fmt.Fprintln(w, "‘query‘ is", query)
	}

	order_field := r.URL.Query().Get("order_field")
	switch order_field {
	case "Id":
		sort.Sort(ById(users))
	case "About":
		sort.Sort(ByAbout(users))
	case "Name":
		sort.Sort(ByName(users))
	case "":
		sort.Sort(ByName(users))
	case "__broken_json":
		fmt.Fprint(w, `{brokenjson}`)
		return
	case "__bad_request":
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"Error": "ErrorBadRequest"}`)
		return
	case "__very_bad_request":
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{brokenjson}`)
		return
	case "__internal_error":
		w.WriteHeader(http.StatusInternalServerError)
		return
	default:
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"Error": "ErrorBadOrderField"}`)
		return
	}
	if order_field != "" {
		// fmt.Fprintln(w, "‘order_field ‘ is", order_field)
	}
	json_r, err := json.Marshal(users[0:min(ilimit, len(users))])
	check(err)
	fmt.Fprint(w, string(json_r))

}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", handler)
	server := http.Server{
		Addr:         ":8080",
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	fmt.Println("starting server at :8080")
	server.ListenAndServe()
}

func TestGeneralCases(t *testing.T) {
	cases := []TestCase{
		TestCase{
			Req: &SearchRequest{
				Limit:      -2,
				Offset:     2,
				Query:      "dolor",
				OrderField: "Id",
				OrderBy:    1,
			},
			IsError: true,
		},
		TestCase{
			Req: &SearchRequest{
				Limit:      26,
				Offset:     2,
				Query:      "dolor",
				OrderField: "Id",
				OrderBy:    1,
			},
			IsError: false,
		},
		TestCase{
			Req: &SearchRequest{
				Limit:      2,
				Offset:     -1,
				Query:      "dolor",
				OrderField: "Id",
				OrderBy:    1,
			},
			IsError: true,
		},
		TestCase{
			Req: &SearchRequest{
				Limit:      2,
				Offset:     1,
				Query:      "dolor",
				OrderField: "Id",
				OrderBy:    1,
			},
			IsError: false,
		},
		TestCase{
			Req: &SearchRequest{
				Limit:      27,
				Offset:     1,
				Query:      "",
				OrderField: "Id",
				OrderBy:    1,
			},
			IsError: false,
		},
		TestCase{
			Req: &SearchRequest{
				Limit:      27,
				Offset:     1,
				Query:      "",
				OrderField: "__broken_json",
				OrderBy:    1,
			},
			IsError: true,
		},
		TestCase{
			Req: &SearchRequest{
				Limit:      27,
				Offset:     1,
				Query:      "",
				OrderField: "__internal_error",
				OrderBy:    1,
			},
			IsError: true,
		},
		TestCase{
			Req: &SearchRequest{
				Limit:      27,
				Offset:     1,
				Query:      "",
				OrderField: "__bad_request",
				OrderBy:    1,
			},
			IsError: true,
		},
		TestCase{
			Req: &SearchRequest{
				Limit:      27,
				Offset:     1,
				Query:      "",
				OrderField: "__very_bad_request",
				OrderBy:    1,
			},
			IsError: true,
		},
		TestCase{
			Req: &SearchRequest{
				Limit:      27,
				Offset:     1,
				Query:      "",
				OrderField: "PIZDA",
				OrderBy:    1,
			},
			IsError: true,
		},
	}

	ts := httptest.NewServer(http.HandlerFunc(handler))
	for caseNum, item := range cases {
		c := &SearchClient{
			AccessToken: "123",
			URL:         ts.URL,
		}
		_, err := c.FindUsers(*item.Req)

		if err != nil && !item.IsError {
			t.Errorf("[%d] unexpected error: %#v", caseNum, err)
		}
		if err == nil && item.IsError {
			t.Errorf("[%d] expected error, got nil", caseNum)
		}
	}
	ts.Close()
}

func TestToken(t *testing.T) {
	cases := []TestCase{
		TestCase{
			Req: &SearchRequest{
				Limit:      27,
				Offset:     1,
				Query:      "",
				OrderField: "__broken_json",
				OrderBy:    1,
			},
			IsError: true,
		},
	}

	ts := httptest.NewServer(http.HandlerFunc(handler))
	for caseNum, item := range cases {
		c := &SearchClient{
			AccessToken: "",
			URL:         ts.URL,
		}
		_, err := c.FindUsers(*item.Req)

		if err != nil && !item.IsError {
			t.Errorf("[%d] unexpected error: %#v", caseNum, err)
		}
		if err == nil && item.IsError {
			t.Errorf("[%d] expected error, got nil", caseNum)
		}
	}
	ts.Close()
}

func TestTimeout(t *testing.T) {
	cases := []TestCase{
		TestCase{
			Req: &SearchRequest{
				Limit:      27,
				Offset:     1,
				Query:      "timeout",
				OrderField: "Name",
				OrderBy:    1,
			},
			IsError: true,
		},
	}

	ts := httptest.NewServer(http.TimeoutHandler(http.HandlerFunc(timeOutHandler), 1 * time.Second, ""))
	for caseNum, item := range cases {
		c := &SearchClient{
			AccessToken: "123",
			URL:         ts.URL,
		}
		ts.CloseClientConnections()
		_, err := c.FindUsers(*item.Req)

		if err != nil && !item.IsError {
			t.Errorf("[%d] unexpected error: %#v", caseNum, err)
		}
		if err == nil && item.IsError {
			t.Errorf("[%d] expected error, got nil", caseNum)
		}
	}
	ts.Close()
}

func TestUnknErr(t *testing.T) {
	cases := []TestCase{
		TestCase{
			Req: &SearchRequest{
				Limit:      27,
				Offset:     1,
				Query:      "timeout",
				OrderField: "Name",
				OrderBy:    1,
			},
			IsError: true,
		},
	}

	ts := httptest.NewServer(http.HandlerFunc(unknHandler))
	for caseNum, item := range cases {
		c := &SearchClient{
			AccessToken: "123",
			URL:         "",
		}
		ts.CloseClientConnections()
		_, err := c.FindUsers(*item.Req)

		if err != nil && !item.IsError {
			t.Errorf("[%d] unexpected error: %#v", caseNum, err)
		}
		if err == nil && item.IsError {
			t.Errorf("[%d] expected error, got nil", caseNum)
		}
	}
	ts.Close()
}

// Warning!!!
// Here be DRAGONS!!!

func CountDecoder() {
	f, err := os.Open("dataset.xml")
	check(err)
	defer f.Close()
	input := bufio.NewReader(f)
	decoder := xml.NewDecoder(input)
	logins := make([]string, 0)
	var login string
	for {
		tok, tokenErr := decoder.Token()
		if tokenErr != nil && tokenErr != io.EOF {
			fmt.Println("error happend", tokenErr)
			break
		} else if tokenErr == io.EOF {
			break
		}
		if tok == nil {
			fmt.Println("t is nil break")
		}
		switch tok := tok.(type) {
		case xml.StartElement:
			if tok.Name.Local == "login" {
				if err := decoder.DecodeElement(&login, &tok); err != nil {
					fmt.Println("error happend", err)
				}
				logins = append(logins, login)
			}
		}
	}
	fmt.Println(logins)
}

func getXmlStruct() ([]XmlUser, error) {
	f, err := os.Open("dataset.xml")
	check(err)
	defer f.Close()
	reader := bufio.NewReader(f)
	xmlData, err := ioutil.ReadAll(reader)
	check(err)
	v := new(Users)
	err = xml.Unmarshal(xmlData, &v)
	if err != nil {
		fmt.Printf("error: %v", err)
		return v.List, err
	}
	return v.List, nil
}
