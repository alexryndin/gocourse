package main

// тут вы пишете код
// обращаю ваше внимание - в этом задании запрещены глобальные переменные

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"

	_ "github.com/go-sql-driver/mysql"
)

type CR map[string]interface{}

const DATABASE = "gocourse"

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
	switch r.Method {
	case http.MethodGet:
		h.ServeGet(w, r)
	case http.MethodPut:
		h.ServePut(w, r)
	case http.MethodPost:
		h.ServePost(w, r)
	case http.MethodDelete:
		h.ServeDelete(w, r)
	default:
		err := ApiError{
			404,
			fmt.Errorf("unknown method"),
		}
		handleError(w, err)
		return
	}

}

func (h *Handler) ServeGet(w http.ResponseWriter, r *http.Request) {
	paths := strings.SplitN(r.URL.Path, "/", -1)
	if len(paths) == 2 {
		if paths[1] == "" {
			h.ShowTables2(w, r)
			return
		} else {
			h.ShowTable(w, r)
			return
		}
	} else if len(paths) == 3 {
		if paths[2] != "" {
			h.ShowRecord(w, r)
			return
		}
	}
	err := ApiError{
		404,
		fmt.Errorf("unknown method"),
	}
	handleError(w, err)
	return
}

func (h *Handler) doChangeData(r *http.Request, update bool, id int) (map[string]interface{}, error) {
	tableName := strings.SplitN(r.URL.Path, "/", -1)[1]
	key, err := h.getPrimaryCol(tableName)
	if err != nil {
		return nil, err
	}
	query := fmt.Sprintf(`SELECT COLUMN_NAME, DATA_TYPE, IS_NULLABLE
		FROM INFORMATION_SCHEMA.COLUMNS
		WHERE TABLE_NAME = '%s' and TABLE_SCHEMA = '%s';`, tableName, DATABASE)
	rows, err := h.DB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	where, what := make([]string, 0, 2), make([]interface{}, 0, 2)
	decoder := json.NewDecoder(r.Body)
	defer r.Body.Close()
	var r_params map[string]interface{}
	if err := decoder.Decode(&r_params); err != nil {
		return nil, err
	}
	for rows.Next() {
		result := make([]interface{}, 3)
		for i, _ := range result {
			result[i] = new(string)
		}
		err := rows.Scan(result...)
		c_name := *result[0].(*string)
		c_type := *result[1].(*string)
		is_nullable := *result[2].(*string)
		if strings.ToLower(c_name) == key {
			if update {
				if _, ok := r_params[c_name]; ok {
					return nil, ApiError{400, fmt.Errorf("field %s have invalid type", c_name)}
				}
			}
			continue
		}
		if v, ok := r_params[c_name]; ok {
			if update {
				where = append(where, fmt.Sprintf("`%s` = ?", c_name))
			} else {
				where = append(where, fmt.Sprintf("`%s`", c_name))
			}
			fmt.Println("cname =", c_name, "type = ", reflect.TypeOf(v))
			switch c_type {
			case "int":
				switch v.(type) {
				case float64:
					what = append(what, v.(int))
				case int:
					what = append(what, v)
				default:
					return nil, ApiError{400, fmt.Errorf("field %s have invalid type", c_name)}
				}
			case "varchar", "text":
				switch v.(type) {
				case string:
					what = append(what, v)
				case nil:
					if is_nullable == "YES" {
						what = append(what, v)
					} else {
						return nil, ApiError{400, fmt.Errorf("field %s have invalid type", c_name)}
					}
				default:
					what = append(what, v)
					return nil, ApiError{400, fmt.Errorf("field %s have invalid type", c_name)}
				}
			default:
				what = append(what, v)

			}
			// what = append(what, r.FormValue(c_name))
		} else {
			if is_nullable == "NO" && !update {
				fmt.Println("what to add - ", c_name)
				switch c_type {
				case "int":
					what = append(what, 0)
				case "varchar", "text":
					what = append(what, "")
				default:
					return nil, ApiError{500, fmt.Errorf("unsupported DB type: %s", c_type)}
				}
				where = append(where, fmt.Sprintf("`%s`", c_name))
			}
		}

		if err != nil {
			return nil, err
		}
	}
	if len(what) > 0 {
		var query string
		if update {
			query = fmt.Sprintf(`UPDATE %s SET %s WHERE %s = ?`, tableName, strings.Join(where, ", "), key)
			what = append(what, id)
			fmt.Println("what ", what)
		} else {
			qstns := strings.Repeat("?, ", len(what))
			query = fmt.Sprintf(`INSERT INTO %s (%s) VALUES (%s)`, tableName, strings.Join(where, ", "), qstns[:len(qstns)-2])
		}
		fmt.Println("query to exec --> ", query)
		r, err := h.DB.Exec(query, what...)
		if err != nil {
			fmt.Println("I'm here")
			fmt.Println(err.Error())
			return nil, err
		}
		lastID, err := r.LastInsertId()
		if err != nil {
			return nil, err
		}
		raf, err := r.RowsAffected()
		if err != nil {
			return nil, err
		}
		resp := make(map[string]interface{})
		if update {
			resp["response"] = CR{"updated": raf}
		} else {
			resp["response"] = CR{key: lastID}
		}
		return resp, nil

	} else {
		return nil, fmt.Errorf("Corrupted query")
	}
}

