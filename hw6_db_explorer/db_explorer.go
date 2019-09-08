package main

// тут вы пишете код
// обращаю ваше внимание - в этом задании запрещены глобальные переменные

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"

	_ "github.com/go-sql-driver/mysql"
)

type CR map[string]interface{}

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

type ApiError struct {
	HTTPStatus int
	Err        error
}

func (ae ApiError) Error() string {
	return ae.Err.Error()
}

type Handler struct {
	DB *sql.DB
}

func NewDbExplorer(db *sql.DB) (*Handler, error) {
	h := &Handler{db}
	http.Handle("/", h)
	return h, nil
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	paths := strings.SplitN(r.URL.Path, "/", -1)
	if len(paths) == 2 {
		if paths[1] == "" {
			h.ShowTables2(w, r)
			return
		} else {
			h.ShowTable(w, r)
			return
		}
	}
	err := ApiError{
		404,
		fmt.Errorf("unknown method"),
	}
	handleError(w, err)
	return
	//	 h.wrapperDoSomeJob(w, r)
}

//func (h *Handler) RequestWrapper(w http.ResponseWriter, r *http.Request) {
//
//}

func AssertColumns(ct []*sql.ColumnType, s []interface{}) {
	for i, v := range ct {
		if nullable, ok := v.Nullable(); ok {
			if nullable {
				switch v.DatabaseTypeName() {
				case "INT":
					s[i] = &sql.NullInt32{}
				case "VARCHAR":
					s[i] = &sql.NullString{}
				case "TEXT":
					s[i] = &sql.NullString{}
				default:
					s[i] = &sql.NullFloat64{}
				}
			} else {
				switch v.DatabaseTypeName() {
				case "INT":
					s[i] = new(int)
				case "VARCHAR":
					s[i] = new(string)
				case "TEXT":
					s[i] = new(string)
				default:
					s[i] = new(float64)
				}
			}
		} else {
			panic("not ok")
		}
		// if v.ScanType().Name() == "RawBytes" {
		// 	s[i] = new(string)

		// } else {
		// 	s[i] = reflect.New(v.ScanType()).Interface()
		// }
	}
}

func retriveValue(ic interface{}) (out interface{}) {
	switch ic.(type) {
	case *sql.NullInt32:
		if value, ok := ic.(*sql.NullInt32); ok {
			out = value.Int32
		}
	case *sql.NullFloat64:
		if value, ok := ic.(*sql.NullFloat64); ok {
			if value.Scan()
			out = value.Float64
		}
	case *sql.NullString:
		if value, ok := ic.(*sql.NullString); ok {
			out = value.String
		}
	case *sql.NullBool:
		if value, ok := ic.(*sql.NullBool); ok {
			out = value.Bool
		}
	default:
		out = ic
	}
	return out
}

func (h *Handler) ShowTable(w http.ResponseWriter, r *http.Request) {

	tableName := strings.SplitN(r.URL.Path, "/", -1)[1]
	query := fmt.Sprintf("SELECT * FROM %s LIMIT 10", tableName)
	rows, err := h.DB.Query(query)
	if err != nil {
		if strings.HasPrefix(err.Error(), "Error 1146") {
			err := ApiError{
				404,
				fmt.Errorf("unknown table"),
			}
			handleError(w, err)
			return
		}
		handleError(w, err)
		return
	}
	ct, err := rows.ColumnTypes()
	if err != nil {
		handleError(w, err)
		return
	}
	records := make([]interface{}, 0, 5)
	resp := make(map[string]interface{})
	for rows.Next() {
		record := make([]interface{}, len(ct))
		AssertColumns(ct, record)
		err = rows.Scan(record...)
		if err != nil {
			handleError(w, err)
			return
		}
		ans := make(map[string]interface{})
		for i, t := range ct {
			ans[t.Name()] = retriveValue(record[i])
		}
		records = append(records, ans)
	}
	resp["response"] = CR{"records": records}
	marshalAndWrite(w, resp, 200)

}

func (h *Handler) ShowTables2(w http.ResponseWriter, r *http.Request) {
	rows, err := h.DB.Query("SHOW TABLES")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, err.Error())
		return
	}
	resp := make(map[string]interface{})
	ans := make(map[string]interface{})
	tables := make([]string, 0, 5)
	for rows.Next() {
		var s string
		rows.Scan(&s)
		tables = append(tables, s)
	}
	ans["tables"] = tables
	resp["response"] = ans

	marshalAndWrite(w, resp, 200)
}

func (h *Handler) ShowTables(w http.ResponseWriter, r *http.Request) {

	rows, err := h.DB.Query("SHOW TABLES")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, err.Error())
		return
	}

	tables := make([]string, 0, 5)
	for rows.Next() {

		var s string
		rows.Scan(&s)

		c, _ := rows.Columns()
		fmt.Println(c)

		ct, err := rows.ColumnTypes()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, err.Error())
			return
		}
		for _, v := range ct {
			//fmt.Fprintln(w, v.Name(), ": ", v.DatabaseTypeName(), ": ", v.ScanType())
			fmt.Println(v.Name(), ": ", v.DatabaseTypeName(), ": ", v.ScanType())
			//	fmt.Fprintln(w, s)
			tables = append(tables, s)
		}
	}

	for _, v := range tables {
		fmt.Println("Tablename to query --> ", v)
		query := fmt.Sprintf("SELECT * from %v", v)
		trows, err := h.DB.Query(query)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, err.Error())
			return
		}
		for trows.Next() {
			ct, err := trows.ColumnTypes()
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprint(w, err.Error())
				return
			}
			for _, v := range ct {
				fmt.Fprintln(w, v.Name(), ": ", v.DatabaseTypeName(), ": ", v.ScanType())
				fmt.Println(v.Name(), ": ", v.DatabaseTypeName(), ": ", v.ScanType())
			}
			vals := make([]interface{}, len(ct))
			AssertColumns(ct, vals)
			//			vals[1] = new(string)
			trows.Scan(vals...)
			fmt.Println(vals[1])
			fmt.Println(reflect.TypeOf(vals[1]))
			fmt.Println(string(*vals[1].(*sql.RawBytes)))

		}

	}
}
