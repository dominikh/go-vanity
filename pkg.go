package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
)

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

func serveIndex(w http.ResponseWriter, r *http.Request) {
	packages, err := loadPackages()
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), 500)
	}
	var lines []string
	for name := range packages {
		lines = append(lines, fmt.Sprintf(`<li><a href="/go/%s">%s</a></li>`, name, name))
	}
	sort.StringSlice(lines).Sort()

	html := fmt.Sprintf(`<html><body><ul>%s</ul></body></html>`, strings.Join(lines, ""))
	_, _ = w.Write([]byte(html))
}

func handler(w http.ResponseWriter, r *http.Request) {
	log.Println("Path:", r.URL.Path)

	if r.URL.Path == "/" {
		serveIndex(w, r)
		return
	}

	// Load the package list on every request because traffic is low
	// and this allows the most easy updating of the list
	packages, err := loadPackages()
	if err != nil {
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

	html := fmt.Sprintf(`<html><head><meta name="go-import" content="%s/%s %s %s"></head><body>Install: go get -u %s/%s<br><a href="%s">Documentation</a><br><a href="%s">Source</a></body></html>`,
		host, pkg.Name, pkg.VCS, pkg.URL,
		host, pkg.Name,
		pkg.Documentation, pkg.Source)

	_, _ = w.Write([]byte(html))
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
	http.HandleFunc("/", handler)
	err := http.ListenAndServe("localhost:8080", nil)
	if err != nil {
		log.Fatal(err)
	}
}
