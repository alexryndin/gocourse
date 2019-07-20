// go build gen/* && ./codegen.exe pack/unpack.go  pack/marshaller.go
// go run pack/*
package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"os"
	"reflect"
	"strconv"
	"strings"
	"text/template"
)

var (
	header = `package main

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
}`

	serve = `
{{range $k, $v:= .}}
func (h *{{$k}}) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	{{- range $v}}
	case "{{.Url}}":
		h.{{.MethodName | ToLower }}Wrapper(w, r)
	{{- end}}
	default:
		err := ApiError{
			404,
			fmt.Errorf("unknown method"),
		}
		handleError(w, err)
		//	 h.wrapperDoSomeJob(w, r)
	}
}
{{end}}`

	wrapper = `
{{range $k, $v:= .}}
{{range $v}}
func (h *{{$k}}) {{ .MethodName | ToLower }}Wrapper(w http.ResponseWriter, r *http.Request) {
{{ if eq .Method "POST" }}
	if r.Method != http.MethodPost {
		err := ApiError{
			406,
			fmt.Errorf("bad method"),
		}
		handleError(w, err)
		return
	}
{{ end }}
{{ if .Auth }}
	if r.Header.Get("X-Auth") != "100500" {
		err := ApiError{
			403,
			fmt.Errorf("unauthorized"),
		}
		handleError(w, err)
		return
	}
{{ end }}
	params := {{.Input}}{}
	if err := params.decode(r); err != nil {
		handleError(w, err)
		return
	}
	if err := params.validate(); err != nil {
		handleError(w, err)
		return
	}
	{{ .MethodName | ToLower }}, err := h.{{.MethodName}}(r.Context(), params)
	if err != nil {
		handleError(w, err)
		return
	}
	resp := make(map[string]interface{})
	resp["response"] = {{ .MethodName | ToLower }}
	resp["error"] = ""
	marshalAndWrite(w, resp, 200)
}
{{end}}
{{end}}
	`
	decoder = `
func (dst *{{.StructName}}) decode(r *http.Request) error {
	{{- range $fv := .Validators -}}
	{{- if eq $fv.FieldType "int" }}
	i, err := strconv.Atoi(r.FormValue("{{ $fv.RequestFieldName | ToLower }}"))
	if err != nil {
		return ApiError{
			http.StatusBadRequest,
			fmt.Errorf("age must be int"),
		}
	}
	dst.{{- $fv.FieldName }} = i
	{{ else }}
	dst.{{ $fv.FieldName }} = r.FormValue("{{ $fv.RequestFieldName | ToLower }}")
	{{- end }}
	{{- end }}
	return nil
}`
	validator = `
func (dst *{{.StructName}}) validate() error {
	{{ range $fv := .Validators }}
	{{- if $fv.Validators.Required -}}
	if dst.{{ $fv.FieldName }} == "" {
		return ApiError{
			http.StatusBadRequest,
			fmt.Errorf("{{ $fv.FieldName | ToLower }} must me not empty"),
		}
	}
	{{ end }}
	{{- if $fv.Validators.Rmin -}}
	if {{if eq $fv.FieldType "string"}}len({{end}}dst.{{ $fv.FieldName }}{{if eq $fv.FieldType "string"}}){{end}} < {{ $fv.Validators.Min }} {
		return ApiError{
			http.StatusBadRequest,
			fmt.Errorf("{{ $fv.FieldName | ToLower }}{{if eq $fv.FieldType "string"}} len{{end}} must be >= {{ $fv.Validators.Min }}"),
		}
	}
	{{ end }}
	{{- if $fv.Validators.Rmax -}}
	if dst.{{if eq $fv.FieldType "string"}}len({{end}}{{ $fv.FieldName }}{{if eq $fv.FieldType "string"}}){{end}} > {{ $fv.Validators.Max }} {
		return ApiError{
			http.StatusBadRequest,
			fmt.Errorf("{{ $fv.FieldName | ToLower }} must be <= {{ $fv.Validators.Max }}"),
		}
	}
	{{ end }}
	{{- if ne $fv.Validators.Dflt "" -}}
	if dst.{{ $fv.FieldName }} == "" {
		dst.{{ $fv.FieldName }} = "{{ $fv.Validators.Dflt }}"
	}
	{{ end }}
	{{- if $fv.Validators.Enum -}}
	{{ $fv.FieldName | ToLower }}_map := map[string]bool{
	{{- range $r := $fv.Validators.Enum }}
		"{{ $r }}": true,
	{{- end }}
	}
	if _, present := {{ $fv.FieldName | ToLower }}_map[dst.{{ $fv.FieldName }}]; !present {
		return ApiError{
			http.StatusBadRequest,
			fmt.Errorf("{{ $fv.FieldName | ToLower }} must be one of [{{StringsJoin $fv.Validators.Enum ", "}}]"),
		}
	}
	{{ end }}
	{{- end }}

	return nil
}

`
)

