package main

import (
	"fmt"
	"io"
	"os"
	"sort"
	"path/filepath"
	"strings"
)

type byName []os.FileInfo

func (fn byName) Len() int { return len(fn) }

func (fn byName) Less(i, j int) bool {
	if fn[i].Name() < fn[j].Name() {
		return true
	}
	return false
}

func (fn byName) Swap(i, j int) { fn[i], fn[j] = fn[j], fn[i] }

func filterFL(fl []os.FileInfo) []os.FileInfo {
	ffl := make([]os.FileInfo, 0, len(fl))
	for _, v := range(fl) {
		if v.IsDir() {
			ffl = append(ffl, v)
		}
	}
	return ffl
}

func fSize(i int64) string {
	if i == 0 {
		return "empty"
	}
	return fmt.Sprintf("%vb", i)
}

func ge(rem int, ges []bool) string {
	var sb strings.Builder
	if len(ges) == 0 {
		if rem != 0 {
			sb.WriteString("├───")
		} else {
			sb.WriteString("└───")
		}
		return sb.String()
	} else {
		for _, v := range ges  {
			if v {
				sb.WriteString("│	")
			} else {
				sb.WriteString("	")
			}
		}
		if rem != 0 {
			sb.WriteString("├───")
		} else {
			sb.WriteString("└───")
		}
	}
	return sb.String()
}

func dirTree(out io.Writer, path string, pf bool) error {
	ge_state := make([]bool, 0, 10)
	return _dirTree (out, path, pf, ge_state)
}

func _dirTree(out io.Writer, path string, pf bool, ges []bool) error {
	if f, err := os.Open(path); err == nil {
		if fl, err := f.Readdir(0); err == nil {
			sort.Sort(byName(fl))
			if !pf {
				fl = filterFL(fl)
			}
			fl_len := len(fl)
			for i, v := range fl {
				var s string
				if pf && !v.IsDir() {
					s = fmt.Sprintf("%v%v (%v)", ge(fl_len - i - 1, ges), v.Name(), fSize(v.Size()))
					fmt.Fprintln(out, s)

				} else if v.IsDir() {
					fmt.Fprintf(out, "%v%v\n", ge(fl_len - i - 1, ges), v.Name())
				}
				if v.IsDir() {
					if fl_len - i == 1 {
						ges := append(ges, false)
						_dirTree(out, filepath.Join(path, v.Name()), pf, ges)
					} else {
						ges := append(ges, true)
						_dirTree(out, filepath.Join(path, v.Name()), pf, ges)
					}
				}
			}
		}
	}
	return nil
}

func main() {
	out := os.Stdout
	if !(len(os.Args) == 2 || len(os.Args) == 3) {
		panic("usage go run main.go . [-f]")
	}
	path := os.Args[1]
	printFiles := len(os.Args) == 3 && os.Args[2] == "-f"
	err := dirTree(out, path, printFiles)
	if err != nil {
		panic(err.Error())
	}
}
