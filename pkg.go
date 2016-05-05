package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	texttemplate "text/template"

	"github.com/prometheus/client_golang/prometheus"
)

var indexTmpl = template.Must(template.New("").Parse(`
<html>
  <body>
    <ul>
      {{range $pkg := .}}
        <li><a href="{{$pkg.Name}}">{{$pkg.Name}}</a></li>
      {{end}}
    </ul>
  </body>
</html>
`))

var pkgTmpl = template.Must(template.New("").Parse(`
<html>
  <head>
    <meta name="go-import" content="{{.Host}}/{{.Pkg.Name}} {{.Pkg.VCS}} {{.Pkg.URL}}">
  </head>
  <body>
    Install: go get -u {{.Host}}/{{.Pkg.Name}} <br>
    <a href="{{.Pkg.Documentation}}">Documentation</a><br>
    <a href="{{.Pkg.Source}}">Source</a>
  </body>
</html>
`))

var (
	requests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pkg_requests_total",
			Help: "Number of requests",
		},
		[]string{"path"},
	)

	errors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pkg_errors_total",
			Help: "Number of errors",
		},
		[]string{"path"},
	)
)

func init() {
	prometheus.MustRegister(requests)
	prometheus.MustRegister(errors)
}

type Package struct {
	VCS           string
	Name          string
	URL           string
	Source        string
	Documentation string
}

var (
	host    = os.Getenv("PKGHOST")
	pkgFile = os.Getenv("PKGFILE")
)

func loadPackages() (map[string]Package, error) {
	var packages map[string]Package
	data, err := ioutil.ReadFile(pkgFile)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(data, &packages)
	return packages, err
}

type Packages []Package

func (p Packages) Len() int {
	return len(p)
}

func (p Packages) Less(i int, j int) bool {
	return p[i].Name < p[j].Name
}

func (p Packages) Swap(i int, j int) {
	p[i], p[j] = p[j], p[i]
}

func serveIndex(w http.ResponseWriter, r *http.Request) {
	packages, err := loadPackages()
	if err != nil {
		errors.WithLabelValues(r.URL.Path).Inc()
		log.Println(err)
		http.Error(w, err.Error(), 500)
	}
	var pkgs Packages
	for _, pkg := range packages {
		pkgs = append(pkgs, pkg)
	}
	sort.Sort(pkgs)
	err = indexTmpl.Execute(w, packages)
	if err, ok := pkgTmpl.Execute(w, pkgs).(texttemplate.ExecError); ok {
		errors.WithLabelValues(r.URL.Path).Inc()
		log.Println("error executing package template:", err)
	}
}

func handler(w http.ResponseWriter, r *http.Request) {
	requests.WithLabelValues(r.URL.Path).Inc()

	if r.URL.Path == "/" {
		serveIndex(w, r)
		return
	}

	// Load the package list on every request because traffic is low
	// and this allows the most easy updating of the list
	packages, err := loadPackages()
	if err != nil {
		errors.WithLabelValues(r.URL.Path).Inc()
		log.Println(err)
		http.Error(w, err.Error(), 500)
	}
	parts := strings.Split(r.URL.Path[1:], "/")
	var pkg Package
	var ok bool
	for i := len(parts); i > 0; i-- {
		name := strings.Join(parts[:i], "/")
		pkg, ok = packages[name]
		if ok {
			break
		}
	}

	if !ok {
		http.Error(w, "No such package", http.StatusNotFound)
		return
	}

	type context struct {
		Host string
		Pkg  Package
	}
	if err, ok := pkgTmpl.Execute(w, context{host, pkg}).(texttemplate.ExecError); ok {
		errors.WithLabelValues(r.URL.Path).Inc()
		log.Println("error executing package template:", err)
	}
}

func main() {
	if host == "" {
		fmt.Fprintln(os.Stderr, "Please specify a valid host with the PKGHOST environment variable")
		os.Exit(1)
	}
	if pkgFile == "" {
		fmt.Fprintln(os.Stderr, "Please specify a valid package file with the PKGFILE environment variable")
		os.Exit(1)
	}

	go func() {
		if err := http.ListenAndServe(":8081", prometheus.Handler()); err != nil {
			log.Fatal(err)
		}
	}()

	http.HandleFunc("/", handler)
	err := http.ListenAndServe("localhost:8080", nil)
	if err != nil {
		log.Fatal(err)
	}
}