type api struct {
	Url        string
	Auth       bool
	Method     string
	Input      string
	Reciever   string
	MethodName string
}

type structDesc struct {
	StructName string
	Validators []*structValidators
}

type structValidators struct {
	FieldName        string
	RequestFieldName string
	FieldType        string
	Validators       *fieldValidators
}

type fieldValidators struct {
	Required  bool
	Min       int
	Rmin      bool
	Max       int
	Rmax      bool
	ParamName string
	Enum      []string
	Dflt      string
}

type tpl struct {
	FieldName string
}

var (
	intTpl = template.Must(template.New("intTpl").Parse(`
	// {{.FieldName}}
	var {{.FieldName}}Raw uint32
	binary.Read(r, binary.LittleEndian, &{{.FieldName}}Raw)
	in.{{.FieldName}} = int({{.FieldName}}Raw)
`))

	strTpl = template.Must(template.New("strTpl").Parse(`
	// {{.FieldName}}
	var {{.FieldName}}LenRaw uint32
	binary.Read(r, binary.LittleEndian, &{{.FieldName}}LenRaw)
	{{.FieldName}}Raw := make([]byte, {{.FieldName}}LenRaw)
	binary.Read(r, binary.LittleEndian, &{{.FieldName}}Raw)
	in.{{.FieldName}} = string({{.FieldName}}Raw)
`))

	apis = make(map[string][]*api)
	sv   = make(map[string][]*structValidators)
)

func parseApi(dcls []ast.Decl, apis map[string][]*api) {
	for _, f := range dcls {
		fun, ok := f.(*ast.FuncDecl)
		if !ok {
			fmt.Printf("SKIP %T is not *ast.FuncDecl\n", f)
			continue
		} else {
			if fun.Doc != nil {
				cmts := fun.Doc.List
				cmt := fun.Doc.List[len(cmts)-1].Text
				cmt = strings.TrimPrefix(cmt, "//")
				cmt = strings.TrimSpace(cmt)
				if strings.HasPrefix(cmt, "apigen:api") {
					jsn := strings.TrimLeft(cmt, "apigen:api")
					jsn = strings.TrimSpace(jsn)
					//	var a api
					a := &api{}
					// TODO: handle error
					json.Unmarshal([]byte(jsn), a)
					fmt.Print(a)
					fname := fun.Name.Name
					a.MethodName = fname
					rcv := fun.Recv.List[0].Type
					// Let's suppose that input structure is always second param
					funParam := fun.Type.Params.List[1].Type
					if funParamA, ok := funParam.(*ast.Ident); !ok {
						fmt.Printf("Fun %v must have identifier as its second param", fun)
					} else {
						paramName := funParamA.Name
						a.Input = paramName
						fmt.Printf("Found api with following params: Method: %v %#v\n", fname, apis)

					}
					if starRcv, ok := rcv.(*ast.StarExpr); !ok {
						// TODO: Error here
						fmt.Println("%#v is not star expr, error", rcv)
					} else {
						fmt.Printf("FUCK!!!2, %T, %v", starRcv, starRcv)
						if expr, ok := starRcv.X.(*ast.Ident); !ok {
							fmt.Printf("Rcv %v must be identifier", rcv)
						} else {
							apiName := expr.Name

							apis[apiName] = append(apis[apiName], a)

						}

					}
				}
			}
		}
	}
}

func parseField(ft string) (*fieldValidators, error) {
	fv := &fieldValidators{}
	fts := strings.Split(ft, ",")
	for _, kv := range fts {
		if kv == "required" {
			fv.Required = true
		} else {
			if strings.Contains(kv, "=") {
				kvs := strings.SplitN(kv, "=", 2)
				switch kvs[0] {
				case "paramname":
					fv.ParamName = kvs[1]
				case "min":
					fv.Rmin = true
					min, err := strconv.Atoi(kvs[1])
					if err != nil {
						return fv, err
					} else {
						fv.Min = min
					}
				case "max":
					fv.Rmax = true
					max, err := strconv.Atoi(kvs[1])
					if err != nil {
						return fv, err
					} else {
						fv.Max = max
					}
				case "enum":
					fv.Enum = strings.Split(kvs[1], "|")
				case "default":
					fv.Dflt = kvs[1]
				}
			}
		}
	}
	return fv, nil

}

