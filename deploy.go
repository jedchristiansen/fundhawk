package main

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"github.com/ncw/swift"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
)

var rs *swift.Connection

var rsUsername = flag.String("rsuser", "", "Rackspace username")
var rsApiKey = flag.String("rskey", "", "Rackspace API key")
var rsBucket = flag.String("bucket", "", "Rackspace Cloud Files bucket")
var rsAssetUrl = flag.String("asseturl", "", "Asset URL")

func PutCloudFile(path string, r io.Reader) error {
	ext := filepath.Ext(path)
	_, err := rs.ObjectPut(*rsBucket, path, r, false, "", contentTypes[ext], swift.Headers{"Cache-Control": "public, max-age=300"})
	return err
}

func PutUncachedCloudFile(path string, r io.Reader) error {
	ext := filepath.Ext(path)
	_, err := rs.ObjectPut(*rsBucket, path, r, false, "", contentTypes[ext], swift.Headers{"Cache-Control": "max-age=0, must-revalidate"})
	return err
}

func AssetPath(a string) string {
	if *upload {
		return *rsAssetUrl + "/" + assets[a]
	}

	return "/assets/" + assets[a]
}

var jsAssets = []string{"lodash.js", "reqwest.js", "search.coffee"}
var assets = map[string]string{"bootstrap.min.css": "", "style.css": "", "application.js": ""}
var contentTypes = map[string]string{
	".css":  "text/css",
	".js":   "text/javascript",
	".txt":  "text/plain",
	".xml":  "text/xml",
	".gif":  "image/gif",
	".html": "text/html; charset=utf-8",
}

func compileJS() {
	out, err := ioutil.TempFile("", "")
	defer out.Close()
	defer os.Remove(out.Name())

	for _, js := range jsAssets {
		f, err := os.Open("assets/" + js)
		defer f.Close()
		if filepath.Ext(js) == ".coffee" {
			coffee := exec.Command("coffee", "-c", "--stdio")
			coffee.Stdin = f
			coffee.Stdout = out
			err = coffee.Run()
			MaybePanic(err)
			continue
		}
		_, err = io.Copy(out, f)
		MaybePanic(err)
	}

	err = exec.Command("uglifyjs", "-m", "-c", "-o", "assets/application.js", out.Name()).Run()
	MaybePanic(err)
}

func writeAssets() {
	compileJS()
	for a := range assets {
		f, err := os.Open("assets/" + a)
		MaybePanic(err)
		defer f.Close()

		h := sha256.New()
		_, err = io.Copy(h, f)
		MaybePanic(err)
		hash := h.Sum(nil)
		f.Seek(0, 0)

		ext := filepath.Ext(a)
		name := a[:len(a)-len(ext)] + "-" + hex.EncodeToString(hash[:4]) + ext

		assets[a] = name
		if *upload {
			_, err = rs.ObjectPut(*rsBucket+"-assets", name, f, false, "", contentTypes[ext], swift.Headers{"Cache-Control": "public, max-age=31556925"})
		} else {
			err = os.MkdirAll("output/assets", os.ModeDir|os.ModePerm)
			MaybePanic(err)
			err = os.MkdirAll("output/firms", os.ModeDir|os.ModePerm)
			MaybePanic(err)

			w, err := os.Create("output/assets/" + name)
			MaybePanic(err)
			_, err = io.Copy(w, f)
			w.Close()
		}
		MaybePanic(err)
	}
}

func init() {
	flag.Parse()

	if *upload {
		rs = &swift.Connection{UserName: *rsUsername, ApiKey: *rsApiKey, AuthUrl: "https://identity.api.rackspacecloud.com/v1.0"}
		err := rs.Authenticate()
		MaybePanic(err)
	}
}