func (h *Handler) ServeDelete(w http.ResponseWriter, r *http.Request) {
	if id, err := strconv.ParseInt(strings.SplitN(r.URL.Path, "/", -1)[2], 10, 32); err != nil {
		handleError(w, err)
		return
	} else {
		r, err := h.DB.Exec("DELETE FROM items WHERE id = ?", id)
		if err != nil {
			handleError(w, err)
		}
		raf, err := r.RowsAffected()
		if err != nil {
			handleError(w, err)
		}
		resp := make(map[string]interface{})
		resp["response"] = CR{"deleted": raf}
		marshalAndWrite(w, resp, 200)
		return
	}
}

func (h *Handler) ServePost(w http.ResponseWriter, r *http.Request) {
	if id, err := strconv.ParseInt(strings.SplitN(r.URL.Path, "/", -1)[2], 10, 32); err != nil {
		handleError(w, err)
		return
	} else {
		resp, err := h.doChangeData(r, true, int(id))
		if err != nil {
			handleError(w, err)
			fmt.Println(err.Error())
			return
		}
		marshalAndWrite(w, resp, 200)
	}
}

func (h *Handler) ServePut(w http.ResponseWriter, r *http.Request) {
	resp, err := h.doChangeData(r, false, 0)
	if err != nil {
		handleError(w, err)
	}
	marshalAndWrite(w, resp, 200)

}

//func (h *Handler) RequestWrapper(w http.ResponseWriter, r *http.Request) {
//
//}

func parseInt(r *http.Request, query string, value *int64) error {
	if r.FormValue(query) != "" {
		if val, err := strconv.ParseInt(r.FormValue(query), 10, 32); err != nil {
			return err
		} else {
			*value = val
		}
	}
	return nil
}

func AssertInterfaces(ct []*sql.ColumnType, s []interface{}) {
	for i, _ := range ct {
		var in interface{}
		s[i] = &in
	}
}

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

func retriveValue(ic interface{}) (out interface{}, err error) {
	switch ic.(type) {
	case *sql.NullInt32:
		if value, ok := ic.(*sql.NullInt32); ok {
			out, err = value.Value()
		}
	case *sql.NullFloat64:
		if value, ok := ic.(*sql.NullFloat64); ok {
			out, err = value.Value()
		}
	case *sql.NullString:
		if value, ok := ic.(*sql.NullString); ok {
			out, err = value.Value()
		}
	case *sql.NullBool:
		if value, ok := ic.(*sql.NullBool); ok {
			out, err = value.Value()
		}
	default:
		out, err = ic, nil
	}
	return out, err
}

func (h *Handler) ShowTable(w http.ResponseWriter, r *http.Request) {
	tableName := strings.SplitN(r.URL.Path, "/", -1)[1]
	var limit, offset int64 = 5, 0
	parseInt(r, "limit", &limit)
	parseInt(r, "offset", &offset)
	query := fmt.Sprintf("SELECT * FROM %s LIMIT ? OFFSET ?", tableName)
	fmt.Println("limit = ", limit)
	fmt.Println("offset = ", offset)
	rows, err := h.DB.Query(query, limit, offset)
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
	defer rows.Close()
	records := make([]interface{}, 0, 5)
	resp := make(map[string]interface{})
	for rows.Next() {
		if ans, err := ScanRow(rows); err != nil {
			handleError(w, err)
			return
		} else {
			records = append(records, ans)
		}
	}
	resp["response"] = CR{"records": records}
	marshalAndWrite(w, resp, 200)

}

func ScanRow(rows *sql.Rows) (map[string]interface{}, error) {
	ct, err := rows.ColumnTypes()
	if err != nil {
		return nil, err
	}
	record := make([]interface{}, len(ct))
	AssertColumns(ct, record)
	if err := rows.Scan(record...); err != nil {
		return nil, err
	}
	ans := make(map[string]interface{})
	for i, t := range ct {
		value, err := retriveValue(record[i])
		if err != nil {
			return nil, err
		}
		ans[t.Name()] = value
	}
	return ans, nil
}

func (h *Handler) getPrimaryCol(tableName string) (string, error) {
	query := fmt.Sprintf(`SELECT COLUMN_NAME
		FROM INFORMATION_SCHEMA.COLUMNS
		WHERE TABLE_NAME = '%s' and TABLE_SCHEMA = '%s' AND COLUMN_KEY='PRI';`, tableName, DATABASE)
	row := h.DB.QueryRow(query)
	key := new(string)
	err := row.Scan(key)
	if err != nil {
		fmt.Println(err.Error())
		return "", err
	}
	return *key, nil
}

func (h *Handler) ShowRecord(w http.ResponseWriter, r *http.Request) {
	tableName := strings.SplitN(r.URL.Path, "/", -1)[1]
	id := strings.SplitN(r.URL.Path, "/", -1)[2]
	key, err := h.getPrimaryCol(tableName)
	if err != nil {
		handleError(w, err)
		return
	}
	query := fmt.Sprintf("SELECT * FROM %s WHERE `%s` = ?", tableName, key)
	rows, err := h.DB.Query(query, id)
	if err != nil {
		fmt.Println(err.Error())
		handleError(w, err)
		return
	}
	defer rows.Close()
	if rows.Next() {
		if ans, err := ScanRow(rows); err != nil {
			handleError(w, err)
			return
		} else {
			resp := make(map[string]interface{})
			resp["response"] = CR{"record": ans}
			marshalAndWrite(w, resp, 200)
			return
		}
	} else {
		err := ApiError{
			404,
			fmt.Errorf("record not found"),
		}
		handleError(w, err)

	}

}

func (h *Handler) ShowTables2(w http.ResponseWriter, r *http.Request) {
	rows, err := h.DB.Query("SHOW TABLES")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, err.Error())
		return
	}
	defer rows.Close()
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