func parseStruct(cs *ast.StructType) ([]*structValidators, error) {
	svs := make([]*structValidators, 0, 5)
	for _, field := range cs.Fields.List {
		if field.Tag == nil {
			continue
		}
		tag := reflect.StructTag(field.Tag.Value[1 : len(field.Tag.Value)-1])
		if p, ok := tag.Lookup("apivalidator"); ok {
			sv := &structValidators{}
			if fv, err := parseField(p); err != nil {
				return svs, err
			} else {
				sv.Validators = fv
				// TODO: Multiple names ?!
				// Yes; - and we use only first one!
				sv.FieldName = field.Names[0].Name
				fmt.Println("FUCK <<>> Found these names:")
				fmt.Println(field.Names)
				fmt.Println("For the structure field")

				ftype := field.Type
				if ftypea, ok := ftype.(*ast.Ident); ok {
					sv.FieldType = ftypea.Name
				} else {
					err := fmt.Errorf("Error: fileld %v doesn't have ident as its type", field)
					return svs, err
				}
				svs = append(svs, sv)
			}
		} else {
			continue
		}
	}
	return svs, nil
}

func parseStructs(dcls []ast.Decl, sv map[string][]*structValidators) error {
	for _, n := range dcls {
		g, ok := n.(*ast.GenDecl)
		if !ok {
			fmt.Printf("SKIP %T is not *ast.GenDecl\n", g)
			continue
		}
		for _, spec := range g.Specs {
			currType, ok := spec.(*ast.TypeSpec)
			if !ok {
				fmt.Printf("SKIP %T is not ast.TypeSpec\n", spec)
				continue
			}
			currStruct, ok := currType.Type.(*ast.StructType)
			if !ok {
				fmt.Printf("SKIP %T is not ast.StructType\n", currStruct)
				continue
			}
			fmt.Printf("process struct %s\n", currType.Name.Name)
			if parsed, err := parseStruct(currStruct); err != nil {
				return err
			} else {
				sv[currType.Name.Name] = parsed
				fmt.Println("It seems like we found following validators: %#v \n", parsed)
			}
			fmt.Println("It seems like we found following validators: %#v \n", sv)
		}
	}
	return nil
}

//		needGen := false
//		for _, v := range m {
//			if currType.Name.Name == v.input {
//				needGen = true
//			}
//		}
//		if !needGen {
//			fmt.Printf("SKIP %v is not requested by any method\n", currType.Name.Name)
//			return
//		}

func main() {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, os.Args[1], nil, parser.ParseComments)
	if err != nil {
		log.Fatal(err)
	}

	out, _ := os.Create(os.Args[2])

	funcMap := template.FuncMap{
		"ToLower":     strings.ToLower,
		"StringsJoin": strings.Join,
	}

	header := template.Must(template.New("header").Funcs(funcMap).Parse(header))
	header.Execute(out, apis)

	// Parse functions to find all to generate methods for
	parseApi(node.Decls, apis)
	parseStructs(node.Decls, sv)
	fmt.Println("FUCK, FOUND FOLLOWING STRUCTS!")
	fmt.Println(sv)
	fmt.Print(apis["MyApi"][0].MethodName)

	serve := template.Must(template.New("serve").Funcs(funcMap).Parse(serve))
	serve.Execute(out, apis)

	wrapper := template.Must(template.New("wrapper").Funcs(funcMap).Parse(wrapper))

	wrapper.Execute(out, apis)

	for _, v := range sv {
		for _, fv := range v {
			if fv.Validators.ParamName != "" {
				fv.RequestFieldName = fv.Validators.ParamName
			} else {
				fv.RequestFieldName = fv.FieldName
			}
		}

	}

	for _, v := range sv {
		for _, fv := range v {
			fmt.Println("PARAMNAME")
			fmt.Println(strings.ToLower(fv.RequestFieldName))
		}

	}

	decoder := template.Must(template.New("decoder").Funcs(funcMap).Parse(decoder))
	validator := template.Must(template.New("validator").Funcs(funcMap).Parse(validator))
	for k, v := range apis {
		_ = k
		for _, m := range v {
			rcv := m.Input
			strct, ok := sv[rcv]
			if !ok {
				fmt.Println("Required struct definition not found, go fuck yourself!!!")
			} else {
				s := structDesc{
					StructName: rcv,
					Validators: strct,
				}
				fmt.Println("Show all ??")
				fmt.Println(strct)
				fmt.Println(m.Input)
				decoder.Execute(out, s)
				validator.Execute(out, s)

			}
		}
	}
	fmt.Println(apis["MyApi"][0].Input)

	//				switch fileType {
	//				case "int":
	//					intTpl.Execute(out, tpl{fieldName})
	//				case "string":
	//					strTpl.Execute(out, tpl{fieldName})
	//				default:
	//					log.Fatalln("unsupported", fileType)
	//				}
	//			}
	//
	//			fmt.Fprintln(out, "	return nil")
	//			fmt.Fprintln(out, "}") // end of Unpack func
	//			fmt.Fprintln(out)      // empty line
	//
	//		}
	//	}
}

// go build gen/* && ./codegen.exe pack/unpack.go  pack/marshaller.go
// go run pack/*
