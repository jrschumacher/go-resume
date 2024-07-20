package commands

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"log"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/usk81/go-resume/schema"
)

var (
	exportCmd = &cobra.Command{
		Use:   "export <json-resume>",
		Short: "converts from JSON-Resume and exports to file",
		Long:  "converts from JSON-Resume and exports to file",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			format := cmd.Flag("format").Value.String()
			templateDir := cmd.Flag("template-dir").Value.String()

			src := args[0]
			dest := cmd.Flag("output").Value.String()

			var r schema.Resume
			if err := ParseResumeFromFile(src, &r); err != nil {
				Exit(err)
			}
			if err := schema.Validation(r); err != nil {
				Exit(err)
			}

			switch format {
			case "html":
				OutputHTML(r, templateDir, dest)
			default:
				Exit(errors.Errorf("%s is not supported", format))
			}
		},
	}
	sr = regexp.MustCompile("[ 　]")
)

func init() {
	exportCmd.Flags().StringP("format", "f", "html", "output format (html)")
	exportCmd.Flags().StringP("template-dir", "t", "themes", "template directory")
	exportCmd.Flags().StringP("output", "o", ".", "output directory")
}

// Name stores name and ruby
type Name struct {
	Name string
	Ruby string
}

func init() {
	RootCmd.AddCommand(exportCmd)
}

// ParseResumeFromFile opens a json file and parse JSON-Resume
func ParseResumeFromFile(fp string, r interface{}) (err error) {
	if !IsExist(fp) {
		return fmt.Errorf("%s doesn't exist", fp)
	}
	f, err := os.Open(fp)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("fail to open file; %s", fp))
	}

	if err = json.NewDecoder(f).Decode(&r); err != nil {
		f.Close()
		return err
	}
	f.Close()
	return nil
}

// OutputHTML outputs HTML file and copy assets
func OutputHTML(r schema.Resume, src string, dst string) (err error) {
	rt := regexp.MustCompile(`^resume(\.html)*\.(template|tpl).*$`)

	if d, err := isDir(src); !d || err != nil {
		if err != nil {
			return err
		}
		return errors.Errorf("%s is not directory", src)
	}

	dst = filepath.Join(dst, "resume")
	if err = os.MkdirAll(dst, os.ModePerm); err != nil {
		return err
	}

	out := filepath.Join(dst, "index.html")
	if isExist(out) {
		return errors.Errorf("%s already exists", out)
	}
	f, err := os.Create(out)
	if err != nil {
		return errors.Wrapf(err, "fail to open HTML file")
	}
	defer f.Close()

	sd, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, s := range sd {
		srcPath := filepath.Join(src, s.Name())
		b, err := isDir(srcPath)
		if err != nil {
			return err
		}
		if b {
			if err := CopyDir(srcPath, filepath.Join(dst, s.Name())); err != nil {
				return err
			}
		} else {
			if rt.MatchString(s.Name()) {
				if err = CreateHTML(f, r, srcPath); err != nil {
					return err
				}
			} else {
				if err := CopyDir(srcPath, filepath.Join(dst, s.Name())); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// CreateHTML creates HTML data
func CreateHTML(w io.Writer, r schema.Resume, tp string) error {
	tpl := filepath.Base(tp)

	fs := template.FuncMap{
		"add": func(a, b int) int { return a + b },
		"sub": func(a, b int) int { return a - b },
		"mul": func(a, b int) int { return a * b },
		"div": func(a, b int) int { return a / b },
		"seq": func(arg int) []struct{} { return make([]struct{}, arg) },
		"commalist": func(args []string) string {
			return strings.Join(args, ",")
		},
		"year": func(arg string) string {
			r, err := time.Parse("2006-01-02", arg)
			// r := time.Now()
			if err != nil {
				log.Printf("fail to get year : %s : %v", arg, err)
				return ""
			}
			return strconv.Itoa(r.Year())
		},
		"month": func(arg string) string {
			r, err := time.Parse("2006-01-02", arg)
			if err != nil {
				log.Printf("fail to get month : %s : %v", arg, err)
				return ""
			}
			return strconv.Itoa(int(r.Month()))
		},
		"day": func(arg string) string {
			r, err := time.Parse("2006-01-02", arg)
			if err != nil {
				log.Printf("fail to get day : %s : %v", arg, err)
				return ""
			}
			return strconv.Itoa(r.Day())
		},
		"nowJP": func() string {
			y, m, d := time.Now().Date()
			mi := int(m)

			sm := strconv.Itoa(mi)
			sd := strconv.Itoa(d)
			if mi < 10 {
				sm = " " + sm
			}
			if d < 10 {
				sd = " " + sd
			}
			return fmt.Sprintf("%d年%s月%s日", y, sm, sd)
		},
		"age": func(arg string) int {
			n := time.Now()
			bd, err := time.Parse("2006-01-02", arg)
			if err != nil {
				log.Printf("fail to parse birthday : %s : %v", arg, err)
				return 0
			}
			bi, _ := strconv.Atoi(bd.Format("20060102"))
			ni, _ := strconv.Atoi(n.Format("20060102"))

			return (ni - bi) / 10000
		},
		"ruby": func(name, ruby string) (result []Name) {
			if name != "" {
				ns := sr.Split(name, -1)
				rs := sr.Split(ruby, -1)
				for i, n := range ns {
					r := Name{Name: n}
					if len(rs) > i {
						r.Ruby = rs[i]
					}
					result = append(result, r)
				}
			}
			return
		},
	}
	tmpl, err := template.New(tpl).Funcs(fs).ParseFiles(tp)
	if err != nil {
		return err
	}
	return tmpl.Execute(w, r)
}

func isDir(p string) (bool, error) {
	if !IsExist(p) {
		return false, errors.Errorf("directory is not found : %s", p)
	}
	src, err := os.Open(p)
	if err != nil {
		return false, errors.Wrapf(err, "fail to access : %s", p)
	}
	defer src.Close()

	fi, err := src.Stat()
	if err != nil {
		return false, errors.Wrapf(err, "fail to get file information : %s", p)
	}
	return fi.IsDir(), nil
}

func isExist(p string) bool {
	_, err := os.Stat(p)
	return !os.IsNotExist(err)
}

// Copy a file `from` path to `to` path
func Copy(src, dst string) error {
	if !IsExist(src) {
		return fmt.Errorf("source file does not exist")
	}
	if IsExist(dst) {
		return fmt.Errorf("destination already exists")
	}
	sf, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("fail to open source file")
	}
	defer sf.Close()

	df, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("fail to create source file")
	}
	defer df.Close()

	_, err = io.Copy(df, sf)
	return err
}

// CopyDir copies a directory source path to destination path
func CopyDir(src, dst string) error {
	walk := func(srcPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		dstPath := filepath.Join(dst, srcPath[len(src):])
		if info.IsDir() {
			return os.MkdirAll(dstPath, 0755)
		}
		return Copy(srcPath, dstPath)
	}
	return filepath.Walk(src, walk)
}
