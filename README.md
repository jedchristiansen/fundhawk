# Fundhawk

Fundhawk does VC analytics based on CrunchBase data. It is a static site
generator written in Go that uploads to Rackspace Cloud Files (served via the
Akamai CDN).

## Usage

```
$ go run *.go -help
Usage of fundhawk:
  -asseturl="": Asset URL
  -bucket="": Rackspace Cloud Files bucket
  -key="": CrunchBase API key
  -path="./data": Path to local data on the filesystem
  -remote=false: Fetch from CrunchBase API instead of local filesystem
  -rskey="": Rackspace API key
  -rsuser="": Rackspace username
  -upload=false: Upload the generated site to Rackspace
  -workers=40: Number of workers to fetch with
```
